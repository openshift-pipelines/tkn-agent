package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

const fixedSelection = `{"steps":[{"id":"checks/check","decision":"run","reason":"Inspect the existing small source check"},{"id":"build/image","decision":"skip","reason":"Image builds stay in CI"}]}`

func endpoint(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	t.Setenv("OPENAI_BASE_URL", server.URL+"/v1/")
	t.Setenv("OPENAI_API_KEY", "fake-provider-credential")
}

func writeFixture(t *testing.T, root, name, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func reply(t *testing.T, w http.ResponseWriter, message json.RawMessage, finish string) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if _, err := fmt.Fprintf(w, `{"id":"fixed","object":"chat.completion","model":"test-model","choices":[{"index":0,"finish_reason":%q,"message":%s}]}`, finish, message); err != nil {
		t.Error(err)
	}
}

func textMessage(t *testing.T, text string) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(map[string]string{"role": "assistant", "content": text})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// LOCAL-007/008/010: exercise real ADK, provider HTTP, tool dispatch and hashing.
func TestSelectInspectsHelperWithoutExecutingIt(t *testing.T) {
	root := t.TempDir()
	marker := filepath.Join(root, "executed")
	helper := "#!/bin/sh\ntouch " + marker + "\n"
	writeFixture(t, root, "hack/check.sh", helper)
	var requests atomic.Int32
	var sawHelper atomic.Bool
	endpoint(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" || r.Method != http.MethodPost {
			t.Errorf("unexpected endpoint: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		var body struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
			ResponseFormat struct {
				Type string `json:"type"`
			} `json:"response_format"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if body.ResponseFormat.Type != "json_schema" {
			t.Error("planner did not request native structured output")
		}
		if requests.Add(1) == 1 {
			reply(t, w, json.RawMessage(`{"role":"assistant","tool_calls":[{"id":"read-1","type":"function","function":{"name":"read_file","arguments":"{\"path\":\"hack/check.sh\"}"}}]}`), "tool_calls")
			return
		}
		for _, message := range body.Messages {
			if message.Role == "tool" && strings.Contains(message.Content, "touch "+marker) {
				sawHelper.Store(true)
			}
		}
		reply(t, w, textMessage(t, fixedSelection), "stop")
	})

	got, err := Select(t.Context(), Request{
		Root: root, Provider: "openai", Model: "test-model",
		Inventory: json.RawMessage(`{"steps":[{"id":"checks/check","script":"./hack/check.sh"},{"id":"build/image","script":"podman build ."}]}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(helper))
	if !sawHelper.Load() || string(got.Selection) != fixedSelection ||
		len(got.Inputs) != 1 || got.Inputs["hack/check.sh"] != hex.EncodeToString(sum[:]) {
		t.Fatalf("selection or inspected inputs incorrect: %+v; helper seen %v", got, sawHelper.Load())
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("helper was executed or marker stat failed: %v", err)
	}
}

func TestSelectRejectsBrokenStructuredOutput(t *testing.T) {
	for _, output := range []string{
		"not JSON", "```json\n" + fixedSelection + "\n```", `{"steps":null}`,
		`{"steps":[{"id":"x/y","decision":"run","reason":"yes","script":"touch executed"}]}`,
		`{"steps":[{"id":"x/y","decision":"maybe","reason":"yes"}]}`,
		`{"steps":[{"id":"x/y","decision":"run","reason":"yes"},{"id":"x/y","decision":"skip","reason":"no"}]}`,
	} {
		t.Run(output, func(t *testing.T) {
			endpoint(t, func(w http.ResponseWriter, _ *http.Request) {
				reply(t, w, textMessage(t, output), "stop")
			})
			_, err := Select(t.Context(), Request{
				Root: t.TempDir(), Provider: "openai", Model: "test-model", Inventory: json.RawMessage(`{"steps":[]}`),
			})
			if err == nil {
				t.Fatal("malformed structured output was accepted")
			}
		})
	}
}

func TestSelectCannotExecuteModelProposedTool(t *testing.T) {
	root := t.TempDir()
	endpoint(t, func(w http.ResponseWriter, _ *http.Request) {
		reply(t, w, json.RawMessage(`{"role":"assistant","tool_calls":[{"id":"bad","type":"function","function":{"name":"execute_script","arguments":"{\"script\":\"touch executed\"}"}}]}`), "tool_calls")
	})
	_, err := Select(t.Context(), Request{
		Root: root, Provider: "openai", Model: "test-model", Inventory: json.RawMessage(`{"steps":[]}`),
	})
	if err == nil {
		t.Fatal("unexpected execution tool was accepted")
	}
	if _, err := os.Stat(filepath.Join(root, "executed")); !os.IsNotExist(err) {
		t.Fatalf("script executed or marker stat failed: %v", err)
	}
}

func TestSelectBoundsRepeatedToolRequests(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "Makefile", "check:\n\ttrue\n")
	var calls atomic.Int32
	endpoint(t, func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		reply(t, w, json.RawMessage(`{"role":"assistant","tool_calls":[{"id":"read","type":"function","function":{"name":"read_file","arguments":"{\"path\":\"Makefile\"}"}}]}`), "tool_calls")
	})
	_, err := Select(t.Context(), Request{
		Root: root, Provider: "openai", Model: "test-model", Inventory: json.RawMessage(`{"steps":[]}`),
	})
	if err == nil || calls.Load() > 8 {
		t.Fatalf("planning loop not bounded: calls=%d err=%v", calls.Load(), err)
	}
}

func TestSelectSurfacesProviderFailure(t *testing.T) {
	endpoint(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		if _, err := io.WriteString(w, `{"error":{"message":"structured output unsupported; fake-provider-credential"}}`); err != nil {
			t.Error(err)
		}
	})
	_, err := Select(t.Context(), Request{
		Root: t.TempDir(), Provider: "openai", Model: "test-model", Inventory: json.RawMessage(`{"steps":[]}`),
	})
	if err == nil || !strings.Contains(err.Error(), "400 Bad Request") || strings.Contains(err.Error(), "fake-provider-credential") {
		t.Fatalf("provider status missing or sensitive response disclosed: %v", err)
	}
}
