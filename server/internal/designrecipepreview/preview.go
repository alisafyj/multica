// Package designrecipepreview serves the cover a community recipe card shows.
//
// Open Design does not ship thumbnails for its templates. A card's cover is
// the template's own example output rendered live: an `example.html` shown in
// a sandboxed frame for prototypes and decks, or a poster image for the media
// templates that have one. This package carries those files the same way the
// design-system catalogue carries its packages — embedded, read-only, keyed by
// the recipe slug — so a cover is a code change with a diff rather than an
// upload into object storage that could drift from the seed row it belongs to.
//
// An example may lean on files beside it — a deck runtime in `assets/`, a
// stylesheet — referenced by relative path. Those travel with it under the
// same relative paths, so the document is served from a directory URL and the
// browser resolves them the way the author expected.
package designrecipepreview

import (
	"embed"
	"errors"
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
	return Preview{Kind: KindPoster, ContentType: contentTypeFor(e.poster), Body: body}, true, nil
}

// Asset is a file an example references beside itself.
type Asset struct {
	ContentType string
	Body        []byte
}

// GetAsset returns the file at rel inside a slug's bundle. ok=false when the
// slug has no cover or the file is absent — the caller answers 404. rel is a
// slash path as the browser resolved it against the document URL; anything
// that is not a plain descending path (empty, absolute, `..`, or a bare
// example.html — that is the document, served by Get) is refused rather than
// cleaned, so a crafted URL cannot reach a neighbouring slug's files.
func GetAsset(slug, rel string) (Asset, bool, error) {
	once.Do(load)
	if _, ok := index[strings.TrimSpace(slug)]; !ok {
		return Asset{}, false, nil
	}
	if rel == "" || rel == "example.html" || !fs.ValidPath(rel) || rel != path.Clean(rel) {
		return Asset{}, false, nil
	}
	for _, segment := range strings.Split(rel, "/") {
		if segment == ".." || segment == "." || segment == "" {
			return Asset{}, false, nil
		}
	}
	body, err := files.ReadFile(path.Join("data", slug, rel))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Asset{}, false, nil
		}
		// embed.FS reports a directory read as an error too; a directory is
		// not an asset.
		return Asset{}, false, nil
	}
	return Asset{ContentType: contentTypeFor(rel), Body: body}, true, nil
}

// contentTypeFor picks the media type from the extension. Bundled files are
// authored, not uploaded, so the extension is trusted; unknown ones are served
// as bytes and the nosniff header keeps the browser from guessing.
func contentTypeFor(name string) string {
	switch strings.ToLower(path.Ext(name)) {
	case ".html", ".htm":
		return "text/html; charset=utf-8"
	case ".js", ".mjs":
		return "text/javascript; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".json":
		return "application/json; charset=utf-8"
	case ".svg":
		return "image/svg+xml"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	case ".avif":
		return "image/avif"
	case ".ico":
		return "image/x-icon"
	case ".woff":
		return "font/woff"
	case ".woff2":
		return "font/woff2"
	case ".ttf":
		return "font/ttf"
	case ".otf":
		return "font/otf"
	case ".mp4":
		return "video/mp4"
	case ".webm":
		return "video/webm"
	case ".mp3":
		return "audio/mpeg"
	case ".wav":
		return "audio/wav"
	case ".ogg":
		return "audio/ogg"
	default:
		return "application/octet-stream"
	}
}
