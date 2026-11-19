package workup

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

func TestRunRejectsUnstructuredOrIncompleteConclusions(t *testing.T) {
	for _, output := range []string{
		"everything healthy", `{"conclusion":"healthy"}`,
		`{"conclusion":"healthy","missing_evidence":null}`,
		`{"conclusion":"healthy","missing_evidence":[],"state":"completed"}`,
	} {
		t.Run(output, func(t *testing.T) {
			opts := fixtureOptions(t, "success")
			providerFixture(t, func(w http.ResponseWriter, _ *http.Request) {
				providerReply(t, w, map[string]string{"role": "assistant", "content": output})
			})
			report, err := Run(t.Context(), opts)
			if err == nil || report.State != "incomplete" {
				t.Fatalf("unstructured evidence conclusion accepted: %+v %v", report, err)
			}
		})
	}
}

func TestRunKeepsRequiredEvidenceGapsSeparateFromScopeLimitations(t *testing.T) {
	for _, requiredGap := range []bool{false, true} {
		name := "bounded-completion"
		if requiredGap {
			name = "required-evidence-missing"
		}
		t.Run(name, func(t *testing.T) {
			opts := fixtureOptions(t, "success")
			opts.Execute = true
			missing := []string{}
			state := "completed"
			if requiredGap {
				missing = []string{"external build log unavailable"}
				state = "incomplete"
			}
			final, err := json.Marshal(map[string]any{
				"conclusion":       "Build input is missing; diagnostic execution succeeded.",
				"missing_evidence": missing, "limitations": []string{"Fleet-wide incident history is outside this fixture scope."},
			})
			if err != nil {
				t.Fatal(err)
			}
			var calls atomic.Int32
			providerFixture(t, func(w http.ResponseWriter, _ *http.Request) {
				if calls.Add(1) == 1 {
					toolReply(t, w, "inspect_pipeline", `{"pipeline":"first-check","params":{"target":"sample"}}`)
				} else {
					providerReply(t, w, map[string]string{"role": "assistant", "content": string(final)})
				}
			})
			report, err := Run(t.Context(), opts)
			if err != nil || report.State != state || len(report.MissingEvidence) != len(missing) ||
				len(report.Runs) != 1 || report.Runs[0].Outcome != "succeeded" {
				t.Fatalf("required evidence accounting changed: %+v %v", report, err)
			}
			data, err := json.Marshal(report)
			if err != nil {
				t.Fatal(err)
			}
			var exposed struct {
				Limitations []string `json:"limitations"`
			}
			if err := json.Unmarshal(data, &exposed); err != nil {
				t.Fatal(err)
			}
			if len(exposed.Limitations) != 1 || exposed.Limitations[0] != "Fleet-wide incident history is outside this fixture scope." {
				t.Fatalf("scope limitation was discarded: %s", data)
			}
		})
	}
}

// WORKUP-005: Pharos protocol success can carry a denied native API read.
func TestRunDoesNotCountMCPProtocolSuccessAsEvidenceValidity(t *testing.T) {
	opts := fixtureOptions(t, "success")
	opts.Execute = true
	addMCP(t, opts, mcpFixture(t, "Kubernetes API Error: 403 - Forbidden"))
	var calls atomic.Int32
	providerFixture(t, func(w http.ResponseWriter, r *http.Request) {
		request := requestText(t, r)
		switch calls.Add(1) {
		case 1:
			toolReply(t, w, "pharos_query", `{}`)
		case 2:
			if !strings.Contains(request, `"evidence_available":false`) {
				t.Error("denied MCP evidence was not marked missing")
			}
			toolReply(t, w, "inspect_pipeline", `{"pipeline":"first-check","params":{"target":"sample"}}`)
		default:
			textReply(t, w, "Everything healthy.")
		}
	})
	report, err := Run(t.Context(), opts)
	if err != nil || report.State != "incomplete" || len(report.MissingEvidence) != 1 ||
		len(report.MCPCalls) != 1 || !report.MCPCalls[0].Success ||
		len(report.Runs) != 1 || report.Runs[0].Outcome != "succeeded" {
		t.Fatalf("protocol completion implied evidence validity: %+v %v", report, err)
	}
}
