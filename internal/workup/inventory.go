package workup

import (
	"encoding/json"
	"slices"
)

// Pipeline descriptions and specs are discovery data, never authorization.
// These exact snapshots also supply every created run's inline pipelineSpec.
func pipelineInventory(pipelines map[string]pipeline) ([]byte, error) {
	type entry struct {
		Name        string          `json:"name"`
		Description string          `json:"description"`
		Spec        json.RawMessage `json:"spec"`
		SpecDigest  string          `json:"spec_digest"`
		Policy      pipelineConfig  `json:"operator_policy"`
	}
	names := make([]string, 0, len(pipelines))
	for name := range pipelines {
		names = append(names, name)
	}
	slices.Sort(names)
	entries := make([]entry, 0, len(names))
	for _, name := range names {
		p := pipelines[name]
		entries = append(entries, entry{Name: name, Description: p.description, Spec: p.spec,
			SpecDigest: digest(p.spec), Policy: p.policy})
	}
	return json.Marshal(entries)
}
