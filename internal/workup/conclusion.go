package workup

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"google.golang.org/genai"
)

type conclusion struct {
	Conclusion      string   `json:"conclusion"`
	MissingEvidence []string `json:"missing_evidence"`
	Limitations     []string `json:"limitations,omitempty"`
}

func conclusionSchema() *genai.Schema {
	return &genai.Schema{Type: genai.TypeObject, Required: []string{"conclusion", "missing_evidence"},
		Properties: map[string]*genai.Schema{
			"conclusion":       {Type: genai.TypeString},
			"missing_evidence": {Type: genai.TypeArray, Items: &genai.Schema{Type: genai.TypeString}},
			"limitations":      {Type: genai.TypeArray, Items: &genai.Schema{Type: genai.TypeString}},
		}}
}

func parseConclusion(text string) (conclusion, error) {
	var c conclusion
	decoder := json.NewDecoder(bytes.NewBufferString(text))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&c); err != nil {
		return c, err
	}
	if err := decoder.Decode(new(json.RawMessage)); !errors.Is(err, io.EOF) {
		return c, errors.New("conclusion contains trailing data")
	}
	if strings.TrimSpace(c.Conclusion) == "" || c.MissingEvidence == nil {
		return c, errors.New("conclusion and explicit missing_evidence array are required")
	}
	for _, evidence := range c.MissingEvidence {
		if strings.TrimSpace(evidence) == "" {
			return c, errors.New("missing evidence entries must be concrete")
		}
	}
	for _, limitation := range c.Limitations {
		if strings.TrimSpace(limitation) == "" {
			return c, errors.New("scope limitations must be concrete")
		}
	}
	return c, nil
}
