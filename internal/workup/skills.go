package workup

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strings"

	"google.golang.org/adk/v2/tool/skilltoolset/skill"
)

// skillSnapshot exposes only explicitly selected immutable SKILL.md contents.
type skillSnapshot struct {
	frontmatter  map[string]*skill.Frontmatter
	instructions map[string]string
}

func loadSkills(cfg config, report *Report) (*skillSnapshot, error) {
	s := &skillSnapshot{frontmatter: map[string]*skill.Frontmatter{}, instructions: map[string]string{}}
	total := 0
	grants := map[string]bool{"inspect_pipeline": true}
	for _, server := range cfg.MCPServers {
		for _, name := range server.Tools {
			grants[server.Name+"_"+name] = true
		}
	}
	for _, selected := range cfg.Skills {
		data, err := boundedFile(selected.Path, cfg.Limits.MaxEvidenceBytes)
		if err != nil {
			return nil, fmt.Errorf("required skill: %w", err)
		}
		total += len(data)
		if total > cfg.Limits.MaxEvidenceBytes {
			return nil, errors.New("selected skills exceed evidence limit")
		}
		fm, body, err := skill.ParseBytes(data)
		if err != nil {
			return nil, err
		}
		if filepath.Base(filepath.Dir(selected.Path)) != fm.Name || s.frontmatter[fm.Name] != nil {
			return nil, errors.New("skill name must match directory and be unique")
		}
		for _, name := range append(slices.Clone(selected.RequiredTools), fm.AllowedTools...) {
			if !grants[name] {
				return nil, fmt.Errorf("required skill tool %q lacks operator grant", name)
			}
		}
		report.SkillDigests[fm.Name] = digest(data)
		s.frontmatter[fm.Name], s.instructions[fm.Name] = fm, body
	}
	return s, nil
}

func (s *skillSnapshot) ListFrontmatters(context.Context) ([]*skill.Frontmatter, error) {
	out := make([]*skill.Frontmatter, 0, len(s.frontmatter))
	names := make([]string, 0, len(s.frontmatter))
	for name := range s.frontmatter {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		out = append(out, s.frontmatter[name])
	}
	return out, nil
}
func (s *skillSnapshot) LoadFrontmatter(_ context.Context, name string) (*skill.Frontmatter, error) {
	fm, ok := s.frontmatter[name]
	if !ok {
		return nil, skill.ErrSkillNotFound
	}
	return fm, nil
}
func (s *skillSnapshot) LoadInstructions(_ context.Context, name string) (string, error) {
	body, ok := s.instructions[name]
	if !ok {
		return "", skill.ErrSkillNotFound
	}
	return strings.TrimSpace(body), nil
}
func (s *skillSnapshot) ListResources(context.Context, string, string) ([]string, error) {
	return []string{}, nil
}
func (s *skillSnapshot) LoadResource(context.Context, string, string) (io.ReadCloser, error) {
	return nil, skill.ErrResourceNotFound
}
