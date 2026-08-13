package opendesign

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	DraftPackageManifestSchema         = "multica.open-design-draft-package/v1"
	DraftPackageValidationSchema       = "multica.open-design-draft-validation/v1"
	BasePackageReferenceSchema         = "multica.open-design-base-package-reference/v1"
	DraftPackageFormat                 = "open-design-project-archive"
	DraftDesignMDPath                  = "DESIGN.md"
	DraftTokensCSSPath                 = "colors_and_type.css"
	DraftUIKitHTMLPath                 = "ui_kits/app/index.html"
	draftArtifactMaxBytes        int64 = 16 << 20
)

type BasePackageReference struct {
	Schema        string `json:"schema"`
	Slot          string `json:"slot"`
	ContentDigest string `json:"content_digest"`
	SourceTaskID  string `json:"source_task_id"`
}

func ValidateBasePackageReference(reference BasePackageReference) error {
	if reference.Schema != BasePackageReferenceSchema {
		return errors.New("Open Design base package reference schema is invalid")
	}
	if reference.Slot != "draft" && reference.Slot != "saved" {
		return errors.New("Open Design base package reference slot is invalid")
	}
	if err := ValidateContentDigest(reference.ContentDigest); err != nil {
		return err
	}
	parsed, err := uuid.Parse(reference.SourceTaskID)
	if err != nil || parsed.String() != reference.SourceTaskID {
		return errors.New("Open Design base package reference source task is invalid")
	}
	return nil
}

type DraftRunReference struct {
	SupervisorRunID string `json:"supervisor_run_id"`
	WorkerRunID     string `json:"worker_run_id"`
	TaskID          string `json:"task_id"`
	DesignSystemID  string `json:"design_system_id"`
	Operation       string `json:"operation"`
}

type DraftArchiveEvidence struct {
	ObjectKey     string               `json:"object_key"`
	ContentDigest string               `json:"content_digest"`
	ArtifactIndex []ArtifactIndexEntry `json:"artifact_index"`
}

type DraftPackageManifest struct {
	Schema             string               `json:"schema"`
	Format             string               `json:"format"`
	Engine             EngineIdentity       `json:"engine"`
	Run                DraftRunReference    `json:"run"`
	Archive            DraftArchiveEvidence `json:"archive"`
	ResultPackage      json.RawMessage      `json:"result_package"`
	CompatibilityFiles DraftArtifactSources `json:"compatibility_files"`
}

type DraftPackageValidation struct {
	Schema  string                     `json:"schema"`
	Passed  bool                       `json:"passed"`
	Audit   PackageAuditReceipt        `json:"audit"`
	Preview PreviewVerificationReceipt `json:"preview"`
}

type DraftArtifactSource struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

type DraftArtifactSources struct {
	DesignMD       DraftArtifactSource `json:"design_md"`
	TokensCSS      DraftArtifactSource `json:"tokens_css"`
	ComponentsHTML DraftArtifactSource `json:"components_html"`
}

type DraftCompatibilityArtifacts struct {
	DesignMD       string
	TokensCSS      string
	ComponentsHTML string
	Sources        DraftArtifactSources
}

type DraftArchiveArtifact struct {
	Path string
	MIME string
	Body []byte
}

