package workup

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// A server advertises its whole catalog before the client can filter it.
// Unselected tool descriptions must not consume the selected evidence budget.
func TestRunSeparatesMCPDiscoveryFromSelectedEvidenceLimit(t *testing.T) {
	opts := fixtureOptions(t, "success")
	data, err := os.ReadFile(opts.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, opts.ConfigPath, strings.ReplaceAll(string(data), "max_evidence_bytes: 131072", "max_evidence_bytes: 65536"))
	server := mcp.NewServer(&mcp.Implementation{Name: "large-catalog", Version: "1"}, nil)
	handler := func(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, any, error) {
		return &mcp.CallToolResult{}, nil, nil
	}
	mcp.AddTool(server, &mcp.Tool{Name: "query", Description: "Read selected evidence."}, handler)
	for _, name := range []string{"ungranted_a", "ungranted_b", "ungranted_c"} {
		mcp.AddTool(server, &mcp.Tool{Name: name, Description: strings.Repeat("x", 24576)}, handler)
	}
	s := httptest.NewServer(mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return server
	}, &mcp.StreamableHTTPOptions{Stateless: true}))
	defer s.Close()
	addMCP(t, opts, s.URL)
	var calls atomic.Int32
	providerFixture(t, func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Tools []struct {
				Function struct {
					Name string `json:"name"`
				} `json:"function"`
			} `json:"tools"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		found := false
		for _, entry := range request.Tools {
			if strings.HasPrefix(entry.Function.Name, "pharos_ungranted_") {
				t.Error("ungranted catalog tool exposed to model")
			}
			if entry.Function.Name == "pharos_query" {
				found = true
			}
		}
		if !found {
			t.Error("selected evidence tool absent from model request")
		}
		calls.Add(1)
		textReply(t, w, "The selected evidence tool is available; no execution requested.")
	})
	report, err := Run(t.Context(), opts)
	if err != nil || report.State != "completed" || calls.Load() != 1 {
		t.Fatalf("full discovery catalog incorrectly consumed selected evidence budget: state=%s calls=%d err=%v",
			report.State, calls.Load(), err)
	}
}
