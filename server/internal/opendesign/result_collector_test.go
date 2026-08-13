package opendesign

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"
)

func TestCollectWorkerRunResultRedactsLocalPathsAndIndexesArchive(t *testing.T) {
	t.Parallel()

	resultPackage := json.RawMessage(`{
		"schema":"open-design.run-result-package.v1",
		"run":{"id":"11111111-1111-4111-8111-111111111111"},
		"workspace":{
			"storage":{"kind":"folder-backed","baseDir":"/private/tmp/open-design/workspace"},
			"provenance":{"kind":"orchestrator-scratch","sourceLabel":"multica-project:crm"}
		},
		"events":{"logPath":"/private/tmp/open-design/events.jsonl"}
	}`)
	manifest := testProjectExportManifest(map[string]testManifestFile{
		"index.html": {
			MIME: "text/html; charset=utf-8",
			Role: "entry",
			Body: "<main>CRM</main>",
		},
		"assets/logo.svg": {
			MIME: "image/svg+xml",
			Role: "asset",
			Body: "<svg></svg>",
		},
	})
	archive := testProjectArchive(t, []testArchiveFile{
		{Path: "index.html", Body: "<main>CRM</main>"},
		{Path: "DESIGN-HANDOFF.md", Body: "# Handoff"},
		{Path: "assets/logo.svg", Body: "<svg></svg>"},
	})

	collected, err := CollectWorkerRunResult(
		resultPackage,
		manifest,
		archive,
		"11111111-1111-4111-8111-111111111111",
		"project-1",
	)
	if err != nil {
		t.Fatalf("CollectWorkerRunResult: %v", err)
	}
	if strings.Contains(string(collected.ResultPackage), "/private/tmp") ||
		strings.Contains(string(collected.ResultPackage), "baseDir") ||
		strings.Contains(string(collected.ResultPackage), "logPath") {
		t.Fatalf("sanitized result package retained local paths: %s", collected.ResultPackage)
	}
	if !strings.Contains(string(collected.ResultPackage), "multica-project:crm") {
		t.Fatalf("sanitized result package lost portable provenance: %s", collected.ResultPackage)
	}

	wantPaths := []string{"DESIGN-HANDOFF.md", "assets/logo.svg", "index.html"}
	if len(collected.ArtifactIndex) != len(wantPaths) {
		t.Fatalf("artifact index = %+v", collected.ArtifactIndex)
	}
	for index, wantPath := range wantPaths {
		if collected.ArtifactIndex[index].Path != wantPath {
			t.Fatalf("artifact index paths = %+v, want %v", collected.ArtifactIndex, wantPaths)
		}
	}
	if collected.ArtifactIndex[0].Role != "other" || collected.ArtifactIndex[0].MIME != "text/markdown; charset=utf-8" {
		t.Fatalf("generated archive entry = %+v", collected.ArtifactIndex[0])
	}
	if collected.ArtifactIndex[1].Role != "asset" || collected.ArtifactIndex[1].MIME != "image/svg+xml" {
		t.Fatalf("manifest-backed asset = %+v", collected.ArtifactIndex[1])
	}
	if collected.ArtifactIndex[2].Role != "entry" || collected.ArtifactIndex[2].MIME != "text/html; charset=utf-8" {
		t.Fatalf("manifest-backed entry = %+v", collected.ArtifactIndex[2])
	}
	if collected.ContentDigest != testContentDigest(collected.ArtifactIndex) {
		t.Fatalf("content digest = %q, want %q", collected.ContentDigest, testContentDigest(collected.ArtifactIndex))
	}
}

func TestCollectWorkerRunResultDigestDoesNotDependOnZipEntryOrder(t *testing.T) {
	t.Parallel()

	manifest := testProjectExportManifest(map[string]testManifestFile{
		"a.txt": {MIME: "text/plain", Role: "source", Body: "alpha"},
		"b.txt": {MIME: "text/plain", Role: "supporting", Body: "beta"},
	})
	resultPackage := json.RawMessage(`{"schema":"open-design.run-result-package.v1","run":{"id":"11111111-1111-4111-8111-111111111111"}}`)
	first, err := CollectWorkerRunResult(resultPackage, manifest, testProjectArchive(t, []testArchiveFile{
		{Path: "a.txt", Body: "alpha"},
		{Path: "b.txt", Body: "beta"},
	}), "11111111-1111-4111-8111-111111111111", "project-1")
	if err != nil {
		t.Fatalf("collect first archive: %v", err)
	}
	second, err := CollectWorkerRunResult(resultPackage, manifest, testProjectArchive(t, []testArchiveFile{
		{Path: "b.txt", Body: "beta"},
		{Path: "a.txt", Body: "alpha"},
	}), "11111111-1111-4111-8111-111111111111", "project-1")
	if err != nil {
		t.Fatalf("collect reordered archive: %v", err)
	}
	if first.ContentDigest != second.ContentDigest {
		t.Fatalf("digest changed with ZIP order: %q != %q", first.ContentDigest, second.ContentDigest)
	}
}

func TestCollectWorkerRunResultRejectsArchiveMissingManifestFile(t *testing.T) {
	t.Parallel()

	manifest := testProjectExportManifest(map[string]testManifestFile{
		"index.html": {MIME: "text/html", Role: "entry", Body: "<main></main>"},
	})
	resultPackage := json.RawMessage(`{"schema":"open-design.run-result-package.v1","run":{"id":"11111111-1111-4111-8111-111111111111"}}`)
	_, err := CollectWorkerRunResult(
		resultPackage,
		manifest,
		testProjectArchive(t, []testArchiveFile{{Path: "other.txt", Body: "other"}}),
		"11111111-1111-4111-8111-111111111111",
		"project-1",
	)
	if err == nil || !strings.Contains(err.Error(), "index.html") {
		t.Fatalf("CollectWorkerRunResult error = %v, want missing index.html", err)
	}
}

