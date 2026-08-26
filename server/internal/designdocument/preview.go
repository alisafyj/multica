package designdocument

import (
	"errors"
	"sort"
	"strings"
)

// DiscoverPreviewTargets derives the browser Preview target set from the file
// index. Every prototype HTML document is a target; the prototype entry is
// always first so the Preview gate opens the document root before sub pages.
func DiscoverPreviewTargets(index []ArtifactIndexEntry) ([]PreviewTarget, error) {
	pages := make([]PreviewTarget, 0)
	hasEntry := false
	seenIDs := make(map[string]struct{})
	for _, entry := range index {
		switch entry.Role {
		case "prototype_entry":
			if hasEntry || entry.Path != prototypeEntryPath || entry.MediaType != "text/html; charset=utf-8" {
				return nil, errors.New("design document prototype entry metadata is invalid")
			}
			if _, exists := seenIDs[prototypeEntryTargetID]; exists {
				return nil, errors.New("design document Preview target IDs must be unique")
			}
			hasEntry = true
			seenIDs[prototypeEntryTargetID] = struct{}{}
		case "prototype_page":
			id, ok := previewTargetID(entry.Path)
			if !ok || entry.MediaType != "text/html; charset=utf-8" {
				return nil, errors.New("design document Preview target metadata is invalid")
			}
			if _, exists := seenIDs[id]; exists {
				return nil, errors.New("design document Preview target IDs must be unique")
			}
			seenIDs[id] = struct{}{}
			pages = append(pages, PreviewTarget{ID: id, Kind: "prototype_page", Path: entry.Path})
		}
	}
	if !hasEntry {
		return nil, errors.New("design document package requires prototype/index.html")
	}
	sort.Slice(pages, func(left, right int) bool { return pages[left].Path < pages[right].Path })
	targets := make([]PreviewTarget, 0, len(pages)+1)
	targets = append(targets, PreviewTarget{ID: prototypeEntryTargetID, Kind: "prototype_entry", Path: prototypeEntryPath})
	targets = append(targets, pages...)
	if len(targets) > maxPreviewTargets {
		return nil, errors.New("design document package contains too many Preview targets")
	}
	return targets, nil
}

const prototypeEntryTargetID = "index"

// previewTargetID turns prototype/orders/list.html into the stable target ID
// orders.list.
func previewTargetID(name string) (string, bool) {
	if !isPrototypeDocumentPath(name) {
		return "", false
	}
	trimmed := strings.TrimSuffix(strings.TrimPrefix(name, prototypeRoot+"/"), ".html")
	id := strings.ReplaceAll(trimmed, "/", ".")
	if !validSemanticID(id) {
		return "", false
	}
	return id, true
}

// PreviewTargetKind maps a design document Preview target kind onto the kind
// vocabulary designpreview validates receipts against: ui_kit for a design
// system's UI Kit, preview for a rendered page. A prototype entry and a
// prototype sub page are both rendered pages, so both map to preview; the
// entry stays identifiable through its target ID and the manifest's
// prototype_entry field.
//
// It lives here because it is a property of this package's target vocabulary,
// and because BOTH the daemon (which builds the receipt) and the server (which
// re-derives the expected target set to validate it) must apply it. Two copies
// that drift make designpreview.ValidateTargetSet reject perfectly good
// receipts.
func PreviewTargetKind(kind string) (string, bool) {
	switch kind {
	case "prototype_entry", "prototype_page":
		return "preview", true
	default:
		return "", false
	}
}
