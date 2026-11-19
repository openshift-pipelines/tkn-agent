package workup

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestRunKeepsNativeDiscoveryCacheOutsideWorkspaceAndRemovesIt(t *testing.T) {
	opts := fixtureOptions(t, "success")
	workspace, cacheParent := t.TempDir(), t.TempDir()
	t.Chdir(workspace)
	t.Setenv("TMPDIR", cacheParent)
	providerFixture(t, func(w http.ResponseWriter, _ *http.Request) {
		textReply(t, w, "Catalog observed; no execution requested.")
	})

	report, err := Run(t.Context(), opts)
	if err != nil || report.State != "completed" {
		t.Fatalf("catalog inspection failed: %+v %v", report, err)
	}
	if _, err := os.Stat(filepath.Join(workspace, ".kube")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("native cache polluted the caller workspace: %v", err)
	}
	entries, err := os.ReadDir(cacheParent)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("invocation-owned native cache was not cleaned: %v", entries)
	}
}
