// Package source reads pipeline definitions without executing or resolving them.
package source

import "gopkg.in/yaml.v3"

// Inventory is the bounded, credential-free context given to the selector.
type Inventory struct {
	Root     string `json:"-"`
	Pipeline string `json:"pipeline"`
	Hash     string `json:"-"`
	Steps    []Step `json:"steps"`
}

// Step retains source execution details; unsupported steps cannot be run.
type Step struct {
	ID          string            `json:"id"`
	Task        string            `json:"task"`
	Image       string            `json:"image"`
	Script      string            `json:"script"`
	Shell       string            `json:"shell"`
	ShellArgs   []string          `json:"shell_args"`
	WorkingDir  string            `json:"working_dir"`
	Env         map[string]string `json:"env"`
	Heavy       bool              `json:"heavy"`
	Unsupported string            `json:"unsupported,omitempty"`
}

type document struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Name         string `yaml:"name"`
		GenerateName string `yaml:"generateName"`
	} `yaml:"metadata"`
	Spec yaml.Node `yaml:"spec"`
}

type pipeline struct {
	Tasks   []yaml.Node `yaml:"tasks"`
	Finally []yaml.Node `yaml:"finally"`
}

type task struct {
	Name       string    `yaml:"name"`
	Spec       yaml.Node `yaml:"taskSpec"`
	Ref        yaml.Node `yaml:"taskRef"`
	RunAfter   []string  `yaml:"runAfter"`
	When       yaml.Node `yaml:"when"`
	Retries    int       `yaml:"retries"`
	OnError    string    `yaml:"onError"`
	Timeout    string    `yaml:"timeout"`
	Workspaces []struct {
		Name      string `yaml:"name"`
		Workspace string `yaml:"workspace"`
		SubPath   string `yaml:"subPath"`
	} `yaml:"workspaces"`
}

type taskSpec struct {
	Steps      []yaml.Node `yaml:"steps"`
	Sidecars   yaml.Node   `yaml:"sidecars"`
	Template   yaml.Node   `yaml:"stepTemplate"`
	Volumes    yaml.Node   `yaml:"volumes"`
	Workspaces []struct {
		Name      string `yaml:"name"`
		MountPath string `yaml:"mountPath"`
		ReadOnly  bool   `yaml:"readOnly"`
		Optional  bool   `yaml:"optional"`
	} `yaml:"workspaces"`
}

type step struct {
	Name        string    `yaml:"name"`
	Description string    `yaml:"description"`
	DisplayName string    `yaml:"displayName"`
	Image       string    `yaml:"image"`
	Script      string    `yaml:"script"`
	Command     []string  `yaml:"command"`
	Args        []string  `yaml:"args"`
	WorkingDir  string    `yaml:"workingDir"`
	OnError     string    `yaml:"onError"`
	Env         []envVar  `yaml:"env"`
	Ref         yaml.Node `yaml:"ref"`
}

type envVar struct {
	Name      string    `yaml:"name"`
	Value     string    `yaml:"value"`
	ValueFrom yaml.Node `yaml:"valueFrom"`
}
