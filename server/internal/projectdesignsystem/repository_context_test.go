package projectdesignsystem

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestValidateRepositoryDesignContextAcceptsAbbreviatedCommitAndNormalizesPaths(t *testing.T) {
	got, err := ValidateRepositoryDesignContext(RepositoryDesignContext{
		SchemaVersion: RepositoryDesignContextSchemaVersion,
		Summary:       "Existing design evidence.",
		Facts: []RepositoryDesignFact{{
			Kind: "framework", Label: "Framework", Value: "React",
			SourcePaths: []string{`packages\\web\\package.json`}, Confidence: 0.9,
		}},
		SourceFiles: []RepositoryDesignSourceFile{{Path: "DESIGN.md", Kind: "design_document"}},
		CommitSHA:   "abcdef1",
		Confidence:  0.8,
	})
	if err != nil {
		t.Fatalf("ValidateRepositoryDesignContext() error = %v", err)
	}
	if got.Facts[0].SourcePaths[0] != "packages/web/package.json" {
		t.Fatalf("normalized source path = %q", got.Facts[0].SourcePaths[0])
	}
}

func TestValidateRepositoryDesignContextRejectsAbsoluteSourcePath(t *testing.T) {
	_, err := ValidateRepositoryDesignContext(RepositoryDesignContext{
		SchemaVersion: RepositoryDesignContextSchemaVersion,
		Summary:       "Existing design evidence.",
		Facts: []RepositoryDesignFact{{
			Kind: "framework", Label: "Framework", Value: "React",
			SourcePaths: []string{"/Users/person/project/package.json"}, Confidence: 0.9,
		}},
		Confidence: 0.8,
	})
	if err == nil {
		t.Fatal("absolute source path was accepted")
	}
}

func TestValidateRepositoryDesignContextPreservesRepresentativeWorkflowContract(t *testing.T) {
	got, err := ValidateRepositoryDesignContext(RepositoryDesignContext{
		SchemaVersion: RepositoryDesignContextSchemaVersion,
		Summary:       "Existing CRM design evidence.",
		Confidence:    0.95,
		RepresentativeWorkflows: []RepositoryDesignWorkflow{{
			Name:        " Customer list management ",
			Purpose:     " Find, filter, and operate on customer records. ",
			SourcePaths: []string{`src\views\customer\list.vue`, "src/views/customer/tableConfig.js"},
			Confidence:  0.98,
			Regions: []RepositoryDesignRegion{
				{
					Name:        " search_and_scope ",
					Purpose:     "Search and select a data scope.",
					VisibleText: []string{" Search customers ", "Advanced filters"},
					Controls:    []string{"keyword input", "data-scope switch"},
					Behaviors:   []string{"Search submits the current keyword."},
					Conditions:  []string{"Advanced conditions appear only after the drawer opens."},
				},
				{
					Name:        "business_metrics",
					VisibleText: []string{"Estimated monthly visit rate", "Estimated monthly amount"},
				},
				{
					Name:       "customer_table",
					Controls:   []string{"row selection", "visit switch", "inline amount editor"},
					Behaviors:  []string{"Editing an amount persists the row value."},
					Conditions: []string{"Inline amount editing is available only for eligible rows."},
				},
			},
			Guardrails: []string{
				"Do not invent a create-customer action.",
				"Do not add generic dashboard metrics.",
			},
		}},
	})
	if err != nil {
		t.Fatalf("ValidateRepositoryDesignContext() error = %v", err)
	}
	if len(got.RepresentativeWorkflows) != 1 {
		t.Fatalf("representative workflows = %+v", got.RepresentativeWorkflows)
	}
	workflow := got.RepresentativeWorkflows[0]
	if workflow.Name != "Customer list management" || workflow.SourcePaths[0] != "src/views/customer/list.vue" {
		t.Fatalf("normalized workflow = %+v", workflow)
	}
	if len(workflow.Regions) != 3 || workflow.Regions[0].Name != "search_and_scope" || workflow.Regions[1].Name != "business_metrics" || workflow.Regions[2].Name != "customer_table" {
		t.Fatalf("ordered workflow regions = %+v", workflow.Regions)
	}
	first := workflow.Regions[0]
	if first.VisibleText[0] != "Search customers" || first.Controls[1] != "data-scope switch" || len(first.Behaviors) != 1 || len(first.Conditions) != 1 {
		t.Fatalf("workflow region details = %+v", first)
	}
	if len(workflow.Guardrails) != 2 || workflow.Guardrails[0] != "Do not invent a create-customer action." {
		t.Fatalf("workflow guardrails = %+v", workflow.Guardrails)
	}
}

