package designdocument

import (
	"sort"
	"strings"
)

// Coverage is the requirement coverage ledger of a design document.
//
// The agent self-assessment it carries is never a pass criterion. The audit
// validates shape and internal consistency only: every reference resolves,
// nothing is declared twice, every declared brief object is accounted for, and
// the design system digest matches the binding. Nothing an agent claims about
// its own work is allowed to turn a failing package into a passing one.
type Coverage struct {
	SchemaVersion string `json:"schema_version"`
	// RequirementCoverage accounts for every non-issue brief requirement.
	RequirementCoverage []CoverageRequirement `json:"requirement_coverage"`
	// IssueRequirementCoverage accounts for every brief requirement that came
	// from the linked task (issue).
	IssueRequirementCoverage []CoverageRequirement   `json:"issue_requirement_coverage"`
	PageCoverage             []CoverageEntry         `json:"page_coverage"`
	StateCoverage            []CoverageEntry         `json:"state_coverage"`
	OverlayCoverage          []CoverageEntry         `json:"overlay_coverage"`
	FlowCoverage             []CoverageEntry         `json:"flow_coverage"`
	InteractionCoverage      []CoverageEntry         `json:"interaction_coverage"`
	DesignSystemConsistency  CoverageDesignSystem    `json:"design_system_consistency"`
	TemplateResidue          CoverageTemplateResidue `json:"template_residue"`
	Uncovered                []CoverageGap           `json:"uncovered"`
	AgentChecks              []CoverageAgentCheck    `json:"agent_checks"`
}

// CoverageRequirement maps one brief requirement onto the pages and states
// that answer it.
type CoverageRequirement struct {
	RequirementID string   `json:"requirement_id"`
	Status        string   `json:"status"`
	PageIDs       []string `json:"page_ids"`
	StateIDs      []string `json:"state_ids"`
	Notes         string   `json:"notes,omitempty"`
}

// CoverageEntry is the coverage status of one declared brief object.
type CoverageEntry struct {
	RefID  string `json:"ref_id"`
	Status string `json:"status"`
	Notes  string `json:"notes,omitempty"`
}

// CoverageDesignSystem records which project saved design system revision the
// prototype was built against. The digest is verified against the binding.
type CoverageDesignSystem struct {
	DesignSystemSHA256 string   `json:"design_system_sha256"`
	Checked            bool     `json:"checked"`
	Findings           []string `json:"findings"`
}

// CoverageTemplateResidue is the agent template residue self-check. The
// platform runs its own residue scan and does not trust this record.
type CoverageTemplateResidue struct {
	Checked  bool     `json:"checked"`
	Findings []string `json:"findings"`
}

// CoverageGap is one declared object the agent reports as not fully covered.
type CoverageGap struct {
	RefID  string `json:"ref_id"`
	Reason string `json:"reason"`
}

// CoverageAgentCheck is an agent declared check result. It is validated for
// shape and then ignored by the verdict.
type CoverageAgentCheck struct {
	ID     string `json:"id"`
	Claim  string `json:"claim"`
	Result string `json:"result"`
}

const coveragePath = "coverage.json"

var coverageStatuses = map[string]struct{}{
	"covered": {}, "partial": {}, "not_covered": {},
}

var coverageAgentResults = map[string]struct{}{
	"pass": {}, "fail": {}, "unknown": {},
}

