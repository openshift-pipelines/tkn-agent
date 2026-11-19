//go:build realrepo

package realrepo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const sourceRevision = "5b34974a82703dfba477bbec73b0658d49ddc69b"
const pipelinePath = ".tekton/test-pipeline-service-static-code-analysis.yaml"
const sharedPath = ".tekton/.tkn-agent/test-pipeline-service-static-code-analysis.yaml.plan.json"

type result struct {
	State    string `json:"state"`
	Message  string `json:"message"`
	Digest   string `json:"plan_digest"`
	Provided int    `json:"provided"`
	Checks   []struct {
		ID      string `json:"id"`
		Outcome string `json:"outcome"`
		Output  string `json:"output"`
	} `json:"checks"`
}

type invocation struct {
	root string
	env  []string
	args []string
}

func command(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), args[0], args[1:]...)
	cmd.Dir, cmd.Env = dir, append(os.Environ(), "GIT_MASTER=1")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s: %v: %s", args[0], err, output)
	}
}

func verify(t *testing.T, call invocation) (result, int) {
	t.Helper()
	binary, err := filepath.Abs("../../bin/tkn-agent")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 4*time.Minute)
	defer cancel()
	args := []string{"verify", "--local", pipelinePath, "--format=json",
		"--bind-checkout", "fetch-repository", "--param", "target_branch=main"}
	cmd := exec.CommandContext(ctx, binary, append(args, call.args...)...)
	cmd.Dir, cmd.Env = call.root, append(os.Environ(), call.env...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	code := 0
	if err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			t.Fatalf("invoke verify: %v: %s", err, stderr.String())
		}
		code = exit.ExitCode()
	}
	var got result
	if err := json.Unmarshal(output, &got); err != nil {
		t.Fatalf("decode CLI report: %v; stdout=%s stderr=%s", err, output, stderr.String())
	}
	return got, code
}

func setup(t *testing.T) (string, []string, *atomic.Int32) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "repository")
	from := os.Getenv("TKN_AGENT_REAL_SOURCE")
	if from == "" {
		from = "https://github.com/openshift-pipelines/pipeline-service.git"
	}
	command(t, "", "git", "-c", "core.hooksPath=/dev/null", "clone", "--no-checkout", from, root)
	command(t, root, "git", "-c", "core.hooksPath=/dev/null", "checkout", "--detach", sourceRevision)
	command(t, root, "git", "remote", "set-url", "origin", "https://github.com/openshift-pipelines/pipeline-service.git")
	calls := new(atomic.Int32)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var request struct {
			Messages []struct {
				Role string `json:"role"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if len(request.Messages) > 0 && request.Messages[len(request.Messages)-1].Role == "tool" {
			selection := `{"steps":[{"id":"fetch-repository/reference","decision":"provided","reason":"Explicit self-checkout binding"},{"id":"shellcheck/run-shellcheck","decision":"run","reason":"Bounded shell validation"},{"id":"yamllint/run-yamllint","decision":"run","reason":"Bounded YAML validation"},{"id":"hadolint/run-hadolint","decision":"skip","reason":"Outside this trial"},{"id":"checkov/run-checkov","decision":"skip","reason":"Outside this trial"},{"id":"grafana-dashboards/run-grafana-dashboards","decision":"skip","reason":"Outside this trial"}]}`
			if _, err := fmt.Fprintf(w, `{"id":"selection","object":"chat.completion","model":"recorded-real","choices":[{"index":0,"finish_reason":"stop","message":{"role":"assistant","content":%q}}]}`, selection); err != nil {
				t.Error(err)
			}
			return
		}
		paths := []string{"ci/images/static-checks/content/bin/shellcheck.sh",
			"ci/images/static-checks/content/bin/yamllint.sh",
			"ci/images/static-checks/content/config/yamllint.yaml", ".shellcheckrc", ".yamllint"}
		type function struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		}
		type call struct {
			ID       string   `json:"id"`
			Type     string   `json:"type"`
			Function function `json:"function"`
		}
		var tools []call
		for i, path := range paths {
			args, err := json.Marshal(map[string]string{"path": path})
			if err != nil {
				t.Error(err)
				return
			}
			tools = append(tools, call{ID: fmt.Sprintf("read%d", i), Type: "function", Function: function{Name: "read_file", Arguments: string(args)}})
		}
		body, err := json.Marshal(tools)
		if err != nil {
			t.Error(err)
			return
		}
		if _, err := fmt.Fprintf(w, `{"id":"inspection","object":"chat.completion","model":"recorded-real","choices":[{"index":0,"finish_reason":"tool_calls","message":{"role":"assistant","tool_calls":%s}}]}`, body); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(server.Close)
	env := []string{"OPENAI_API_KEY=recorded-real-credential", "OPENAI_BASE_URL=" + server.URL + "/v1/",
		"TKN_AGENT_PROVIDER=openai", "TKN_AGENT_MODEL=recorded-real", "XDG_CACHE_HOME=" + t.TempDir()}
	return root, env, calls
}
