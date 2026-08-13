package projectdesignsystem

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path"
	"strings"
)

const (
	RepositoryDesignContextSchemaVersion = "multica.repository-design-context/v1"
	MaxRepositoryDesignContextBytes      = 256 << 10
	maxRepositoryFacts                   = 80
	maxRepositorySourceFiles             = 100
	maxRepositoryConflicts               = 30
	maxRepositoryRepresentativeWorkflows = 12
	maxRepositoryWorkflowRegions         = 24
	maxRepositoryWorkflowRegionItems     = 40
	maxRepositoryWorkflowRegionAssets    = 20
	maxRepositoryWorkflowGuardrails      = 30
)

type RepositoryDesignFact struct {
	Kind        string   `json:"kind"`
	Label       string   `json:"label"`
	Value       string   `json:"value"`
	SourcePaths []string `json:"source_paths"`
	Confidence  float64  `json:"confidence"`
}

type RepositoryDesignSourceFile struct {
	Path string `json:"path"`
	Kind string `json:"kind"`
}

type RepositoryDesignConflict struct {
	Label          string   `json:"label"`
	RepositoryFact string   `json:"repository_fact"`
	UserIntent     string   `json:"user_intent"`
	SourcePaths    []string `json:"source_paths"`
}

type RepositoryDesignAsset struct {
	Role       string `json:"role"`
	Reference  string `json:"reference"`
	SourcePath string `json:"source_path"`
}

type RepositoryDesignRegion struct {
	Name        string                  `json:"name"`
	Purpose     string                  `json:"purpose"`
	VisibleText []string                `json:"visible_text"`
	Controls    []string                `json:"controls"`
	Behaviors   []string                `json:"behaviors"`
	Conditions  []string                `json:"conditions"`
	Layout      []string                `json:"layout"`
	Appearance  []string                `json:"appearance"`
	Assets      []RepositoryDesignAsset `json:"assets"`
}

type RepositoryDesignWorkflow struct {
	Name        string                   `json:"name"`
	Purpose     string                   `json:"purpose"`
	SourcePaths []string                 `json:"source_paths"`
	Confidence  float64                  `json:"confidence"`
	Regions     []RepositoryDesignRegion `json:"regions"`
	Guardrails  []string                 `json:"guardrails"`
}

type RepositoryDesignContext struct {
	SchemaVersion           string                       `json:"schema_version"`
	Summary                 string                       `json:"summary"`
	SuggestedBrief          string                       `json:"suggested_brief"`
	Facts                   []RepositoryDesignFact       `json:"facts"`
	SourceFiles             []RepositoryDesignSourceFile `json:"source_files"`
	RepresentativeWorkflows []RepositoryDesignWorkflow   `json:"representative_workflows"`
	CommitSHA               string                       `json:"commit_sha,omitempty"`
	Confidence              float64                      `json:"confidence"`
	Conflicts               []RepositoryDesignConflict   `json:"conflicts"`
}

