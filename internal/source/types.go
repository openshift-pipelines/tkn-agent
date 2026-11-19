// Package source reads pipeline definitions without executing or resolving them.
package source

import "gopkg.in/yaml.v3"

const StateDirectory = ".tkn-agent"

// Request supplies explicit, reviewable adaptations to source execution.
type Request struct {
	Root         string
	Pipeline     string
	Parameters   map[string]string
	CheckoutTask string
}

// Checkout records a provided self-checkout, not an executed remote Task.
type Checkout struct {
	Task         string `json:"task"`
	Repository   string `json:"repository"`
	Revision     string `json:"revision"`
	Reference    string `json:"reference"`
	Workspace    string `json:"workspace"`
	Subdirectory string `json:"subdirectory"`
}

// Inventory is the bounded, credential-free context given to the selector.
type Inventory struct {
	Root       string            `json:"-"`
	Pipeline   string            `json:"pipeline"`
	Hash       string            `json:"-"`
	Steps      []Step            `json:"steps"`
	Parameters map[string]string `json:"parameters,omitempty"`
	Checkout   *Checkout         `json:"checkout,omitempty"`
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
	Provided    bool              `json:"provided,omitempty"`
	PullPolicy  string            `json:"pull_policy"`
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
	Tasks      []yaml.Node `yaml:"tasks"`
	Finally    []yaml.Node `yaml:"finally"`
	Params     []parameter `yaml:"params"`
	Workspaces []struct {
		Name string `yaml:"name"`
	} `yaml:"workspaces"`
}

type task struct {
	Name       string      `yaml:"name"`
	Spec       yaml.Node   `yaml:"taskSpec"`
	Ref        yaml.Node   `yaml:"taskRef"`
	RunAfter   []string    `yaml:"runAfter"`
	When       yaml.Node   `yaml:"when"`
	Retries    int         `yaml:"retries"`
	OnError    string      `yaml:"onError"`
	Timeout    string      `yaml:"timeout"`
	Params     []parameter `yaml:"params"`
	Workspaces []struct {
		Name      string `yaml:"name"`
		Workspace string `yaml:"workspace"`
		SubPath   string `yaml:"subPath"`
	} `yaml:"workspaces"`
}

type taskSpec struct {
	Params     []parameter `yaml:"params"`
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
	PullPolicy  string    `yaml:"imagePullPolicy"`
	Script      string    `yaml:"script"`
	Command     []string  `yaml:"command"`
	Args        []string  `yaml:"args"`
	WorkingDir  string    `yaml:"workingDir"`
	OnError     string    `yaml:"onError"`
	Env         []envVar  `yaml:"env"`
	Ref         yaml.Node `yaml:"ref"`
}

type parameter struct {
	Name    string    `yaml:"name"`
	Type    string    `yaml:"type"`
	Default yaml.Node `yaml:"default"`
	Value   yaml.Node `yaml:"value"`
}

type executionContext struct {
	definitions map[string]yaml.Node
	parameters  map[string]string
	checkout    *Checkout
}

type envVar struct {
	Name      string    `yaml:"name"`
	Value     string    `yaml:"value"`
	ValueFrom yaml.Node `yaml:"valueFrom"`
}
