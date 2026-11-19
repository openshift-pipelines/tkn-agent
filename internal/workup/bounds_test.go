package workup

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"testing"
)

func TestCompoundParameterConstraintsRejectWrongScalarTypes(t *testing.T) {
	for _, test := range []struct {
		typ, raw string
		allowed  bool
	}{
		{"array", `["ok"]`, true}, {"array", `[null]`, false}, {"array", `[3]`, false},
		{"array", `["forbidden"]`, false}, {"array", `["ok","ok","ok"]`, false},
		{"object", `{"a":"ok"}`, true}, {"object", `{"a":null}`, false},
		{"object", `{"a":[]}`, false}, {"object", `{"long-key":"ok"}`, false},
	} {
		t.Run(test.typ+test.raw, func(t *testing.T) {
			rule := paramRule{Type: test.typ, Enum: []string{"ok"}, MaxLength: 2}
			err := rule.validate(json.RawMessage(test.raw))
			if (err == nil) != test.allowed {
				t.Fatalf("constraint validation: %s %v", test.raw, err)
			}
		})
	}
}

// WORKUP-008: tool-call and model-call budgets stop actual SDK dispatch.
func TestRunBoundsRepeatedModelAndToolCalls(t *testing.T) {
	for _, boundary := range []string{"model", "tool"} {
		t.Run(boundary, func(t *testing.T) {
			opts := fixtureOptions(t, "success")
			data, err := os.ReadFile(opts.ConfigPath)
			if err != nil {
				t.Fatal(err)
			}
			config := string(data)
			if boundary == "model" {
				config = strings.ReplaceAll(config, "max_model_calls: 12", "max_model_calls: 2")
			} else {
				config = strings.ReplaceAll(config, "max_tool_calls: 24", "max_tool_calls: 2")
			}
			writeFile(t, opts.ConfigPath, config)
			var calls atomic.Int32
			providerFixture(t, func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				toolReply(t, w, "list_skills", `{}`)
			})
			report, err := Run(t.Context(), opts)
			wantCalls := int32(2)
			if boundary == "tool" {
				wantCalls = 3
			}
			if err == nil || report.State != "incomplete" || calls.Load() != wantCalls || report.Usage.ToolCalls != 2 {
				t.Fatalf("budget not enforced: %+v calls=%d err=%v", report, calls.Load(), err)
			}
		})
	}
}

func TestRunBoundsModelContextBeforeTransmission(t *testing.T) {
	opts := fixtureOptions(t, "success")
	data, err := os.ReadFile(opts.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, opts.ConfigPath, strings.ReplaceAll(string(data), "max_evidence_bytes: 131072", "max_evidence_bytes: 1024"))
	var calls atomic.Int32
	providerFixture(t, func(w http.ResponseWriter, _ *http.Request) { calls.Add(1); textReply(t, w, "bad") })
	report, err := Run(t.Context(), opts)
	if err == nil || calls.Load() != 0 || report.Usage.ModelCalls != 0 || !strings.Contains(err.Error(), "context exceeds") {
		t.Fatalf("oversize context transmitted: %+v %v", report, err)
	}
}