func ValidateRepositoryDesignContext(value RepositoryDesignContext) (RepositoryDesignContext, error) {
	value.SchemaVersion = strings.TrimSpace(value.SchemaVersion)
	value.Summary = strings.TrimSpace(value.Summary)
	value.SuggestedBrief = strings.TrimSpace(value.SuggestedBrief)
	value.CommitSHA = strings.TrimSpace(value.CommitSHA)
	if value.SchemaVersion != RepositoryDesignContextSchemaVersion {
		return RepositoryDesignContext{}, fmt.Errorf("schema_version must be %q", RepositoryDesignContextSchemaVersion)
	}
	if value.Summary == "" || len(value.Summary) > 4000 {
		return RepositoryDesignContext{}, errors.New("summary is required and must not exceed 4000 bytes")
	}
	if len(value.SuggestedBrief) > 8000 {
		return RepositoryDesignContext{}, errors.New("suggested_brief exceeds 8000 bytes")
	}
	if value.Confidence < 0 || value.Confidence > 1 {
		return RepositoryDesignContext{}, errors.New("confidence must be between 0 and 1")
	}
	if value.CommitSHA != "" && !validCommitSHA(value.CommitSHA) {
		return RepositoryDesignContext{}, errors.New("commit_sha must be a hexadecimal Git object id")
	}
	if len(value.Facts) > maxRepositoryFacts {
		return RepositoryDesignContext{}, fmt.Errorf("facts exceed the limit of %d", maxRepositoryFacts)
	}
	for index := range value.Facts {
		fact := &value.Facts[index]
		fact.Kind = strings.TrimSpace(fact.Kind)
		fact.Label = strings.TrimSpace(fact.Label)
		fact.Value = strings.TrimSpace(fact.Value)
		if fact.Kind == "" || len(fact.Kind) > 64 || fact.Label == "" || len(fact.Label) > 160 || fact.Value == "" || len(fact.Value) > 2000 {
			return RepositoryDesignContext{}, fmt.Errorf("facts[%d] has invalid kind, label, or value", index)
		}
		if fact.Confidence < 0 || fact.Confidence > 1 {
			return RepositoryDesignContext{}, fmt.Errorf("facts[%d].confidence must be between 0 and 1", index)
		}
		paths, err := normalizeRepositoryPaths(fact.SourcePaths, 20, true)
		if err != nil {
			return RepositoryDesignContext{}, fmt.Errorf("facts[%d].source_paths: %w", index, err)
		}
		fact.SourcePaths = paths
	}
	if len(value.SourceFiles) > maxRepositorySourceFiles {
		return RepositoryDesignContext{}, fmt.Errorf("source_files exceed the limit of %d", maxRepositorySourceFiles)
	}
	for index := range value.SourceFiles {
		source := &value.SourceFiles[index]
		normalized, err := normalizeRepositoryPath(source.Path)
		if err != nil {
			return RepositoryDesignContext{}, fmt.Errorf("source_files[%d].path: %w", index, err)
		}
		source.Path = normalized
		source.Kind = strings.TrimSpace(source.Kind)
		if source.Kind == "" || len(source.Kind) > 64 {
			return RepositoryDesignContext{}, fmt.Errorf("source_files[%d].kind is required and must not exceed 64 bytes", index)
		}
	}
	if len(value.RepresentativeWorkflows) > maxRepositoryRepresentativeWorkflows {
		return RepositoryDesignContext{}, fmt.Errorf("representative_workflows exceed the limit of %d", maxRepositoryRepresentativeWorkflows)
	}
	for workflowIndex := range value.RepresentativeWorkflows {
		workflow := &value.RepresentativeWorkflows[workflowIndex]
		workflow.Name = strings.TrimSpace(workflow.Name)
		workflow.Purpose = strings.TrimSpace(workflow.Purpose)
		if workflow.Name == "" || len(workflow.Name) > 160 || workflow.Purpose == "" || len(workflow.Purpose) > 2000 {
			return RepositoryDesignContext{}, fmt.Errorf("representative_workflows[%d] has invalid name or purpose", workflowIndex)
		}
		if workflow.Confidence < 0 || workflow.Confidence > 1 {
			return RepositoryDesignContext{}, fmt.Errorf("representative_workflows[%d].confidence must be between 0 and 1", workflowIndex)
		}
		paths, err := normalizeRepositoryPaths(workflow.SourcePaths, 20, true)
		if err != nil {
			return RepositoryDesignContext{}, fmt.Errorf("representative_workflows[%d].source_paths: %w", workflowIndex, err)
		}
		workflow.SourcePaths = paths
		if len(workflow.Regions) == 0 || len(workflow.Regions) > maxRepositoryWorkflowRegions {
			return RepositoryDesignContext{}, fmt.Errorf("representative_workflows[%d].regions must contain between 1 and %d entries", workflowIndex, maxRepositoryWorkflowRegions)
		}
		for regionIndex := range workflow.Regions {
			region := &workflow.Regions[regionIndex]
			region.Name = strings.TrimSpace(region.Name)
			region.Purpose = strings.TrimSpace(region.Purpose)
			if region.Name == "" || len(region.Name) > 160 || len(region.Purpose) > 2000 {
				return RepositoryDesignContext{}, fmt.Errorf("representative_workflows[%d].regions[%d] has invalid name or purpose", workflowIndex, regionIndex)
			}
			for fieldName, field := range map[string]*[]string{
				"visible_text": &region.VisibleText,
				"controls":     &region.Controls,
				"behaviors":    &region.Behaviors,
				"conditions":   &region.Conditions,
				"layout":       &region.Layout,
				"appearance":   &region.Appearance,
			} {
				normalized, err := normalizeRepositoryTextList(*field, maxRepositoryWorkflowRegionItems)
				if err != nil {
					return RepositoryDesignContext{}, fmt.Errorf("representative_workflows[%d].regions[%d].%s: %w", workflowIndex, regionIndex, fieldName, err)
				}
				*field = normalized
			}
			if len(region.Assets) > maxRepositoryWorkflowRegionAssets {
				return RepositoryDesignContext{}, fmt.Errorf("representative_workflows[%d].regions[%d].assets exceed the limit of %d", workflowIndex, regionIndex, maxRepositoryWorkflowRegionAssets)
			}
			for assetIndex := range region.Assets {
				asset := &region.Assets[assetIndex]
				asset.Role = strings.TrimSpace(asset.Role)
				asset.Reference = strings.TrimSpace(asset.Reference)
				if asset.Role == "" || len(asset.Role) > 160 {
					return RepositoryDesignContext{}, fmt.Errorf("representative_workflows[%d].regions[%d].assets[%d].role is required and must not exceed 160 bytes", workflowIndex, regionIndex, assetIndex)
				}
				reference, err := normalizeRepositoryAssetReference(asset.Reference)
				if err != nil {
					return RepositoryDesignContext{}, fmt.Errorf("representative_workflows[%d].regions[%d].assets[%d].reference: %w", workflowIndex, regionIndex, assetIndex, err)
				}
				asset.Reference = reference
				sourcePath, err := normalizeRepositoryPath(asset.SourcePath)
				if err != nil {
					return RepositoryDesignContext{}, fmt.Errorf("representative_workflows[%d].regions[%d].assets[%d].source_path: %w", workflowIndex, regionIndex, assetIndex, err)
				}
				asset.SourcePath = sourcePath
			}
			if region.Assets == nil {
				region.Assets = []RepositoryDesignAsset{}
			}
			if region.Purpose == "" && len(region.VisibleText) == 0 && len(region.Controls) == 0 && len(region.Behaviors) == 0 && len(region.Conditions) == 0 && len(region.Layout) == 0 && len(region.Appearance) == 0 && len(region.Assets) == 0 {
				return RepositoryDesignContext{}, fmt.Errorf("representative_workflows[%d].regions[%d] is empty", workflowIndex, regionIndex)
			}
		}
		guardrails, err := normalizeRepositoryTextList(workflow.Guardrails, maxRepositoryWorkflowGuardrails)
		if err != nil {
			return RepositoryDesignContext{}, fmt.Errorf("representative_workflows[%d].guardrails: %w", workflowIndex, err)
		}
		workflow.Guardrails = guardrails
	}
	if len(value.Conflicts) > maxRepositoryConflicts {
		return RepositoryDesignContext{}, fmt.Errorf("conflicts exceed the limit of %d", maxRepositoryConflicts)
	}
	for index := range value.Conflicts {
		conflict := &value.Conflicts[index]
		conflict.Label = strings.TrimSpace(conflict.Label)
		conflict.RepositoryFact = strings.TrimSpace(conflict.RepositoryFact)
		conflict.UserIntent = strings.TrimSpace(conflict.UserIntent)
		if conflict.Label == "" || len(conflict.Label) > 160 || conflict.RepositoryFact == "" || len(conflict.RepositoryFact) > 2000 || conflict.UserIntent == "" || len(conflict.UserIntent) > 2000 {
			return RepositoryDesignContext{}, fmt.Errorf("conflicts[%d] has invalid label or values", index)
		}
		paths, err := normalizeRepositoryPaths(conflict.SourcePaths, 20, true)
		if err != nil {
			return RepositoryDesignContext{}, fmt.Errorf("conflicts[%d].source_paths: %w", index, err)
		}
		conflict.SourcePaths = paths
	}
	if value.Facts == nil {
		value.Facts = []RepositoryDesignFact{}
	}
	if value.SourceFiles == nil {
		value.SourceFiles = []RepositoryDesignSourceFile{}
	}
	if value.RepresentativeWorkflows == nil {
		value.RepresentativeWorkflows = []RepositoryDesignWorkflow{}
	}
	if value.Conflicts == nil {
		value.Conflicts = []RepositoryDesignConflict{}
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return RepositoryDesignContext{}, err
	}
	if len(encoded) > MaxRepositoryDesignContextBytes {
		return RepositoryDesignContext{}, errors.New("repository design context exceeds its size limit")
	}
	return value, nil
}

