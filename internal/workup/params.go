package workup

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
)

func (p pipeline) validatePolicy() error {
	declared := map[string]bool{}
	for _, param := range p.parsed.Params {
		rule, ok := p.policy.Params[param.Name]
		typ := param.Type
		if typ == "" {
			typ = "string"
		}
		if !ok || declared[param.Name] || rule.Type != typ {
			return fmt.Errorf("pipeline %s parameter %s lacks matching operator rule", p.policy.Name, param.Name)
		}
		declared[param.Name] = true
		if len(param.Default) != 0 {
			if err := rule.validate(param.Default); err != nil {
				return fmt.Errorf("pipeline default %s: %w", param.Name, err)
			}
		}
	}
	for name := range p.policy.Params {
		if !declared[name] {
			return fmt.Errorf("operator parameter %s absent from Pipeline", name)
		}
	}
	results := map[string]bool{}
	for _, r := range p.parsed.Results {
		results[r.Name] = true
	}
	for _, name := range p.policy.RequiredResults {
		if !results[name] {
			return fmt.Errorf("required result %s absent from Pipeline", name)
		}
	}
	return nil
}

func (p pipeline) plan(args inspectArgs) (Plan, error) {
	params := map[string]json.RawMessage{}
	for name, value := range args.Params {
		rule, ok := p.policy.Params[name]
		if !ok {
			return Plan{}, fmt.Errorf("unauthorized parameter %s", name)
		}
		if err := rule.validate(value); err != nil {
			return Plan{}, fmt.Errorf("parameter %s: %w", name, err)
		}
		params[name] = value
	}
	for _, declared := range p.parsed.Params {
		if _, ok := params[declared.Name]; ok {
			continue
		}
		if p.policy.Params[declared.Name].Required {
			return Plan{}, fmt.Errorf("missing operator-required parameter %s", declared.Name)
		}
		if len(declared.Default) > 0 {
			params[declared.Name] = declared.Default
		} else {
			return Plan{}, fmt.Errorf("missing required parameter %s", declared.Name)
		}
	}
	return Plan{Pipeline: p.policy.Name, Params: params, SpecDigest: digest(p.spec)}, nil
}

func (r paramRule) validate(raw json.RawMessage) error {
	if string(raw) == "null" {
		return errors.New("null parameters are forbidden")
	}
	var values []json.RawMessage
	switch r.Type {
	case "string":
		values = []json.RawMessage{raw}
	case "array":
		if err := json.Unmarshal(raw, &values); err != nil {
			return errors.New("expected string array")
		}
	case "object":
		var object map[string]json.RawMessage
		if err := json.Unmarshal(raw, &object); err != nil {
			return errors.New("expected string object")
		}
		for name, value := range object {
			if len(name) > r.MaxLength {
				return errors.New("object key exceeds limit")
			}
			values = append(values, value)
		}
	default:
		return errors.New("unsupported parameter type")
	}
	if len(values) > r.MaxLength {
		return errors.New("parameter cardinality exceeds limit")
	}
	expression, err := regexp.Compile(r.Pattern)
	if err != nil {
		return err
	}
	for _, scalar := range values {
		var value string
		if string(scalar) == "null" {
			return errors.New("expected non-null string")
		}
		if err := json.Unmarshal(scalar, &value); err != nil {
			return errors.New("expected string")
		}
		if len(value) > r.MaxLength || (len(r.Enum) > 0 && !slices.Contains(r.Enum, value)) ||
			!expression.MatchString(value) {
			return errors.New("value violates operator constraint")
		}
	}
	return nil
}
