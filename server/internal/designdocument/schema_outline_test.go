package designdocument

import (
	"strings"
	"testing"
)

// The outline exists so the agent is told the field names the parser accepts.
// Its whole value is that it is generated: a rename in the struct has to show
// up here without anyone remembering to edit prose.
func TestSchemaOutlineUsesTheWireNames(t *testing.T) {
	outline := SchemaOutline(Brief{})
	for _, want := range []string{
		"schema_version: string",
		"requirement_summary: string",
		"mock_data_scenarios: [{id: string, summary: string}]",
		"non_goals: [string]",
	} {
		if !strings.Contains(outline, want) {
			t.Fatalf("brief outline is missing %q\ngot:\n%s", want, outline)
		}
	}
	// Go names must never leak: an agent copying `RequirementSummary` writes a
	// field the strict decoder rejects.
	if strings.Contains(outline, "RequirementSummary") || strings.Contains(outline, "MockDataScenarios") {
		t.Fatalf("brief outline leaked Go field names:\n%s", outline)
	}
}

// A `?` is the difference between "you may leave this out" and "this name is
// not accepted", and the agent has no other way to tell them apart.
func TestSchemaOutlineMarksOptionalFields(t *testing.T) {
	outline := SchemaOutline(Coverage{})
	if !strings.Contains(outline, "notes?: string") {
		t.Fatalf("an omitempty field is not marked optional:\n%s", outline)
	}
	if !strings.Contains(outline, "design_system_sha256: string") {
		t.Fatalf("a required field was marked optional or dropped:\n%s", outline)
	}
	if strings.Contains(outline, "omitempty") {
		t.Fatalf("the tag option leaked into the outline:\n%s", outline)
	}
}

// Every run without a pinned design system produced a finished package that
// the gate then rejected, because the binding demanded a digest for a system
// that did not exist. Empty means "nothing was pinned" — the same reading the
// base revision digest already had one line below.
func TestValidateBindingAcceptsNoPinnedDesignSystem(t *testing.T) {
	binding := validBinding()
	binding.DesignSystemSHA256 = ""
	if err := validateBinding(binding); err != nil {
		t.Fatalf("an unpinned run was rejected: %v", err)
	}

	binding.DesignSystemSHA256 = "not-a-digest"
	if err := validateBinding(binding); err == nil {
		t.Fatal("a malformed design system digest was accepted")
	}
}
