// Package designrecipepreview serves the cover a community recipe card shows.
//
// Open Design does not ship thumbnails for its templates. A card's cover is
// the template's own example output rendered live: an `example.html` shown in
// a sandboxed frame for prototypes and decks, or a poster image for the media
// templates that have one. This package carries those files the same way the
// design-system catalogue carries its packages — embedded, read-only, keyed by
// the recipe slug — so a cover is a code change with a diff rather than an
// upload into object storage that could drift from the seed row it belongs to.
package designrecipepreview

import (
	"embed"
	"io/fs"
	"path"
	"strings"
	"sync"
)

//go:embed all:data
var files embed.FS

// Kind says what the cover is, so the client picks a frame or an image
// without probing.
type Kind string

const (
	KindNone   Kind = ""
	KindHTML   Kind = "html"
	KindPoster Kind = "poster"
)

// Preview is the cover one recipe has, if any.
type Preview struct {
	Kind Kind
	// ContentType is set for posters, whose extension decides it.
	ContentType string
	Body        []byte
}

var (
	once   sync.Once
	index  map[string]entry
	loaded bool
)

type entry struct {
	html   bool
	poster string // file name, "" when absent
}

func load() {
	dirs, err := fs.ReadDir(files, "data")
	if err != nil {
		return
	}
	index = make(map[string]entry, len(dirs))
	for _, dir := range dirs {
		if !dir.IsDir() {
			continue
		}
		var e entry
		if _, err := fs.Stat(files, path.Join("data", dir.Name(), "example.html")); err == nil {
			e.html = true
		}
		for _, name := range []string{"preview.png", "preview.jpg", "preview.webp"} {
			if _, err := fs.Stat(files, path.Join("data", dir.Name(), name)); err == nil {
				e.poster = name
				break
			}
		}
		if e.html || e.poster != "" {
			index[dir.Name()] = e
		}
	}
	loaded = true
}

// KindFor reports which cover a slug has. Cheap: it reads an in-memory index,
// so the list handler can stamp every row without opening a file.
func KindFor(slug string) Kind {
	once.Do(load)
	e, ok := index[strings.TrimSpace(slug)]
	switch {
	case !ok:
		return KindNone
	case e.html:
		return KindHTML
	case e.poster != "":
		return KindPoster
	default:
		return KindNone
	}
}

// Get returns the cover for a slug. ok=false when the slug has none — the
// caller answers 404. The slug indexes a map built from directory names, so a
// hostile value never reaches ReadFile as a path.
func Get(slug string) (Preview, bool, error) {
	once.Do(load)
	e, ok := index[strings.TrimSpace(slug)]
	if !ok {
		return Preview{}, false, nil
	}
	if e.html {
		body, err := files.ReadFile(path.Join("data", slug, "example.html"))
		if err != nil {
			return Preview{}, false, err
		}
		return Preview{Kind: KindHTML, ContentType: "text/html; charset=utf-8", Body: body}, true, nil
	}
	body, err := files.ReadFile(path.Join("data", slug, e.poster))
	if err != nil {
		return Preview{}, false, err
	}
	ct := "image/png"
	switch path.Ext(e.poster) {
	case ".jpg", ".jpeg":
		ct = "image/jpeg"
	case ".webp":
		ct = "image/webp"
	}
	return Preview{Kind: KindPoster, ContentType: ct, Body: body}, true, nil
}
