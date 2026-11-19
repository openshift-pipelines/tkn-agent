package source

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func checkoutFixture(t *testing.T, text string) Request {
	t.Helper()
	root, path := fixture(t, text)
	for _, args := range [][]string{{"init", "--quiet", root}, {"-C", root, "remote", "add", "origin", "https://github.com/openshift-pipelines/pipeline-service.git"}} {
		cmd := exec.CommandContext(t.Context(), "git", args...)
		cmd.Env = append(os.Environ(), "GIT_MASTER=1")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("prepare checkout: %v: %s", err, out)
		}
	}
	return Request{Root: root, Pipeline: path, CheckoutTask: "fetch-repository", Parameters: map[string]string{"target_branch": "main"}}
}

func TestExplicitCheckoutResolvesWorkspaceAndParameterScope(t *testing.T) {
	data, err := os.ReadFile("../../evals/fixtures/pipeline-service/static-checks.yaml")
	if err != nil {
		t.Fatal(err)
	}
	request := checkoutFixture(t, string(data))

	got, err := Load(t.Context(), request)

	if err != nil {
		t.Fatal(err)
	}
	if got.Checkout == nil || got.Checkout.Subdirectory != "source" || got.Checkout.Workspace != "shared-workspace" || !got.Steps[0].Provided {
		t.Fatalf("checkout materialization not represented: %+v", got)
	}
	if got.Steps[1].Unsupported != "" || got.Steps[1].Image != "quay.io/redhat-pipeline-service/static-checks:main" ||
		got.Steps[1].PullPolicy != "Always" || got.Steps[1].WorkingDir != "/workspace/source" {
		t.Fatalf("source execution not preserved: %+v", got.Steps[1])
	}
}

func TestCheckoutBindingRejectsDifferentOrAdditionalRequirements(t *testing.T) {
	data, err := os.ReadFile("../../evals/fixtures/pipeline-service/static-checks.yaml")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, before, after string
	}{
		{"different repository", "{{ repo_url }}", "https://github.com/another/repository"},
		{"different revision", "{{ revision }}", "another-branch"},
		{"unsafe subdirectory", "value: source", "value: ../outside"},
		{"unknown checkout setup", "name: subdirectory", "name: bootstrap-script"},
		{"conditional checkout", "name: fetch-repository\n", "name: fetch-repository\n        when: []\n"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			request := checkoutFixture(t, strings.Replace(string(data), test.before, test.after, 1))

			if _, err := Load(t.Context(), request); err == nil {
				t.Fatal("checkout adaptation silently omitted a source requirement")
			}
		})
	}
}