func ExtractDraftCompatibilityArtifacts(archive []byte, artifactIndex []ArtifactIndexEntry, contentDigest string) (DraftCompatibilityArtifacts, error) {
	if err := validateDraftArtifactIndex(artifactIndex, contentDigest); err != nil {
		return DraftCompatibilityArtifacts{}, err
	}
	archiveIndex, err := indexProjectArchive(archive, nil)
	if err != nil {
		return DraftCompatibilityArtifacts{}, err
	}
	if err := compareDraftArtifactIndex(archiveIndex, artifactIndex); err != nil {
		return DraftCompatibilityArtifacts{}, err
	}

	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return DraftCompatibilityArtifacts{}, fmt.Errorf("parse Open Design project archive: %w", err)
	}
	indexed := make(map[string]ArtifactIndexEntry, len(artifactIndex))
	for _, entry := range artifactIndex {
		indexed[entry.Path] = entry
	}
	contents := make(map[string]string, 3)
	for _, file := range reader.File {
		if file.Name != DraftDesignMDPath && file.Name != DraftTokensCSSPath && file.Name != DraftUIKitHTMLPath {
			continue
		}
		if file.UncompressedSize64 > uint64(draftArtifactMaxBytes) {
			return DraftCompatibilityArtifacts{}, fmt.Errorf("Open Design draft source %q exceeds the size limit", file.Name)
		}
		entry, openErr := file.Open()
		if openErr != nil {
			return DraftCompatibilityArtifacts{}, fmt.Errorf("open Open Design draft source %q: %w", file.Name, openErr)
		}
		body, readErr := io.ReadAll(io.LimitReader(entry, draftArtifactMaxBytes+1))
		closeErr := entry.Close()
		if readErr != nil || closeErr != nil {
			return DraftCompatibilityArtifacts{}, fmt.Errorf("read Open Design draft source %q: %w", file.Name, errors.Join(readErr, closeErr))
		}
		if int64(len(body)) > draftArtifactMaxBytes || !utf8.Valid(body) {
			return DraftCompatibilityArtifacts{}, fmt.Errorf("Open Design draft source %q is not valid bounded UTF-8", file.Name)
		}
		contents[file.Name] = string(body)
	}
	for _, required := range []string{DraftDesignMDPath, DraftTokensCSSPath, DraftUIKitHTMLPath} {
		if _, ok := contents[required]; !ok {
			return DraftCompatibilityArtifacts{}, fmt.Errorf("Open Design archive is missing draft source %q", required)
		}
	}

	return DraftCompatibilityArtifacts{
		DesignMD:       contents[DraftDesignMDPath],
		TokensCSS:      contents[DraftTokensCSSPath],
		ComponentsHTML: contents[DraftUIKitHTMLPath],
		Sources: DraftArtifactSources{
			DesignMD:       draftArtifactSource(indexed[DraftDesignMDPath]),
			TokensCSS:      draftArtifactSource(indexed[DraftTokensCSSPath]),
			ComponentsHTML: draftArtifactSource(indexed[DraftUIKitHTMLPath]),
		},
	}, nil
}

