package workup

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The executable shim models only native API protocol and state transitions.
// It never invokes a cluster, shell, model or container.
func TestMain(m *testing.M) {
	if filepath.Base(os.Args[0]) == "kubectl" {
		if err := nativeShim(os.Args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	if len(os.Args) > 1 && os.Args[1] == "workup-mcp-helper" {
		if err := stdioServer(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

type nativeFixture struct {
	Mode      string `json:"mode"`
	NotifyURL string `json:"notify_url"`
}

func nativeShim(args []string) error {
	if len(args) < 7 || args[0] != "--kubeconfig" || args[2] != "--context" || args[3] != "fixture-context" ||
		args[4] != "--namespace" || args[5] != "fixture-ns" {
		return fmt.Errorf("explicit selectors missing")
	}
	if os.Getenv("OPENAI_API_KEY") != "" || os.Getenv("UNRELATED_SECRET") != "" {
		return fmt.Errorf("host credentials inherited")
	}
	root := filepath.Dir(args[1])
	data, err := os.ReadFile(args[1])
	if err != nil {
		return err
	}
	var fixture nativeFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		return err
	}
	cmd := args[6:]
	var output []byte
	switch cmd[0] {
	case "get":
		cache := filepath.Join(os.Getenv("HOME"), ".kube", "cache")
		if err := os.MkdirAll(cache, 0o700); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(cache, "discovery"), []byte("native discovery metadata"), 0o600); err != nil {
			return err
		}
		switch cmd[1] {
		case "pipeline":
			output = []byte(fmt.Sprintf(`{"metadata":{"name":%q,"uid":"pipeline-uid"},"spec":{"description":"original snapshot","params":[{"name":"target","type":"string"}],"results":[{"name":"diagnosis"}],"tasks":[]}}`, cmd[2]))
		case "pipelinerun":
			data, err := os.ReadFile(filepath.Join(root, cmd[2]+".json"))
			if err != nil {
				return err
			}
			var object map[string]any
			if err := json.Unmarshal(data, &object); err != nil {
				return err
			}
			status := "True"
			if fixture.Mode == "failed" {
				status = "False"
			}
			if fixture.Mode == "cancel" {
				status = "Unknown"
			}
			results := []map[string]string{{"name": "diagnosis", "value": "missing BUILD_TARGET: unhealthy workload"}}
			if fixture.Mode == "missing" {
				results = nil
			}
			object["status"] = map[string]any{"conditions": []map[string]string{{"type": "Succeeded", "status": status, "reason": "Fixture" + status}}, "results": results}
			output, err = json.Marshal(object)
			if err != nil {
				return err
			}
			if fixture.Mode == "cancel" {
				if err := os.WriteFile(filepath.Join(root, "running"), []byte("ready"), 0o600); err != nil {
					return err
				}
				if fixture.NotifyURL != "" {
					response, err := http.Post(fixture.NotifyURL, "text/plain", strings.NewReader("running"))
					if err != nil {
						return err
					}
					if err := response.Body.Close(); err != nil {
						return err
					}
				}
			}
		}
	case "create":
		var object map[string]any
		if err := json.NewDecoder(os.Stdin).Decode(&object); err != nil {
			return err
		}
		meta := object["metadata"].(map[string]any)
		name := meta["name"].(string)
		path := filepath.Join(root, name+".json")
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("duplicate create")
		}
		meta["uid"] = name + "-uid"
		meta["resourceVersion"] = "1"
		output, err = json.Marshal(object)
		if err != nil {
			return err
		}
		if err := os.WriteFile(path, output, 0o600); err != nil {
			return err
		}
	case "patch":
		data, err := os.ReadFile(filepath.Join(root, cmd[2]+".json"))
		if err != nil {
			return err
		}
		var object nativeObject
		if err := json.Unmarshal(data, &object); err != nil {
			return err
		}
		var patch []struct{ Op, Path, Value string }
		if len(cmd) < 6 || json.Unmarshal([]byte(cmd[5]), &patch) != nil || len(patch) != 3 ||
			patch[0].Op != "test" || patch[0].Value != object.Metadata.UID ||
			patch[1].Op != "test" || patch[1].Value != object.Metadata.ResourceVersion ||
			patch[2].Path != "/spec/status" || patch[2].Value != "Cancelled" {
			return fmt.Errorf("unsafe cancellation patch")
		}
		if err := os.WriteFile(filepath.Join(root, cmd[2]+".cancelled"), []byte(object.Metadata.UID), 0o600); err != nil {
			return err
		}
		output = data
	default:
		return fmt.Errorf("unapproved native command")
	}
	_, err = os.Stdout.Write(output)
	return err
}

func fixtureOptions(t *testing.T, mode string) Options {
	t.Helper()
	root := t.TempDir()
	bin, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(bin, filepath.Join(root, "kubectl")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", root+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("UNRELATED_SECRET", "must-not-inherit")
	kube := filepath.Join(root, "kube.json")
	writeFile(t, kube, fmt.Sprintf(`{"mode":%q}`, mode))
	skill := filepath.Join(root, "diagnostic", "SKILL.md")
	writeFile(t, skill, "---\nname: diagnostic\ndescription: Inspect an operator-selected workload.\n---\nUse diagnostic Pipelines and actual evidence.\n")
	config := fmt.Sprintf(`version: 1
skills:
  - path: %s
pipelines:
  - name: first-check
    params:
      target: {type: string, required: true, enum: [sample, follow], max_length: 64}
    required_results: [diagnosis]
  - name: second-check
    params:
      target: {type: string, required: true, enum: [sample, follow], max_length: 64}
    required_results: [diagnosis]
execution: {service_account: reviewed-reader}
limits: {max_model_calls: 12, max_tool_calls: 24, max_runs: 3, timeout_seconds: 30, max_evidence_bytes: 131072}
`, skill)
	path := filepath.Join(root, "workup.yaml")
	writeFile(t, path, config)
	return Options{ConfigPath: path, Goal: "Investigate sample build", Provider: "openai", Model: "fixture-model",
		Kubeconfig: kube, Context: "fixture-context", Namespace: "fixture-ns"}
}

func providerFixture(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	s := httptest.NewServer(handler)
	t.Cleanup(s.Close)
	t.Setenv("OPENAI_BASE_URL", s.URL+"/v1/")
	t.Setenv("OPENAI_API_KEY", "fixture-provider-secret")
}
func providerReply(t *testing.T, w http.ResponseWriter, message any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	data, err := json.Marshal(message)
	if err != nil {
		t.Error(err)
		return
	}
	if _, err := fmt.Fprintf(w, `{"id":"fixture","object":"chat.completion","model":"fixture-model","choices":[{"index":0,"finish_reason":"stop","message":%s}],"usage":{"prompt_tokens":10,"completion_tokens":2,"total_tokens":12}}`, data); err != nil {
		t.Error(err)
	}
}
func toolReply(t *testing.T, w http.ResponseWriter, name string, args string) {
	t.Helper()
	providerReply(t, w, map[string]any{"role": "assistant", "tool_calls": []any{map[string]any{
		"id": "fixture-" + name, "type": "function", "function": map[string]string{"name": name, "arguments": args}}}})
}
func textReply(t *testing.T, w http.ResponseWriter, text string) {
	t.Helper()
	content, err := json.Marshal(conclusion{Conclusion: text, MissingEvidence: []string{}})
	if err != nil {
		t.Error(err)
		return
	}
	providerReply(t, w, map[string]string{"role": "assistant", "content": string(content)})
}

func runFiles(t *testing.T, opts Options) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(filepath.Dir(opts.Kubeconfig), "workup-*.json"))
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func requestText(t *testing.T, r *http.Request) string {
	t.Helper()
	var body json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		t.Error(err)
	}
	return strings.ReplaceAll(string(body), `\"`, `"`)
}
