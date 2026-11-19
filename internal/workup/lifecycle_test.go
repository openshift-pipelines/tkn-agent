package workup

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// WORKUP-006: cancellation preserves the created UID and cancels only that run.
func TestRunCancellationPreservesOwnedRunReceipt(t *testing.T) {
	opts := fixtureOptions(t, "cancel")
	opts.Execute = true
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	notify := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		cancel()
		w.WriteHeader(http.StatusNoContent)
	}))
	defer notify.Close()
	writeFile(t, opts.Kubeconfig, fmt.Sprintf(`{"mode":"cancel","notify_url":%q}`, notify.URL))
	providerFixture(t, func(w http.ResponseWriter, _ *http.Request) {
		toolReply(t, w, "inspect_pipeline", `{"pipeline":"first-check","params":{"target":"sample"}}`)
	})
	report, err := Run(ctx, opts)
	if err == nil || report.State != "cancelled" || len(report.Runs) != 1 ||
		report.Runs[0].UID == "" || report.Runs[0].Cancellation != "requested" || report.Runs[0].Outcome != "cancelled" {
		t.Fatalf("cancellation receipt incorrect: %+v %v", report, err)
	}
	path := filepath.Join(filepath.Dir(opts.Kubeconfig), report.Runs[0].Name+".cancelled")
	data, readErr := os.ReadFile(path)
	if readErr != nil || string(data) != report.Runs[0].UID {
		t.Fatalf("UID cancellation missing: %s %v", data, readErr)
	}
}

// WORKUP-009: real command transport receives only operator-mapped variables.
func stdioServer() error {
	if os.Getenv("UNRELATED_SECRET") != "" || os.Getenv("OPENAI_API_KEY") != "" || os.Getenv("MCP_TOKEN") == "" {
		return fmt.Errorf("stdio inherited forbidden env or lacks explicit env")
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "stdio-fixture", Version: "1"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "query", Description: "Read isolated diagnostic evidence."},
		func(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, any, error) {
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "isolated MCP evidence " + os.Getenv("MCP_TOKEN")}}}, nil, nil
		})
	return server.Run(context.Background(), &mcp.StdioTransport{})
}

func TestRunStdioUsesOnlyExplicitEnvironment(t *testing.T) {
	opts := fixtureOptions(t, "success")
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(opts.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, opts.ConfigPath, string(data)+fmt.Sprintf(`
mcp_servers:
  - name: isolated
    transport: stdio
    command: %s
    args: [workup-mcp-helper]
    env: {MCP_TOKEN: TEST_MCP_TOKEN}
    tools: [query]
`, binary))
	t.Setenv("TEST_MCP_TOKEN", "explicit-mcp-secret")
	var calls atomic.Int32
	providerFixture(t, func(w http.ResponseWriter, r *http.Request) {
		request := requestText(t, r)
		if strings.Contains(request, "explicit-mcp-secret") {
			t.Error("MCP credential entered model request")
		}
		if calls.Add(1) == 1 {
			toolReply(t, w, "isolated_query", `{}`)
		} else {
			if !strings.Contains(request, "isolated MCP evidence") {
				t.Error("stdio MCP evidence lost")
			}
			textReply(t, w, "Evidence read; native checks remain unexecuted.")
		}
	})
	report, err := Run(t.Context(), opts)
	if err != nil || len(report.MCPCalls) != 1 || !report.MCPCalls[0].Success {
		t.Fatalf("stdio failed or receipt missing: %+v %v", report, err)
	}
}

func TestRunRejectsMCPRedirectBeforeModel(t *testing.T) {
	opts := fixtureOptions(t, "success")
	var leaked atomic.Bool
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		leaked.Store(true)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer target.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer redirect.Close()
	addMCP(t, opts, redirect.URL)
	var calls atomic.Int32
	providerFixture(t, func(w http.ResponseWriter, _ *http.Request) { calls.Add(1); textReply(t, w, "bad") })
	report, err := Run(t.Context(), opts)
	if err == nil || leaked.Load() || calls.Load() != 0 || report.Usage.ModelCalls != 0 {
		t.Fatalf("MCP redirect followed or preflight bypassed: %+v %v", report, err)
	}
}

// WORKUP-008: completed earlier receipts survive a later provider failure.
func TestRunRetainsReceiptsOnLaterProviderError(t *testing.T) {
	opts := fixtureOptions(t, "success")
	opts.Execute = true
	var calls atomic.Int32
	providerFixture(t, func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			toolReply(t, w, "inspect_pipeline", `{"pipeline":"first-check","params":{"target":"sample"}}`)
		} else {
			w.WriteHeader(http.StatusBadRequest)
			if _, err := w.Write([]byte(`{"error":{"message":"fixture-provider-secret"}}`)); err != nil {
				t.Error(err)
			}
		}
	})
	report, err := Run(t.Context(), opts)
	data, marshalErr := json.Marshal(report)
	if err == nil || marshalErr != nil || len(report.Runs) != 1 || report.Runs[0].Outcome != "succeeded" ||
		strings.Contains(string(data), "fixture-provider-secret") || strings.Contains(err.Error(), "fixture-provider-secret") {
		t.Fatalf("partial receipts lost or secret leaked: %+v %v", report, err)
	}
}
