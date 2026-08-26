package designdocument

import "fmt"

// Critique is the agent's own record of the review loop it ran before it
// finished the prototype (DC-050): five lenses, a score per lens per round,
// the findings each round raised and whether they were resolved.
//
// It is a report in the same sense as coverage: the platform validates its
// shape and coherence so the workspace can render it faithfully, and nothing
// in it — no score, no unresolved finding, no outcome — moves the audit
// verdict. A draft is formed by Audit and Preview alone.
type Critique struct {
	SchemaVersion string `json:"schema_version"`
	// Threshold is the per-lens score the loop aimed for.
	Threshold int `json:"threshold"`
	// MaxRounds is the round cap the loop was run with.
	MaxRounds int `json:"max_rounds"`
	// Outcome is how the loop ended: every lens reached the threshold, the
	// cap was hit first, or the loop was not run at all.
	Outcome string          `json:"outcome"`
	Rounds  []CritiqueRound `json:"rounds"`
}

// CritiqueRound is one pass through the five lenses.
type CritiqueRound struct {
	Index    int               `json:"index"`
	Scores   map[string]int    `json:"scores"`
	Findings []CritiqueFinding `json:"findings"`
}

// CritiqueFinding is one thing a lens asked to change.
type CritiqueFinding struct {
	Lens     string `json:"lens"`
	Severity string `json:"severity"`
	Summary  string `json:"summary"`
	Resolved bool   `json:"resolved"`
}

const (
	// CritiqueSchemaV1 identifies the critique report document.
	CritiqueSchemaV1 = "multica.design-document-critique/v1"
	critiquePath     = "critique.json"
	// critiqueMaxRounds mirrors Open Design's round cap.
	critiqueMaxRounds = 10
	critiqueMaxScore  = 10
	// critiqueMaxFindings bounds one round's findings so a report stays a
	// report rather than a transcript dump.
	critiqueMaxFindings = 64
	critiqueMaxSummary  = 1000
)

// critiqueLenses are Open Design's five reviewer roles.
var critiqueLenses = map[string]struct{}{
	"designer": {}, "critic": {}, "brand": {}, "a11y": {}, "copy": {},
}

var critiqueSeverities = map[string]struct{}{
	"must_fix": {}, "should_fix": {}, "note": {},
}

var critiqueOutcomes = map[string]struct{}{
	"passed": {}, "stopped_at_max_rounds": {}, "not_run": {},
}

// auditCritique validates an optional critique.json. It reports shape and
// coherence problems only; the numbers inside are the agent's and are never
// judged here.
func auditCritique(raw []byte) []Diagnostic {
	var critique Critique
	if err := decodeStrictJSON(raw, &critique); err != nil {
		return []Diagnostic{errorDiagnostic("critique_invalid", critiquePath, "critique.json is not a valid critique document: "+err.Error())}
	}
	if critique.SchemaVersion != CritiqueSchemaV1 {
		return []Diagnostic{errorDiagnostic("critique_schema_invalid", critiquePath, "critique.json schema is not "+CritiqueSchemaV1)}
	}
	diagnostics := make([]Diagnostic, 0)
	if critique.Threshold < 0 || critique.Threshold > critiqueMaxScore {
		diagnostics = append(diagnostics, errorDiagnostic("critique_score_invalid", critiquePath, "critique threshold must be between 0 and 10"))
	}
	if critique.MaxRounds < 1 || critique.MaxRounds > critiqueMaxRounds {
		diagnostics = append(diagnostics, errorDiagnostic("critique_invalid", critiquePath, "critique max_rounds must be between 1 and 10"))
	}
	if _, ok := critiqueOutcomes[critique.Outcome]; !ok {
		diagnostics = append(diagnostics, errorDiagnostic("critique_outcome_invalid", critiquePath, "critique outcome must be passed, stopped_at_max_rounds or not_run"))
	}
	if len(critique.Rounds) == 0 || len(critique.Rounds) > critiqueMaxRounds {
		diagnostics = append(diagnostics, errorDiagnostic("critique_invalid", critiquePath, "critique must record between 1 and 10 rounds"))
		return diagnostics
	}
	if critique.MaxRounds >= 1 && len(critique.Rounds) > critique.MaxRounds {
		diagnostics = append(diagnostics, errorDiagnostic("critique_invalid", critiquePath, "critique records more rounds than its max_rounds"))
	}
	for position, round := range critique.Rounds {
		if round.Index != position+1 {
			diagnostics = append(diagnostics, errorDiagnostic("critique_invalid", critiquePath, fmt.Sprintf("critique round %d must carry index %d", position+1, position+1)))
		}
		if len(round.Scores) != len(critiqueLenses) {
			diagnostics = append(diagnostics, errorDiagnostic("critique_lens_invalid", critiquePath, fmt.Sprintf("critique round %d must score exactly the five lenses", round.Index)))
		}
		for lens, score := range round.Scores {
			if _, ok := critiqueLenses[lens]; !ok {
				diagnostics = append(diagnostics, errorDiagnostic("critique_lens_invalid", critiquePath, "critique lens "+lens+" is not one of designer, critic, brand, a11y, copy"))
			}
			if score < 0 || score > critiqueMaxScore {
				diagnostics = append(diagnostics, errorDiagnostic("critique_score_invalid", critiquePath, "critique scores must be between 0 and 10"))
			}
		}
		if len(round.Findings) > critiqueMaxFindings {
			diagnostics = append(diagnostics, errorDiagnostic("critique_invalid", critiquePath, fmt.Sprintf("critique round %d records too many findings", round.Index)))
		}
		for _, finding := range round.Findings {
			if _, ok := critiqueLenses[finding.Lens]; !ok {
				diagnostics = append(diagnostics, errorDiagnostic("critique_lens_invalid", critiquePath, "critique finding lens "+finding.Lens+" is not one of designer, critic, brand, a11y, copy"))
			}
			if _, ok := critiqueSeverities[finding.Severity]; !ok {
				diagnostics = append(diagnostics, errorDiagnostic("critique_severity_invalid", critiquePath, "critique finding severity must be must_fix, should_fix or note"))
			}
			if finding.Summary == "" || len(finding.Summary) > critiqueMaxSummary {
				diagnostics = append(diagnostics, errorDiagnostic("critique_invalid", critiquePath, "critique findings need a summary of at most 1000 bytes"))
			}
		}
	}
	return diagnostics
}
