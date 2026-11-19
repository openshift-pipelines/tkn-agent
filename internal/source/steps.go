package source

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var heavyCommand = regexp.MustCompile(`(?i)\b(docker|podman|buildah|skopeo|buildctl|kaniko|kubectl|helm|cosign|goreleaser)\b|\boc\s|\bgit\s+push\b|\bgh\s+(release|api)\b`)
var heavyImage = regexp.MustCompile(`(?i)/(?:[^/]*kaniko[^/]*|buildah|buildkit|[^/]*dind)(?::|@|/|$)`)
var gitCommand = regexp.MustCompile(`\bgit\s+`)
var substitution = regexp.MustCompile(`\$\((?:params|tasks|workspaces|context|results|steps)\.[^)]*\)|\{\{`)
var credentialName = regexp.MustCompile(`(?i)password|passwd|token|secret|api[_-]?key|authorization`)

func taskSteps(node yaml.Node, context executionContext, final bool) ([]Step, error) {
	var task task
	if err := node.Decode(&task); err != nil {
		return nil, err
	}
	if task.Name == "" {
		return nil, errors.New("every pipeline task needs a name")
	}
	if context.checkout != nil && task.Name == context.checkout.Task {
		return []Step{{ID: task.Name + "/reference", Task: task.Name, Provided: true}}, nil
	}
	spec := task.Spec
	if spec.Kind == 0 && task.Ref.Kind != 0 {
		var ref struct {
			Name     string `yaml:"name"`
			Resolver string `yaml:"resolver"`
		}
		if err := task.Ref.Decode(&ref); err != nil {
			return nil, err
		}
		if ref.Resolver == "" {
			spec = context.definitions["Task/"+ref.Name]
		}
	}
	if spec.Kind == 0 {
		return []Step{{ID: task.Name + "/reference", Task: task.Name, Unsupported: "task reference is not available in this file"}}, nil
	}
	var body taskSpec
	if err := spec.Decode(&body); err != nil {
		return nil, err
	}
	note := unexpected(node, "name", "taskSpec", "taskRef", "runAfter", "when", "retries", "onError", "timeout", "params", "workspaces", "description", "displayName")
	if note == "" {
		note = unexpected(spec, "steps", "workspaces", "description", "displayName", "params", "sidecars", "stepTemplate", "volumes", "results")
	}
	switch {
	case final:
		note = "finally tasks remain in outer-loop CI"
	case task.When.Kind != 0 || task.Retries != 0 || task.OnError != "" || task.Timeout != "":
		note = "task dependencies, conditions, retries or lifecycle overrides require a later capability"
	case body.Sidecars.Kind != 0:
		note = "required sidecar service is not supported"
	case body.Template.Kind != 0 || body.Volumes.Kind != 0:
		note = "step templates and additional volumes are not supported"
	}
	for _, dependency := range task.RunAfter {
		if context.checkout == nil || dependency != context.checkout.Task {
			note = "task dependency is not provided by the explicit checkout binding"
		}
	}
	parameters, err := scopedParameters(body, task.Params, context.parameters)
	if err != nil {
		return nil, err
	}
	bindings := make(map[string]string)
	for name, value := range parameters {
		bindings["$(params."+name+")"] = value
	}
	sourceWorkspace := "source"
	if context.checkout != nil {
		sourceWorkspace = context.checkout.Workspace
	}
	for _, workspace := range task.Workspaces {
		if workspace.Workspace != sourceWorkspace || workspace.SubPath != "" {
			note = "only an explicit source workspace without subPath is supported"
		}
		bindings["$(workspaces."+workspace.Name+".path)"] = "/workspace"
	}
	for _, declaration := range body.Workspaces {
		if _, ok := bindings["$(workspaces."+declaration.Name+".path)"]; !ok || declaration.MountPath != "" || declaration.ReadOnly {
			note = "workspace binding or access mode is not supported"
		}
	}
	var result []Step
	for index, raw := range body.Steps {
		var original step
		if err := raw.Decode(&original); err != nil {
			return nil, err
		}
		if original.Name == "" {
			return nil, fmt.Errorf("task %s step %d needs a name", task.Name, index)
		}
		s := makeStep(original, task.Name, bindings)
		if note != "" {
			s.Unsupported = note
		}
		if extra := unexpected(raw, "name", "description", "displayName", "image", "imagePullPolicy", "script", "command", "args", "workingDir", "env", "onError", "ref"); extra != "" {
			s.Unsupported = extra
		}
		result = append(result, s)
	}
	return result, nil
}

