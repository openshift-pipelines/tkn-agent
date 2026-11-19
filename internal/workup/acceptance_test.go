package workup

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

// WORKUP-001/007/008/010: real ADK, skilltoolset, mcptoolset, provider HTTP
// and native API shim. Later choices require actual earlier tool evidence.
func TestRunAdaptsToActualEvidenceAndReusesIdenticalExecution(t *testing.T) {
	opts := fixtureOptions(t, "success")
	opts.Execute = true
	addMCP(t, opts, mcpFixture(t, mcpEvidence))
	var calls atomic.Int32
	providerFixture(t, func(w http.ResponseWriter, r *http.Request) {
		request := requestText(t, r)
		if strings.Contains(request, "fixture-provider-secret") {
			t.Error("credential entered model request")
		}
		switch calls.Add(1) {
		case 1:
			if !strings.Contains(request, "original snapshot") {
				t.Error("actual Pipeline spec missing from model inventory")
			}
			toolReply(t, w, "load_skill", `{"name":"diagnostic"}`)
		case 2:
			if !strings.Contains(request, "Use diagnostic Pipelines and actual evidence") {
				t.Error("skill not loaded")
			}
			toolReply(t, w, "pharos_query", `{}`)
		case 3:
			if !strings.Contains(request, "MCP actual BUILD_TARGET missing") {
				t.Error("MCP evidence not fed back")
			}
			toolReply(t, w, "inspect_pipeline", `{"pipeline":"first-check","params":{"target":"sample"}}`)
		case 4:
			if !strings.Contains(request, "missing BUILD_TARGET: unhealthy workload") {
				t.Error("native evidence not fed back")
			}
			toolReply(t, w, "inspect_pipeline", `{"pipeline":"second-check","params":{"target":"follow"}}`)
		case 5:
			toolReply(t, w, "inspect_pipeline", `{"pipeline":"first-check","params":{"target":"sample"}}`)
		default:
			textReply(t, w, "Build unhealthy: BUILD_TARGET is missing. Diagnostic execution succeeded.")
		}
	})
	report, err := Run(t.Context(), opts)
	if err != nil {
		t.Fatalf("runtime: %v; report %+v", err, report)
	}
	if report.State != "completed" || len(report.Runs) != 2 || len(report.Plans) != 2 ||
		len(report.MCPCalls) != 1 || !report.MCPCalls[0].Success || len(runFiles(t, opts)) != 2 ||
		report.Usage.ModelCalls != 6 || report.Usage.ToolCalls != 5 || report.Usage.TotalTokens != 72 ||
		!strings.Contains(report.Conclusion, "unhealthy") {
		t.Fatalf("adaptive receipts incorrect: %+v", report)
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "fixture-provider-secret") {
		t.Fatal("report leaked credential")
	}
	for _, name := range runFiles(t, opts) {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		var run struct {
			Spec struct {
				PipelineSpec struct {
					Description string `json:"description"`
				} `json:"pipelineSpec"`
				PipelineRef     json.RawMessage `json:"pipelineRef"`
				TaskRunTemplate struct {
					ServiceAccountName string `json:"serviceAccountName"`
				} `json:"taskRunTemplate"`
			} `json:"spec"`
		}
		if err := json.Unmarshal(data, &run); err != nil {
			t.Fatal(err)
		}
		if run.Spec.PipelineSpec.Description != "original snapshot" || len(run.Spec.PipelineRef) != 0 ||
			run.Spec.TaskRunTemplate.ServiceAccountName != "reviewed-reader" {
			t.Fatalf("native run lost pinned spec or trusted SA: %s", data)
		}
	}
}

// WORKUP-002: execution permission comes only from explicit options.
func TestRunPreservesReviewPlansWithoutCreatingRuns(t *testing.T) {
	opts := fixtureOptions(t, "success")
	var calls atomic.Int32
	providerFixture(t, func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			toolReply(t, w, "inspect_pipeline", `{"pipeline":"first-check","params":{"target":"sample"}}`)
		} else {
			textReply(t, w, "Plan requires operator execution authorization.")
		}
	})
	report, err := Run(t.Context(), opts)
	if err != nil || report.State != "review-required" || len(report.Plans) != 1 ||
		len(report.Runs) != 0 || len(runFiles(t, opts)) != 0 {
		t.Fatalf("review performed execution or lost plan: %+v %v", report, err)
	}
}

