//go:build realrepo

package realrepo

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeInput(t *testing.T, name, text string) {
	t.Helper()
	if err := os.WriteFile(name, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRealPipelineReviewDefectRepairAndReuse(t *testing.T) {
	root, env, calls := setup(t)
	pending, code := verify(t, invocation{root: root, env: env})
	if code != 2 || pending.State != "review-required" || pending.Provided != 1 || len(pending.Checks) != 0 {
		t.Fatalf("expected review without execution: code=%d %+v", code, pending)
	}
	if _, err := os.Stat(filepath.Join(root, sharedPath)); !os.IsNotExist(err) {
		t.Fatalf("unapproved plan written to source: %v", err)
	}
	started := time.Now()
	clean, code := verify(t, invocation{root: root, env: env, args: []string{"--approve-plan", pending.Digest}})
	t.Logf("accepted clean wall time: %s", time.Since(started))
	if code != 0 || clean.State != "passed" || len(clean.Checks) != 2 {
		t.Fatalf("real clean checks did not pass: code=%d %+v", code, clean)
	}
	shared, err := os.ReadFile(filepath.Join(root, sharedPath))
	if err != nil {
		t.Fatal(err)
	}
	var stored struct {
		Inputs map[string]string `json:"inputs"`
	}
	if err := json.Unmarshal(shared, &stored); err != nil {
		t.Fatal(err)
	}
	if stored.Inputs[".yamllint"] != "absent" || !strings.HasPrefix(stored.Inputs[".shellcheckrc"], "link:") || strings.Contains(string(shared), "accepted_digest") ||
		strings.Contains(string(shared), "recorded-real-credential") || strings.Contains(string(shared), root) {
		t.Fatal("shared plan lost optional input provenance or contains local state")
	}
	generatedCalls := calls.Load()
	noProvider := append(append([]string{}, env...), "OPENAI_API_KEY=", "TKN_AGENT_MODEL=")
	writeInput(t, filepath.Join(root, "tkn_agent_eval_unquoted.sh"), "#!/usr/bin/env bash\nprintf '%s\\n' $1\n")
	writeInput(t, filepath.Join(root, "tkn-agent-eval-duplicate.yaml"), "---\nevaluation:\n  enabled: true\n  enabled: false\n")
	started = time.Now()
	broken, code := verify(t, invocation{root: root, env: noProvider})
	t.Logf("credential-free defective replay wall time: %s", time.Since(started))
	if code != 1 || broken.State != "failed" || len(broken.Checks) != 2 {
		t.Fatalf("real defects were not executed and rejected: code=%d %+v", code, broken)
	}
	found := make(map[string]bool)
	for _, check := range broken.Checks {
		found[check.ID] = check.Outcome == "failed" &&
			(strings.Contains(check.Output, "SC2086") || strings.Contains(check.Output, "key-duplicates"))
	}
	if !found["shellcheck/run-shellcheck"] || !found["yamllint/run-yamllint"] || calls.Load() != generatedCalls || broken.Digest != pending.Digest {
		t.Fatalf("missing defect coverage or unexpected replanning: %+v calls=%d", broken, calls.Load())
	}
	writeInput(t, filepath.Join(root, "tkn_agent_eval_unquoted.sh"), "#!/usr/bin/env bash\nprintf '%s\\n' \"$1\"\n")
	writeInput(t, filepath.Join(root, "tkn-agent-eval-duplicate.yaml"), "---\nevaluation:\n  enabled: true\n")
	started = time.Now()
	repaired, code := verify(t, invocation{root: root, env: noProvider})
	t.Logf("credential-free repaired replay wall time: %s", time.Since(started))
	if code != 0 || repaired.State != "passed" || len(repaired.Checks) != 2 || calls.Load() != generatedCalls {
		t.Fatalf("repaired controls failed: code=%d %+v", code, repaired)
	}
	unchanged, err := os.ReadFile(filepath.Join(root, sharedPath))
	if err != nil {
		t.Fatal(err)
	}
	if string(unchanged) != string(shared) {
		t.Fatal("warm replay rewrote the shared plan")
	}
	helper := "ci/images/static-checks/content/bin/shellcheck.sh"
	body, err := os.ReadFile(filepath.Join(root, helper))
	if err != nil {
		t.Fatal(err)
	}
	writeInput(t, filepath.Join(root, helper), string(body)+"\n# inspected helper changed\n")
	changed, code := verify(t, invocation{root: root, env: env})
	if code != 2 || changed.State != "review-required" || changed.Digest == pending.Digest || len(changed.Checks) != 0 || calls.Load() <= generatedCalls {
		t.Fatalf("helper change reused stale planning: code=%d %+v", code, changed)
	}
}
