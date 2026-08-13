package opendesign

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractDraftCompatibilityArtifactsRevalidatesArchiveEvidence(t *testing.T) {
	t.Parallel()

	const runID = "11111111-1111-4111-8111-111111111111"
	archive := testProjectArchive(t, []testArchiveFile{
		{Path: DraftDesignMDPath, Body: "# CRM Design System\n"},
		{Path: DraftTokensCSSPath, Body: ":root { --color-primary: #1677ff; }"},
		{Path: DraftUIKitHTMLPath, Body: "<!doctype html><main>CRM</main>"},
	})
	manifest := testProjectExportManifest(map[string]testManifestFile{
		DraftDesignMDPath:  {MIME: "text/markdown; charset=utf-8", Role: "source", Body: "# CRM Design System\n"},
		DraftTokensCSSPath: {MIME: "text/css; charset=utf-8", Role: "artifact", Body: ":root { --color-primary: #1677ff; }"},
		DraftUIKitHTMLPath: {MIME: "text/html; charset=utf-8", Role: "entry", Body: "<!doctype html><main>CRM</main>"},
	})
	result, err := CollectWorkerRunResult(
		json.RawMessage(`{"schema":"open-design.run-result-package.v1","run":{"id":"11111111-1111-4111-8111-111111111111"}}`),
		manifest,
		archive,
		runID,
		"project-1",
	)
	if err != nil {
		t.Fatalf("CollectWorkerRunResult: %v", err)
	}

	artifacts, err := ExtractDraftCompatibilityArtifacts(archive, result.ArtifactIndex, result.ContentDigest)
	if err != nil {
		t.Fatalf("ExtractDraftCompatibilityArtifacts: %v", err)
	}
	if artifacts.DesignMD != "# CRM Design System\n" ||
		artifacts.TokensCSS != ":root { --color-primary: #1677ff; }" ||
		artifacts.ComponentsHTML != "<!doctype html><main>CRM</main>" {
		t.Fatalf("compatibility artifacts = %+v", artifacts)
	}
	for _, source := range []DraftArtifactSource{artifacts.Sources.DesignMD, artifacts.Sources.TokensCSS, artifacts.Sources.ComponentsHTML} {
		if source.Path == "" || source.Size <= 0 || len(source.SHA256) != 64 {
			t.Fatalf("draft source evidence is incomplete: %+v", source)
		}
	}

	tampered := append([]ArtifactIndexEntry(nil), result.ArtifactIndex...)
	tampered[0].SHA256 = strings.Repeat("b", 64)
	if _, err := ExtractDraftCompatibilityArtifacts(archive, tampered, result.ContentDigest); err == nil || !strings.Contains(err.Error(), "artifact index") {
		t.Fatalf("tampered artifact index error = %v", err)
	}
}

func TestValidateBasePackageReference(t *testing.T) {
	valid := BasePackageReference{
		Schema:        BasePackageReferenceSchema,
		Slot:          "saved",
		ContentDigest: "sha256:" + strings.Repeat("a", 64),
		SourceTaskID:  "11111111-1111-4111-8111-111111111111",
	}
	if err := ValidateBasePackageReference(valid); err != nil {
		t.Fatalf("ValidateBasePackageReference(valid): %v", err)
	}

	for name, mutate := range map[string]func(*BasePackageReference){
		"schema":      func(reference *BasePackageReference) { reference.Schema = "unknown" },
		"slot":        func(reference *BasePackageReference) { reference.Slot = "other" },
		"digest":      func(reference *BasePackageReference) { reference.ContentDigest = "sha256:bad" },
		"source task": func(reference *BasePackageReference) { reference.SourceTaskID = "" },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := valid
			mutate(&candidate)
			if err := ValidateBasePackageReference(candidate); err == nil {
				t.Fatalf("ValidateBasePackageReference(%s) succeeded", name)
			}
		})
	}
}

func TestExtractProjectArchiveValidatesDigestAndWritesFreshFiles(t *testing.T) {
	t.Parallel()

	archive := testProjectArchive(t, []testArchiveFile{
		{Path: "DESIGN.md", Body: "# CRM Design System\n"},
		{Path: "ui_kits/app/index.html", Body: "<!doctype html><main>CRM</main>"},
	})
	index, err := indexProjectArchive(archive, nil)
	if err != nil {
		t.Fatalf("indexProjectArchive: %v", err)
	}
	destination := t.TempDir()
	if err := ExtractProjectArchive(archive, digestArtifactIndex(index), destination); err != nil {
		t.Fatalf("ExtractProjectArchive: %v", err)
	}
	for path, want := range map[string]string{
		"DESIGN.md":              "# CRM Design System\n",
		"ui_kits/app/index.html": "<!doctype html><main>CRM</main>",
	} {
		got, err := os.ReadFile(filepath.Join(destination, filepath.FromSlash(path)))
		if err != nil {
			t.Fatalf("read extracted %s: %v", path, err)
		}
		if string(got) != want {
			t.Fatalf("extracted %s = %q, want %q", path, got, want)
		}
	}

	tamperedDigest := "sha256:" + strings.Repeat("b", 64)
	if err := ExtractProjectArchive(archive, tamperedDigest, t.TempDir()); err == nil || !strings.Contains(err.Error(), "content digest") {
		t.Fatalf("tampered digest error = %v", err)
	}
}

