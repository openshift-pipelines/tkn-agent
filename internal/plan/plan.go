// Package plan validates a model's scope against immutable source execution.
package plan

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/openshift-pipelines/tkn-agent/internal/source"
)

// Policy is part of the reviewed plan, rather than an unreviewed runtime override.
type Policy struct {
	TimeoutSeconds int64  `json:"timeout_seconds"`
	Runtime        string `json:"runtime"`
	Network        string `json:"network"`
	Workspace      string `json:"workspace"`
}

func (p Policy) local() Policy {
	p.Runtime, p.Network, p.Workspace = "podman", "bridge", "private-snapshot"
	return p
}

// Step combines a source step with the model's scope decision.
type Step struct {
	Source   source.Step `json:"source"`
	Decision string      `json:"decision"`
	Reason   string      `json:"reason"`
	ImageID  string      `json:"image_id,omitempty"`
	Platform string      `json:"platform,omitempty"`
}

// Plan contains all source steps, including explicitly omitted checks.
type Plan struct {
	Version  int               `json:"version"`
	Pipeline string            `json:"pipeline"`
	Inputs   map[string]string `json:"inputs"`
	Steps    []Step            `json:"steps"`
	Policy   Policy            `json:"policy"`
}

// Request supplies the proposal and independently observed planning inputs.
type Request struct {
	Inventory source.Inventory
	Selection json.RawMessage
	Inputs    map[string]string
	Policy    Policy
}

type decision struct {
	ID       string `json:"id"`
	Decision string `json:"decision"`
	Reason   string `json:"reason"`
}

// Build permits selection only; executable fields always come from source.
func Build(ctx context.Context, request Request) (Plan, error) {
	if request.Policy.TimeoutSeconds < 1 {
		return Plan{}, errors.New("check timeout must be at least one second")
	}
	var proposal struct {
		Steps []decision `json:"steps"`
	}
	decoder := json.NewDecoder(bytes.NewReader(request.Selection))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&proposal); err != nil {
		return Plan{}, fmt.Errorf("invalid selection: %w", err)
	}
	if err := decoder.Decode(new(json.RawMessage)); !errors.Is(err, io.EOF) {
		return Plan{}, errors.New("selection has trailing data")
	}
	src := request.Inventory
	if len(proposal.Steps) != len(src.Steps) {
		return Plan{}, errors.New("selection must account for every source step")
	}
	chosen := make(map[string]decision)
	for _, d := range proposal.Steps {
		if _, duplicate := chosen[d.ID]; duplicate || d.Reason == "" {
			return Plan{}, errors.New("selection has a duplicate ID or missing reason")
		}
		switch d.Decision {
		case "run", "skip", "unresolved":
		default:
			return Plan{}, fmt.Errorf("%s: invalid decision", d.ID)
		}
		chosen[d.ID] = d
	}
	result := Plan{Version: 1, Pipeline: src.Pipeline, Policy: request.Policy.local(), Inputs: map[string]string{src.Pipeline: src.Hash}}
	for path, hash := range request.Inputs {
		if path == src.Pipeline && hash != src.Hash {
			return Plan{}, errors.New("pipeline changed during planning")
		}
		result.Inputs[path] = hash
	}
	for _, s := range src.Steps {
		d, exists := chosen[s.ID]
		if !exists {
			return Plan{}, fmt.Errorf("selection does not account for %s", s.ID)
		}
		if d.Decision == "run" && (s.Heavy || s.Unsupported != "") {
			return Plan{}, fmt.Errorf("%s cannot run locally: heavy/outer-loop work or %s", s.ID, s.Unsupported)
		}
		result.Steps = append(result.Steps, Step{Source: s, Decision: d.Decision, Reason: d.Reason})
	}
	current, err := InputsCurrent(ctx, src.Root, result.Inputs)
	if err != nil {
		return Plan{}, err
	}
	if !current {
		return Plan{}, errors.New("planning input changed; analyze again")
	}
	return result, nil
}

// Digest identifies the complete effective plan, including pinned images.
func Digest(p Plan) (string, error) {
	data, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

// InputsCurrent checks declared inputs inside the given root, without escaping it.
func InputsCurrent(ctx context.Context, root string, inputs map[string]string) (current bool, resultErr error) {
	directory, err := os.OpenRoot(root)
	if err != nil {
		return false, err
	}
	defer func() { resultErr = errors.Join(resultErr, directory.Close()) }()
	for name, expected := range inputs {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		if !filepath.IsLocal(name) {
			return false, errors.New("planning input escapes the repository")
		}
		info, err := directory.Lstat(name)
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if !info.Mode().IsRegular() {
			return false, errors.New("planning inputs must be regular files")
		}
		file, err := directory.Open(name)
		if err != nil {
			return false, err
		}
		h := sha256.New()
		_, copyErr := io.Copy(h, file)
		if err := errors.Join(copyErr, file.Close()); err != nil {
			return false, err
		}
		if hex.EncodeToString(h.Sum(nil)) != expected {
			return false, nil
		}
	}
	return true, nil
}
