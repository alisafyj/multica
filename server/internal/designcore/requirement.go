package designcore

import "encoding/json"

type RequirementCore struct {
	Version      string           `json:"version"`
	Title        string           `json:"title"`
	Summary      string           `json:"summary,omitempty"`
	PageType     string           `json:"pageType"`
	TabKey       string           `json:"tabKey,omitempty"`
	BusinessGoal string           `json:"businessGoal,omitempty"`
	TargetUsers  []string         `json:"targetUsers,omitempty"`
	Entity       KeyLabel         `json:"entity"`
	Fields       []KeyLabel       `json:"fields"`
	Filters      []KeyLabel       `json:"filters,omitempty"`
	TableColumns []KeyLabel       `json:"tableColumns,omitempty"`
	FormFields   []KeyLabel       `json:"formFields,omitempty"`
	Sections     []KeyLabel       `json:"sections,omitempty"`
	Actions      []KeyLabel       `json:"actions,omitempty"`
	States       []string         `json:"states,omitempty"`
	Constraints  []string         `json:"constraints,omitempty"`
	SourceRefs   []map[string]any `json:"sourceRefs,omitempty"`
}

type KeyLabel struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

type PatchOperation struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value any    `json:"value,omitempty"`
}

func ParseRequirementCore(raw []byte) (RequirementCore, error) {
	var requirement RequirementCore
	if err := json.Unmarshal(raw, &requirement); err != nil {
		return RequirementCore{}, err
	}
	return requirement, nil
}

func ParsePatchOperations(raw []byte) ([]PatchOperation, error) {
	var operations []PatchOperation
	if err := json.Unmarshal(raw, &operations); err != nil {
		return nil, err
	}
	return operations, nil
}
