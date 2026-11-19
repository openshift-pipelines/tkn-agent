//go:build workup_live

package workup_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/openshift-pipelines/tkn-agent/internal/workup"
)

// TestLiveWorkup is an explicit paid-provider and real-cluster evaluation.
// Missing prerequisites fail this requested suite; ordinary CI does not run it.
func TestLiveWorkup(t *testing.T) {
	keys := []string{"WORKUP_CONFIG", "WORKUP_KUBECONFIG", "WORKUP_CONTEXT",
		"WORKUP_NAMESPACE", "WORKUP_TARGET", "WORKUP_TARGET_UID", "WORKUP_CONTROL",
		"WORKUP_CONTROL_UID", "TKN_AGENT_MODEL"}
	settings := make(map[string]string, len(keys))
	for _, key := range keys {
		settings[key] = os.Getenv(key)
		if settings[key] == "" {
			t.Fatalf("live evaluation requires %s", key)
		}
	}
	binary, err := filepath.Abs("../../bin/tkn-agent")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Minute)
	defer cancel()
	goal := "Investigate PipelineRun " + settings["WORKUP_TARGET"] +
		", UID " + settings["WORKUP_TARGET_UID"] +
		", source kubernetes, namespace " + settings["WORKUP_NAMESPACE"] +
		". Execute native diagnostic procedures: verify actual per-step exit status through one procedure, " +
		"and independently compare effective build input against the passing control through another. " +
		"Choose the appropriate Pipelines from the reviewed catalog; an MCP-only explanation does not fulfill this request. " +
		"The matching passing control is " + settings["WORKUP_CONTROL"] + ", UID " + settings["WORKUP_CONTROL_UID"] +
		"; read it through MCP for comparison, but execute diagnostic Pipelines only against the failed target. " +
		"Use actual logs and diagnostic execution evidence. Keep required evidence gaps separate from out-of-scope limitations. " +
		"Do not modify or retry the target."
	cmd := exec.CommandContext(ctx, binary, "workup", goal,
		"--config="+settings["WORKUP_CONFIG"],
		"--kubeconfig="+settings["WORKUP_KUBECONFIG"],
		"--context="+settings["WORKUP_CONTEXT"],
		"--namespace="+settings["WORKUP_NAMESPACE"],
		"--provider=openai", "--model="+settings["TKN_AGENT_MODEL"],
		"--execute", "--format=json")
	var stderr strings.Builder
	cmd.Stderr = &stderr
	output, runErr := cmd.Output()
	var report workup.Report
	if err := json.Unmarshal(output, &report); err != nil {
		t.Fatalf("CLI did not produce a structured report: %v; stderr=%s", err, stderr.String())
	}
	if dir := os.Getenv("WORKUP_EVIDENCE_DIR"); dir != "" {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "live-report.json"), output, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if runErr != nil || report.State != "completed" {
		t.Fatalf("workup incomplete: error=%v state=%s reason=%s stderr=%s",
			runErr, report.State, report.Reason, stderr.String())
	}
	var selected []string
	for _, run := range report.Runs {
		selected = append(selected, run.Pipeline)
		if run.Name == "" || run.UID == "" || run.Outcome != "succeeded" {
			t.Fatalf("diagnostic lacks a successful native receipt: %+v", run)
		}
		var raw string
		if err := json.Unmarshal(run.Results["evidence"], &raw); err != nil {
			t.Fatalf("missing actual string evidence result: %v", err)
		}
		var evidence struct {
			PipelineRun string `json:"pipelinerun"`
			UID         string `json:"uid"`
		}
		if err := json.Unmarshal([]byte(raw), &evidence); err != nil {
			t.Fatalf("invalid diagnostic JSON: %v", err)
		}
		if evidence.PipelineRun != settings["WORKUP_TARGET"] || evidence.UID != settings["WORKUP_TARGET_UID"] {
			t.Fatalf("diagnostic targeted a different workload: %+v", evidence)
		}
		if run.Pipeline == "inspect-build-inputs" {
			var controlText string
			if err := json.Unmarshal(run.Results["control-evidence"], &controlText); err != nil {
				t.Fatalf("required native control comparison absent: %v", err)
			}
			var control struct {
				PipelineRun string `json:"pipelinerun"`
				UID         string `json:"uid"`
				Present     bool   `json:"BUILD_TARGET_present"`
				Log         string `json:"log_excerpt"`
				Conditions  []struct {
					Type   string `json:"type"`
					Status string `json:"status"`
				} `json:"conditions"`
			}
			if err := json.Unmarshal([]byte(controlText), &control); err != nil {
				t.Fatal(err)
			}
			if control.PipelineRun != settings["WORKUP_CONTROL"] || control.UID != settings["WORKUP_CONTROL_UID"] ||
				!control.Present || !strings.Contains(control.Log, "Configuration accepted for target demo") {
				t.Fatalf("comparison lacks actual passing-control evidence: %+v", control)
			}
			succeeded := false
			for _, condition := range control.Conditions {
				if condition.Type == "Succeeded" && condition.Status == "True" {
					succeeded = true
				}
			}
			if !succeeded {
				t.Fatal("comparison did not establish actual control success")
			}
		}
		checkNativeReceipt(t, settings, run)
	}
	slices.Sort(selected)
	if !slices.Equal(selected, []string{"inspect-build-inputs", "inspect-build-state"}) {
		t.Fatalf("required execution evidence absent: got %v", selected)
	}
	foundMCP := false
	foundLogs := false
	for _, call := range report.MCPCalls {
		if call.Server == "evidence" && call.Tool == "get_kubernetes_resource" && call.Success {
			foundMCP = true
		}
		if call.Server == "evidence" && call.Tool == "get_pipelinerun_logs" && call.Success {
			foundLogs = true
		}
	}
	if !foundMCP || !foundLogs {
		t.Fatal("required resource or log MCP evidence was not retrieved")
	}
	if report.Usage.ModelCalls < 2 || report.Usage.TotalTokens <= 0 {
		t.Fatalf("no measured model/evidence reasoning loop: %+v", report.Usage)
	}
	t.Logf("completed with %d native diagnostics, %d model calls and %d tokens",
		len(report.Runs), report.Usage.ModelCalls, report.Usage.TotalTokens)
}

