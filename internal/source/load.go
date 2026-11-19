package source

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Load inventories one PipelineRun or Pipeline and same-file Task/Pipeline refs.
// Unsupported execution features are reported on their steps, never discarded.
func Load(ctx context.Context, request Request) (inventory Inventory, resultErr error) {
	root, filename := request.Root, request.Pipeline
	if err := ctx.Err(); err != nil {
		return Inventory{}, err
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return Inventory{}, err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return Inventory{}, err
	}
	if !filepath.IsAbs(filename) {
		filename = filepath.Join(root, filename)
	}
	filename, err = filepath.EvalSymlinks(filename)
	if err != nil {
		return Inventory{}, err
	}
	relative, err := filepath.Rel(root, filename)
	if err != nil || !filepath.IsLocal(relative) {
		return Inventory{}, errors.New("pipeline must be inside the repository root")
	}
	if IsStatePath(relative) {
		return Inventory{}, errors.New("agent state cannot be used as pipeline source")
	}
	directory, err := os.OpenRoot(root)
	if err != nil {
		return Inventory{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, directory.Close()) }()
	info, err := directory.Lstat(relative)
	if err != nil || !info.Mode().IsRegular() {
		return Inventory{}, errors.New("pipeline source must be a regular file")
	}
	file, err := directory.Open(relative)
	if err != nil {
		return Inventory{}, fmt.Errorf("read pipeline: %w", err)
	}
	data, readErr := io.ReadAll(io.LimitReader(file, (4<<20)+1))
	if err := errors.Join(readErr, file.Close()); err != nil {
		return Inventory{}, fmt.Errorf("read pipeline: %w", err)
	}
	if len(data) > 4<<20 {
		return Inventory{}, errors.New("pipeline exceeds the 4 MiB input limit")
	}
	documents, err := decodeDocuments(data)
	if err != nil {
		return Inventory{}, err
	}
	definitions := make(map[string]yaml.Node)
	var entries []document
	for _, doc := range documents {
		key := doc.Kind + "/" + doc.Metadata.Name
		switch doc.Kind {
		case "Task", "Pipeline":
			if _, exists := definitions[key]; exists {
				return Inventory{}, fmt.Errorf("duplicate definition %s", key)
			}
			definitions[key] = doc.Spec
		}
		if doc.Kind == "PipelineRun" {
			entries = append(entries, doc)
		}
	}
	if len(entries) == 0 {
		for _, doc := range documents {
			if doc.Kind == "Pipeline" {
				entries = append(entries, doc)
			}
		}
	}
	if len(entries) != 1 {
		return Inventory{}, errors.New("name a file containing exactly one PipelineRun or Pipeline")
	}
	doc := entries[0]
	if doc.APIVersion != "tekton.dev/v1" && doc.APIVersion != "tekton.dev/v1beta1" {
		return Inventory{}, fmt.Errorf("unsupported API version %q", doc.APIVersion)
	}
	spec := doc.Spec
	var workspaceBindings []yaml.Node
	var supplied []parameter
	if doc.Kind == "PipelineRun" {
		var run struct {
			PipelineSpec yaml.Node `yaml:"pipelineSpec"`
			PipelineRef  struct {
				Name     string `yaml:"name"`
				Resolver string `yaml:"resolver"`
			} `yaml:"pipelineRef"`
			Workspaces []yaml.Node `yaml:"workspaces"`
			Params     []parameter `yaml:"params"`
		}
		if err := spec.Decode(&run); err != nil {
			return Inventory{}, err
		}
		spec = run.PipelineSpec
		if spec.Kind == 0 && run.PipelineRef.Resolver == "" {
			spec = definitions["Pipeline/"+run.PipelineRef.Name]
		}
		workspaceBindings, supplied = run.Workspaces, run.Params
	}
	sum := sha256.Sum256(data)
	result := Inventory{Root: root, Pipeline: filepath.ToSlash(relative), Hash: hex.EncodeToString(sum[:])}
	if spec.Kind == 0 {
		if request.CheckoutTask != "" {
			return Inventory{}, errors.New("checkout binding needs an available pipeline definition")
		}
		result.Steps = []Step{{ID: "pipeline/reference", Task: "pipeline", Unsupported: "pipeline reference is not available in this file"}}
		return result, nil
	}
	var body pipeline
	if err := spec.Decode(&body); err != nil {
		return Inventory{}, err
	}
	result.Parameters, err = parameterValues(body.Params, supplied, request.Parameters)
	if err != nil {
		return Inventory{}, err
	}
	request.Root = root
	result.Checkout, err = bindCheckout(ctx, request, body, result.Parameters)
	if err != nil {
		return Inventory{}, err
	}
	context := executionContext{definitions: definitions, parameters: result.Parameters, checkout: result.Checkout}
	workspace := "source"
	if result.Checkout != nil {
		workspace = result.Checkout.Workspace
	}
	workspaceNote := ""
	for _, binding := range workspaceBindings {
		var bound struct {
			Name string `yaml:"name"`
		}
		if err := binding.Decode(&bound); err != nil {
			return Inventory{}, err
		}
		if extra := unexpected(binding, "name", "emptyDir", "persistentVolumeClaim", "volumeClaimTemplate"); bound.Name == workspace && extra != "" {
			workspaceNote = "resource workspace binding is not supported: " + extra
		}
	}
	seen := make(map[string]bool)
	for i, node := range append(body.Tasks, body.Finally...) {
		steps, err := taskSteps(node, context, i >= len(body.Tasks))
		if err != nil {
			return Inventory{}, err
		}
		for _, s := range steps {
			if workspaceNote != "" {
				s.Unsupported = workspaceNote
			}
			if seen[s.ID] {
				return Inventory{}, fmt.Errorf("duplicate source step %s", s.ID)
			}
			seen[s.ID] = true
			result.Steps = append(result.Steps, s)
		}
	}
	if len(result.Steps) == 0 {
		return Inventory{}, errors.New("pipeline contains no steps")
	}
	return result, nil
}

func decodeDocuments(data []byte) ([]document, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	var result []document
	for {
		var node yaml.Node
		if err := decoder.Decode(&node); err != nil {
			if errors.Is(err, io.EOF) {
				return result, nil
			}
			return nil, fmt.Errorf("decode pipeline YAML: %w", err)
		}
		if len(node.Content) == 0 {
			continue
		}
		if err := validateNode(&node); err != nil {
			return nil, err
		}
		var doc document
		if err := node.Decode(&doc); err != nil {
			return nil, err
		}
		result = append(result, doc)
	}
}

func validateNode(node *yaml.Node) error {
	if node.Kind == yaml.AliasNode {
		return errors.New("YAML aliases are not supported in the initial local scope")
	}
	if node.Kind == yaml.MappingNode {
		seen := make(map[string]bool)
		for i := 0; i < len(node.Content); i += 2 {
			key := node.Content[i]
			if key.Kind != yaml.ScalarNode || key.Value == "<<" || seen[key.Value] {
				return fmt.Errorf("unsupported or duplicate YAML key at line %d", key.Line)
			}
			seen[key.Value] = true
		}
	}
	for _, child := range node.Content {
		if err := validateNode(child); err != nil {
			return err
		}
	}
	return nil
}
