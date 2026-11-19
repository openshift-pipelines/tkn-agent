package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/openshift-pipelines/tkn-agent/internal/agent"
	"github.com/openshift-pipelines/tkn-agent/internal/plan"
	"github.com/openshift-pipelines/tkn-agent/internal/runner"
	"github.com/openshift-pipelines/tkn-agent/internal/source"
)

type report struct {
	State             string           `json:"state"`
	Message           string           `json:"message"`
	Digest            string           `json:"plan_digest,omitempty"`
	Plan              *plan.Plan       `json:"plan,omitempty"`
	Selected          int              `json:"selected"`
	Skipped           int              `json:"skipped"`
	Unresolved        int              `json:"unresolved"`
	Checks            []runner.Receipt `json:"checks"`
	Snapshot          string           `json:"snapshot_digest,omitempty"`
	ClusterCIRequired bool             `json:"cluster_ci_required"`
	Provided          int              `json:"provided"`
	SharedPlan        string           `json:"shared_plan,omitempty"`
}

func verification(ctx context.Context, opts options) (report, int) {
	result := report{State: "blocked", ClusterCIRequired: true}
	block := func(err error) (report, int) {
		result.State = "blocked"
		result.Message = err.Error()
		return result, 2
	}
	if !opts.local {
		return block(fmt.Errorf("--local is required; remote execution is not selected implicitly"))
	}
	if opts.timeout < time.Second || opts.timeout%time.Second != 0 {
		return block(fmt.Errorf("timeout must be a whole number of seconds, at least one"))
	}
	parameters := make(map[string]string)
	for _, raw := range opts.parameters {
		name, value, exists := strings.Cut(raw, "=")
		if _, duplicate := parameters[name]; !exists || name == "" || duplicate {
			return block(fmt.Errorf("each --param must uniquely specify NAME=VALUE"))
		}
		parameters[name] = value
	}
	src, err := source.Load(ctx, source.Request{Root: opts.root, Pipeline: opts.pipeline, Parameters: parameters, CheckoutTask: opts.checkout})
	if err != nil {
		return block(err)
	}
	policy := plan.Policy{TimeoutSeconds: int64(opts.timeout / time.Second)}
	var cached plan.Cache
	current := false
	if opts.refresh {
		cached.SharedHash, err = plan.SharedHash(ctx, src)
	} else {
		cached, current, err = plan.Read(ctx, src, policy)
	}
	if err != nil {
		return block(err)
	}
	result.SharedPlan = plan.SharedPath(src)
	if !current {
		generated, err := selectScope(ctx, src, opts)
		if err != nil {
			return block(err)
		}
		p, err := plan.Build(ctx, plan.Request{Inventory: src, Selection: generated.Selection, Inputs: generated.Inputs, Policy: policy})
		if err != nil {
			return block(err)
		}
		cached.Plan = p
	}
	for _, s := range cached.Plan.Steps {
		switch s.Decision {
		case "run":
			result.Selected++
		case "skip":
			result.Skipped++
		case "unresolved":
			result.Unresolved++
		case "provided":
			result.Provided++
		}
	}
	if result.Selected == 0 {
		result.State, result.Message, result.Plan = "no-local-checks", "No supported local check was selected; this is not a validation pass", &cached.Plan
		if err := plan.Save(ctx, src, cached); err != nil {
			return block(err)
		}
		return result, 2
	}
	cached.Plan, err = runner.Pin(ctx, cached.Plan)
	if err != nil {
		return block(err)
	}
	result.Digest, err = plan.Digest(cached.Plan)
	if err != nil {
		return block(err)
	}
	if opts.approve != "" && opts.approve != result.Digest {
		return block(fmt.Errorf("approval digest does not match the current plan; review it again"))
	}
	if opts.approve == result.Digest {
		cached.Accepted = result.Digest
		cached, err = plan.SaveShared(ctx, src, cached)
		if err != nil {
			return block(err)
		}
	}
	if err := plan.Save(ctx, src, cached); err != nil {
		return block(err)
	}
	if cached.Accepted != result.Digest {
		result.State, result.Message, result.Plan = "review-required", "Review the selected checks, source commands, image identities and execution policy", &cached.Plan
		return result, 2
	}
	execution, err := runner.Run(ctx, src.Root, cached.Plan)
	result.Checks, result.Snapshot = execution.Checks, execution.Snapshot
	if err != nil {
		return block(err)
	}
	result.State, result.Message = "passed", "The reviewed local scope passed"
	code := 0
	for _, check := range execution.Checks {
		switch check.Outcome {
		case "failed":
			if code == 0 {
				result.State, result.Message, code = "failed", "A selected source check failed", 1
			}
		case "blocked", "not-run":
			result.State, result.Message, code = "blocked", "Selected coverage is incomplete; inspect check output", 2
		}
	}
	if len(execution.Checks) != result.Selected {
		return block(fmt.Errorf("missing execution receipts for selected checks"))
	}
	return result, code
}

func selectScope(ctx context.Context, src source.Inventory, opts options) (agent.Result, error) {
	eligible := false
	for _, step := range src.Steps {
		eligible = eligible || (!step.Provided && !step.Heavy && step.Unsupported == "")
	}
	if !eligible {
		type decision struct {
			ID       string `json:"id"`
			Decision string `json:"decision"`
			Reason   string `json:"reason"`
		}
		proposal := struct {
			Steps []decision `json:"steps"`
		}{}
		for _, step := range src.Steps {
			d := decision{ID: step.ID, Decision: "unresolved", Reason: step.Unsupported}
			if step.Provided {
				d.Decision, d.Reason = "provided", "Explicit reviewed self-checkout binding"
			} else if step.Heavy {
				d.Decision, d.Reason = "skip", "Heavy or outer-loop operation"
			}
			proposal.Steps = append(proposal.Steps, d)
		}
		data, err := json.Marshal(proposal)
		return agent.Result{Selection: data}, err
	}
	inventory := struct {
		source.Inventory
		Capabilities []string `json:"local_capabilities"`
	}{Inventory: src, Capabilities: []string{
		"Podman, private Git-listed source snapshot, literal environment, source scripts",
		"Snapshot includes tracked and non-ignored untracked files, but no Git metadata",
		"Only the explicit checkout is provided; its dependencies, workspace aliases and string bindings are resolved",
		"No other task dependencies, results, conditions, matrix, services, remote execution or credential bindings",
		"Inspect helpers; leave checks needing unsupported context unresolved",
	}}
	data, err := json.Marshal(inventory)
	if err != nil {
		return agent.Result{}, err
	}
	provider := opts.provider
	if provider == "" {
		provider = "openai"
	}
	return agent.Select(ctx, agent.Request{Root: src.Root, Inventory: data, Provider: provider, Model: opts.model})
}
