package workup

import (
	"encoding/json"
	"errors"
	"strings"
	"sync"

	adkagent "google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/tool"
	"google.golang.org/genai"
)

type accounting struct {
	report   *Report
	limits   limits
	secrets  []string
	mu       *sync.Mutex
	mcpTools map[string]string
	failure  error
}

func redactText(s string, secrets []string) string {
	for _, secret := range secrets {
		if secret != "" {
			s = strings.ReplaceAll(s, secret, "[REDACTED]")
		}
	}
	return s
}
func redactJSON(data []byte, secrets []string) []byte {
	s := string(data)
	for _, secret := range secrets {
		if secret == "" {
			continue
		}
		escaped, _ := json.Marshal(secret)
		s = strings.ReplaceAll(s, string(escaped[1:len(escaped)-1]), "[REDACTED]")
	}
	return []byte(s)
}

func (a *accounting) beforeModel(_ adkagent.Context, request *model.LLMRequest) (*model.LLMResponse, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.failure != nil {
		return nil, a.failure
	}
	// Bound instructions, tool schemas and history together, after all toolset
	// processors have added their content. Tools are excluded by the SDK tag.
	data, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	if len(data) > a.limits.MaxEvidenceBytes {
		return nil, errors.New("model context exceeds evidence limit")
	}
	// Decode into fresh storage: schema objects in Config are shared with
	// functiontool's compiled validators and must never be unmarshaled into.
	var clean model.LLMRequest
	if err := json.Unmarshal(redactJSON(data, a.secrets), &clean); err != nil {
		return nil, err
	}
	clean.Tools = request.Tools
	*request = clean
	if a.report.Usage.ModelCalls >= a.limits.MaxModelCalls {
		return nil, errors.New("model call limit reached")
	}
	a.report.Usage.ModelCalls++
	return nil, nil
}
func (a *accounting) afterModel(_ adkagent.Context, response *model.LLMResponse, err error) (*model.LLMResponse, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err != nil {
		return nil, err
	}
	if response != nil && response.UsageMetadata != nil {
		u := response.UsageMetadata
		a.report.Usage.InputTokens += int64(u.PromptTokenCount)
		a.report.Usage.OutputTokens += int64(u.CandidatesTokenCount)
		a.report.Usage.TotalTokens += int64(u.TotalTokenCount)
	}
	if response != nil && (response.ErrorCode != "" || response.Interrupted ||
		(response.FinishReason != "" && response.FinishReason != genai.FinishReasonStop)) {
		return nil, errors.New("model response interrupted, failed or incomplete")
	}
	return nil, nil
}
func (a *accounting) beforeTool(_ adkagent.Context, t tool.Tool, args map[string]any) (map[string]any, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.failure != nil {
		return nil, a.failure
	}
	if a.report.Usage.ToolCalls >= a.limits.MaxToolCalls {
		a.failure = errors.New("tool call limit reached")
		return nil, a.failure
	}
	a.report.Usage.ToolCalls++
	if t.Name() == "inspect_pipeline" {
		for name := range args {
			if name != "pipeline" && name != "params" {
				a.failure = errors.New("inspect_pipeline accepts only pipeline and params")
				return nil, a.failure
			}
		}
	}
	data, err := json.Marshal(args)
	if err != nil {
		return nil, err
	}
	if len(data) > a.limits.MaxEvidenceBytes {
		a.failure = errors.New("tool arguments exceed evidence limit")
		return nil, a.failure
	}
	// Never transmit credential values even if a model echoes one as an arg.
	for _, secret := range a.secrets {
		if secret != "" && strings.Contains(string(data), string(mustJSONString(secret))) {
			a.failure = errors.New("tool arguments contain configured credentials")
			return nil, a.failure
		}
	}
	return nil, nil
}
func mustJSONString(s string) []byte {
	data, _ := json.Marshal(s)
	return data[1 : len(data)-1]
}
func (a *accounting) afterTool(_ adkagent.Context, t tool.Tool, args, result map[string]any, callErr error) (map[string]any, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if callErr != nil {
		message := redactText(callErr.Error(), a.secrets)
		if len(message) > 4096 {
			message = message[:4096] + " [error truncated; evidence incomplete]"
		}
		callErr = errors.New(message)
	}
	data, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	if server, ok := a.mcpTools[t.Name()]; ok {
		input, err := json.Marshal(args)
		if err != nil {
			return nil, err
		}
		receipt := MCPReceipt{Server: server, Tool: strings.TrimPrefix(t.Name(), server+"_"),
			ArgumentsDigest: digest(redactJSON(input, a.secrets)), ResultDigest: digest(redactJSON(data, a.secrets)), Success: callErr == nil}
		if callErr != nil {
			receipt.Error = redactText(callErr.Error(), a.secrets)
		}
		a.report.MCPCalls = append(a.report.MCPCalls, receipt)
		// Pharos can complete MCP successfully while returning an API denial.
		// Keep protocol success in its receipt, but never count denied evidence
		// as present even if the model later emits an empty missing list.
		if callErr == nil && strings.Contains(string(data), "Kubernetes API Error:") {
			a.report.MissingEvidence = append(a.report.MissingEvidence,
				server+"/"+receipt.Tool+": Kubernetes API error prevented evidence collection")
			result["evidence_available"] = false
			result["evidence_error"] = "Kubernetes API error prevented evidence collection"
			data, err = json.Marshal(result)
			if err != nil {
				return nil, err
			}
		}
	}
	if callErr != nil {
		a.failure = callErr
		return nil, errors.New(redactText(callErr.Error(), a.secrets))
	}
	if len(data) > a.limits.MaxEvidenceBytes {
		a.failure = errors.New("tool result exceeds evidence limit")
		return nil, a.failure
	}
	var clean map[string]any
	if err := json.Unmarshal(redactJSON(data, a.secrets), &clean); err != nil {
		return nil, err
	}
	return clean, nil
}
