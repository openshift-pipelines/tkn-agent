package workup

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	adkagent "google.golang.org/adk/v2/agent"
)

type inspectArgs struct {
	Pipeline string                     `json:"pipeline"`
	Params   map[string]json.RawMessage `json:"params"`
}
type execution struct {
	native    nativeClient
	cfg       config
	pipelines map[string]pipeline
	report    *Report
	seen      map[string]int
	mu        *sync.Mutex
}

func (e *execution) inspect(ctx adkagent.Context, args inspectArgs) (RunReceipt, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	p, ok := e.pipelines[args.Pipeline]
	if !ok {
		return RunReceipt{}, errors.New("pipeline lacks operator authorization")
	}
	plan, err := p.plan(args)
	if err != nil {
		return RunReceipt{}, err
	}
	keyBytes, err := json.Marshal(plan)
	if err != nil {
		return RunReceipt{}, err
	}
	key := digest(keyBytes)
	if idx, ok := e.seen[key]; ok {
		if idx < 0 {
			return RunReceipt{Pipeline: p.policy.Name, Outcome: "review-required"}, nil
		}
		return e.report.Runs[idx], nil
	}
	e.report.Plans = append(e.report.Plans, plan)
	if !e.native.opts.Execute {
		e.seen[key] = -1
		return RunReceipt{Pipeline: p.policy.Name, Outcome: "review-required"}, nil
	}
	if len(e.report.Runs) >= e.cfg.Limits.MaxRuns {
		return RunReceipt{}, errors.New("native run limit reached")
	}
	idx := len(e.report.Runs)
	name := fmt.Sprintf("workup-%s-%d", e.native.owner, idx+1)
	e.report.Runs = append(e.report.Runs, RunReceipt{Pipeline: p.policy.Name, Name: name, Outcome: "unfinished", Results: map[string]json.RawMessage{}})
	e.seen[key] = idx
	params := make([]struct {
		Name  string          `json:"name"`
		Value json.RawMessage `json:"value"`
	}, 0, len(plan.Params))
	names := make([]string, 0, len(plan.Params))
	for name := range plan.Params {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		params = append(params, struct {
			Name  string          `json:"name"`
			Value json.RawMessage `json:"value"`
		}{name, plan.Params[name]})
	}
	body, err := json.Marshal(map[string]any{
		"apiVersion": "tekton.dev/v1", "kind": "PipelineRun",
		"metadata": map[string]any{"name": name, "namespace": e.native.opts.Namespace, "labels": map[string]string{"tkn-agent-workup": e.native.owner}},
		"spec": map[string]any{"pipelineSpec": p.spec, "params": params,
			"taskRunTemplate": map[string]string{"serviceAccountName": e.cfg.Execution.ServiceAccount},
			"timeouts":        map[string]string{"pipeline": fmt.Sprintf("%ds", e.cfg.Limits.TimeoutSeconds)}},
	})
	if err != nil {
		return e.report.Runs[idx], err
	}
	data, err := e.native.command(ctx, []string{"create", "-f", "-", "-o", "json"}, body)
	if err != nil {
		return e.report.Runs[idx], err
	}
	var created nativeObject
	if err := json.Unmarshal(data, &created); err != nil {
		return e.report.Runs[idx], err
	}
	if created.Metadata.Name != name || created.Metadata.UID == "" {
		return e.report.Runs[idx], errors.New("created PipelineRun identity missing")
	}
	e.report.Runs[idx].UID = created.Metadata.UID
	for {
		obj, err := e.native.get(ctx, "pipelinerun", name)
		if err != nil {
			return e.report.Runs[idx], err
		}
		if obj.Metadata.UID != e.report.Runs[idx].UID {
			return e.report.Runs[idx], errors.New("PipelineRun UID changed")
		}
		if updateReceipt(&e.report.Runs[idx], obj, p.policy.RequiredResults) {
			return e.report.Runs[idx], nil
		}
		timer := time.NewTimer(200 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return e.report.Runs[idx], ctx.Err()
		case <-timer.C:
		}
	}
}

func updateReceipt(receipt *RunReceipt, obj nativeObject, required []string) bool {
	receipt.Results = map[string]json.RawMessage{}
	for _, result := range obj.Status.Results {
		receipt.Results[result.Name] = result.Value
	}
	for _, condition := range obj.Status.Conditions {
		if condition.Type != "Succeeded" || condition.Status == "Unknown" {
			continue
		}
		receipt.Reason = condition.Reason
		receipt.MissingResults = nil
		for _, name := range required {
			if value, ok := receipt.Results[name]; !ok || string(value) == "null" {
				receipt.MissingResults = append(receipt.MissingResults, name)
			}
		}
		receipt.Outcome = "succeeded"
		if len(receipt.MissingResults) > 0 {
			receipt.Outcome = "missing-results"
		}
		if condition.Status != "True" {
			receipt.Outcome = "failed"
		}
		return true
	}
	return false
}

func (e *execution) cleanup(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	var result error
	for i := range e.report.Runs {
		r := &e.report.Runs[i]
		if r.Outcome != "unfinished" {
			continue
		}
		obj, err := e.native.get(ctx, "pipelinerun", r.Name)
		if err == nil && (obj.Metadata.Labels["tkn-agent-workup"] != e.native.owner ||
			(r.UID != "" && obj.Metadata.UID != r.UID)) {
			err = errors.New("refusing cancellation: ownership mismatch")
		}
		if err == nil {
			r.UID = obj.Metadata.UID
			if updateReceipt(r, obj, e.pipelines[r.Pipeline].policy.RequiredResults) {
				continue
			}
			patch, marshalErr := json.Marshal([]map[string]string{
				{"op": "test", "path": "/metadata/uid", "value": r.UID},
				{"op": "test", "path": "/metadata/resourceVersion", "value": obj.Metadata.ResourceVersion},
				{"op": "add", "path": "/spec/status", "value": "Cancelled"},
			})
			if marshalErr != nil {
				err = marshalErr
			} else {
				_, err = e.native.command(ctx, []string{"patch", "pipelinerun", r.Name, "--type=json", "-p", string(patch), "-o", "json"}, nil)
				if err != nil {
					// A concurrent terminal status update must win over cancellation.
					latest, readErr := e.native.get(ctx, "pipelinerun", r.Name)
					if readErr == nil && latest.Metadata.UID == r.UID &&
						updateReceipt(r, latest, e.pipelines[r.Pipeline].policy.RequiredResults) {
						r.Cancellation = "not-needed-completed"
						continue
					}
					err = errors.Join(err, readErr)
				}
			}
		}
		if err != nil {
			r.Cancellation = "failed"
			result = errors.Join(result, err)
		} else {
			r.Cancellation = "requested"
			r.Outcome = "cancelled"
		}
	}
	return result
}
