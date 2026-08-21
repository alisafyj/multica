package daemon

import (
	"encoding/json"
	"strings"
	"testing"
)

// The rendered form and the prompt that produces it were written on opposite
// sides of the wire, so the control vocabulary is the thing most likely to
// drift: a type the prompt offers but the parser does not know degrades to a
// text box, silently turning a designed choice into a free-text question.
// packages/core/designs/agent-ui.ts holds the other half of this list.
func TestDesignDocumentPromptOffersOnlyRenderableControls(t *testing.T) {
	renderable := []string{
		"radio", "checkbox", "select", "text", "textarea", "number", "range",
		"date", "time", "datetime-local", "color", "url", "email", "tel",
		"switch", "direction-cards",
	}
	prompt := designDocumentQuestionForm()
	for _, control := range renderable {
		if !strings.Contains(prompt, control) {
			t.Errorf("prompt does not offer the renderable control %q", control)
		}
	}
	// The tag has to be exact: the parser matches on it, and a paraphrase
	// leaves raw markup in the user's prose.
	for _, fragment := range []string{"<question-form", "</question-form>", `id="direction"`} {
		if !strings.Contains(prompt, fragment) {
			t.Errorf("prompt lacks the literal fragment %q", fragment)
		}
	}
}

// A one-shot task cannot receive an answer, so an agent told to "ask first"
// would either stall or ask into the void. This rule is the inversion of
// upstream's and has to survive edits to the surrounding prompt.
func TestDesignDocumentPromptForbidsWaitingForAnAnswer(t *testing.T) {
	prompt := designDocumentQuestionForm()
	if !strings.Contains(prompt, "Never stop and wait for an answer") {
		t.Fatal("prompt does not forbid waiting for an answer")
	}
	if !strings.Contains(prompt, "finish the design under your best assumption") {
		t.Fatal("prompt does not tell the agent to finish before asking")
	}
}

// The example has to be valid JSON in the shape the parser expects, or the
// first thing every agent copies is a block that renders as prose.
func TestDesignDocumentPromptExampleParsesAsAForm(t *testing.T) {
	prompt := designDocumentQuestionForm()
	start := strings.Index(prompt, "{\"questions\"")
	if start == -1 {
		t.Fatal("prompt carries no example form body")
	}
	end := strings.Index(prompt[start:], "\n")
	if end == -1 {
		t.Fatal("example body is not on its own line")
	}
	var payload struct {
		Questions []struct {
			ID      string   `json:"id"`
			Label   string   `json:"label"`
			Type    string   `json:"type"`
			Options []string `json:"options"`
		} `json:"questions"`
	}
	if err := json.Unmarshal([]byte(prompt[start:start+end]), &payload); err != nil {
		t.Fatalf("example form body is not valid JSON: %v", err)
	}
	if len(payload.Questions) == 0 || payload.Questions[0].Label == "" || payload.Questions[0].Type == "" {
		t.Fatalf("example form body is not a usable question: %+v", payload.Questions)
	}
}
