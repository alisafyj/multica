package designdocument

import (
	"sort"
	"strings"
)

// Brief is the lightweight semantic layer of a design document. It expresses
// intent, structure and stable semantic identity; it deliberately does not
// express pixel coordinates, a component tree or CSS implementation.
type Brief struct {
	SchemaVersion      string                  `json:"schema_version"`
	Goal               string                  `json:"goal"`
	RequirementSummary string                  `json:"requirement_summary"`
	Requirements       []BriefRequirement      `json:"requirements"`
	Pages              []BriefPage             `json:"pages"`
	Flows              []BriefFlow             `json:"flows"`
	MockDataScenarios  []BriefMockDataScenario `json:"mock_data_scenarios"`
	Accessibility      []BriefExpectation      `json:"accessibility"`
	Interactions       []BriefExpectation      `json:"interactions"`
	NonGoals           []string                `json:"non_goals"`
}

// BriefRequirement is one requirement the document answers.
type BriefRequirement struct {
	ID      string `json:"id"`
	Summary string `json:"summary"`
	// Origin records where the requirement came from so coverage can separate
	// user requirements from task (issue) requirements.
	Origin string `json:"origin"`
}

// BriefPage is a page or sub page of the design document.
type BriefPage struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	// ParentID is set for a sub page and must reference another declared page.
	ParentID string `json:"parent_id,omitempty"`
	// Entry is the package path of the prototype file that renders this page.
	Entry    string          `json:"entry"`
	States   []BriefState    `json:"states"`
	Overlays []BriefOverlay  `json:"overlays"`
	Blocks   []BriefNamedRef `json:"blocks"`
}

// BriefState is one page state such as loading, empty, error or success.
type BriefState struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Kind  string `json:"kind"`
}

// BriefOverlay is a dialog, drawer, menu or other floating layer of a page.
type BriefOverlay struct {
	ID    string `json:"id"`
	Kind  string `json:"kind"`
	Label string `json:"label"`
}

// BriefNamedRef is a named block or expectation carrying a stable semantic ID.
type BriefNamedRef struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// BriefFlow is a key user flow across pages and states.
type BriefFlow struct {
	ID    string          `json:"id"`
	Title string          `json:"title"`
	Steps []BriefFlowStep `json:"steps"`
}

// BriefFlowStep is one step of a flow. StateID, when present, must be a state
// declared on the referenced page.
type BriefFlowStep struct {
	PageID  string `json:"page_id"`
	StateID string `json:"state_id,omitempty"`
	Action  string `json:"action"`
}

// BriefMockDataScenario is one mock data scenario the prototype demonstrates.
type BriefMockDataScenario struct {
	ID      string `json:"id"`
	Summary string `json:"summary"`
}

// BriefExpectation is one accessibility or key interaction expectation.
type BriefExpectation struct {
	ID      string `json:"id"`
	Summary string `json:"summary"`
}

// briefIndex is the resolved semantic ID space of a brief. Every declared ID
// lives in one namespace, in all, so a coverage reference resolves without a
// kind hint and a duplicate ID is caught wherever it is reused. The narrower
// maps carry the kinds coverage has to enumerate, and states and overlays map
// back to their owning page so a reference can be checked for the right owner.
type briefIndex struct {
	requirements map[string]BriefRequirement
	pages        map[string]BriefPage
	states       map[string]string
	overlays     map[string]string
	flows        map[string]struct{}
	interactions map[string]struct{}
	entries      map[string]string
	all          map[string]struct{}
}

func newBriefIndex() *briefIndex {
	return &briefIndex{
		requirements: make(map[string]BriefRequirement),
		pages:        make(map[string]BriefPage),
		states:       make(map[string]string),
		overlays:     make(map[string]string),
		flows:        make(map[string]struct{}),
		interactions: make(map[string]struct{}),
		entries:      make(map[string]string),
		all:          make(map[string]struct{}),
	}
}

var briefRequirementOrigins = map[string]struct{}{
	"user_input": {}, "issue": {}, "repository": {}, "assumption": {},
}

var briefOverlayKinds = map[string]struct{}{
	"dialog": {}, "drawer": {}, "menu": {}, "popover": {}, "sheet": {}, "toast": {},
}

const briefPath = "brief.json"

