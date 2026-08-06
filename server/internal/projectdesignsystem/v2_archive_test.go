package projectdesignsystem

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestCollectV2DirectoryBuildsDeterministicManifestAndArchive(t *testing.T) {
	root := copyV2Fixture(t)
	binding := validV2Binding()

	first, err := CollectV2Directory(root, binding)
	if err != nil {
		t.Fatalf("CollectV2Directory() error = %v", err)
	}
	second, err := CollectV2Directory(root, binding)
	if err != nil {
		t.Fatalf("second CollectV2Directory() error = %v", err)
	}
	if !bytes.Equal(first.Archive, second.Archive) {
		t.Fatal("CollectV2Directory() archive is not byte deterministic")
	}
	if !reflect.DeepEqual(first.Manifest, second.Manifest) {
		t.Fatal("CollectV2Directory() manifest is not deterministic")
	}
	if first.Manifest.SchemaVersion != PackageSchemaV2 || first.Manifest.Binding != binding {
		t.Fatalf("manifest identity = %#v", first.Manifest)
	}
	if !strings.HasPrefix(first.Manifest.ContentDigest, "sha256:") || len(first.Manifest.ContentDigest) != 71 {
		t.Fatalf("content digest = %q", first.Manifest.ContentDigest)
	}
	if !sort.SliceIsSorted(first.Manifest.Files, func(i, j int) bool {
		return first.Manifest.Files[i].Path < first.Manifest.Files[j].Path
	}) {
		t.Fatalf("manifest files are not sorted: %#v", first.Manifest.Files)
	}
	if len(first.Manifest.PreviewTargets) != 1 || first.Manifest.PreviewTargets[0].Path != "ui-kit/index.html" {
		t.Fatalf("preview targets = %#v", first.Manifest.PreviewTargets)
	}
	design, err := ReadV2Artifact(first.Archive, first.Manifest.Files, "DESIGN.md")
	if err != nil {
		t.Fatalf("ReadV2Artifact() error = %v", err)
	}
	wantDesign, err := os.ReadFile(filepath.Join(root, "DESIGN.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(design, wantDesign) {
		t.Fatal("ReadV2Artifact() returned different bytes")
	}

	digestA, err := SnapshotDigest(json.RawMessage(`{"project":"crm","platform":"web"}`))
	if err != nil {
		t.Fatalf("SnapshotDigest() error = %v", err)
	}
	digestB, err := SnapshotDigest(json.RawMessage(" { \n \"platform\" : \"web\", \"project\" : \"crm\" } "))
	if err != nil {
		t.Fatalf("SnapshotDigest() reordered error = %v", err)
	}
	if digestA != digestB {
		t.Fatalf("SnapshotDigest() = %q and %q for equivalent JSON", digestA, digestB)
	}
}

func TestCollectV2DirectoryRequiresStableCoreAndPreview(t *testing.T) {
	for _, name := range []string{"DESIGN.md", "tokens.css", "source/index.json", "ui-kit/index.html"} {
		t.Run(name, func(t *testing.T) {
			root := copyV2Fixture(t)
			if err := os.Remove(filepath.Join(root, filepath.FromSlash(name))); err != nil {
				t.Fatal(err)
			}
			if _, err := CollectV2Directory(root, validV2Binding()); err == nil {
				t.Fatalf("CollectV2Directory() accepted package without %s", name)
			}
		})
	}
}

func TestCollectV2DirectoryRejectsUnknownTopLevelFiles(t *testing.T) {
	for _, name := range []string{"README.txt", "manifest.json", "components.html"} {
		t.Run(name, func(t *testing.T) {
			root := copyV2Fixture(t)
			if err := os.WriteFile(filepath.Join(root, name), []byte("unexpected"), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := CollectV2Directory(root, validV2Binding()); err == nil {
				t.Fatalf("CollectV2Directory() accepted undeclared file %s", name)
			}
		})
	}
}

func TestCollectV2DirectoryRejectsUndeclaredDirectories(t *testing.T) {
	for _, name := range []string{"source/private", "ui-kit/nested", "preview/nested"} {
		t.Run(name, func(t *testing.T) {
			root := copyV2Fixture(t)
			if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(name)), 0o755); err != nil {
				t.Fatal(err)
			}
			_, err := CollectV2Directory(root, validV2Binding())
			assertV2ErrorCode(t, err, "archive_path_undeclared")
		})
	}
}

