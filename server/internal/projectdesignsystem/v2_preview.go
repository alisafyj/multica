package projectdesignsystem

import (
	"errors"
	"path"
	"sort"
	"strings"
)

func DiscoverV2PreviewTargets(index []ArtifactIndexEntry) ([]PreviewTarget, error) {
	previews := make([]PreviewTarget, 0)
	hasUIKit := false
	seenIDs := make(map[string]struct{})
	for _, entry := range index {
		switch {
		case entry.Path == "ui-kit/index.html":
			if hasUIKit || entry.Role != "ui_kit" || entry.MediaType != "text/html; charset=utf-8" {
				return nil, errors.New("V2 UI Kit index metadata is invalid")
			}
			hasUIKit = true
		case strings.HasPrefix(entry.Path, "preview/") && path.Ext(entry.Path) == ".html":
			if entry.Role != "preview" || entry.MediaType != "text/html; charset=utf-8" {
				return nil, errors.New("V2 Preview metadata is invalid")
			}
			id := strings.TrimSuffix(path.Base(entry.Path), ".html")
			if !validLocatorID(id) {
				return nil, errors.New("V2 Preview filename is not a stable target ID")
			}
			if _, exists := seenIDs[id]; exists {
				return nil, errors.New("V2 Preview target IDs must be unique")
			}
			seenIDs[id] = struct{}{}
			previews = append(previews, PreviewTarget{ID: id, Kind: "preview", Path: entry.Path})
		}
	}
	sort.Slice(previews, func(left, right int) bool {
		return previews[left].Path < previews[right].Path
	})
	targets := make([]PreviewTarget, 0, len(previews)+1)
	if hasUIKit {
		targets = append(targets, PreviewTarget{ID: "ui-kit", Kind: "ui_kit", Path: "ui-kit/index.html"})
	}
	targets = append(targets, previews...)
	if len(targets) == 0 {
		return nil, errors.New("V2 package requires at least one UI Kit or Preview target")
	}
	if len(targets) > maxV2PreviewTargets {
		return nil, errors.New("V2 package contains too many Preview targets")
	}
	return targets, nil
}
