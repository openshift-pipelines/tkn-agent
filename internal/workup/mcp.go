package workup

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"slices"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	adkagent "google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/tool"
	"google.golang.org/adk/v2/tool/mcptoolset"
)

// Discovery advertises the full server catalog before filtering. Bound its wire
// response separately from selected tool evidence and the model context.
const maxMCPResponseBytes = 1 << 20

type checkedSet struct {
	tool.Toolset
	required []string
}

func (s checkedSet) Tools(ctx adkagent.ReadonlyContext) ([]tool.Tool, error) {
	tools, err := s.Toolset.Tools(ctx)
	if err != nil {
		return nil, err
	}
	names := map[string]bool{}
	for _, t := range tools {
		names[t.Name()] = true
	}
	for _, name := range s.required {
		if !names[name] {
			return nil, fmt.Errorf("required MCP tool %q unavailable", name)
		}
	}
	return tools, nil
}

type bearerTransport struct {
	base  http.RoundTripper
	token string
	limit int
}

func (t bearerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	if t.token != "" {
		r.Header.Set("Authorization", "Bearer "+t.token)
	}
	response, err := t.base.RoundTrip(r)
	if err != nil {
		return nil, err
	}
	response.Body = &limitedBody{ReadCloser: response.Body, remaining: int64(t.limit)}
	return response, nil
}

type limitedBody struct {
	io.ReadCloser
	remaining int64
}

func (b *limitedBody) Read(p []byte) (int, error) {
	if b.remaining <= 0 {
		return 0, errors.New("MCP response exceeds evidence limit")
	}
	if int64(len(p)) > b.remaining {
		p = p[:b.remaining]
	}
	n, err := b.ReadCloser.Read(p)
	b.remaining -= int64(n)
	return n, err
}

func environmentReference(name string) (string, error) {
	if !envName.MatchString(name) {
		return "", errors.New("invalid environment reference")
	}
	value, ok := os.LookupEnv(name)
	if !ok || value == "" {
		return "", fmt.Errorf("required environment reference %q missing", name)
	}
	return value, nil
}

func mcpSet(ctx context.Context, server serverConfig) (tool.Toolset, io.Closer, []string, error) {
	var transport mcp.Transport
	var secrets []string
	switch server.Transport {
	case "streamable_http":
		u, err := url.Parse(server.URL)
		if err != nil || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return nil, nil, nil, errors.New("MCP requires credential-free absolute endpoint URL")
		}
		ip := net.ParseIP(u.Hostname())
		loopbackHTTP := u.Scheme == "http" && (u.Hostname() == "localhost" || ip.IsLoopback())
		if u.Scheme != "https" && !loopbackHTTP {
			return nil, nil, nil, errors.New("MCP HTTP requires HTTPS except explicit loopback")
		}
		token := ""
		if server.BearerTokenEnv != "" {
			token, err = environmentReference(server.BearerTokenEnv)
			if err != nil {
				return nil, nil, nil, err
			}
			secrets = append(secrets, token)
		}
		base := http.DefaultTransport.(*http.Transport).Clone()
		base.Proxy = nil
		base.ResponseHeaderTimeout = 15 * time.Second
		transport = &mcp.StreamableClientTransport{Endpoint: server.URL, HTTPClient: &http.Client{
			Timeout: 30 * time.Second, Transport: bearerTransport{base: base, token: token, limit: maxMCPResponseBytes},
			CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("MCP redirects are forbidden") },
		}}
	case "stdio":
		cmd := exec.CommandContext(ctx, server.Command, server.Args...)
		cmd.Env = []string{}
		cmd.WaitDelay = time.Second
		for _, name := range slices.Sorted(maps.Keys(server.Env)) {
			if !envName.MatchString(name) {
				return nil, nil, nil, errors.New("invalid child environment name")
			}
			value, err := environmentReference(server.Env[name])
			if err != nil {
				return nil, nil, nil, err
			}
			cmd.Env = append(cmd.Env, name+"="+value)
			secrets = append(secrets, value)
		}
		// Keep the command transport opaque to ADK's reconnect copier, which
		// otherwise drops CommandContext's cancellation binding.
		transport = singleTransport{&mcp.CommandTransport{Command: cmd, TerminateDuration: 3 * time.Second}}
	}
	set, err := mcptoolset.New(mcptoolset.Config{Transport: transport,
		ToolNamePrefix: server.Name, ToolFilter: tool.AllowedToolsPredicate(server.Tools)})
	if err != nil {
		return nil, nil, nil, err
	}
	required := make([]string, 0, len(server.Tools))
	for _, name := range server.Tools {
		required = append(required, server.Name+"_"+name)
	}
	return checkedSet{Toolset: set, required: required}, set.(io.Closer), secrets, nil
}

// singleTransport preserves the caller's context-bound command lifecycle.
type singleTransport struct{ mcp.Transport }
