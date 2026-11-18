package plan

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/openshift-pipelines/tkn-agent/internal/source"
)

func inventory(t *testing.T) source.Inventory {
	t.Helper()
	data, err := os.ReadFile("../../evals/fixtures/mixed-checks.yaml")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	path := filepath.Join(root, "pipeline.yaml")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := source.Load(t.Context(), root, path)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestInspectedHelperChangeInvalidatesAcceptance(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	src := inventory(t)
	helper := filepath.Join(src.Root, "check.sh")
	content := []byte("echo check\n")
	if err := os.WriteFile(helper, content, 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(content)
	policy := Policy{TimeoutSeconds: 180}
	p, err := Build(t.Context(), Request{Inventory: src, Selection: json.RawMessage(selection),
		Inputs: map[string]string{"check.sh": hex.EncodeToString(sum[:])}, Policy: policy})
	if err != nil {
		t.Fatal(err)
	}
	digest, err := Digest(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := Save(t.Context(), src, Cache{Plan: p, Accepted: digest}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(helper, []byte("echo changed\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, current, err := Read(t.Context(), src, policy); err != nil || current {
		t.Fatalf("changed inspected helper retained acceptance: current=%v err=%v", current, err)
	}
}

const selection = `{"steps":[{"id":"lint/check","decision":"run","reason":"Small source check"},{"id":"image/build","decision":"skip","reason":"Image building stays in CI"}]}`

func TestSelectionRetainsExactSourceExecution(t *testing.T) {
	src := inventory(t)

	got, err := Build(context.Background(), Request{
		Inventory: src, Selection: json.RawMessage(selection), Policy: Policy{TimeoutSeconds: 180},
	})

	if err != nil {
		t.Fatal(err)
	}
	if got.Steps[0].Source.Script != src.Steps[0].Script || got.Steps[0].Source.Image != src.Steps[0].Image ||
		got.Steps[0].Decision != "run" || got.Steps[1].Decision != "skip" {
		t.Fatalf("source execution or scope changed: %+v", got.Steps)
	}
}

func TestModelCannotExpandOrWeakenLocalScope(t *testing.T) {
	src := inventory(t)
	for _, proposal := range []string{
		`{"steps":[{"id":"lint/check","decision":"run","reason":"yes"}]}`,
		`{"steps":[{"id":"lint/check","decision":"run","reason":"yes"},{"id":"image/build","decision":"run","reason":"yes"}]}`,
		`{"steps":[{"id":"unknown/check","decision":"run","reason":"yes"},{"id":"image/build","decision":"skip","reason":"yes"}]}`,
		`{"steps":[{"id":"lint/check","decision":"run","reason":"yes","script":"exit 0"},{"id":"image/build","decision":"skip","reason":"yes"}]}`,
	} {
		t.Run(proposal, func(t *testing.T) {
			if _, err := Build(t.Context(), Request{Inventory: src, Selection: json.RawMessage(proposal), Policy: Policy{TimeoutSeconds: 180}}); err == nil {
				t.Fatal("invalid scope or replacement execution was accepted")
			}
		})
	}
}

func TestCachedAcceptanceCannotSurviveDefinitionChange(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	src := inventory(t)
	policy := Policy{TimeoutSeconds: 180}
	p, err := Build(t.Context(), Request{Inventory: src, Selection: json.RawMessage(selection), Policy: policy})
	if err != nil {
		t.Fatal(err)
	}
	digest, err := Digest(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := Save(t.Context(), src, Cache{Plan: p, Accepted: digest}); err != nil {
		t.Fatal(err)
	}
	if _, current, err := Read(t.Context(), src, policy); err != nil || !current {
		t.Fatalf("accepted current plan not reusable: current=%v err=%v", current, err)
	}
	path := filepath.Join(src.Root, src.Pipeline)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, []byte("\n# changed definition\n")...), 0o600); err != nil {
		t.Fatal(err)
	}
	changed, err := source.Load(t.Context(), src.Root, path)
	if err != nil {
		t.Fatal(err)
	}

	if _, current, err := Read(t.Context(), changed, policy); err != nil || current {
		t.Fatalf("stale definition retained acceptance: current=%v err=%v", current, err)
	}
}
