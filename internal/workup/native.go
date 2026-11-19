package workup

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type pipelineSpec struct {
	Params []struct {
		Name    string          `json:"name"`
		Type    string          `json:"type"`
		Default json.RawMessage `json:"default"`
	} `json:"params"`
	Results []struct {
		Name string `json:"name"`
	} `json:"results"`
}
type pipeline struct {
	policy      pipelineConfig
	spec        json.RawMessage
	parsed      pipelineSpec
	description string
}
type nativeObject struct {
	Metadata struct {
		Name            string            `json:"name"`
		UID             string            `json:"uid"`
		ResourceVersion string            `json:"resourceVersion"`
		Labels          map[string]string `json:"labels"`
		Annotations     map[string]string `json:"annotations"`
	} `json:"metadata"`
	Spec   json.RawMessage `json:"spec"`
	Status struct {
		Conditions []struct {
			Type   string `json:"type"`
			Status string `json:"status"`
			Reason string `json:"reason"`
		} `json:"conditions"`
		Results []struct {
			Name  string          `json:"name"`
			Value json.RawMessage `json:"value"`
		} `json:"results"`
	} `json:"status"`
}
type nativeClient struct {
	opts   Options
	limit  int
	owner  string
	binary string
	home   string
}
type capBuffer struct {
	bytes.Buffer
	limit int
}

func (b *capBuffer) Write(p []byte) (int, error) {
	if len(p) > b.limit-b.Len() {
		return 0, errors.New("native output exceeds evidence limit")
	}
	return b.Buffer.Write(p)
}
func (n nativeClient) command(ctx context.Context, args []string, input []byte) ([]byte, error) {
	fixed := []string{"--kubeconfig", n.opts.Kubeconfig, "--context", n.opts.Context, "--namespace", n.opts.Namespace}
	cmd := exec.CommandContext(ctx, n.binary, append(fixed, args...)...)
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "LANG=C", "HOME=" + n.home}
	cmd.Stdin = bytes.NewReader(input)
	cmd.WaitDelay = time.Second
	out := &capBuffer{limit: n.limit}
	cmd.Stdout = out
	cmd.Stderr = &capBuffer{limit: n.limit}
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("native %s: %w", args[0], err)
	}
	return out.Bytes(), nil
}
func (n nativeClient) get(ctx context.Context, kind, name string) (nativeObject, error) {
	data, err := n.command(ctx, []string{"get", kind, name, "-o", "json"}, nil)
	var obj nativeObject
	if err != nil {
		return obj, err
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		return obj, fmt.Errorf("native response: %w", err)
	}
	if obj.Metadata.Name != name || obj.Metadata.UID == "" {
		return obj, errors.New("native response lacks expected identity")
	}
	if kind == "pipelinerun" && obj.Metadata.ResourceVersion == "" {
		return obj, errors.New("native PipelineRun response lacks resourceVersion")
	}
	return obj, nil
}

func (n nativeClient) discover(ctx context.Context, cfg config) (map[string]pipeline, error) {
	out := map[string]pipeline{}
	for _, policy := range cfg.Pipelines {
		obj, err := n.get(ctx, "pipeline", policy.Name)
		if err != nil {
			return nil, err
		}
		var spec pipelineSpec
		if err := json.Unmarshal(obj.Spec, &spec); err != nil {
			return nil, err
		}
		p := pipeline{policy: policy, spec: obj.Spec, parsed: spec, description: obj.Metadata.Annotations["description"]}
		if err := p.validatePolicy(); err != nil {
			return nil, err
		}
		out[policy.Name] = p
	}
	return out, nil
}

func newNative(opts Options, limit int) (nativeClient, error) {
	if !filepath.IsAbs(opts.Kubeconfig) || opts.Context == "" || opts.Namespace == "" ||
		!resourceName.MatchString(opts.Namespace) || strings.HasPrefix(opts.Context, "-") {
		return nativeClient{}, errors.New("explicit kubeconfig, context and namespace are required")
	}
	if _, err := os.Stat(opts.Kubeconfig); err != nil {
		return nativeClient{}, err
	}
	binary, err := exec.LookPath("kubectl")
	if err != nil {
		return nativeClient{}, err
	}
	home, err := os.MkdirTemp("", "tkn-agent-workup-native-")
	if err != nil {
		return nativeClient{}, err
	}
	absoluteHome, err := filepath.Abs(home)
	if err != nil {
		return nativeClient{}, errors.Join(err, os.RemoveAll(home))
	}
	return nativeClient{opts: opts, limit: limit, owner: strings.ToLower(rand.Text()), binary: binary, home: absoluteHome}, nil
}
