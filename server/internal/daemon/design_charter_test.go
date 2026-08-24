package daemon

import (
	"encoding/json"
	"strings"
	"testing"
)

func designDocumentPromptForCharter(t *testing.T) string {
	t.Helper()
	task := Task{
		IssueID: "issue-1",
		DesignDocumentContext: json.RawMessage(
			`{"type":"design_document_task","operation":"generate","execution_ready":true}`),
	}
	return BuildPrompt(task, "opencode")
}

// The two rules below are contracts with code that already exists, not taste.
// Losing either leaves a consumer in the workspace with no producer in the
// prompt — the exact state this charter was added to fix.
func TestDesignCharterCarriesTheWorkspaceMarkupContracts(t *testing.T) {
	prompt := designDocumentPromptForCharter(t)

	// element-descriptor.ts resolves an annotation to an element through these
	// attributes, best-first, before falling back to a positional selector.
	for _, attribute := range []string{"data-page", "data-state", "data-flow", "data-block"} {
		if !strings.Contains(prompt, attribute) {
			t.Fatalf("charter no longer asks for %s; the annotation picker has no stable handle to resolve against", attribute)
		}
	}

	// prototype-canvas.tsx mounts the document with allow-same-origin and
	// WITHOUT allow-scripts, so a page built entirely by JS renders blank there.
	if !strings.Contains(prompt, "scripts DISABLED") {
		t.Fatal("charter no longer warns that the annotation canvas runs without scripts")
	}
}

// The craft standard is the half of design quality no structural check can
// reach: a package can pass Audit and Preview and still be visually flat.
func TestDesignCharterStatesTheCraftStandard(t *testing.T) {
	prompt := designDocumentPromptForCharter(t)

	for _, want := range []string{
		"expert product designer",
		"hierarchy",
		"hover",
		"disabled",
		"Lorem ipsum",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("craft standard missing %q", want)
		}
	}
}

// Ordering is load-bearing: the standard has to frame the stages, not trail
// them as an afterthought the agent reads after it has already planned.
func TestDesignCharterPrecedesTheTaskStages(t *testing.T) {
	prompt := designDocumentPromptForCharter(t)

	charter := strings.Index(prompt, "Craft standard:")
	stages := strings.Index(prompt, "Stages (one Agent session")
	if charter < 0 || stages < 0 {
		t.Fatalf("charter=%d stages=%d, want both present", charter, stages)
	}
	if charter > stages {
		t.Fatal("craft standard is stacked after the stages; it must frame them")
	}
}

// The charter belongs to page design. The design-system prompt has its own
// contract (tokens.css, data-design-node-*) and must not inherit this one.
func TestDesignCharterDoesNotLeakIntoOtherTaskPrompts(t *testing.T) {
	if strings.Contains(buildProjectDesignSystemPrompt(), "Craft standard:") {
		t.Fatal("the page-design craft standard leaked into the design system prompt")
	}
	if strings.Contains(buildQuickCreatePrompt(Task{}), "Craft standard:") {
		t.Fatal("the page-design craft standard leaked into the quick-create prompt")
	}
}