func TestCollectV2DirectoryRejectsSymlinkHardlinkAndTraversal(t *testing.T) {
	t.Run("symlink", func(t *testing.T) {
		root := copyV2Fixture(t)
		if err := os.Symlink(filepath.Join(root, "DESIGN.md"), filepath.Join(root, "assets", "design-link.md")); err != nil {
			t.Skipf("symlink unsupported: %v", err)
		}
		_, err := CollectV2Directory(root, validV2Binding())
		assertV2ErrorCode(t, err, "archive_link_forbidden")
	})

	t.Run("hardlink", func(t *testing.T) {
		root := copyV2Fixture(t)
		if err := os.Link(filepath.Join(root, "DESIGN.md"), filepath.Join(root, "assets", "design-hardlink.md")); err != nil {
			t.Skipf("hardlink unsupported: %v", err)
		}
		_, err := CollectV2Directory(root, validV2Binding())
		assertV2ErrorCode(t, err, "archive_hardlink_forbidden")
	})

	t.Run("archive traversal", func(t *testing.T) {
		entries := readV2ZipEntries(t, collectValidV2(t, validV2Binding()).Archive)
		ordered := v2ZipEntriesFromMap(entries)
		ordered = append(ordered, v2ZipEntry{name: "../escape", contents: []byte("outside")})
		pkg, err := ValidateV2Archive(buildV2Zip(t, ordered), validV2Binding())
		assertV2DiagnosticCode(t, pkg.Audit, err, "archive_path_invalid")
	})
}