func TestValidateRepositoryDesignContextPreservesRegionVisualFidelityContract(t *testing.T) {
	var input RepositoryDesignContext
	if err := json.Unmarshal([]byte(`{
		"schema_version":"multica.repository-design-context/v1",
		"summary":"Existing CRM design evidence.",
		"confidence":0.95,
		"representative_workflows":[{
			"name":"Customer list management",
			"purpose":"Find and operate on customer records.",
			"source_paths":["src/views/customer/list.vue"],
			"confidence":0.98,
			"regions":[{
				"name":"business_metrics",
				"purpose":"Show configured business estimates.",
				"visible_text":["Estimated monthly visit rate"],
				"controls":[],
				"behaviors":[],
				"conditions":[],
				"layout":["Two 240px by 80px cards in a horizontal row with a 16px gap."],
				"appearance":["Each card uses a subtle gray surface, 8px radius, and a 42px leading icon."],
				"assets":[{
					"role":"Estimated visit metric icon",
					"reference":"https://static.soyoung.com/sy-pre/visit.png",
					"source_path":"src/views/customer/list.vue"
				}]
			}],
			"guardrails":[]
		}]
	}`), &input); err != nil {
		t.Fatalf("decode repository design context: %v", err)
	}

	got, err := ValidateRepositoryDesignContext(input)
	if err != nil {
		t.Fatalf("ValidateRepositoryDesignContext() error = %v", err)
	}
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal validated context: %v", err)
	}
	for _, want := range []string{`"layout"`, `"appearance"`, `"assets"`, `"source_path"`, `"https://static.soyoung.com/sy-pre/visit.png"`} {
		if !strings.Contains(string(encoded), want) {
			t.Fatalf("validated visual fidelity contract lost %s: %s", want, encoded)
		}
	}
}

func TestValidateRepositoryDesignContextRejectsUnsafeRegionAssetReference(t *testing.T) {
	var input RepositoryDesignContext
	if err := json.Unmarshal([]byte(`{
		"schema_version":"multica.repository-design-context/v1",
		"summary":"Existing CRM design evidence.",
		"confidence":0.95,
		"representative_workflows":[{
			"name":"Customer list management",
			"purpose":"Find and operate on customer records.",
			"source_paths":["src/views/customer/list.vue"],
			"confidence":0.98,
			"regions":[{
				"name":"business_metrics",
				"visible_text":["Estimated monthly visit rate"],
				"assets":[{
					"role":"Estimated visit metric icon",
					"reference":"http://example.com/visit.png",
					"source_path":"src/views/customer/list.vue"
				}]
			}],
			"guardrails":[]
		}]
	}`), &input); err != nil {
		t.Fatalf("decode repository design context: %v", err)
	}

	if _, err := ValidateRepositoryDesignContext(input); err == nil {
		t.Fatal("unsafe HTTP asset reference was accepted")
	}
}

func TestValidateRepositoryDesignContextRejectsInvalidRepresentativeWorkflows(t *testing.T) {
	tests := []struct {
		name      string
		workflows []RepositoryDesignWorkflow
	}{
		{
			name: "missing workflow name",
			workflows: []RepositoryDesignWorkflow{{
				SourcePaths: []string{"src/views/customer/list.vue"},
				Confidence:  0.9,
				Regions:     []RepositoryDesignRegion{{Name: "filters", Controls: []string{"input"}}},
			}},
		},
		{
			name: "workflow without regions",
			workflows: []RepositoryDesignWorkflow{{
				Name: "Customer list", SourcePaths: []string{"src/views/customer/list.vue"}, Confidence: 0.9,
			}},
		},
		{
			name: "empty region",
			workflows: []RepositoryDesignWorkflow{{
				Name: "Customer list", SourcePaths: []string{"src/views/customer/list.vue"}, Confidence: 0.9,
				Regions: []RepositoryDesignRegion{{Name: "filters"}},
			}},
		},
		{
			name: "absolute workflow source path",
			workflows: []RepositoryDesignWorkflow{{
				Name: "Customer list", SourcePaths: []string{"/Users/person/project/src/views/customer/list.vue"}, Confidence: 0.9,
				Regions: []RepositoryDesignRegion{{Name: "filters", Controls: []string{"input"}}},
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidateRepositoryDesignContext(RepositoryDesignContext{
				SchemaVersion:           RepositoryDesignContextSchemaVersion,
				Summary:                 "Existing CRM design evidence.",
				Confidence:              0.9,
				RepresentativeWorkflows: tt.workflows,
			})
			if err == nil {
				t.Fatal("invalid representative workflow was accepted")
			}
		})
	}
}