func TestValidateRunResultRequestRejectsLocalPathsAndMismatchedDigest(t *testing.T) {
	t.Parallel()

	index := []ArtifactIndexEntry{{
		Path: "index.html", Role: "entry", MIME: "text/html", Size: 13,
		SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}}
	request := RunResultRequest{
		OpenDesignRunID:  "11111111-1111-4111-8111-111111111111",
		ResultPackage:    json.RawMessage(`{"schema":"open-design.run-result-package.v1","run":{"id":"11111111-1111-4111-8111-111111111111"}}`),
		ArtifactIndex:    index,
		ArchiveObjectKey: "workspaces/workspace-1/open-design-runs/task-1/archive.zip",
		ContentDigest:    testContentDigest(index),
	}
	if err := ValidateRunResultRequest(request, request.OpenDesignRunID); err != nil {
		t.Fatalf("ValidateRunResultRequest: %v", err)
	}

	localPath := request
	localPath.ResultPackage = json.RawMessage(`{"schema":"open-design.run-result-package.v1","run":{"id":"11111111-1111-4111-8111-111111111111"},"workspace":{"storage":{"baseDir":"/private/tmp/workspace"}}}`)
	if err := ValidateRunResultRequest(localPath, request.OpenDesignRunID); err == nil || !strings.Contains(err.Error(), "local path") {
		t.Fatalf("local-path validation error = %v", err)
	}

	badDigest := request
	badDigest.ContentDigest = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	if err := ValidateRunResultRequest(badDigest, request.OpenDesignRunID); err == nil || !strings.Contains(err.Error(), "content digest") {
		t.Fatalf("digest validation error = %v", err)
	}

	spacedPath := request
	spacedPath.ArtifactIndex = append([]ArtifactIndexEntry(nil), request.ArtifactIndex...)
	spacedPath.ArtifactIndex[0].Path = " index.html"
	spacedPath.ContentDigest = testContentDigest(spacedPath.ArtifactIndex)
	if err := ValidateRunResultRequest(spacedPath, request.OpenDesignRunID); err == nil || !strings.Contains(err.Error(), "normalized relative") {
		t.Fatalf("non-normalized path validation error = %v", err)
	}
}

func TestValidateProjectArchiveContentDigest(t *testing.T) {
	t.Parallel()

	archive := testProjectArchive(t, []testArchiveFile{{Path: "index.html", Body: "<main></main>"}})
	index, err := indexProjectArchive(archive, nil)
	if err != nil {
		t.Fatalf("indexProjectArchive: %v", err)
	}
	contentDigest := digestArtifactIndex(index)
	if err := ValidateProjectArchiveContentDigest(archive, contentDigest); err != nil {
		t.Fatalf("ValidateProjectArchiveContentDigest: %v", err)
	}
	if err := ValidateProjectArchiveContentDigest(archive, "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("mismatched archive digest error = %v", err)
	}
}

type testManifestFile struct {
	MIME string
	Role string
	Body string
}

func testProjectExportManifest(files map[string]testManifestFile) json.RawMessage {
	type manifestFile struct {
		Name      string   `json:"name"`
		Path      string   `json:"path"`
		LocalPath string   `json:"localPath"`
		Type      string   `json:"type"`
		Size      int      `json:"size"`
		Mtime     int64    `json:"mtime"`
		Kind      string   `json:"kind"`
		MIME      string   `json:"mime"`
		Included  bool     `json:"included"`
		Role      string   `json:"role"`
		Reasons   []string `json:"reasons"`
	}
	payload := struct {
		Schema    string         `json:"schema"`
		ProjectID string         `json:"projectId"`
		Files     []manifestFile `json:"files"`
	}{
		Schema:    ProjectExportManifestSchema,
		ProjectID: "project-1",
	}
	for path, file := range files {
		payload.Files = append(payload.Files, manifestFile{
			Name:      path,
			Path:      path,
			LocalPath: "/private/tmp/open-design/workspace/" + path,
			Type:      "file",
			Size:      len(file.Body),
			Mtime:     1,
			Kind:      "text",
			MIME:      file.MIME,
			Included:  true,
			Role:      file.Role,
			Reasons:   []string{"visible-project-file"},
		})
	}
	encoded, _ := json.Marshal(payload)
	return encoded
}

type testArchiveFile struct {
	Path string
	Body string
}

func testProjectArchive(t *testing.T, files []testArchiveFile) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for _, file := range files {
		entry, err := writer.Create(file.Path)
		if err != nil {
			t.Fatalf("create ZIP entry %q: %v", file.Path, err)
		}
		if _, err := entry.Write([]byte(file.Body)); err != nil {
			t.Fatalf("write ZIP entry %q: %v", file.Path, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close ZIP: %v", err)
	}
	return buffer.Bytes()
}

func testContentDigest(index []ArtifactIndexEntry) string {
	hasher := sha256.New()
	for _, entry := range index {
		fmt.Fprintf(hasher, "%s\x00%s\x00%s\x00", entry.Path, strconv.FormatInt(entry.Size, 10), entry.SHA256)
	}
	return "sha256:" + hex.EncodeToString(hasher.Sum(nil))
}