// auditBrief decodes brief.json strictly and validates its internal
// consistency. It returns the resolved semantic index so coverage validation
// and the manifest projection can reuse it.
func auditBrief(raw []byte) (Brief, *briefIndex, []Diagnostic) {
	var brief Brief
	if err := decodeStrictJSON(raw, &brief); err != nil {
		return Brief{}, nil, []Diagnostic{errorDiagnostic("brief_invalid", briefPath, "brief is invalid: "+err.Error())}
	}
	diagnostics := make([]Diagnostic, 0)
	if brief.SchemaVersion != BriefSchemaV1 {
		diagnostics = append(diagnostics, errorDiagnostic("brief_schema_invalid", briefPath, "brief schema is invalid"))
	}
	if strings.TrimSpace(brief.Goal) == "" || strings.TrimSpace(brief.RequirementSummary) == "" {
		diagnostics = append(diagnostics, errorDiagnostic("brief_summary_missing", briefPath, "brief requires a goal and a requirement summary"))
	}
	if brief.Requirements == nil || brief.Pages == nil || brief.Flows == nil || brief.MockDataScenarios == nil ||
		brief.Accessibility == nil || brief.Interactions == nil || brief.NonGoals == nil {
		diagnostics = append(diagnostics, errorDiagnostic("brief_shape_invalid", briefPath, "brief arrays must all be present"))
	}
	if len(brief.Requirements) == 0 || len(brief.Pages) == 0 {
		diagnostics = append(diagnostics, errorDiagnostic("brief_empty", briefPath, "brief must declare at least one requirement and one page"))
	}

	index := newBriefIndex()
	for _, requirement := range brief.Requirements {
		diagnostics = append(diagnostics, index.claim(requirement.ID, "requirement")...)
		if strings.TrimSpace(requirement.Summary) == "" {
			diagnostics = append(diagnostics, errorDiagnostic("brief_requirement_invalid", briefPath, "brief requirements require a summary"))
		}
		if _, ok := briefRequirementOrigins[requirement.Origin]; !ok {
			diagnostics = append(diagnostics, errorDiagnostic("brief_requirement_origin_invalid", briefPath, "brief requirement origin is invalid"))
		}
		index.requirements[requirement.ID] = requirement
	}
	for _, page := range brief.Pages {
		diagnostics = append(diagnostics, index.claim(page.ID, "page")...)
		if strings.TrimSpace(page.Title) == "" {
			diagnostics = append(diagnostics, errorDiagnostic("brief_page_invalid", briefPath, "brief pages require a title"))
		}
		if page.States == nil || page.Overlays == nil || page.Blocks == nil {
			diagnostics = append(diagnostics, errorDiagnostic("brief_shape_invalid", briefPath, "brief page arrays must all be present"))
		}
		if !isPrototypeDocumentPath(page.Entry) {
			diagnostics = append(diagnostics, errorDiagnostic("brief_page_entry_invalid", briefPath, "brief page entry must be a prototype HTML path"))
		} else if owner, exists := index.entries[page.Entry]; exists {
			diagnostics = append(diagnostics, errorDiagnostic("brief_page_entry_duplicate", briefPath, "prototype page "+page.Entry+" is claimed by "+owner+" and "+page.ID))
		} else {
			index.entries[page.Entry] = page.ID
		}
		index.pages[page.ID] = page
		for _, state := range page.States {
			diagnostics = append(diagnostics, index.claim(state.ID, "state")...)
			if strings.TrimSpace(state.Label) == "" || strings.TrimSpace(state.Kind) == "" {
				diagnostics = append(diagnostics, errorDiagnostic("brief_state_invalid", briefPath, "brief states require a label and a kind"))
			}
			index.states[state.ID] = page.ID
		}
		for _, overlay := range page.Overlays {
			diagnostics = append(diagnostics, index.claim(overlay.ID, "overlay")...)
			if strings.TrimSpace(overlay.Label) == "" {
				diagnostics = append(diagnostics, errorDiagnostic("brief_overlay_invalid", briefPath, "brief overlays require a label"))
			}
			if _, ok := briefOverlayKinds[overlay.Kind]; !ok {
				diagnostics = append(diagnostics, errorDiagnostic("brief_overlay_kind_invalid", briefPath, "brief overlay kind is invalid"))
			}
			index.overlays[overlay.ID] = page.ID
		}
		for _, block := range page.Blocks {
			diagnostics = append(diagnostics, index.claim(block.ID, "block")...)
			if strings.TrimSpace(block.Label) == "" {
				diagnostics = append(diagnostics, errorDiagnostic("brief_block_invalid", briefPath, "brief named blocks require a label"))
			}
		}
	}
	for _, scenario := range brief.MockDataScenarios {
		diagnostics = append(diagnostics, index.claim(scenario.ID, "mock data scenario")...)
		if strings.TrimSpace(scenario.Summary) == "" {
			diagnostics = append(diagnostics, errorDiagnostic("brief_mock_data_invalid", briefPath, "brief mock data scenarios require a summary"))
		}
	}
	for _, expectation := range brief.Accessibility {
		diagnostics = append(diagnostics, index.claim(expectation.ID, "accessibility expectation")...)
		if strings.TrimSpace(expectation.Summary) == "" {
			diagnostics = append(diagnostics, errorDiagnostic("brief_expectation_invalid", briefPath, "brief expectations require a summary"))
		}
	}
	for _, expectation := range brief.Interactions {
		diagnostics = append(diagnostics, index.claim(expectation.ID, "interaction expectation")...)
		if strings.TrimSpace(expectation.Summary) == "" {
			diagnostics = append(diagnostics, errorDiagnostic("brief_expectation_invalid", briefPath, "brief expectations require a summary"))
		}
		index.interactions[expectation.ID] = struct{}{}
	}
	for _, flow := range brief.Flows {
		diagnostics = append(diagnostics, index.claim(flow.ID, "flow")...)
		if strings.TrimSpace(flow.Title) == "" || len(flow.Steps) == 0 {
			diagnostics = append(diagnostics, errorDiagnostic("brief_flow_invalid", briefPath, "brief flows require a title and at least one step"))
		}
		index.flows[flow.ID] = struct{}{}
	}

	diagnostics = append(diagnostics, auditBriefReferences(brief, index)...)
	for _, goal := range brief.NonGoals {
		if strings.TrimSpace(goal) == "" {
			diagnostics = append(diagnostics, errorDiagnostic("brief_non_goal_invalid", briefPath, "brief non goals must be non-empty"))
		}
	}
	return brief, index, diagnostics
}

