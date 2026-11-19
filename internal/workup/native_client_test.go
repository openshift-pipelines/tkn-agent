//go:build native_client

package workup

import (
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// This explicit integration uses the real kubectl against a local API server.
// It needs neither a cluster credential nor a provider connection.
func TestRealNativeClientUsesPrivateHomeAndCleansDiscovery(t *testing.T) {
	realClient, err := exec.LookPath("kubectl")
	if err != nil {
		t.Fatal("native_client integration requires an existing kubectl:", err)
	}
	originalPath := os.Getenv("PATH")
	opts := fixtureOptions(t, "success")
	t.Setenv("PATH", filepath.Dir(realClient)+string(os.PathListSeparator)+originalPath)
	api := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer native-fixture-token" {
			http.Error(w, "fixture identity missing", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		var response any
		group := map[string]any{
			"name":             "tekton.dev",
			"versions":         []map[string]string{{"groupVersion": "tekton.dev/v1", "version": "v1"}},
			"preferredVersion": map[string]string{"groupVersion": "tekton.dev/v1", "version": "v1"},
		}
		switch r.URL.Path {
		case "/api":
			response = map[string]any{"kind": "APIVersions", "apiVersion": "v1", "versions": []string{"v1"}}
		case "/apis":
			response = map[string]any{"kind": "APIGroupList", "apiVersion": "v1", "groups": []any{group}}
		case "/apis/tekton.dev":
			response = group
		case "/api/v1":
			response = map[string]any{"kind": "APIResourceList", "apiVersion": "v1", "groupVersion": "v1", "resources": []any{}}
		case "/apis/tekton.dev/v1":
			response = map[string]any{"kind": "APIResourceList", "apiVersion": "v1", "groupVersion": "tekton.dev/v1",
				"resources": []map[string]any{{"name": "pipelines", "singularName": "pipeline",
					"namespaced": true, "kind": "Pipeline", "verbs": []string{"get", "list"}}}}
		default:
			prefix := "/apis/tekton.dev/v1/namespaces/fixture-ns/pipelines/"
			if !strings.HasPrefix(r.URL.Path, prefix) {
				http.NotFound(w, r)
				return
			}
			name := strings.TrimPrefix(r.URL.Path, prefix)
			response = map[string]any{"kind": "Pipeline", "apiVersion": "tekton.dev/v1",
				"metadata": map[string]string{"name": name, "uid": "fixture-" + name},
				"spec": map[string]any{
					"params":  []map[string]string{{"name": "target", "type": "string"}},
					"results": []map[string]string{{"name": "diagnosis"}}, "tasks": []any{},
				}}
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Error(err)
		}
	}))
	defer api.Close()
	certificate := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: api.Certificate().Raw})
	kubeconfig := "apiVersion: v1\nkind: Config\nclusters:\n- name: fixture\n  cluster:\n    server: " + api.URL +
		"\n    certificate-authority-data: " + base64.StdEncoding.EncodeToString(certificate) +
		"\nusers:\n- name: fixture\n  user:\n    token: native-fixture-token\ncontexts:\n- name: fixture-context\n  context:\n    cluster: fixture\n    user: fixture\n"
	writeFile(t, opts.Kubeconfig, kubeconfig)
	workspace, temporary := t.TempDir(), t.TempDir()
	t.Chdir(workspace)
	t.Setenv("TMPDIR", temporary)
	providerFixture(t, func(w http.ResponseWriter, _ *http.Request) {
		textReply(t, w, "Actual native discovery completed; no execution requested.")
	})

	report, err := Run(t.Context(), opts)
	if err != nil || report.State != "completed" {
		t.Fatalf("real native discovery failed: state=%s reason=%s error=%v", report.State, report.Reason, err)
	}
	if _, err := os.Stat(filepath.Join(workspace, ".kube")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("real native discovery polluted the workspace: %v", err)
	}
	entries, err := os.ReadDir(temporary)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("real native discovery cache leaked after invocation: %v", entries)
	}
}
