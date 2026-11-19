package workup

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const mcpEvidence = "MCP actual BUILD_TARGET missing; fixture-provider-secret"

func mcpFixture(t *testing.T, evidence string) string {
	t.Helper()
	server := mcp.NewServer(&mcp.Implementation{Name: "fixture", Version: "1"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "query", Description: "Read actual failure evidence."},
		func(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, any, error) {
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: evidence}}}, nil, nil
		})
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{JSONResponse: true, Stateless: true})
	s := httptest.NewServer(handler)
	t.Cleanup(s.Close)
	return s.URL
}

func addMCP(t *testing.T, opts Options, url string) {
	t.Helper()
	data, err := os.ReadFile(opts.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, opts.ConfigPath, string(data)+"\nmcp_servers:\n  - name: pharos\n    transport: streamable_http\n    url: "+url+"\n    tools: [query]\n")
}
