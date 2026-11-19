package source

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var parameterReference = regexp.MustCompile(`\$\((params\.[A-Za-z0-9_.-]+)\)`)

// IsStatePath identifies reserved state that cannot be a planning input.
func IsStatePath(name string) bool {
	for _, part := range strings.Split(filepath.ToSlash(name), "/") {
		if strings.EqualFold(part, StateDirectory) {
			return true
		}
	}
	return false
}

func replaceParameters(value string, parameters map[string]string) string {
	return parameterReference.ReplaceAllStringFunc(value, func(token string) string {
		name := strings.TrimSuffix(strings.TrimPrefix(token, "$(params."), ")")
		if replacement, ok := parameters[name]; ok {
			return replacement
		}
		return token
	})
}

func parameterValues(declarations, supplied []parameter, overrides map[string]string) (map[string]string, error) {
	result := make(map[string]string)
	declared := make(map[string]string)
	for _, declaration := range declarations {
		if _, exists := declared[declaration.Name]; exists || declaration.Name == "" {
			return nil, fmt.Errorf("invalid or duplicate parameter declaration %q", declaration.Name)
		}
		declared[declaration.Name] = declaration.Type
		if (declaration.Type == "" || declaration.Type == "string") && !credentialName.MatchString(declaration.Name) &&
			declaration.Default.Kind == yaml.ScalarNode && declaration.Default.Tag == "!!str" {
			result[declaration.Name] = declaration.Default.Value
		}
	}
	seen := make(map[string]bool)
	for _, supplied := range supplied {
		if seen[supplied.Name] {
			return nil, fmt.Errorf("duplicate supplied parameter %q", supplied.Name)
		}
		seen[supplied.Name] = true
		kind, exists := declared[supplied.Name]
		if !exists {
			return nil, fmt.Errorf("undeclared parameter %q", supplied.Name)
		}
		if (kind == "" || kind == "string") && !credentialName.MatchString(supplied.Name) &&
			supplied.Value.Kind == yaml.ScalarNode && supplied.Value.Tag == "!!str" {
			result[supplied.Name] = supplied.Value.Value
		} else {
			delete(result, supplied.Name)
		}
	}
	for name, value := range overrides {
		kind, exists := declared[name]
		if !exists || (kind != "" && kind != "string") || credentialName.MatchString(name) {
			return nil, fmt.Errorf("override requires a declared non-secret string parameter: %q", name)
		}
		result[name] = value
	}
	if len(result) == 0 {
		return nil, nil
	}
	return result, nil
}

func scopedParameters(body taskSpec, supplied []parameter, parent map[string]string) (map[string]string, error) {
	resolved := append([]parameter(nil), supplied...)
	for i := range resolved {
		resolved[i].Value.Value = replaceParameters(resolved[i].Value.Value, parent)
	}
	return parameterValues(body.Params, resolved, nil)
}
