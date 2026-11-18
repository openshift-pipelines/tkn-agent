package runner

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/openshift-pipelines/tkn-agent/internal/plan"
)

// Receipt describes one actual invocation, independently of the proposed scope.
type Receipt struct {
	ID        string `json:"id"`
	Outcome   string `json:"outcome"`
	ExitCode  int    `json:"exit_code"`
	Output    string `json:"output"`
	Container string `json:"container"`
}

// Result reports source identity and actual coverage, including blocked starts.
type Result struct {
	Snapshot string    `json:"snapshot_digest"`
	Checks   []Receipt `json:"checks"`
}

type execution struct {
	workspace string
	policy    plan.Policy
}

// Run executes only reviewed run decisions against a private source snapshot.
func Run(ctx context.Context, root string, p plan.Plan) (result Result, resultErr error) {
	private, err := os.MkdirTemp("", "tkn-agent-")
	if err != nil {
		return result, err
	}
	defer func() {
		if err := os.RemoveAll(private); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("clean source snapshot %s: %w", private, err))
		}
	}()
	private, err = filepath.EvalSymlinks(private)
	if err != nil {
		return result, err
	}
	workspace := filepath.Join(private, "workspace")
	if err := os.Mkdir(workspace, 0o755); err != nil {
		return result, err
	}
	snap, err := copySource(ctx, root, workspace)
	if err != nil {
		return result, err
	}
	current, err := plan.InputsCurrent(ctx, workspace, p.Inputs)
	if err != nil || !current {
		return result, errors.Join(err, errors.New("planning inputs are missing or changed in the source snapshot"))
	}
	result.Snapshot = snap.Digest
	executor := execution{workspace: snap.Directory, policy: p.Policy}
	stopped := make(map[string]bool)
	for _, step := range p.Steps {
		if step.Decision != "run" {
			continue
		}
		if stopped[step.Source.Task] {
			result.Checks = append(result.Checks, Receipt{ID: step.Source.ID, Outcome: "not-run", ExitCode: -1})
			continue
		}
		receipt, err := executor.check(ctx, step)
		result.Checks = append(result.Checks, receipt)
		if err != nil {
			return result, err
		}
		if receipt.Outcome != "passed" {
			stopped[step.Source.Task] = true
		}
	}
	return result, nil
}

func (e execution) check(ctx context.Context, step plan.Step) (Receipt, error) {
	id := make([]byte, 12)
	if _, err := rand.Read(id); err != nil {
		return Receipt{}, err
	}
	receipt := Receipt{ID: step.Source.ID, Container: "tkn-agent-" + hex.EncodeToString(id), ExitCode: -1}
	args := []string{"run", "--rm", "--http-proxy=false", "--name", receipt.Container, "--platform", step.Platform,
		"--network", e.policy.Network, "-i", "-v", e.workspace + ":/workspace:Z",
		"-w", step.Source.WorkingDir, "-e", "WORKSPACE=/workspace"}
	names := make([]string, 0, len(step.Source.Env))
	for name := range step.Source.Env {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		args = append(args, "-e", name+"="+step.Source.Env[name])
	}
	args = append(args, "--entrypoint", step.Source.Shell, step.ImageID)
	args = append(args, step.Source.ShellArgs...)
	runCtx, cancel := context.WithTimeout(ctx, time.Duration(e.policy.TimeoutSeconds)*time.Second)
	defer cancel()
	cmd := exec.CommandContext(runCtx, "podman", args...)
	cmd.Stdin = strings.NewReader(step.Source.Script)
	var output bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &output
	err := cmd.Run()
	receipt.Output = output.String()
	if err == nil {
		receipt.ExitCode, receipt.Outcome = 0, "passed"
		return receipt, nil
	}
	if runCtx.Err() != nil {
		receipt.Outcome = "blocked"
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		if cleaned, cleanupErr := exec.CommandContext(cleanupCtx, "podman", "rm", "--force", "--time", "0", "--ignore", receipt.Container).CombinedOutput(); cleanupErr != nil {
			return receipt, fmt.Errorf("cancel/cleanup %s: %w: %s", receipt.Container, cleanupErr, cleaned)
		}
		receipt.Outcome = "blocked"
		return receipt, fmt.Errorf("check cancelled or timed out: %w", runCtx.Err())
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		receipt.ExitCode = exit.ExitCode()
		receipt.Outcome = "failed"
		if exit.ExitCode() >= 125 {
			receipt.Outcome = "blocked"
		}
		return receipt, nil
	}
	receipt.Outcome = "blocked"
	return receipt, fmt.Errorf("start Podman check: %w", err)
}