// auditBriefReferences resolves every intra-brief reference. Parent pages must
// exist and must not form a cycle, and every flow step must land on a page and
// on a state that page actually declares.
func auditBriefReferences(brief Brief, index *briefIndex) []Diagnostic {
	diagnostics := make([]Diagnostic, 0)
	for _, page := range brief.Pages {
		if page.ParentID == "" {
			continue
		}
		if _, exists := index.pages[page.ParentID]; !exists {
			diagnostics = append(diagnostics, errorDiagnostic("brief_parent_unresolved", briefPath, "brief page parent "+page.ParentID+" is not declared"))
			continue
		}
		if briefParentCycle(index, page) {
			diagnostics = append(diagnostics, errorDiagnostic("brief_parent_cycle", briefPath, "brief page "+page.ID+" is part of a parent cycle"))
		}
	}
	for _, flow := range brief.Flows {
		for _, step := range flow.Steps {
			if strings.TrimSpace(step.Action) == "" {
				diagnostics = append(diagnostics, errorDiagnostic("brief_flow_step_invalid", briefPath, "brief flow steps require an action"))
			}
			if _, exists := index.pages[step.PageID]; !exists {
				diagnostics = append(diagnostics, errorDiagnostic("brief_flow_page_unresolved", briefPath, "brief flow "+flow.ID+" references undeclared page "+step.PageID))
				continue
			}
			if step.StateID == "" {
				continue
			}
			owner, exists := index.states[step.StateID]
			if !exists {
				diagnostics = append(diagnostics, errorDiagnostic("brief_flow_state_unresolved", briefPath, "brief flow "+flow.ID+" references undeclared state "+step.StateID))
				continue
			}
			if owner != step.PageID {
				diagnostics = append(diagnostics, errorDiagnostic("brief_flow_state_mismatch", briefPath, "brief flow state "+step.StateID+" is not declared on page "+step.PageID))
			}
		}
	}
	return diagnostics
}

func briefParentCycle(index *briefIndex, page BriefPage) bool {
	seen := map[string]struct{}{page.ID: {}}
	current := page
	for current.ParentID != "" {
		if _, cycle := seen[current.ParentID]; cycle {
			return true
		}
		seen[current.ParentID] = struct{}{}
		next, exists := index.pages[current.ParentID]
		if !exists {
			return false
		}
		current = next
	}
	return false
}

// claim registers one semantic ID in the single brief ID namespace.
func (index *briefIndex) claim(id, kind string) []Diagnostic {
	if !validSemanticID(id) {
		return []Diagnostic{errorDiagnostic("brief_id_invalid", briefPath, "brief "+kind+" ID is not a stable semantic ID")}
	}
	if _, exists := index.all[id]; exists {
		return []Diagnostic{errorDiagnostic("brief_id_duplicate", briefPath, "brief semantic ID "+id+" is declared more than once")}
	}
	index.all[id] = struct{}{}
	return nil
}

func (index *briefIndex) declared(id string) bool {
	_, exists := index.all[id]
	return exists
}

func briefPageIndex(brief Brief) []PageIndexEntry {
	pages := make([]PageIndexEntry, 0, len(brief.Pages))
	for _, page := range brief.Pages {
		states := make([]string, 0, len(page.States))
		for _, state := range page.States {
			states = append(states, state.ID)
		}
		pages = append(pages, PageIndexEntry{
			ID:       page.ID,
			Title:    page.Title,
			ParentID: page.ParentID,
			Entry:    page.Entry,
			StateIDs: states,
		})
	}
	sort.Slice(pages, func(left, right int) bool { return pages[left].ID < pages[right].ID })
	return pages
}

func briefFlowIndex(brief Brief) []FlowIndexEntry {
	flows := make([]FlowIndexEntry, 0, len(brief.Flows))
	for _, flow := range brief.Flows {
		flows = append(flows, FlowIndexEntry{ID: flow.ID, Title: flow.Title})
	}
	sort.Slice(flows, func(left, right int) bool { return flows[left].ID < flows[right].ID })
	return flows
}