// auditCoverage decodes coverage.json strictly and checks it against the brief
// semantic index and the package binding.
func auditCoverage(raw []byte, index *briefIndex, binding PackageBinding) []Diagnostic {
	var coverage Coverage
	if err := decodeStrictJSON(raw, &coverage); err != nil {
		return []Diagnostic{errorDiagnostic("coverage_invalid", coveragePath, "coverage is invalid: "+err.Error())}
	}
	diagnostics := make([]Diagnostic, 0)
	if coverage.SchemaVersion != CoverageSchemaV1 {
		diagnostics = append(diagnostics, errorDiagnostic("coverage_schema_invalid", coveragePath, "coverage schema is invalid"))
	}
	if coverage.RequirementCoverage == nil || coverage.IssueRequirementCoverage == nil || coverage.PageCoverage == nil ||
		coverage.StateCoverage == nil || coverage.OverlayCoverage == nil || coverage.FlowCoverage == nil ||
		coverage.InteractionCoverage == nil || coverage.Uncovered == nil || coverage.AgentChecks == nil ||
		coverage.DesignSystemConsistency.Findings == nil || coverage.TemplateResidue.Findings == nil {
		diagnostics = append(diagnostics, errorDiagnostic("coverage_shape_invalid", coveragePath, "coverage arrays must all be present"))
	}
	if index == nil {
		// The brief did not decode, so no coverage reference can be resolved.
		return diagnostics
	}

	statuses := make(map[string]string)
	diagnostics = append(diagnostics, auditCoverageRequirements(coverage, index, statuses)...)
	diagnostics = append(diagnostics, auditCoverageEnumeration("page", coverage.PageCoverage, briefPageIDs(index), statuses)...)
	diagnostics = append(diagnostics, auditCoverageEnumeration("state", coverage.StateCoverage, keySet(index.states), statuses)...)
	diagnostics = append(diagnostics, auditCoverageEnumeration("overlay", coverage.OverlayCoverage, keySet(index.overlays), statuses)...)
	diagnostics = append(diagnostics, auditCoverageEnumeration("flow", coverage.FlowCoverage, index.flows, statuses)...)
	diagnostics = append(diagnostics, auditCoverageEnumeration("interaction", coverage.InteractionCoverage, index.interactions, statuses)...)
	diagnostics = append(diagnostics, auditCoverageGaps(coverage, index, statuses)...)

	if coverage.DesignSystemConsistency.DesignSystemSHA256 != binding.DesignSystemSHA256 {
		diagnostics = append(diagnostics, errorDiagnostic("coverage_design_system_mismatch", coveragePath,
			"coverage does not reference the project saved design system digest of this task"))
	}
	for _, finding := range coverage.DesignSystemConsistency.Findings {
		if strings.TrimSpace(finding) == "" {
			diagnostics = append(diagnostics, errorDiagnostic("coverage_finding_invalid", coveragePath, "coverage findings must be non-empty"))
		}
	}
	for _, finding := range coverage.TemplateResidue.Findings {
		if strings.TrimSpace(finding) == "" {
			diagnostics = append(diagnostics, errorDiagnostic("coverage_finding_invalid", coveragePath, "coverage findings must be non-empty"))
		}
	}

	// Agent declared checks are shape checked and then ignored. A package never
	// passes because an agent said it checked something.
	seenChecks := make(map[string]struct{}, len(coverage.AgentChecks))
	for _, check := range coverage.AgentChecks {
		if !validSemanticID(check.ID) {
			diagnostics = append(diagnostics, errorDiagnostic("coverage_agent_check_invalid", coveragePath, "agent check IDs must be stable semantic IDs"))
			continue
		}
		if _, exists := seenChecks[check.ID]; exists {
			diagnostics = append(diagnostics, errorDiagnostic("coverage_agent_check_duplicate", coveragePath, "agent check ID "+check.ID+" is declared more than once"))
			continue
		}
		seenChecks[check.ID] = struct{}{}
		if strings.TrimSpace(check.Claim) == "" {
			diagnostics = append(diagnostics, errorDiagnostic("coverage_agent_check_invalid", coveragePath, "agent checks require a claim"))
		}
		if _, ok := coverageAgentResults[check.Result]; !ok {
			diagnostics = append(diagnostics, errorDiagnostic("coverage_agent_check_invalid", coveragePath, "agent check result is invalid"))
		}
	}
	return diagnostics
}

func auditCoverageRequirements(coverage Coverage, index *briefIndex, statuses map[string]string) []Diagnostic {
	diagnostics := make([]Diagnostic, 0)
	seen := make(map[string]struct{}, len(index.requirements))
	lists := []struct {
		entries []CoverageRequirement
		issue   bool
	}{
		{entries: coverage.RequirementCoverage, issue: false},
		{entries: coverage.IssueRequirementCoverage, issue: true},
	}
	for _, list := range lists {
		for _, entry := range list.entries {
			requirement, exists := index.requirements[entry.RequirementID]
			if !exists {
				diagnostics = append(diagnostics, errorDiagnostic("coverage_reference_unresolved", coveragePath,
					"coverage references undeclared requirement "+entry.RequirementID))
				continue
			}
			if (requirement.Origin == "issue") != list.issue {
				diagnostics = append(diagnostics, errorDiagnostic("coverage_requirement_list_mismatch", coveragePath,
					"requirement "+entry.RequirementID+" is recorded in the wrong coverage list for its origin"))
			}
			if _, duplicate := seen[entry.RequirementID]; duplicate {
				diagnostics = append(diagnostics, errorDiagnostic("coverage_duplicate", coveragePath,
					"requirement "+entry.RequirementID+" is covered more than once"))
				continue
			}
			seen[entry.RequirementID] = struct{}{}
			if _, ok := coverageStatuses[entry.Status]; !ok {
				diagnostics = append(diagnostics, errorDiagnostic("coverage_status_invalid", coveragePath, "coverage status is invalid"))
			} else {
				statuses[entry.RequirementID] = entry.Status
			}
			if entry.PageIDs == nil || entry.StateIDs == nil {
				diagnostics = append(diagnostics, errorDiagnostic("coverage_shape_invalid", coveragePath, "coverage requirement arrays must all be present"))
			}
			pages := make(map[string]struct{}, len(entry.PageIDs))
			for _, pageID := range entry.PageIDs {
				if _, ok := index.pages[pageID]; !ok {
					diagnostics = append(diagnostics, errorDiagnostic("coverage_reference_unresolved", coveragePath,
						"coverage references undeclared page "+pageID))
					continue
				}
				pages[pageID] = struct{}{}
			}
			for _, stateID := range entry.StateIDs {
				owner, ok := index.states[stateID]
				if !ok {
					diagnostics = append(diagnostics, errorDiagnostic("coverage_reference_unresolved", coveragePath,
						"coverage references undeclared state "+stateID))
					continue
				}
				if _, listed := pages[owner]; !listed {
					diagnostics = append(diagnostics, errorDiagnostic("coverage_state_page_missing", coveragePath,
						"coverage state "+stateID+" belongs to page "+owner+" which the same entry does not list"))
				}
			}
		}
	}
	for _, id := range sortedKeys(index.requirements) {
		if _, exists := seen[id]; !exists {
			diagnostics = append(diagnostics, errorDiagnostic("coverage_requirement_missing", coveragePath,
				"coverage does not account for requirement "+id))
		}
	}
	return diagnostics
}

