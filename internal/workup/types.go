// Package workup runs operator-authorized, bounded native diagnostic work.
package workup

import (
	"encoding/json"
	"time"
)

// Options identifies explicit operator inputs. Execute defaults to review only.
type Options struct {
	ConfigPath, Goal, Provider, Model, Kubeconfig, Context, Namespace string
	Execute                                                           bool
	Timeout                                                           time.Duration
}

// Report preserves all available evidence, including on error.
type Report struct {
	State           string            `json:"state"`
	Reason          string            `json:"reason"`
	Message         string            `json:"message"`
	Conclusion      string            `json:"conclusion"`
	MissingEvidence []string          `json:"missing_evidence"`
	Limitations     []string          `json:"limitations"`
	ConfigDigest    string            `json:"config_digest"`
	SkillDigests    map[string]string `json:"skill_digests"`
	MCPCalls        []MCPReceipt      `json:"mcp_calls"`
	Plans           []Plan            `json:"plans"`
	Runs            []RunReceipt      `json:"runs"`
	Usage           Usage             `json:"usage"`
}

// Usage counts actual model/tool activity; tokens are provider-reported.
type Usage struct {
	Provider     string `json:"provider"`
	Model        string `json:"model"`
	ModelCalls   int    `json:"model_calls"`
	ToolCalls    int    `json:"tool_calls"`
	InputTokens  int64  `json:"input_tokens"`
	OutputTokens int64  `json:"output_tokens"`
	TotalTokens  int64  `json:"total_tokens"`
}

// MCPReceipt deliberately excludes raw tool arguments and responses.
type MCPReceipt struct {
	Server          string `json:"server"`
	Tool            string `json:"tool"`
	ArgumentsDigest string `json:"arguments_digest"`
	ResultDigest    string `json:"result_digest"`
	Success         bool   `json:"success"`
	Error           string `json:"error,omitempty"`
}

// Plan is validated against the fetched Pipeline and operator constraints.
type Plan struct {
	Pipeline   string                     `json:"pipeline"`
	Params     map[string]json.RawMessage `json:"params"`
	SpecDigest string                     `json:"spec_digest"`
}

// RunReceipt records native outcomes, which do not establish workload health.
type RunReceipt struct {
	Pipeline       string                     `json:"pipeline"`
	Name           string                     `json:"name"`
	UID            string                     `json:"uid"`
	Outcome        string                     `json:"outcome"`
	Reason         string                     `json:"reason"`
	Results        map[string]json.RawMessage `json:"results"`
	MissingResults []string                   `json:"missing_results"`
	Cancellation   string                     `json:"cancellation,omitempty"`
}

type config struct {
	Version    int              `yaml:"version"`
	Skills     []skillConfig    `yaml:"skills"`
	MCPServers []serverConfig   `yaml:"mcp_servers"`
	Pipelines  []pipelineConfig `yaml:"pipelines"`
	Execution  struct {
		ServiceAccount string `yaml:"service_account"`
	} `yaml:"execution"`
	Limits limits `yaml:"limits"`
}
type skillConfig struct {
	Path          string   `yaml:"path"`
	RequiredTools []string `yaml:"required_tools"`
}
type serverConfig struct {
	Name           string            `yaml:"name"`
	Transport      string            `yaml:"transport"`
	URL            string            `yaml:"url"`
	Command        string            `yaml:"command"`
	Args           []string          `yaml:"args"`
	Env            map[string]string `yaml:"env"`
	BearerTokenEnv string            `yaml:"bearer_token_env"`
	Tools          []string          `yaml:"tools"`
}
type pipelineConfig struct {
	Name            string               `yaml:"name"`
	Params          map[string]paramRule `yaml:"params"`
	RequiredResults []string             `yaml:"required_results"`
}
type paramRule struct {
	Type      string   `yaml:"type"`
	Required  bool     `yaml:"required"`
	Enum      []string `yaml:"enum"`
	Pattern   string   `yaml:"pattern"`
	MaxLength int      `yaml:"max_length"`
}
type limits struct {
	MaxModelCalls    int `yaml:"max_model_calls"`
	MaxToolCalls     int `yaml:"max_tool_calls"`
	MaxRuns          int `yaml:"max_runs"`
	TimeoutSeconds   int `yaml:"timeout_seconds"`
	MaxEvidenceBytes int `yaml:"max_evidence_bytes"`
}
