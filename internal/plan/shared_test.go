package plan

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSharedPlanReusesAnalysisWithoutTransferringApproval(t *testing.T) {
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
	if _, err := SaveShared(t.Context(), src, Cache{Plan: p, Accepted: digest}); err != nil {
		t.Fatal(err)
	}
	shared, err := os.ReadFile(filepath.Join(src.Root, SharedPath(src)))
	if err != nil {
		t.Fatal(err)
	}
	other := inventory(t)
	path := filepath.Join(other.Root, SharedPath(other))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, shared, 0o600); err != nil {
		t.Fatal(err)
	}

	got, current, err := Read(t.Context(), other, policy)

	if err != nil || !current || got.Accepted != "" || len(got.Plan.Steps) != 2 {
		t.Fatalf("shared analysis missing or approval transferred: current=%v err=%v cached=%+v", current, err, got)
	}
}

func TestSharedPlanRejectsAlteredSourceExecution(t *testing.T) {
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
	if _, err := SaveShared(t.Context(), src, Cache{Plan: p, Accepted: digest}); err != nil {
		t.Fatal(err)
	}
	p.Steps[0].Source.Script = "exit 0"
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src.Root, SharedPath(src)), data, 0o600); err != nil {
		t.Fatal(err)
	}

	if _, _, err := Read(t.Context(), src, policy); err == nil {
		t.Fatal("shared plan replaced a source check")
	}
}

func TestPlanningStateCannotBecomeItsOwnInput(t *testing.T) {
	src := inventory(t)
	name := ".tkn-agent/state.json"
	body := []byte("{}\n")
	if err := os.Mkdir(filepath.Join(src.Root, ".tkn-agent"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src.Root, name), body, 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(body)

	_, err := Build(t.Context(), Request{Inventory: src, Selection: json.RawMessage(selection),
		Inputs: map[string]string{name: hex.EncodeToString(sum[:])}, Policy: Policy{TimeoutSeconds: 180}})

	if err == nil {
		t.Fatal("state self-reference was accepted as a planning input")
	}
}

func TestAbsentConfigurationAppearanceInvalidatesPlanning(t *testing.T) {
	root := t.TempDir()
	inputs := map[string]string{"optional.conf": "absent"}
	if current, err := InputsCurrent(t.Context(), root, inputs); err != nil || !current {
		t.Fatalf("recorded absence was not reusable: current=%v err=%v", current, err)
	}
	if err := os.WriteFile(filepath.Join(root, "optional.conf"), []byte("enabled=true\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if current, err := InputsCurrent(t.Context(), root, inputs); err != nil || current {
		t.Fatalf("new configuration retained stale planning: current=%v err=%v", current, err)
	}
}

func TestSharedPlanChangedDuringReviewIsNotOverwritten(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	src := inventory(t)
	p, err := Build(t.Context(), Request{Inventory: src, Selection: json.RawMessage(selection), Policy: Policy{TimeoutSeconds: 180}})
	if err != nil {
		t.Fatal(err)
	}
	digest, err := Digest(p)
	if err != nil {
		t.Fatal(err)
	}
	current, err := SaveShared(t.Context(), src, Cache{Plan: p, Accepted: digest})
	if err != nil {
		t.Fatal(err)
	}
	current.Plan.Steps[0].Reason = "new reviewed scope explanation"
	current.Accepted, err = Digest(current.Plan)
	if err != nil {
		t.Fatal(err)
	}
	other := current.Plan
	other.Steps = append([]Step(nil), other.Steps...)
	other.Steps[0].Reason = "another writer's explanation"
	changed, err := json.Marshal(other)
	if err != nil {
		t.Fatal(err)
	}
	name := filepath.Join(src.Root, SharedPath(src))
	if err := os.WriteFile(name, changed, 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := SaveShared(t.Context(), src, current); err == nil {
		t.Fatal("another writer's shared plan was overwritten")
	}
	retained, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	if string(retained) != string(changed) {
		t.Fatal("conflicting shared plan content changed")
	}
}
