// Package runner executes reviewed source checks using existing Podman.
package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"slices"
	"time"

	"github.com/openshift-pipelines/tkn-agent/internal/plan"
	"github.com/openshift-pipelines/tkn-agent/internal/source"
)

// Pin records immutable local image identities before plan acceptance.
func Pin(ctx context.Context, p plan.Plan) (plan.Plan, error) {
	p.Steps = slices.Clone(p.Steps)
	resolved := make(map[string]image)
	for i, step := range p.Steps {
		if step.Decision != "run" {
			continue
		}
		key := step.Source.Image + "\x00" + step.Source.PullPolicy
		pinned, exists := resolved[key]
		if !exists {
			var err error
			pinned, err = inspectImage(ctx, step.Source)
			if err != nil {
				return plan.Plan{}, err
			}
			resolved[key] = pinned
		}
		p.Steps[i].ImageID = pinned.ID
		p.Steps[i].Platform = pinned.OS + "/" + pinned.Architecture
	}
	return p, nil
}

type image struct {
	ID           string `json:"Id"`
	OS           string `json:"Os"`
	Architecture string `json:"Architecture"`
}

func inspectImage(ctx context.Context, step source.Step) (image, error) {
	ref := step.Image
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	exists := true
	if err := exec.CommandContext(ctx, "podman", "image", "exists", ref).Run(); err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 1 {
			return image{}, fmt.Errorf("podman unavailable: %w", err)
		}
		exists = false
	}
	if !exists && step.PullPolicy == "Never" {
		return image{}, errors.New("source image is not cached and its pull policy is Never")
	}
	if step.PullPolicy == "Always" || !exists {
		if output, err := exec.CommandContext(ctx, "podman", "pull", ref).CombinedOutput(); err != nil {
			return image{}, fmt.Errorf("pull %s: %w: %s", ref, err, output)
		}
	}
	output, err := exec.CommandContext(ctx, "podman", "image", "inspect", ref).Output()
	if err != nil {
		return image{}, fmt.Errorf("inspect %s: %w", ref, err)
	}
	var images []image
	if err := json.Unmarshal(output, &images); err != nil {
		return image{}, fmt.Errorf("decode image identity: %w", err)
	}
	if len(images) != 1 || images[0].ID == "" || images[0].OS == "" || images[0].Architecture == "" {
		return image{}, errors.New("podman did not provide an unambiguous image identity/platform")
	}
	return images[0], nil
}