func ReadDraftArchiveArtifact(
	archive []byte,
	artifactIndex []ArtifactIndexEntry,
	contentDigest string,
	artifactPath string,
) (DraftArchiveArtifact, error) {
	if err := validateDraftArtifactIndex(artifactIndex, contentDigest); err != nil {
		return DraftArchiveArtifact{}, err
	}
	name, err := validateArchivePath(artifactPath)
	if err != nil {
		return DraftArchiveArtifact{}, fmt.Errorf("invalid Open Design archive artifact path %q: %w", artifactPath, err)
	}
	var expected ArtifactIndexEntry
	foundInIndex := false
	for _, entry := range artifactIndex {
		if entry.Path == name {
			expected = entry
			foundInIndex = true
			break
		}
	}
	if !foundInIndex {
		return DraftArchiveArtifact{}, fmt.Errorf("Open Design archive artifact %q is not indexed", name)
	}
	if expected.Size > maxCollectedArchiveFileBytes {
		return DraftArchiveArtifact{}, fmt.Errorf("Open Design archive artifact %q exceeds the size limit", name)
	}

	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return DraftArchiveArtifact{}, fmt.Errorf("parse Open Design project archive: %w", err)
	}
	seen := make(map[string]struct{}, len(reader.File))
	var selected *zip.File
	for _, file := range reader.File {
		entryName := strings.TrimSuffix(file.Name, "/")
		if entryName == "" || file.FileInfo().IsDir() {
			continue
		}
		if _, err := validateArchivePath(entryName); err != nil {
			return DraftArchiveArtifact{}, fmt.Errorf("invalid Open Design archive entry %q: %w", file.Name, err)
		}
		if file.Mode()&fs.ModeSymlink != 0 || !file.Mode().IsRegular() {
			return DraftArchiveArtifact{}, fmt.Errorf("Open Design archive entry %q is not a regular file", entryName)
		}
		if _, exists := seen[entryName]; exists {
			return DraftArchiveArtifact{}, fmt.Errorf("Open Design archive contains duplicate entry %q", entryName)
		}
		seen[entryName] = struct{}{}
		if entryName == name {
			selected = file
		}
	}
	if selected == nil {
		return DraftArchiveArtifact{}, fmt.Errorf("Open Design archive is missing indexed artifact %q", name)
	}
	if selected.UncompressedSize64 != uint64(expected.Size) {
		return DraftArchiveArtifact{}, fmt.Errorf("Open Design archive artifact %q size does not match the artifact index", name)
	}

	opened, err := selected.Open()
	if err != nil {
		return DraftArchiveArtifact{}, fmt.Errorf("open Open Design archive artifact %q: %w", name, err)
	}
	body, readErr := io.ReadAll(io.LimitReader(opened, expected.Size+1))
	closeErr := opened.Close()
	if readErr != nil || closeErr != nil {
		return DraftArchiveArtifact{}, fmt.Errorf("read Open Design archive artifact %q: %w", name, errors.Join(readErr, closeErr))
	}
	if int64(len(body)) != expected.Size {
		return DraftArchiveArtifact{}, fmt.Errorf("Open Design archive artifact %q size does not match the artifact index", name)
	}
	digest := sha256.Sum256(body)
	if hex.EncodeToString(digest[:]) != expected.SHA256 {
		return DraftArchiveArtifact{}, fmt.Errorf("Open Design archive artifact %q digest does not match the artifact index", name)
	}
	return DraftArchiveArtifact{Path: name, MIME: expected.MIME, Body: body}, nil
}

func validateDraftArtifactIndex(index []ArtifactIndexEntry, contentDigest string) error {
	if err := ValidateContentDigest(contentDigest); err != nil {
		return err
	}
	if len(index) == 0 || len(index) > maxCollectedArchiveFiles {
		return errors.New("Open Design artifact index has an invalid entry count")
	}
	for position, entry := range index {
		name, err := validateArchivePath(entry.Path)
		if err != nil {
			return fmt.Errorf("invalid Open Design artifact index path %q: %w", entry.Path, err)
		}
		if position > 0 && index[position-1].Path >= name {
			return errors.New("Open Design artifact index must be sorted by unique path")
		}
		if entry.Role != normalizedArtifactRole(entry.Role) || strings.TrimSpace(entry.MIME) == "" || entry.Size < 0 || !sha256Pattern.MatchString(entry.SHA256) {
			return fmt.Errorf("Open Design artifact index entry %q is invalid", name)
		}
	}
	if digestArtifactIndex(index) != contentDigest {
		return errors.New("Open Design artifact index does not match the content digest")
	}
	return nil
}

func compareDraftArtifactIndex(archiveIndex, persistedIndex []ArtifactIndexEntry) error {
	if len(archiveIndex) != len(persistedIndex) {
		return errors.New("Open Design archive does not match the persisted artifact index")
	}
	for position := range archiveIndex {
		actual := archiveIndex[position]
		expected := persistedIndex[position]
		if actual.Path != expected.Path || actual.Size != expected.Size || actual.SHA256 != expected.SHA256 {
			return fmt.Errorf("Open Design archive entry %q does not match the persisted artifact index", expected.Path)
		}
	}
	return nil
}

func draftArtifactSource(entry ArtifactIndexEntry) DraftArtifactSource {
	return DraftArtifactSource{Path: entry.Path, Size: entry.Size, SHA256: entry.SHA256}
}
