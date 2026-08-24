package daemon

import (
	"encoding/json"
	"strings"
	"testing"
)

func designDocumentPromptForCharter(t *testing.T) string {
	t.Helper()
	return designDocumentPromptWithRecipe(t, "")
}

func designDocumentPromptWithRecipe(t *testing.T, recipe string) string {
	t.Helper()
	context := `{"type":"design_document_task","operation":"generate","execution_ready":true`
	if recipe != "" {
		context += `,"recipe":"` + recipe + `"`
	}
	context += `}`
	return BuildPrompt(Task{IssueID: "issue-1", DesignDocumentContext: json.RawMessage(context)}, "opencode")
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

// The boundary only holds if it outranks what follows, so its position is part
// of the contract, not a formatting preference.
func TestUntrustedInputGuardLeadsTheDesignDocumentPrompt(t *testing.T) {
	prompt := designDocumentPromptForCharter(t)

	guard := strings.Index(prompt, "Input boundary")
	role := strings.Index(prompt, "You are running as a product page designer")
	if guard != 0 {
		t.Fatalf("input boundary starts at %d, want the very top of the prompt", guard)
	}
	if role < guard {
		t.Fatal("the role line precedes the input boundary; the boundary must outrank everything after it")
	}
	// Naming the locations is what makes the rule actionable — "be careful
	// with untrusted input" is advice a model cannot act on.
	for _, want := range []string{"Repository files", "reference attachments", "issue", "base/"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("input boundary no longer names %q as untrusted content", want)
		}
	}
}

// The daemon normalises every backend's plan mechanism into one `todo_write`
// message and the workspace renders it as a checklist. Without this
// instruction that renderer has no upstream.
func TestDesignDocumentPromptAsksForAPlan(t *testing.T) {
	prompt := designDocumentPromptForCharter(t)

	if !strings.Contains(prompt, "Keep a working plan") {
		t.Fatal("nothing asks the agent for a plan; the workspace checklist would never receive one")
	}
	for _, tool := range []string{"update_plan", "TodoWrite"} {
		if !strings.Contains(prompt, tool) {
			t.Fatalf("plan instruction does not name %s, so a backend that only has that tool may not use it", tool)
		}
	}
}

// Picking a scenario chip used to change one string in the request and nothing
// the agent could act on: wireframe and high-fidelity mockup produced the same
// instructions.
func TestBuiltinRecipesCarryDistinctWorkflows(t *testing.T) {
	wireframe := designDocumentPromptWithRecipe(t, "wireframe")
	mockup := designDocumentPromptWithRecipe(t, "ui-mockup")

	if !strings.Contains(wireframe, "Recipe: wireframe") {
		t.Fatal("the wireframe chip contributes no workflow")
	}
	if !strings.Contains(mockup, "Recipe: high-fidelity UI mockup") {
		t.Fatal("the ui-mockup chip contributes no workflow")
	}
	if wireframe == mockup {
		t.Fatal("wireframe and ui-mockup produce identical prompts; the recipe is still only a label")
	}
	// The recipe shapes the stages, so it has to be read before them.
	if strings.Index(wireframe, "Recipe:") > strings.Index(wireframe, "Stages (one Agent session") {
		t.Fatal("the recipe body trails the stages it is supposed to shape")
	}
}

// A community recipe's body is already in the brief the user reviewed and
// could edit. Emitting a second one here would fight that wording.
func TestUnknownAndDefaultRecipesContributeNoBody(t *testing.T) {
	for _, recipe := range []string{"", "default", "dashboard"} {
		prompt := designDocumentPromptWithRecipe(t, recipe)
		if strings.Contains(prompt, "Recipe:\n") {
			t.Fatalf("recipe %q emitted a body; built-in bodies must not cover community slugs", recipe)
		}
	}
}