func makeStep(original step, taskName string, bindings map[string]string) Step {
	s := Step{
		ID: taskName + "/" + original.Name, Task: taskName, Image: original.Image,
		Script: original.Script, WorkingDir: original.WorkingDir,
		Env: make(map[string]string), Shell: "sh", ShellArgs: []string{"-e", "-s"},
	}
	replace := func(value string) string {
		for key, path := range bindings {
			value = strings.ReplaceAll(value, key, path)
		}
		return value
	}
	s.Script, s.WorkingDir, s.Image = replace(s.Script), replace(s.WorkingDir), replace(s.Image)
	s.PullPolicy = original.PullPolicy
	if s.PullPolicy == "" {
		s.PullPolicy = "IfNotPresent"
		tag := strings.LastIndex(s.Image, ":")
		if !strings.Contains(s.Image, "@") && (tag <= strings.LastIndex(s.Image, "/") || strings.HasSuffix(s.Image, ":latest")) {
			s.PullPolicy = "Always"
		}
	}
	if s.WorkingDir == "" {
		s.WorkingDir = "/workspace"
	}
	text := original.Script + "\n" + strings.Join(append(original.Command, original.Args...), " ")
	s.Heavy = heavyCommand.MatchString(text) || heavyImage.MatchString(original.Image) || strings.Contains(strings.ToLower(original.Description+" "+original.DisplayName), "(skip-local)")
	switch {
	case s.PullPolicy != "Always" && s.PullPolicy != "IfNotPresent" && s.PullPolicy != "Never":
		s.Unsupported = "image pull policy is not supported"
	case s.Image == "" || strings.HasPrefix(s.Image, "-") || strings.ContainsAny(s.Image, " \t\n"):
		s.Unsupported = "image is missing, invalid or unresolved"
	case original.Ref.Kind != 0 || s.Script == "" || len(original.Command) > 0 || len(original.Args) > 0:
		s.Unsupported = "only inline script steps are supported"
	case original.OnError != "":
		s.Unsupported = "step error-continuation policy is not supported"
	case gitCommand.MatchString(original.Script):
		s.Unsupported = "Git-metadata-dependent scripts are outside the initial snapshot capability"
	case !strings.HasPrefix(s.WorkingDir, "/workspace/") && s.WorkingDir != "/workspace":
		s.Unsupported = "working directory must be within the source workspace"
	}
	if strings.HasPrefix(s.Script, "#!") {
		first, _, _ := strings.Cut(s.Script, "\n")
		parts := strings.Fields(strings.TrimPrefix(first, "#!"))
		if len(parts) == 2 && parts[0] == "/usr/bin/env" {
			parts = parts[1:]
		}
		if len(parts) != 1 || (parts[0] != "/bin/sh" && parts[0] != "/bin/bash" && parts[0] != "sh" && parts[0] != "bash") {
			s.Unsupported = "script interpreter is not supported"
		} else {
			s.Shell = parts[0]
			s.ShellArgs = []string{"-s"}
		}
	}
	for _, env := range original.Env {
		if credentialName.MatchString(env.Name) || env.ValueFrom.Kind != 0 {
			s.Unsupported = "credential or referenced environment requires an explicit binding"
			continue
		}
		s.Env[env.Name] = replace(env.Value)
	}
	values := s.Image + s.Script + s.WorkingDir
	for _, value := range s.Env {
		values += value
	}
	if substitution.MatchString(values) {
		s.Unsupported = "unresolved Tekton or PaC substitution"
	}
	if filepath.Clean(s.WorkingDir) != s.WorkingDir {
		s.Unsupported = "working directory contains an unsafe path"
	}
	return s
}

func unexpected(node yaml.Node, allowed ...string) string {
	for i := 0; i+1 < len(node.Content); i += 2 {
		key := node.Content[i].Value
		found := false
		for _, name := range allowed {
			found = found || key == name
		}
		if !found {
			return "unsupported execution field " + key
		}
	}
	return ""
}