func normalizeRepositoryTextList(values []string, limit int) ([]string, error) {
	if len(values) > limit {
		return nil, fmt.Errorf("items exceed the limit of %d", limit)
	}
	normalized := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		item := strings.TrimSpace(value)
		if item == "" || len(item) > 2000 {
			return nil, errors.New("items must be non-empty and must not exceed 2000 bytes")
		}
		if _, exists := seen[item]; exists {
			continue
		}
		seen[item] = struct{}{}
		normalized = append(normalized, item)
	}
	return normalized, nil
}

func normalizeRepositoryPaths(values []string, limit int, required bool) ([]string, error) {
	if required && len(values) == 0 {
		return nil, errors.New("at least one repository-relative path is required")
	}
	if len(values) > limit {
		return nil, fmt.Errorf("paths exceed the limit of %d", limit)
	}
	normalized := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		item, err := normalizeRepositoryPath(value)
		if err != nil {
			return nil, err
		}
		if _, exists := seen[item]; exists {
			continue
		}
		seen[item] = struct{}{}
		normalized = append(normalized, item)
	}
	return normalized, nil
}

func normalizeRepositoryPath(value string) (string, error) {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	if value == "" || len(value) > 512 || strings.HasPrefix(value, "/") || strings.HasPrefix(value, "//") {
		return "", errors.New("path must be a non-empty repository-relative path")
	}
	first := value
	if index := strings.IndexByte(first, '/'); index >= 0 {
		first = first[:index]
	}
	if strings.Contains(first, ":") {
		return "", errors.New("path must not contain an absolute volume prefix")
	}
	cleaned := path.Clean(value)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", errors.New("path escapes the repository root")
	}
	return cleaned, nil
}

func normalizeRepositoryAssetReference(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 2048 {
		return "", errors.New("reference is required and must not exceed 2048 bytes")
	}
	parsed, err := url.Parse(value)
	if err == nil && parsed.IsAbs() {
		if parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
			return "", errors.New("remote reference must be a credential-free HTTPS URL without query or fragment")
		}
		return parsed.String(), nil
	}
	return normalizeRepositoryPath(value)
}

func validCommitSHA(value string) bool {
	if len(value) < 7 || len(value) > 64 {
		return false
	}
	for _, char := range value {
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') && (char < 'A' || char > 'F') {
			return false
		}
	}
	return true
}
