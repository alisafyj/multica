package execenv

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// pinHermesTaskModel avoids creating an agent for the shared default model only
// to reconstruct it via ACP set_model. Only an explicitly qualified model on
// the already configured provider is pinned. Cross-provider selection remains
// the runtime's responsibility; credentials and shared configuration never move.
func pinHermesTaskModel(home, requested string) error {
	requested = strings.TrimSpace(requested)
	provider, model, ok := strings.Cut(strings.TrimPrefix(requested, "custom:"), ":")
	if !ok || provider == "" || model == "" {
		return nil
	}
	path := filepath.Join(home, "config.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var doc yaml.Node
	if yaml.Unmarshal(data, &doc) != nil {
		// The normal overlay preserves malformed user configuration verbatim.
		// An optimization must not change that existing runtime error behavior.
		return nil
	}
	top := yamlDocumentRoot(&doc)
	if top == nil {
		return nil
	}
	configured := yamlMapValue(top, "model")
	if configured == nil || configured.Kind != yaml.MappingNode {
		return nil
	}
	currentProvider := yamlMapValue(configured, "provider")
	if currentProvider == nil || currentProvider.Kind != yaml.ScalarNode ||
		strings.TrimPrefix(strings.TrimSpace(currentProvider.Value), "custom:") != provider {
		return nil
	}
	currentModel := yamlMapValue(configured, "default")
	if currentModel != nil && currentModel.Kind == yaml.ScalarNode && currentModel.Value == model {
		return nil
	}
	yamlSetMapValue(configured, "default", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: model})
	return marshalYAMLToFile(&doc, path)
}
