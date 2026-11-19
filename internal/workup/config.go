package workup

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"

	"gopkg.in/yaml.v3"
)

var resourceName = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)
var toolName = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]*$`)
var envName = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func boundedFile(path string, limit int) (out []byte, resultErr error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, f.Close()) }()
	data, err := io.ReadAll(io.LimitReader(f, int64(limit)+1))
	if err != nil {
		return nil, err
	}
	if len(data) > limit {
		return nil, errors.New("file exceeds evidence limit")
	}
	return data, nil
}

func loadConfig(path string) (config, string, error) {
	var cfg config
	data, err := boundedFile(path, 256<<10)
	if err != nil {
		return cfg, "", err
	}
	d := yaml.NewDecoder(bytes.NewReader(data))
	d.KnownFields(true)
	if err := d.Decode(&cfg); err != nil {
		return cfg, "", fmt.Errorf("config: %w", err)
	}
	if err := d.Decode(new(yaml.Node)); !errors.Is(err, io.EOF) {
		return cfg, "", errors.New("config must contain exactly one document")
	}
	if cfg.Version != 1 || len(cfg.Skills) == 0 || len(cfg.Pipelines) == 0 {
		return cfg, "", errors.New("config version 1, skills and pipelines are required")
	}
	if len(cfg.Skills) > 32 || len(cfg.MCPServers) > 16 || len(cfg.Pipelines) > 32 {
		return cfg, "", errors.New("config exceeds skill/server/Pipeline count limit")
	}
	l := cfg.Limits
	if l.MaxModelCalls < 1 || l.MaxModelCalls > 32 || l.MaxToolCalls < 1 || l.MaxToolCalls > 128 ||
		l.MaxRuns < 1 || l.MaxRuns > 16 || l.TimeoutSeconds < 1 || l.TimeoutSeconds > 900 ||
		l.MaxEvidenceBytes < 1024 || l.MaxEvidenceBytes > 262144 {
		return cfg, "", errors.New("config limits missing or outside hard bounds")
	}
	if !resourceName.MatchString(cfg.Execution.ServiceAccount) {
		return cfg, "", errors.New("trusted execution service_account is required")
	}
	names := map[string]bool{}
	for _, p := range cfg.Pipelines {
		if !resourceName.MatchString(p.Name) || names[p.Name] || len(p.RequiredResults) == 0 {
			return cfg, "", errors.New("pipelines require unique names and required_results")
		}
		names[p.Name] = true
		for name, r := range p.Params {
			if name == "" || !slices.Contains([]string{"string", "array", "object"}, r.Type) ||
				r.MaxLength < 1 || r.MaxLength > l.MaxEvidenceBytes {
				return cfg, "", errors.New("parameter requires type and bounded max_length")
			}
			if _, err := regexp.Compile(r.Pattern); err != nil {
				return cfg, "", err
			}
		}
	}
	names = map[string]bool{}
	exposed := map[string]bool{"inspect_pipeline": true, "list_skills": true, "load_skill": true, "load_skill_resource": true}
	for _, s := range cfg.MCPServers {
		if !toolName.MatchString(s.Name) || names[s.Name] || len(s.Tools) == 0 {
			return cfg, "", errors.New("MCP servers require unique names and explicit tools")
		}
		names[s.Name] = true
		seen := map[string]bool{}
		for _, t := range s.Tools {
			if !toolName.MatchString(t) || seen[t] || exposed[s.Name+"_"+t] {
				return cfg, "", errors.New("invalid or duplicate MCP tool")
			}
			seen[t] = true
			exposed[s.Name+"_"+t] = true
		}
		switch s.Transport {
		case "streamable_http":
			if s.Command != "" || len(s.Env) != 0 || len(s.Args) != 0 {
				return cfg, "", errors.New("HTTP MCP has stdio settings")
			}
		case "stdio":
			if !filepath.IsAbs(s.Command) || s.URL != "" || s.BearerTokenEnv != "" {
				return cfg, "", errors.New("stdio requires absolute command and no HTTP settings")
			}
		default:
			return cfg, "", errors.New("MCP transport must be streamable_http or stdio")
		}
	}
	for _, s := range cfg.Skills {
		if !filepath.IsAbs(s.Path) || filepath.Base(s.Path) != "SKILL.md" {
			return cfg, "", errors.New("skills require absolute SKILL.md paths")
		}
	}
	return cfg, digest(data), nil
}
