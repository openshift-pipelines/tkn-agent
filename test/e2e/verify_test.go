//go:build e2e

package e2e

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

type report struct {
	State   string `json:"state"`
	Digest  string `json:"plan_digest"`
	Message string `json:"message"`
	Checks  []struct {
		ID        string `json:"id"`
		Outcome   string `json:"outcome"`
		Output    string `json:"output"`
		Container string `json:"container"`
	} `json:"checks"`
}

func TestDeadlineRemovesOwnedContainer(t *testing.T) {
	root, env, _ := setup(t)
	path := filepath.Join(root, "pipeline.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Replace(string(data), "grep -qx valid message.txt", "sleep 30", 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	pending, code := invoke(t, invocation{root: root, env: env, args: []string{"--timeout=1s"}})
	if code != 2 || pending.State != "review-required" {
		t.Fatalf("review failed: %+v", pending)
	}
	got, code := invoke(t, invocation{root: root, env: env, args: []string{"--timeout=1s", "--approve-plan", pending.Digest}})
	if code != 2 || got.State != "blocked" || len(got.Checks) != 1 || got.Checks[0].Container == "" {
		t.Fatalf("deadline did not block: code=%d %+v", code, got)
	}
	err = exec.CommandContext(t.Context(), "podman", "container", "exists", got.Checks[0].Container).Run()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 1 {
		t.Fatalf("owned container remains after deadline or probe failed: %v; report=%+v", err, got)
	}
}

type invocation struct {
	root string
	env  []string
	args []string
	hook bool
}

func invoke(t *testing.T, call invocation) (report, int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
	defer cancel()
	binary, err := filepath.Abs("../../bin/tkn-agent")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(ctx, binary, append([]string{"verify", "--local", "pipeline.yaml", "--format=json"}, call.args...)...)
	if call.hook {
		hook, err := filepath.Abs("../../examples/pre-commit")
		if err != nil {
			t.Fatal(err)
		}
		cmd = exec.CommandContext(ctx, "sh", hook)
		call.env = append(call.env, "TKN_AGENT_PIPELINE=pipeline.yaml", "TKN_AGENT_FORMAT=json", "PATH="+filepath.Dir(binary)+":"+os.Getenv("PATH"))
	}
	cmd.Dir, cmd.Env = call.root, append(os.Environ(), call.env...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	code := 0
	if err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			t.Fatalf("invoke CLI: %v: %s", err, stderr.String())
		}
		code = exit.ExitCode()
	}
	var got report
	if err := json.Unmarshal(output, &got); err != nil {
		t.Fatalf("invalid report: %v\nstdout=%s\nstderr=%s", err, output, stderr.String())
	}
	return got, code
}

func setup(t *testing.T) (string, []string, *atomic.Int32) {
	t.Helper()
	root := t.TempDir()
	cmd := exec.CommandContext(t.Context(), "git", "init", "--quiet", root)
	cmd.Env = append(os.Environ(), "GIT_MASTER=1")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("initialize fixture: %v: %s", err, output)
	}
	pipeline, err := os.ReadFile("../../evals/fixtures/mixed-checks.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "pipeline.yaml"), pipeline, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "message.txt"), []byte("valid\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	calls := new(atomic.Int32)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var body struct {
			Messages []struct {
				Content json.RawMessage `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if encoded, err := json.Marshal(body); err != nil || !strings.Contains(string(encoded), "lint/check") || !strings.Contains(string(encoded), "image/build") {
			t.Error("selector did not receive both small and heavy pipeline parts")
		}
		selection := `{"steps":[{"id":"lint/check","decision":"run","reason":"Small source check"},{"id":"image/build","decision":"skip","reason":"Image build remains in CI"}]}`
		w.Header().Set("Content-Type", "application/json")
		if _, err := fmt.Fprintf(w, `{"id":"recorded","object":"chat.completion","model":"fixture-model","choices":[{"index":0,"finish_reason":"stop","message":{"role":"assistant","content":%q}}]}`, selection); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(server.Close)
	env := []string{"GIT_MASTER=1", "TKN_AGENT_PROVIDER=openai", "TKN_AGENT_MODEL=fixture-model",
		"OPENAI_API_KEY=fixture-credential", "OPENAI_BASE_URL=" + server.URL + "/v1/",
		"XDG_CACHE_HOME=" + t.TempDir()}
	return root, env, calls
}

func TestPodmanSelectionReviewAndPrecommitReplay(t *testing.T) {
	root, env, calls := setup(t)

	pending, code := invoke(t, invocation{root: root, env: env})
	if code != 2 || pending.State != "review-required" || pending.Digest == "" || len(pending.Checks) != 0 {
		t.Fatalf("unreviewed scope executed or review missing: code=%d %+v", code, pending)
	}
	clean, code := invoke(t, invocation{root: root, env: env, args: []string{"--approve-plan", pending.Digest}})
	if code != 0 || clean.State != "passed" || len(clean.Checks) != 1 ||
		clean.Checks[0].ID != "lint/check" || clean.Checks[0].Outcome != "passed" {
		t.Fatalf("clean local scope failed or heavy task executed: code=%d %+v", code, clean)
	}
	freshContext := append(append([]string{}, env...), "OPENAI_API_KEY=", "TKN_AGENT_MODEL=", "XDG_CACHE_HOME="+t.TempDir())
	inherited, code := invoke(t, invocation{root: root, env: freshContext})
	if code != 2 || inherited.State != "review-required" || inherited.Digest != pending.Digest || len(inherited.Checks) != 0 || calls.Load() != 1 {
		t.Fatalf("shared plan reanalysis or inherited approval in fresh context: code=%d %+v calls=%d", code, inherited, calls.Load())
	}
	if err := os.WriteFile(filepath.Join(root, "message.txt"), []byte("invalid\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	noProvider := append(append([]string{}, env...), "OPENAI_API_KEY=", "TKN_AGENT_MODEL=")
	broken, code := invoke(t, invocation{root: root, env: noProvider, hook: true})
	if code != 1 || broken.State != "failed" || len(broken.Checks) != 1 || broken.Checks[0].Outcome != "failed" || calls.Load() != 1 {
		t.Fatalf("precommit missed dirty source or invoked a model: code=%d calls=%d %+v", code, calls.Load(), broken)
	}
	if broken.Digest != pending.Digest {
		t.Fatal("application edit invalidated unchanged reviewed scope")
	}
	path := filepath.Join(root, "pipeline.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, []byte("\n# changed definition\n")...), 0o600); err != nil {
		t.Fatal(err)
	}
	changed, code := invoke(t, invocation{root: root, env: env})
	if code != 2 || changed.State != "review-required" || changed.Digest == pending.Digest || len(changed.Checks) != 0 {
		t.Fatalf("changed definition retained acceptance: code=%d %+v", code, changed)
	}
}

func TestMissingImageInterpreterCannotPass(t *testing.T) {
	root, env, _ := setup(t)
	path := filepath.Join(root, "pipeline.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Replace(string(data), "#!/bin/sh", "#!/bin/bash", 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	pending, code := invoke(t, invocation{root: root, env: env})
	if code != 2 || pending.State != "review-required" {
		t.Fatalf("review failed: %+v", pending)
	}
	got, code := invoke(t, invocation{root: root, env: env, args: []string{"--approve-plan", pending.Digest}})
	if code != 2 || got.State != "blocked" || len(got.Checks) != 1 || got.Checks[0].Outcome != "blocked" {
		t.Fatalf("missing interpreter produced a successful verdict: code=%d %+v", code, got)
	}
}
