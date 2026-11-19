package source

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

func repositoryIdentity(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "git@github.com:") {
		raw = "https://github.com/" + strings.TrimPrefix(raw, "git@github.com:")
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Hostname() != "github.com" || parsed.Port() != "" ||
		(parsed.Scheme != "https" && parsed.Scheme != "ssh") || parsed.RawQuery != "" || parsed.Fragment != "" ||
		(parsed.User != nil && (parsed.Scheme != "ssh" || parsed.User.Username() != "git")) {
		return "", errors.New("self-checkout binding requires a credential-free GitHub repository origin")
	}
	repo := strings.TrimSuffix(strings.Trim(parsed.Path, "/"), ".git")
	parts := strings.Split(repo, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" || strings.Contains(repo, "..") {
		return "", errors.New("invalid checkout repository identity")
	}
	return "https://github.com/" + strings.ToLower(repo), nil
}

func bindCheckout(ctx context.Context, request Request, body pipeline, parameters map[string]string) (*Checkout, error) {
	if request.CheckoutTask == "" {
		return nil, nil
	}
	for _, node := range body.Tasks {
		var task task
		if err := node.Decode(&task); err != nil {
			return nil, err
		}
		if task.Name != request.CheckoutTask {
			continue
		}
		if task.Spec.Kind != 0 || task.Ref.Kind == 0 || len(task.RunAfter) != 0 || task.When.Kind != 0 ||
			task.Retries != 0 || task.OnError != "" || task.Timeout != "" ||
			unexpected(node, "name", "taskRef", "params", "workspaces", "description", "displayName") != "" {
			return nil, errors.New("checkout binding cannot omit task execution or lifecycle requirements")
		}
		var reference struct {
			Name string `yaml:"name"`
			Kind string `yaml:"kind"`
		}
		if err := task.Ref.Decode(&reference); err != nil {
			return nil, err
		}
		if reference.Name != "git-clone" || (reference.Kind != "" && reference.Kind != "Task" && reference.Kind != "ClusterTask") ||
			unexpected(task.Ref, "name", "kind") != "" {
			return nil, errors.New("checkout binding supports only the explicitly reviewed git-clone reference contract")
		}
		values := make(map[string]string)
		for _, p := range task.Params {
			if _, duplicate := values[p.Name]; duplicate || (p.Name != "url" && p.Name != "revision" && p.Name != "subdirectory") ||
				p.Value.Kind != yaml.ScalarNode || p.Value.Tag != "!!str" {
				return nil, errors.New("checkout parameters contain unsupported setup requirements")
			}
			values[p.Name] = replaceParameters(p.Value.Value, parameters)
		}
		origin, err := exec.CommandContext(ctx, "git", "-C", request.Root, "remote", "get-url", "origin").Output()
		if err != nil {
			return nil, fmt.Errorf("read checkout repository identity: %w", err)
		}
		identity, err := repositoryIdentity(string(origin))
		if err != nil {
			return nil, err
		}
		sourceURL := values["url"]
		if strings.Join(strings.Fields(sourceURL), "") == "{{repo_url}}" {
			sourceURL = identity
		}
		requested, err := repositoryIdentity(sourceURL)
		if err != nil || requested != identity {
			return nil, errors.New("checkout binding cannot replace another repository")
		}
		if strings.Join(strings.Fields(values["revision"]), "") != "{{revision}}" {
			return nil, errors.New("checkout binding requires the current event revision, not a different branch or pinned revision")
		}
		top, err := exec.CommandContext(ctx, "git", "-C", request.Root, "rev-parse", "--show-toplevel").Output()
		if err != nil {
			return nil, fmt.Errorf("read checkout root: %w", err)
		}
		canonical, err := filepath.EvalSymlinks(strings.TrimSpace(string(top)))
		if err != nil || canonical != request.Root {
			return nil, errors.New("checkout binding requires the complete repository root")
		}
		subdir := values["subdirectory"]
		if subdir != "" && (!filepath.IsLocal(subdir) || filepath.Clean(subdir) != subdir || subdir == "." || substitution.MatchString(subdir)) {
			return nil, errors.New("checkout subdirectory must be a safe repository-relative path")
		}
		if len(task.Workspaces) != 1 || task.Workspaces[0].Name != "output" || task.Workspaces[0].SubPath != "" {
			return nil, errors.New("checkout binding requires one output workspace without subPath")
		}
		workspace := task.Workspaces[0].Workspace
		for _, declared := range body.Workspaces {
			if workspace != "" && declared.Name == workspace {
				return &Checkout{Task: task.Name, Repository: identity, Revision: "current-worktree",
					Reference: reference.Kind + "/" + reference.Name, Workspace: workspace, Subdirectory: subdir}, nil
			}
		}
		return nil, errors.New("checkout output workspace is not declared by the pipeline")
	}
	return nil, fmt.Errorf("checkout task %q is not in the pipeline", request.CheckoutTask)
}
