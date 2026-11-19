package runner

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openshift-pipelines/tkn-agent/internal/plan"
	"github.com/openshift-pipelines/tkn-agent/internal/source"
)

func TestImagePullPolicyDeterminesResolvedStoreIdentity(t *testing.T) {
	for _, test := range []struct {
		policy string
		cached bool
		want   string
	}{
		{"Always", true, "new"},
		{"IfNotPresent", true, "old"},
		{"IfNotPresent", false, "new"},
		{"Never", false, ""},
	} {
		t.Run(test.policy+test.want, func(t *testing.T) {
			bin := t.TempDir()
			store := filepath.Join(t.TempDir(), "image")
			script := `#!/bin/sh
set -eu
case "$1 $2" in
  "image exists") test -f "$IMAGE_TEST_STORE" ;;
  "pull test-image:1") printf new > "$IMAGE_TEST_STORE" ;;
  "image inspect") printf '[{"Id":"%s","Os":"linux","Architecture":"arm64"}]' "$(head -c 3 "$IMAGE_TEST_STORE")" ;;
  *) exit 125 ;;
esac
`
			if err := os.WriteFile(filepath.Join(bin, "podman"), []byte(script), 0o755); err != nil {
				t.Fatal(err)
			}
			if test.cached {
				if err := os.WriteFile(store, []byte("old"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("IMAGE_TEST_STORE", store)
			proposal := plan.Plan{Steps: []plan.Step{{Decision: "run",
				Source: source.Step{Image: "test-image:1", PullPolicy: test.policy}}}}

			got, err := Pin(context.Background(), proposal)

			if test.want == "" {
				if err == nil || !strings.Contains(err.Error(), "Never") {
					t.Fatalf("missing Never image did not block: %v", err)
				}
				if _, err := os.Stat(store); !os.IsNotExist(err) {
					t.Fatal("Never policy materialized a missing image")
				}
				return
			}
			if err != nil || got.Steps[0].ImageID != test.want {
				t.Fatalf("wrong resolved store state: %+v %v", got, err)
			}
		})
	}
}
