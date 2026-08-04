package opendesign

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"
)

func TestBuildRunEvidenceArchiveProducesDeterministicPortableBundle(t *testing.T) {
	t.Parallel()

	archive := testProjectArchive(t, []testArchiveFile{{Path: "DESIGN.md", Body: "# CRM\n"}})
	artifactIndex, err := indexProjectArchive(archive, nil)
	if err != nil {
		t.Fatalf("index project archive: %v", err)
	}
	contentDigest := digestArtifactIndex(artifactIndex)
	input := RunEvidenceArchiveInput{
		Run: RunEvidenceReference{
			SupervisorRunID: "22222222-2222-4222-8222-222222222222",
			WorkerRunID:     "11111111-1111-4111-8111-111111111111",
			TaskID:          "33333333-3333-4333-8333-333333333333",
			WorkspaceID:     "44444444-4444-4444-8444-444444444444",
			ProjectID:       "55555555-5555-4555-8555-555555555555",
			DesignSystemID:  "66666666-6666-4666-8666-666666666666",
			Operation:       "generate",
			Status:          RunStatusSucceeded,
			AdapterID:       "opencode",
			Model:           "anthropic/claude-sonnet-4-5",
			CreatedAt:       "2026-08-03T08:00:00Z",
			StartedAt:       "2026-08-03T08:00:01Z",
			FinishedAt:      "2026-08-03T08:10:00Z",
		},
		Engine:              PinnedEngineIdentity(),
		AgentSnapshot:       json.RawMessage(`{"adapter_id":"opencode","multica_agent_id":"77777777-7777-4777-8777-777777777777"}`),
		InputSnapshot:       json.RawMessage(`{"brief":"Create CRM","platform":"web"}`),
		WorkspaceProvenance: json.RawMessage(`{"kind":"orchestrator-scratch","writeback":"external"}`),
		Preflight:           json.RawMessage(`{"schema":"multica.open-design-preflight/v1"}`),
		Events:              json.RawMessage(`[{"id":1,"event":"start","data":{"status":"running"}}]`),
		ResultPackage:       json.RawMessage(`{"schema":"open-design.run-result-package.v1","run":{"id":"11111111-1111-4111-8111-111111111111"}}`),
		ArtifactIndex:       artifactIndex,
		ArchiveObjectKey:    "workspaces/test/package.zip",
		ContentDigest:       contentDigest,
		AuditReport:         json.RawMessage(`{"schema":"multica.open-design-package-audit/v1","audit":{"ok":true}}`),
		PreviewReceipt:      json.RawMessage(`{"schema":"multica.open-design-preview-verification/v1","verification":{"passed":true}}`),
		Failure:             json.RawMessage(`{}`),
		ProjectArchive:      archive,
	}

	first, firstDigest, err := BuildRunEvidenceArchive(input)
	if err != nil {
		t.Fatalf("BuildRunEvidenceArchive: %v", err)
	}
	second, secondDigest, err := BuildRunEvidenceArchive(input)
	if err != nil {
		t.Fatalf("BuildRunEvidenceArchive second pass: %v", err)
	}
	if !bytes.Equal(first, second) || firstDigest != secondDigest {
		t.Fatalf("evidence archive is not deterministic: digest %q vs %q", firstDigest, secondDigest)
	}
	if err := ValidateContentDigest(firstDigest); err != nil {
		t.Fatalf("evidence digest = %q: %v", firstDigest, err)
	}

	files := readEvidenceArchiveFiles(t, first)
	for _, path := range []string{
		"manifest.json",
		"project/archive.zip",
		"run/agent.json",
		"run/artifact-index.json",
		"run/audit.json",
		"run/events.json",
		"run/failure.json",
		"run/input.json",
		"run/preflight.json",
		"run/preview.json",
		"run/result-package.json",
		"run/workspace-provenance.json",
	} {
		if _, ok := files[path]; !ok {
			t.Fatalf("evidence archive is missing %q", path)
		}
	}
	if !bytes.Equal(files["project/archive.zip"], archive) {
		t.Fatal("embedded project archive changed")
	}
	var manifest RunEvidenceManifest
	if err := json.Unmarshal(files["manifest.json"], &manifest); err != nil {
		t.Fatalf("decode evidence manifest: %v", err)
	}
	if manifest.Schema != RunEvidenceManifestSchema || manifest.Run.Status != RunStatusSucceeded || manifest.Archive.ContentDigest != contentDigest || len(manifest.Files) != len(files)-1 {
		t.Fatalf("evidence manifest = %+v", manifest)
	}
}

func TestBuildRunEvidenceArchiveRejectsNonTerminalRun(t *testing.T) {
	t.Parallel()

	_, _, err := BuildRunEvidenceArchive(RunEvidenceArchiveInput{
		Run:    RunEvidenceReference{Status: RunStatusRunning},
		Engine: PinnedEngineIdentity(),
	})
	if err == nil {
		t.Fatal("BuildRunEvidenceArchive accepted a running Run")
	}
}

func TestBuildRunEvidenceArchiveRejectsProjectArchiveDigestMismatch(t *testing.T) {
	t.Parallel()

	_, _, err := BuildRunEvidenceArchive(RunEvidenceArchiveInput{
		Run: RunEvidenceReference{
			SupervisorRunID: "22222222-2222-4222-8222-222222222222",
			Status:          RunStatusAgentFailed,
			WorkerRunID:     "11111111-1111-4111-8111-111111111111",
			TaskID:          "33333333-3333-4333-8333-333333333333",
			WorkspaceID:     "44444444-4444-4444-8444-444444444444",
			ProjectID:       "55555555-5555-4555-8555-555555555555",
			DesignSystemID:  "22222222-2222-4222-8222-222222222222",
			Operation:       "generate",
			AdapterID:       "opencode",
			CreatedAt:       "2026-08-03T08:00:00Z",
		},
		Engine:              PinnedEngineIdentity(),
		ArchiveObjectKey:    "workspaces/test/package.zip",
		ContentDigest:       "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ProjectArchive:      testProjectArchive(t, []testArchiveFile{{Path: "DESIGN.md", Body: "# CRM\n"}}),
		ArtifactIndex:       []ArtifactIndexEntry{},
		AgentSnapshot:       json.RawMessage(`{}`),
		InputSnapshot:       json.RawMessage(`{}`),
		Events:              json.RawMessage(`[]`),
		Failure:             json.RawMessage(`{"code":"open_design_agent_failed"}`),
		Preflight:           json.RawMessage(`{}`),
		WorkspaceProvenance: json.RawMessage(`{}`),
	})
	if err == nil || !strings.Contains(err.Error(), "archive does not match the content digest") {
		t.Fatalf("BuildRunEvidenceArchive digest mismatch error = %v", err)
	}
}

func readEvidenceArchiveFiles(t *testing.T, archive []byte) map[string][]byte {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		t.Fatalf("open evidence ZIP: %v", err)
	}
	files := make(map[string][]byte, len(reader.File))
	for _, file := range reader.File {
		entry, err := file.Open()
		if err != nil {
			t.Fatalf("open evidence entry %q: %v", file.Name, err)
		}
		body, readErr := io.ReadAll(entry)
		closeErr := entry.Close()
		if readErr != nil || closeErr != nil {
			t.Fatalf("read evidence entry %q: read=%v close=%v", file.Name, readErr, closeErr)
		}
		files[file.Name] = body
	}
	return files
}
