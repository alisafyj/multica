package daemon

import "strings"

// designerCharter is the craft standard a page design is held to, stacked
// above the task contract in buildDesignDocumentPrompt.
//
// The rest of that prompt says what to produce and what is forbidden: the
// package layout, the offline rule, no delegation, the immutable base. None of
// it says what SEPARATES a good page from a page that merely satisfies the
// contract. A capable agent handed only a contract produces something that
// passes Audit and Preview and still looks like a template — which is the gap
// this charter exists to close, and the reason the platform cannot outsource
// design quality to whichever agent happens to be bound to the run.
//
// Open Design carries the same layer as a designer identity charter stacked
// under every generation. This is that idea implemented natively (DC-047's
// "behaviour baseline, native implementation"), not a port of their text: the
// rules below are the ones that hold in THIS product, where the deliverable is
// a versioned package rendered in three different frames rather than loose
// HTML in a chat workspace.
func designerCharter() string {
	var b strings.Builder

	b.WriteString("Craft standard:\n")

	// Identity first. "Expert designer" is not flattery — it sets which
	// failure the agent should fear. An agent optimising for "did I answer the
	// brief" ships a wireframe; one optimising for "would a designer sign
	// this" ships a page.
	b.WriteString("- You are an expert product designer, not a code generator that happens to emit HTML. The requirement is the floor, not the target: a page that satisfies every line of the brief and still looks like an untouched template has failed this task.\n")
	b.WriteString("- Design for the medium the requirement actually names — an admin console, a marketing page, a mobile flow and a data dashboard have different densities, rhythms and defaults. Do not apply generic web-page tropes (hero band, three feature cards, centred everything) to a screen that is not a marketing page.\n")

	// The craft rules. Each one is a thing reviewers repeatedly catch and a
	// contract check never will, because a package can be structurally perfect
	// and visually flat.
	b.WriteString("- Build hierarchy with weight, size, spacing and colour role — not with borders and boxes. Nesting a bordered card inside a bordered panel inside a bordered section is the default failure mode of generated UI; prefer whitespace and typographic contrast to another rounded rectangle.\n")
	b.WriteString("- Keep spacing on one consistent scale, and keep the same rhythm across pages. Ad-hoc values (11px here, 13px there) read as sloppiness even when nobody can name why.\n")
	b.WriteString("- Every interactive element needs its full state set: rest, hover, active, focus-visible, disabled, and — where the flow implies them — loading, empty, and error. A prototype whose buttons only have a rest state is a mockup, not a prototype.\n")
	b.WriteString("- Text must stay readable against its background at every size you use, and an icon-only control must carry an accessible name. Do not express a text tone by making a solid colour transparent; use the design system's own muted / secondary role instead.\n")
	b.WriteString("- Write real content. Realistic names, plausible numbers, copy in the product's own voice and language. Lorem ipsum, `Item 1 / Item 2 / Item 3`, and placeholder rectangles standing in for content are all failures of this task, because the user is judging a design they cannot read.\n")

	// The two rules below are not taste — they are hard requirements of the
	// surfaces this package is rendered in, and neither is discoverable from
	// the package contract alone.
	b.WriteString("- Name the regions a reviewer will point at. Put `data-page`, `data-state`, `data-flow` and `data-block` attributes on the pages, states, flows and named blocks `brief.json` declares, using that same stable ID. The workspace resolves an annotation to an element through exactly these attributes; without them a user's mark-up lands on a fragile positional selector and the next revision loses it.\n")
	b.WriteString("- The annotation canvas renders the prototype with scripts DISABLED, so whatever a page shows before any JavaScript runs is what a reviewer marks up and what a raster export captures. Ship the default page and its default state as real markup, and let scripts switch, filter and animate from there. A page that renders empty until a script populates it is blank on that surface.\n")

	b.WriteString("\n")
	return b.String()
}