func checkNativeReceipt(t *testing.T, settings map[string]string, receipt workup.RunReceipt) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "kubectl",
		"--kubeconfig="+settings["WORKUP_KUBECONFIG"], "--context="+settings["WORKUP_CONTEXT"],
		"--namespace="+settings["WORKUP_NAMESPACE"], "get", "pipelinerun", receipt.Name, "-o=json")
	data, err := cmd.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			t.Fatalf("independent native receipt lookup failed: %s", exit.Stderr)
		}
		t.Fatal(err)
	}
	var native struct {
		Metadata struct {
			UID string `json:"uid"`
		} `json:"metadata"`
		Status struct {
			Conditions []struct {
				Type   string `json:"type"`
				Status string `json:"status"`
			} `json:"conditions"`
			Results []struct {
				Name  string          `json:"name"`
				Value json.RawMessage `json:"value"`
			} `json:"results"`
		} `json:"status"`
	}
	if err := json.Unmarshal(data, &native); err != nil {
		t.Fatal(err)
	}
	if native.Metadata.UID != receipt.UID {
		t.Fatalf("receipt UID %s differs from native UID %s", receipt.UID, native.Metadata.UID)
	}
	succeeded := false
	for _, condition := range native.Status.Conditions {
		if condition.Type == "Succeeded" && condition.Status == "True" {
			succeeded = true
		}
	}
	if !succeeded {
		t.Fatal("diagnostic receipt claims success without a native successful condition")
	}
	for name, value := range receipt.Results {
		matched := false
		for _, result := range native.Status.Results {
			if result.Name == name && string(result.Value) == string(value) {
				matched = true
			}
		}
		if !matched {
			t.Fatalf("reported result %s differs from native PipelineRun results", name)
		}
	}
}
