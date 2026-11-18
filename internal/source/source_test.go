package source

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(t *testing.T, text string) (string, string) {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, "pipeline.yaml")
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	return root, path
}

func TestInventoryPreservesChecksAndIdentifiesHeavyWork(t *testing.T) {
	data, err := os.ReadFile("../../evals/fixtures/mixed-checks.yaml")
	if err != nil {
		t.Fatal(err)
	}
	root, path := fixture(t, string(data))

	got, err := Load(context.Background(), root, path)

	if err != nil {
		t.Fatal(err)
	}
	if len(got.Steps) != 2 || got.Steps[0].ID != "lint/check" || got.Steps[1].ID != "image/build" {
		t.Fatalf("unexpected inventory: %+v", got.Steps)
	}
	if got.Steps[0].Unsupported != "" || got.Steps[0].Heavy || got.Steps[0].WorkingDir != "/workspace" {
		t.Fatalf("source workspace mapping failed: %+v", got.Steps[0])
	}
	if !got.Steps[1].Heavy || !strings.Contains(got.Steps[1].Script, "buildah bud") {
		t.Fatalf("image build was lost or allowed: %+v", got.Steps[1])
	}
}

func TestUnsupportedInputsCannotBecomeRunnableInventory(t *testing.T) {
	data, err := os.ReadFile("../../evals/fixtures/mixed-checks.yaml")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		old  string
		new  string
	}{
		{"result dependency", "grep -qx valid message.txt", "echo $(tasks.producer.results.value)"},
		{"secret environment", "workingDir: $(workspaces.source.path)", "env:\n                - name: PASSWORD\n                  value: never-send-this\n              workingDir: $(workspaces.source.path)"},
		{"matrix", "name: lint\n        workspaces:", "name: lint\n        matrix:\n          params:\n            - name: version\n              value: [one, two]\n        workspaces:"},
		{"secret workspace", "emptyDir: {}", "secret:\n        secretName: private-input"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root, path := fixture(t, strings.Replace(string(data), tt.old, tt.new, 1))

			got, err := Load(context.Background(), root, path)

			if err != nil {
				t.Fatal(err)
			}
			if got.Steps[0].Unsupported == "" {
				t.Fatal("unsupported check was presented as runnable")
			}
			encoded, err := json.Marshal(got)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(encoded), "never-send-this") {
				t.Fatal("credential value entered model inventory")
			}
		})
	}
}

func TestDuplicateSourceIDsAreRejected(t *testing.T) {
	data, err := os.ReadFile("../../evals/fixtures/mixed-checks.yaml")
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Replace(string(data), "name: image", "name: lint", 1)
	text = strings.Replace(text, "name: build", "name: check", 1)
	root, path := fixture(t, text)

	if _, err := Load(context.Background(), root, path); err == nil {
		t.Fatal("duplicate source IDs were accepted")
	}
}