// WORKUP-004: model content cannot expand operator policy.
func TestRunRejectsUnauthorizedModelArguments(t *testing.T) {
	for _, args := range []string{
		`{"pipeline":"unknown","params":{}}`,
		`{"pipeline":"first-check","params":{"target":"forbidden"}}`,
		`{"pipeline":"first-check","params":{"target":["sample"]}}`,
		`{"pipeline":"first-check","params":{"target":"sample","extra":"x"}}`,
		`{"pipeline":"first-check","params":{}}`,
		`{"pipeline":"first-check","params":{"target":"sample"},"serviceAccountName":"admin"}`,
	} {
		t.Run(args, func(t *testing.T) {
			opts := fixtureOptions(t, "success")
			opts.Execute = true
			providerFixture(t, func(w http.ResponseWriter, _ *http.Request) { toolReply(t, w, "inspect_pipeline", args) })
			report, err := Run(t.Context(), opts)
			if err == nil || report.State != "incomplete" || len(runFiles(t, opts)) != 0 {
				t.Fatalf("unauthorized model input accepted: %+v %v", report, err)
			}
		})
	}
}

// WORKUP-005: diagnostic status and evidence completeness override model prose.
func TestRunAccountsForActualFailuresAndMissingResults(t *testing.T) {
	for _, test := range []struct{ mode, state, outcome string }{
		{"missing", "incomplete", "missing-results"}, {"failed", "failed", "failed"},
	} {
		t.Run(test.mode, func(t *testing.T) {
			opts := fixtureOptions(t, test.mode)
			opts.Execute = true
			var calls atomic.Int32
			providerFixture(t, func(w http.ResponseWriter, _ *http.Request) {
				if calls.Add(1) == 1 {
					toolReply(t, w, "inspect_pipeline", `{"pipeline":"first-check","params":{"target":"sample"}}`)
				} else {
					textReply(t, w, "Everything healthy.")
				}
			})
			report, err := Run(t.Context(), opts)
			if err != nil || report.State != test.state || len(report.Runs) != 1 ||
				report.Runs[0].Outcome != test.outcome || report.Runs[0].UID == "" || report.Runs[0].Reason == "" {
				t.Fatalf("native outcome misreported: %+v %v", report, err)
			}
			if test.mode == "missing" && (len(report.MissingEvidence) != 1 || len(report.Runs[0].MissingResults) != 1) {
				t.Fatalf("missing evidence lost: %+v", report)
			}
		})
	}
}

// WORKUP-003: missing selected skills and required MCP tools block pre-model.
func TestRunPreflightsRequiredSkillsAndMCPTools(t *testing.T) {
	for _, mode := range []string{"skill", "tool", "grant"} {
		t.Run(mode, func(t *testing.T) {
			opts := fixtureOptions(t, "success")
			switch mode {
			case "skill":
				if err := os.Remove(filepath.Join(filepath.Dir(opts.Kubeconfig), "diagnostic", "SKILL.md")); err != nil {
					t.Fatal(err)
				}
			case "tool":
				addMCP(t, opts, mcpFixture(t, mcpEvidence))
				data, err := os.ReadFile(opts.ConfigPath)
				if err != nil {
					t.Fatal(err)
				}
				writeFile(t, opts.ConfigPath, strings.ReplaceAll(string(data), "tools: [query]", "tools: [absent]"))
			case "grant":
				path := filepath.Join(filepath.Dir(opts.Kubeconfig), "diagnostic", "SKILL.md")
				writeFile(t, path, "---\nname: diagnostic\ndescription: Inspect.\nallowed-tools: [ungranted]\n---\nInspect.\n")
			}
			var calls atomic.Int32
			providerFixture(t, func(w http.ResponseWriter, _ *http.Request) { calls.Add(1); textReply(t, w, "bad") })
			report, err := Run(t.Context(), opts)
			if err == nil || calls.Load() != 0 || report.Usage.ModelCalls != 0 || len(runFiles(t, opts)) != 0 {
				t.Fatalf("missing prerequisite reached model: %+v %v", report, err)
			}
		})
	}
}