func TestExtractProjectArchiveRejectsReservedAndExistingPaths(t *testing.T) {
	t.Parallel()

	reserved := testProjectArchive(t, []testArchiveFile{{Path: ".agent_context/task.json", Body: "{}"}})
	reservedIndex, err := indexProjectArchive(reserved, nil)
	if err != nil {
		t.Fatalf("index reserved archive: %v", err)
	}
	if err := ExtractProjectArchive(reserved, digestArtifactIndex(reservedIndex), t.TempDir()); err == nil || !strings.Contains(err.Error(), "reserved") {
		t.Fatalf("reserved path error = %v", err)
	}

	archive := testProjectArchive(t, []testArchiveFile{{Path: "DESIGN.md", Body: "# New\n"}})
	index, err := indexProjectArchive(archive, nil)
	if err != nil {
		t.Fatalf("index collision archive: %v", err)
	}
	destination := t.TempDir()
	existing := filepath.Join(destination, "DESIGN.md")
	if err := os.WriteFile(existing, []byte("# Existing\n"), 0o644); err != nil {
		t.Fatalf("write existing file: %v", err)
	}
	if err := ExtractProjectArchive(archive, digestArtifactIndex(index), destination); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("existing path error = %v", err)
	}
	got, err := os.ReadFile(existing)
	if err != nil || string(got) != "# Existing\n" {
		t.Fatalf("existing file changed: %q, %v", got, err)
	}
}

func TestExtractDraftCompatibilityArtifactsRejectsMissingSourceFile(t *testing.T) {
	t.Parallel()

	archive := testProjectArchive(t, []testArchiveFile{
		{Path: DraftDesignMDPath, Body: "# CRM Design System\n"},
		{Path: DraftUIKitHTMLPath, Body: "<!doctype html><main>CRM</main>"},
	})
	index, err := indexProjectArchive(archive, nil)
	if err != nil {
		t.Fatalf("indexProjectArchive: %v", err)
	}
	_, err = ExtractDraftCompatibilityArtifacts(archive, index, digestArtifactIndex(index))
	if err == nil || !strings.Contains(err.Error(), DraftTokensCSSPath) {
		t.Fatalf("missing source error = %v", err)
	}
}

func TestReadDraftArchiveArtifactReturnsDigestBoundFile(t *testing.T) {
	t.Parallel()

	archive := testProjectArchive(t, []testArchiveFile{
		{Path: DraftDesignMDPath, Body: "# CRM Design System\n"},
		{Path: DraftTokensCSSPath, Body: ":root { --color-primary: #1677ff; }"},
		{Path: DraftUIKitHTMLPath, Body: "<!doctype html><link rel=\"stylesheet\" href=\"../../colors_and_type.css\"><main>CRM</main>"},
		{Path: "ui_kits/app/components/Button.js", Body: "window.Button = () => 'Save';"},
	})
	index, err := indexProjectArchive(archive, nil)
	if err != nil {
		t.Fatalf("indexProjectArchive: %v", err)
	}
	digest := digestArtifactIndex(index)

	artifact, err := ReadDraftArchiveArtifact(archive, index, digest, DraftUIKitHTMLPath)
	if err != nil {
		t.Fatalf("ReadDraftArchiveArtifact: %v", err)
	}
	if artifact.Path != DraftUIKitHTMLPath || artifact.MIME != "text/html; charset=utf-8" {
		t.Fatalf("archive artifact metadata = %+v", artifact)
	}
	if string(artifact.Body) != "<!doctype html><link rel=\"stylesheet\" href=\"../../colors_and_type.css\"><main>CRM</main>" {
		t.Fatalf("archive artifact body = %q", artifact.Body)
	}

	tampered := append([]ArtifactIndexEntry(nil), index...)
	for position := range tampered {
		if tampered[position].Path == DraftUIKitHTMLPath {
			tampered[position].SHA256 = strings.Repeat("b", 64)
			break
		}
	}
	if _, err := ReadDraftArchiveArtifact(archive, tampered, digestArtifactIndex(tampered), DraftUIKitHTMLPath); err == nil || !strings.Contains(err.Error(), "digest") {
		t.Fatalf("tampered archive artifact error = %v", err)
	}
	if _, err := ReadDraftArchiveArtifact(archive, index, digest, "../DESIGN.md"); err == nil {
		t.Fatal("ReadDraftArchiveArtifact accepted a traversal path")
	}
}
