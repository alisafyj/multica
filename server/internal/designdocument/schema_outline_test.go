package designdocument

import (
	"reflect"
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

// The enum tags exist so the prompt can name a field's allowed values instead
// of calling it "string" — a run wrote "task.json brief" for a requirement's
// origin, which describes where the requirement came from perfectly and is not
// one of the four words the audit accepts.
//
// Their whole value is being the same values the audit enforces, so this holds
// each tag to the map it stands for. Adding a case to one and not the other
// fails here rather than at the gate, on someone's finished package.
func TestSchemaOutlineEnumsMatchTheValidators(t *testing.T) {
	for name, tc := range map[string]struct {
		value   any
		field   string
		allowed map[string]struct{}
	}{
		"brief requirement origin": {Brief{}, "", briefRequirementOrigins},
		"brief overlay kind":       {BriefOverlay{}, "Kind", briefOverlayKinds},
		"coverage entry status":    {CoverageEntry{}, "Status", coverageStatuses},
		"coverage requirement status": {
			CoverageRequirement{}, "Status", coverageStatuses,
		},
		"agent check result": {CoverageAgentCheck{}, "Result", coverageAgentResults},
	} {
		t.Run(name, func(t *testing.T) {
			value, field := tc.value, tc.field
			if field == "" {
				value, field = BriefRequirement{}, "Origin"
			}
			structField, ok := reflect.TypeOf(value).FieldByName(field)
			if !ok {
				t.Fatalf("%T has no field %s", value, field)
			}
			tag := structField.Tag.Get("enum")
			if tag == "" {
				t.Fatalf("%T.%s has no enum tag, so the prompt calls it a bare string", value, field)
			}
			tagged := map[string]struct{}{}
			for _, item := range strings.Split(tag, ",") {
				tagged[strings.TrimSpace(item)] = struct{}{}
			}
			if len(tagged) != len(tc.allowed) {
				t.Fatalf("enum tag %q has %d values, the validator accepts %d", tag, len(tagged), len(tc.allowed))
			}
			for want := range tc.allowed {
				if _, ok := tagged[want]; !ok {
					t.Fatalf("the validator accepts %q but the enum tag does not list it: %q", want, tag)
				}
			}
		})
	}
}

// The rendered form has to be readable as a choice, not as a type.
func TestSchemaOutlineRendersAllowedValues(t *testing.T) {
	outline := SchemaOutline(Brief{})
	if !strings.Contains(outline, `origin: "user_input" | "issue" | "repository" | "assumption"`) {
		t.Fatalf("the brief outline does not spell out the origins:\n%s", outline)
	}
	if strings.Contains(outline, "origin: string") {
		t.Fatalf("the brief outline still calls a closed set a string:\n%s", outline)
	}
}