func TestCollectV2DirectoryEnforcesFileCountAndByteLimits(t *testing.T) {
	t.Run("file count", func(t *testing.T) {
		root := copyV2Fixture(t)
		for index := 0; index < 512; index++ {
			name := filepath.Join(root, "assets", "generated", formatV2TestIndex(index)+".png")
			if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(name, []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		_, err := CollectV2Directory(root, validV2Binding())
		assertV2ErrorCode(t, err, "archive_file_count_exceeded")
	})

	t.Run("single file bytes", func(t *testing.T) {
		root := copyV2Fixture(t)
		if err := os.WriteFile(filepath.Join(root, "assets", "oversized.png"), bytes.Repeat([]byte{'x'}, (16<<20)+1), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := CollectV2Directory(root, validV2Binding())
		assertV2ErrorCode(t, err, "archive_file_too_large")
	})
}

func TestValidateV2ArchiveRecomputesEveryDigest(t *testing.T) {
	collected := collectValidV2(t, validV2Binding())

	t.Run("empty archive", func(t *testing.T) {
		pkg, err := ValidateV2Archive(buildV2Zip(t, nil), validV2Binding())
		assertV2DiagnosticCode(t, pkg.Audit, err, "manifest_missing")
	})

	t.Run("artifact bytes", func(t *testing.T) {
		entries := readV2ZipEntries(t, collected.Archive)
		entries["DESIGN.md"] = append(entries["DESIGN.md"], []byte("\nTampered.\n")...)
		archive := buildV2ZipFromMap(t, entries)
		pkg, err := ValidateV2Archive(archive, validV2Binding())
		assertV2DiagnosticCode(t, pkg.Audit, err, "manifest_index_mismatch")
	})

	t.Run("manifest index", func(t *testing.T) {
		entries := readV2ZipEntries(t, collected.Archive)
		var manifest ManifestV2
		if err := json.Unmarshal(entries["manifest.json"], &manifest); err != nil {
			t.Fatal(err)
		}
		manifest.Files[0].Role = "tampered"
		entries["manifest.json"], _ = json.Marshal(manifest)
		pkg, err := ValidateV2Archive(buildV2ZipFromMap(t, entries), validV2Binding())
		assertV2DiagnosticCode(t, pkg.Audit, err, "manifest_index_mismatch")
	})

	t.Run("duplicate archive entry", func(t *testing.T) {
		entries := readV2ZipEntries(t, collected.Archive)
		ordered := make([]v2ZipEntry, 0, len(entries)+1)
		for name, contents := range entries {
			ordered = append(ordered, v2ZipEntry{name: name, contents: contents})
		}
		ordered = append(ordered, v2ZipEntry{name: "DESIGN.md", contents: entries["DESIGN.md"]})
		pkg, err := ValidateV2Archive(buildV2Zip(t, ordered), validV2Binding())
		assertV2DiagnosticCode(t, pkg.Audit, err, "archive_duplicate_path")
	})

	t.Run("zip bomb", func(t *testing.T) {
		entries := readV2ZipEntries(t, collected.Archive)
		entries["assets/bomb.png"] = bytes.Repeat([]byte{'0'}, (16<<20)+1)
		pkg, err := ValidateV2Archive(buildV2ZipFromMap(t, entries), validV2Binding())
		assertV2DiagnosticCode(t, pkg.Audit, err, "archive_file_too_large")
	})
}

func TestValidateV2ArchivePreflightsEOCDMetadata(t *testing.T) {
	tests := []struct {
		name    string
		archive []byte
		code    string
	}{
		{
			name:    "entry count",
			archive: buildV2EOCD(nil, 0, 0, maxV2Files+1, maxV2Files+1, 0, 0),
			code:    "archive_file_count_exceeded",
		},
		{
			name:    "ZIP64 sentinel",
			archive: buildV2EOCD(nil, 0, 0, ^uint16(0), ^uint16(0), ^uint32(0), ^uint32(0)),
			code:    "archive_invalid",
		},
		{
			name: "ZIP64 locator",
			archive: buildV2EOCD([]byte{
				0x50, 0x4b, 0x06, 0x07,
				0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
			}, 0, 0, 0, 0, 0, 0),
			code: "archive_invalid",
		},
		{
			name:    "multi disk",
			archive: buildV2EOCD(nil, 1, 1, 0, 0, 0, 0),
			code:    "archive_invalid",
		},
		{
			name:    "central directory bounds",
			archive: buildV2EOCD(nil, 0, 0, 0, 0, 1, 22),
			code:    "archive_invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkg, err := ValidateV2Archive(tt.archive, validV2Binding())
			assertV2DiagnosticCode(t, pkg.Audit, err, tt.code)
		})
	}
}

func TestValidateV2ArchiveBindsTaskInputAndBaseDigest(t *testing.T) {
	generate := validV2Binding()
	collected := collectValidV2(t, generate)

	mismatchedTask := generate
	mismatchedTask.TaskID = "task-other"
	if _, err := ValidateV2Archive(collected.Archive, mismatchedTask); err == nil {
		t.Fatal("ValidateV2Archive() accepted a different task binding")
	}

	root := copyV2Fixture(t)
	writeV2SourceIndex(t, root, SourceIndex{
		SchemaVersion:       SourceIndexSchemaV1,
		InputSnapshotSHA256: "sha256:" + strings.Repeat("b", 64),
		Evidence:            []SourceEvidence{},
		Conflicts:           []SourceConflict{},
		Fallbacks:           []SourceFallback{},
	})
	if _, err := CollectV2Directory(root, generate); err == nil {
		t.Fatal("CollectV2Directory() accepted a source index for another input snapshot")
	}

	adjust := generate
	adjust.Operation = "adjust"
	adjust.BasePackageSHA256 = "sha256:" + strings.Repeat("c", 64)
	adjusted := collectValidV2(t, adjust)
	mismatchedBase := adjust
	mismatchedBase.BasePackageSHA256 = "sha256:" + strings.Repeat("d", 64)
	if _, err := ValidateV2Archive(adjusted.Archive, mismatchedBase); err == nil {
		t.Fatal("ValidateV2Archive() accepted a different base package digest")
	}

	missingBase := adjust
	missingBase.BasePackageSHA256 = ""
	if _, err := CollectV2Directory(copyV2Fixture(t), missingBase); err == nil {
		t.Fatal("CollectV2Directory() accepted adjust without a base digest")
	}
}

func TestDiscoverV2PreviewTargetsPrefersUIKitAndSortsPreviews(t *testing.T) {
	index := []ArtifactIndexEntry{
		{Path: "preview/zeta.html", Role: "preview", MediaType: "text/html; charset=utf-8"},
		{Path: "preview/alpha.html", Role: "preview", MediaType: "text/html; charset=utf-8"},
		{Path: "ui-kit/index.html", Role: "ui_kit", MediaType: "text/html; charset=utf-8"},
	}
	targets, err := DiscoverV2PreviewTargets(index)
	if err != nil {
		t.Fatalf("DiscoverV2PreviewTargets() error = %v", err)
	}
	want := []PreviewTarget{
		{ID: "ui-kit", Kind: "ui_kit", Path: "ui-kit/index.html"},
		{ID: "alpha", Kind: "preview", Path: "preview/alpha.html"},
		{ID: "zeta", Kind: "preview", Path: "preview/zeta.html"},
	}
	if !reflect.DeepEqual(targets, want) {
		t.Fatalf("targets = %#v, want %#v", targets, want)
	}

	tooMany := make([]ArtifactIndexEntry, 0, 9)
	for index := 0; index < 9; index++ {
		tooMany = append(tooMany, ArtifactIndexEntry{
			Path:      "preview/preview-" + formatV2TestIndex(index) + ".html",
			Role:      "preview",
			MediaType: "text/html; charset=utf-8",
		})
	}
	if _, err := DiscoverV2PreviewTargets(tooMany); err == nil {
		t.Fatal("DiscoverV2PreviewTargets() accepted more than eight targets")
	}
}

func TestDiscoverV2PreviewTargetsRejectsUIKitIDCollision(t *testing.T) {
	index := []ArtifactIndexEntry{
		{Path: "ui-kit/index.html", Role: "ui_kit", MediaType: "text/html; charset=utf-8"},
		{Path: "preview/ui-kit.html", Role: "preview", MediaType: "text/html; charset=utf-8"},
	}
	if _, err := DiscoverV2PreviewTargets(index); err == nil {
		t.Fatal("DiscoverV2PreviewTargets() accepted duplicate UI Kit and Preview target IDs")
	}
}

func TestDiscoverV2PreviewTargetsRejectsInvalidPreviewPaths(t *testing.T) {
	for _, previewPath := range []string{
		"preview/nested/a.html",
		"preview/../a.html",
		"preview//a.html",
		"preview/./a.html",
		`preview\a.html`,
		"assets/a.html",
	} {
		t.Run(previewPath, func(t *testing.T) {
			index := []ArtifactIndexEntry{
				{Path: "ui-kit/index.html", Role: "ui_kit", MediaType: "text/html; charset=utf-8"},
				{Path: previewPath, Role: "preview", MediaType: "text/html; charset=utf-8"},
			}
			if _, err := DiscoverV2PreviewTargets(index); err == nil {
				t.Fatalf("DiscoverV2PreviewTargets() accepted invalid Preview path %q", previewPath)
			}
		})
	}
}

func validV2Binding() PackageBinding {
	return PackageBinding{
		WorkspaceID:         "workspace-1",
		ProjectID:           "project-1",
		DesignSystemID:      "design-system-1",
		TaskID:              "task-1",
		AgentID:             "agent-1",
		Operation:           "generate",
		InputSnapshotSHA256: "sha256:" + strings.Repeat("a", 64),
	}
}

func collectValidV2(t *testing.T, binding PackageBinding) CollectedV2Package {
	t.Helper()
	root := copyV2Fixture(t)
	if binding.InputSnapshotSHA256 != validV2Binding().InputSnapshotSHA256 {
		writeV2SourceIndex(t, root, SourceIndex{
			SchemaVersion:       SourceIndexSchemaV1,
			InputSnapshotSHA256: binding.InputSnapshotSHA256,
			Evidence: []SourceEvidence{{
				ID:         "crm-orders-page",
				Kind:       "repository_fact",
				Summary:    "The CRM order page uses a dense table layout.",
				References: []string{"apps/crm/orders/page.tsx"},
			}},
			Conflicts: []SourceConflict{},
			Fallbacks: []SourceFallback{},
		})
	}
	collected, err := CollectV2Directory(root, binding)
	if err != nil {
		t.Fatalf("CollectV2Directory() error = %v", err)
	}
	return collected
}

func copyV2Fixture(t *testing.T) string {
	t.Helper()
	destination := t.TempDir()
	err := filepath.WalkDir(filepath.Join("testdata", "v2-valid"), func(source string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(filepath.Join("testdata", "v2-valid"), source)
		if err != nil || relative == "." {
			return err
		}
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		contents, err := os.ReadFile(source)
		if err != nil {
			return err
		}
		return os.WriteFile(target, contents, 0o644)
	})
	if err != nil {
		t.Fatalf("copy V2 fixture: %v", err)
	}
	return destination
}

func writeV2SourceIndex(t *testing.T, root string, source SourceIndex) {
	t.Helper()
	encoded, err := json.Marshal(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "source", "index.json"), encoded, 0o644); err != nil {
		t.Fatal(err)
	}
}

type v2ZipEntry struct {
	name     string
	contents []byte
}

func buildV2Zip(t *testing.T, entries []v2ZipEntry) []byte {
	t.Helper()
	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	for _, entry := range entries {
		file, err := writer.Create(entry.name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write(entry.contents); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func buildV2ZipFromMap(t *testing.T, entries map[string][]byte) []byte {
	t.Helper()
	return buildV2Zip(t, v2ZipEntriesFromMap(entries))
}

func buildV2EOCD(prefix []byte, diskNumber, centralDirectoryDisk, entriesOnDisk, totalEntries uint16, centralDirectorySize, centralDirectoryOffset uint32) []byte {
	archive := make([]byte, len(prefix)+22)
	copy(archive, prefix)
	eocd := archive[len(prefix):]
	binary.LittleEndian.PutUint32(eocd[0:4], 0x06054b50)
	binary.LittleEndian.PutUint16(eocd[4:6], diskNumber)
	binary.LittleEndian.PutUint16(eocd[6:8], centralDirectoryDisk)
	binary.LittleEndian.PutUint16(eocd[8:10], entriesOnDisk)
	binary.LittleEndian.PutUint16(eocd[10:12], totalEntries)
	binary.LittleEndian.PutUint32(eocd[12:16], centralDirectorySize)
	binary.LittleEndian.PutUint32(eocd[16:20], centralDirectoryOffset)
	return archive
}

func v2ZipEntriesFromMap(entries map[string][]byte) []v2ZipEntry {
	names := make([]string, 0, len(entries))
	for name := range entries {
		names = append(names, name)
	}
	sort.Strings(names)
	ordered := make([]v2ZipEntry, 0, len(names))
	for _, name := range names {
		ordered = append(ordered, v2ZipEntry{name: name, contents: entries[name]})
	}
	return ordered
}

func readV2ZipEntries(t *testing.T, archive []byte) map[string][]byte {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		t.Fatal(err)
	}
	entries := make(map[string][]byte, len(reader.File))
	for _, file := range reader.File {
		stream, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		contents, err := io.ReadAll(stream)
		closeErr := stream.Close()
		if err != nil || closeErr != nil {
			t.Fatalf("read zip entry %s: %v", file.Name, err)
		}
		entries[file.Name] = contents
	}
	return entries
}

func formatV2TestIndex(index int) string {
	return string([]byte{
		byte('0' + (index/100)%10),
		byte('0' + (index/10)%10),
		byte('0' + index%10),
	})
}

func assertV2ErrorCode(t *testing.T, err error, code string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), code) {
		t.Fatalf("error = %v, want diagnostic code %q", err, code)
	}
}

func assertV2DiagnosticCode(t *testing.T, report AuditReport, err error, code string) {
	t.Helper()
	assertV2ErrorCode(t, err, code)
	for _, diagnostic := range report.Diagnostics {
		if diagnostic.Code == code {
			return
		}
	}
	t.Fatalf("diagnostics = %#v, want code %q", report.Diagnostics, code)
}