// auditCoverageEnumeration requires the coverage list to account for every
// declared object of one kind exactly once.
func auditCoverageEnumeration(kind string, entries []CoverageEntry, expected map[string]struct{}, statuses map[string]string) []Diagnostic {
	diagnostics := make([]Diagnostic, 0)
	seen := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		if _, exists := expected[entry.RefID]; !exists {
			diagnostics = append(diagnostics, errorDiagnostic("coverage_reference_unresolved", coveragePath,
				"coverage references undeclared "+kind+" "+entry.RefID))
			continue
		}
		if _, duplicate := seen[entry.RefID]; duplicate {
			diagnostics = append(diagnostics, errorDiagnostic("coverage_duplicate", coveragePath,
				kind+" "+entry.RefID+" is covered more than once"))
			continue
		}
		seen[entry.RefID] = struct{}{}
		if _, ok := coverageStatuses[entry.Status]; !ok {
			diagnostics = append(diagnostics, errorDiagnostic("coverage_status_invalid", coveragePath, "coverage status is invalid"))
			continue
		}
		statuses[entry.RefID] = entry.Status
	}
	for _, id := range sortedSet(expected) {
		if _, exists := seen[id]; !exists {
			diagnostics = append(diagnostics, errorDiagnostic("coverage_"+kind+"_missing", coveragePath,
				"coverage does not account for "+kind+" "+id))
		}
	}
	return diagnostics
}

// auditCoverageGaps keeps the gap list and the status list consistent in both
// directions: every gap points at a declared object that is not fully covered,
// and every partially covered object is listed as a gap with a reason.
func auditCoverageGaps(coverage Coverage, index *briefIndex, statuses map[string]string) []Diagnostic {
	diagnostics := make([]Diagnostic, 0)
	gaps := make(map[string]struct{}, len(coverage.Uncovered))
	for _, gap := range coverage.Uncovered {
		if !index.declared(gap.RefID) {
			diagnostics = append(diagnostics, errorDiagnostic("coverage_reference_unresolved", coveragePath,
				"coverage gap references undeclared ID "+gap.RefID))
			continue
		}
		if _, duplicate := gaps[gap.RefID]; duplicate {
			diagnostics = append(diagnostics, errorDiagnostic("coverage_duplicate", coveragePath,
				"coverage gap "+gap.RefID+" is declared more than once"))
			continue
		}
		gaps[gap.RefID] = struct{}{}
		if strings.TrimSpace(gap.Reason) == "" {
			diagnostics = append(diagnostics, errorDiagnostic("coverage_gap_reason_missing", coveragePath,
				"coverage gap "+gap.RefID+" requires a reason"))
		}
		if statuses[gap.RefID] == "covered" {
			diagnostics = append(diagnostics, errorDiagnostic("coverage_gap_inconsistent", coveragePath,
				"coverage gap "+gap.RefID+" is also reported as fully covered"))
		}
	}
	for _, id := range sortedStatusKeys(statuses) {
		if statuses[id] == "covered" {
			continue
		}
		if _, listed := gaps[id]; !listed {
			diagnostics = append(diagnostics, errorDiagnostic("coverage_gap_missing", coveragePath,
				id+" is not fully covered but is missing from the coverage gap list"))
		}
	}
	return diagnostics
}

func briefPageIDs(index *briefIndex) map[string]struct{} {
	ids := make(map[string]struct{}, len(index.pages))
	for id := range index.pages {
		ids[id] = struct{}{}
	}
	return ids
}

func keySet(values map[string]string) map[string]struct{} {
	ids := make(map[string]struct{}, len(values))
	for id := range values {
		ids[id] = struct{}{}
	}
	return ids
}

func sortedSet(values map[string]struct{}) []string {
	ids := make([]string, 0, len(values))
	for id := range values {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func sortedKeys(values map[string]BriefRequirement) []string {
	ids := make([]string, 0, len(values))
	for id := range values {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func sortedStatusKeys(values map[string]string) []string {
	ids := make([]string, 0, len(values))
	for id := range values {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
