package workup

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// WORKUP-003: missing trusted configuration must fail before provider access.
func TestRunRejectsMissingConfigurationBeforeModel(t *testing.T) {
	report, err := Run(t.Context(), Options{
		ConfigPath: filepath.Join(t.TempDir(), "absent.yaml"),
		Goal:       "inspect sample", Provider: "openai", Model: "fixture",
		Kubeconfig: "/explicit/config", Context: "explicit", Namespace: "tests",
	})
	if err == nil || report.State != "incomplete" || report.Usage.ModelCalls != 0 {
		t.Fatalf("missing config accepted or model called: %+v %v", report, err)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o700); err != nil {
		t.Fatal(err)
	}
}

// WORKUP-003: strict config is a trust boundary.
func TestRunRejectsUnknownConfigAndMissingSelectors(t *testing.T) {
	for _, input := range []string{"\nsurprise: true\n", "\nversion: 1\n", "\n---\nversion: 1\n"} {
		t.Run(strings.ReplaceAll(input, "\n", "_"), func(t *testing.T) {
			opts := fixtureOptions(t, "success")
			data, readErr := os.ReadFile(opts.ConfigPath)
			if readErr != nil {
				t.Fatal(readErr)
			}
			writeFile(t, opts.ConfigPath, string(data)+input)
			providerFixture(t, func(w http.ResponseWriter, _ *http.Request) { textReply(t, w, "No missing evidence.") })
			r, err := Run(t.Context(), opts)
			if err == nil || r.Usage.ModelCalls != 0 || len(r.Runs) != 0 {
				t.Fatalf("invalid configuration accepted: %+v %v", r, err)
			}
		})
	}
}

func TestRunRequiresEveryExplicitClusterSelector(t *testing.T) {
	for _, field := range []string{"kubeconfig", "context", "namespace"} {
		t.Run(field, func(t *testing.T) {
			opts := fixtureOptions(t, "success")
			switch field {
			case "kubeconfig":
				opts.Kubeconfig = ""
			case "context":
				opts.Context = ""
			case "namespace":
				opts.Namespace = ""
			}
			r, err := Run(t.Context(), opts)
			if err == nil || !strings.Contains(err.Error(), "explicit") || r.Usage.ModelCalls != 0 {
				t.Fatalf("ambient cluster selector accepted: %+v %v", r, err)
			}
		})
	}
}
