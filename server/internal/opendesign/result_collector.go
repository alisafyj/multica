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
	"mime"
	"path"
	"sort"
	"strconv"
	"strings"
)

const (
	ProjectExportManifestSchema             = "open-design.project-export-manifest.v1"
	maxCollectedArchiveFiles                = 20_000
	maxCollectedArchiveFileBytes      int64 = 128 << 20
	maxCollectedArchiveAggregateBytes       = 512 << 20
)

type ArtifactIndexEntry struct {
	Path   string `json:"path"`
	Role   string `json:"role"`
	MIME   string `json:"mime"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

type CollectedRunResult struct {
	ResultPackage    json.RawMessage
	ArtifactIndex    []ArtifactIndexEntry
	ArchiveObjectKey string
	ContentDigest    string
}

type projectExportManifest struct {
	Schema    string                      `json:"schema"`
	ProjectID string                      `json:"projectId"`
	Files     []projectExportManifestFile `json:"files"`
}

type projectExportManifestFile struct {
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	MIME     string `json:"mime"`
	Included bool   `json:"included"`
	Role     string `json:"role"`
}

type manifestArtifactMetadata struct {
	Size int64
	MIME string
	Role string
}

func ValidateRunResultRequest(request RunResultRequest, expectedRunID string) error {
	if request.OpenDesignRunID != expectedRunID {
		return errors.New("Open Design result callback does not match the active worker run")
	}
	if request.ArchiveObjectKey == "" || strings.TrimSpace(request.ArchiveObjectKey) != request.ArchiveObjectKey {
		return errors.New("Open Design result callback has no persisted archive object key")
	}
	if err := validateWorkerResultPackage(request.ResultPackage, expectedRunID); err != nil {
		return err
	}
	for _, pathParts := range [][]string{{"workspace", "storage", "baseDir"}, {"events", "logPath"}} {
		present, err := nestedJSONFieldPresent(request.ResultPackage, pathParts...)
		if err != nil {
			return err
		}
		if present {
			return errors.New("Open Design result package contains a local path")
		}
	}
	if len(request.ArtifactIndex) == 0 || len(request.ArtifactIndex) > maxCollectedArchiveFiles {
		return errors.New("Open Design artifact index has an invalid entry count")
	}
	for index, entry := range request.ArtifactIndex {
		name, err := validateArchivePath(entry.Path)
		if err != nil {
			return fmt.Errorf("invalid Open Design artifact index path %q: %w", entry.Path, err)
		}
		if index > 0 && request.ArtifactIndex[index-1].Path >= name {
			return errors.New("Open Design artifact index must be sorted by unique path")
		}
		if entry.Role != normalizedArtifactRole(entry.Role) || strings.TrimSpace(entry.MIME) == "" || entry.Size < 0 || !sha256Pattern.MatchString(entry.SHA256) {
			return fmt.Errorf("Open Design artifact index entry %q is invalid", name)
		}
	}
	if request.ContentDigest != digestArtifactIndex(request.ArtifactIndex) {
		return errors.New("Open Design content digest does not match the artifact index")
	}
	return nil
}

func ValidateProjectArchiveContentDigest(archive []byte, contentDigest string) error {
	if err := ValidateContentDigest(contentDigest); err != nil {
		return err
	}
	artifactIndex, err := indexProjectArchive(archive, nil)
	if err != nil {
		return err
	}
	if digestArtifactIndex(artifactIndex) != contentDigest {
		return errors.New("Open Design archive does not match the content digest")
	}
	return nil
}

func ValidateContentDigest(contentDigest string) error {
	if !strings.HasPrefix(contentDigest, "sha256:") || !sha256Pattern.MatchString(strings.TrimPrefix(contentDigest, "sha256:")) {
		return errors.New("Open Design archive content digest is invalid")
	}
	return nil
}

func CollectWorkerRunResult(
	resultPackage json.RawMessage,
	manifestJSON json.RawMessage,
	archive []byte,
	workerRunID string,
	projectID string,
) (CollectedRunResult, error) {
	if err := validateWorkerResultPackage(resultPackage, workerRunID); err != nil {
		return CollectedRunResult{}, err
	}
	sanitizedResultPackage, err := sanitizeWorkerResultPackage(resultPackage)
	if err != nil {
		return CollectedRunResult{}, err
	}
	manifestFiles, err := parseProjectExportManifest(manifestJSON, projectID)
	if err != nil {
		return CollectedRunResult{}, err
	}
	artifactIndex, err := indexProjectArchive(archive, manifestFiles)
	if err != nil {
		return CollectedRunResult{}, err
	}
	return CollectedRunResult{
		ResultPackage: sanitizedResultPackage,
		ArtifactIndex: artifactIndex,
		ContentDigest: digestArtifactIndex(artifactIndex),
	}, nil
}

func sanitizeWorkerResultPackage(raw json.RawMessage) (json.RawMessage, error) {
	var result map[string]json.RawMessage
	if err := json.Unmarshal(raw, &result); err != nil || result == nil {
		return nil, errors.New("Open Design result package is invalid JSON")
	}
	if err := removeNestedJSONField(result, "workspace", "storage", "baseDir"); err != nil {
		return nil, fmt.Errorf("sanitize Open Design workspace storage: %w", err)
	}
	if err := removeNestedJSONField(result, "events", "logPath"); err != nil {
		return nil, fmt.Errorf("sanitize Open Design event log: %w", err)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("encode sanitized Open Design result package: %w", err)
	}
	return encoded, nil
}

func removeNestedJSONField(object map[string]json.RawMessage, pathParts ...string) error {
	if len(pathParts) == 0 {
		return nil
	}
	key := pathParts[0]
	raw, exists := object[key]
	if !exists {
		return nil
	}
	if len(pathParts) == 1 {
		delete(object, key)
		return nil
	}
	var child map[string]json.RawMessage
	if err := json.Unmarshal(raw, &child); err != nil || child == nil {
		return fmt.Errorf("field %q must be an object", key)
	}
	if err := removeNestedJSONField(child, pathParts[1:]...); err != nil {
		return err
	}
	encoded, err := json.Marshal(child)
	if err != nil {
		return err
	}
	object[key] = encoded
	return nil
}

func nestedJSONFieldPresent(raw json.RawMessage, pathParts ...string) (bool, error) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil || object == nil {
		return false, errors.New("Open Design result package is invalid JSON")
	}
	for index, key := range pathParts {
		value, exists := object[key]
		if !exists {
			return false, nil
		}
		if index == len(pathParts)-1 {
			return true, nil
		}
		if err := json.Unmarshal(value, &object); err != nil || object == nil {
			return false, fmt.Errorf("Open Design result package field %q must be an object", key)
		}
	}
	return false, nil
}

func parseProjectExportManifest(raw json.RawMessage, projectID string) (map[string]manifestArtifactMetadata, error) {
	var manifest projectExportManifest
	if len(raw) == 0 || json.Unmarshal(raw, &manifest) != nil {
		return nil, errors.New("Open Design project export manifest is invalid JSON")
	}
	if manifest.Schema != ProjectExportManifestSchema {
		return nil, fmt.Errorf("Open Design project export manifest schema %q does not match %q", manifest.Schema, ProjectExportManifestSchema)
	}
	if manifest.ProjectID != projectID {
		return nil, errors.New("Open Design project export manifest does not match the active project")
	}
	files := make(map[string]manifestArtifactMetadata, len(manifest.Files))
	for _, file := range manifest.Files {
		if !file.Included {
			continue
		}
		name, err := validateArchivePath(file.Name)
		if err != nil {
			return nil, fmt.Errorf("invalid Open Design manifest file %q: %w", file.Name, err)
		}
		if file.Size < 0 {
			return nil, fmt.Errorf("Open Design manifest file %q has invalid size", name)
		}
		if _, exists := files[name]; exists {
			return nil, fmt.Errorf("Open Design project export manifest contains duplicate file %q", name)
		}
		files[name] = manifestArtifactMetadata{
			Size: file.Size,
			MIME: strings.TrimSpace(file.MIME),
			Role: normalizedArtifactRole(file.Role),
		}
	}
	return files, nil
}

func indexProjectArchive(archive []byte, manifestFiles map[string]manifestArtifactMetadata) ([]ArtifactIndexEntry, error) {
	if len(archive) == 0 {
		return nil, errors.New("Open Design project archive is empty")
	}
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return nil, fmt.Errorf("parse Open Design project archive: %w", err)
	}
	if len(reader.File) > maxCollectedArchiveFiles {
		return nil, errors.New("Open Design project archive contains too many entries")
	}

	index := make([]ArtifactIndexEntry, 0, len(reader.File))
	seen := make(map[string]struct{}, len(reader.File))
	var aggregateSize int64
	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}
		name, err := validateArchivePath(file.Name)
		if err != nil {
			return nil, fmt.Errorf("invalid Open Design archive entry %q: %w", file.Name, err)
		}
		if file.Mode()&fs.ModeSymlink != 0 {
			return nil, fmt.Errorf("Open Design archive entry %q is a symlink", name)
		}
		if _, exists := seen[name]; exists {
			return nil, fmt.Errorf("Open Design project archive contains duplicate entry %q", name)
		}
		seen[name] = struct{}{}
		if file.UncompressedSize64 > uint64(maxCollectedArchiveFileBytes) {
			return nil, fmt.Errorf("Open Design archive entry %q exceeds the size limit", name)
		}

		hasher := sha256.New()
		entry, err := file.Open()
		if err != nil {
			return nil, fmt.Errorf("open Open Design archive entry %q: %w", name, err)
		}
		written, copyErr := io.Copy(hasher, io.LimitReader(entry, maxCollectedArchiveFileBytes+1))
		closeErr := entry.Close()
		if copyErr != nil || closeErr != nil {
			return nil, fmt.Errorf("read Open Design archive entry %q: %w", name, errors.Join(copyErr, closeErr))
		}
		if written > maxCollectedArchiveFileBytes || uint64(written) != file.UncompressedSize64 {
			return nil, fmt.Errorf("Open Design archive entry %q has an invalid size", name)
		}
		aggregateSize += written
		if aggregateSize > maxCollectedArchiveAggregateBytes {
			return nil, errors.New("Open Design project archive exceeds the aggregate size limit")
		}

		metadata, declared := manifestFiles[name]
		if declared && metadata.Size != written {
			return nil, fmt.Errorf("Open Design archive entry %q does not match manifest size", name)
		}
		role := "other"
		mimeType := archiveEntryMIME(name)
		if declared {
			role = metadata.Role
			if metadata.MIME != "" {
				mimeType = metadata.MIME
			}
		}
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}
		index = append(index, ArtifactIndexEntry{
			Path:   name,
			Role:   role,
			MIME:   mimeType,
			Size:   written,
			SHA256: hex.EncodeToString(hasher.Sum(nil)),
		})
	}
	for name := range manifestFiles {
		if _, exists := seen[name]; !exists {
			return nil, fmt.Errorf("Open Design project archive is missing manifest file %q", name)
		}
	}
	sort.Slice(index, func(left, right int) bool {
		return index[left].Path < index[right].Path
	})
	return index, nil
}

func archiveEntryMIME(name string) string {
	extension := strings.ToLower(path.Ext(name))
	if extension == ".md" {
		return "text/markdown; charset=utf-8"
	}
	if mimeType := mime.TypeByExtension(extension); mimeType != "" {
		return mimeType
	}
	return "application/octet-stream"
}

func validateArchivePath(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if value != trimmed || value == "" || strings.Contains(value, "\\") || !fs.ValidPath(value) {
		return "", errors.New("path must be a normalized relative slash path")
	}
	return value, nil
}

func normalizedArtifactRole(value string) string {
	switch strings.TrimSpace(value) {
	case "entry", "artifact", "supporting", "asset", "source", "other":
		return strings.TrimSpace(value)
	default:
		return "other"
	}
}

func digestArtifactIndex(index []ArtifactIndexEntry) string {
	hasher := sha256.New()
	for _, entry := range index {
		_, _ = io.WriteString(hasher, entry.Path)
		_ = writeDigestSeparator(hasher)
		_, _ = io.WriteString(hasher, strconv.FormatInt(entry.Size, 10))
		_ = writeDigestSeparator(hasher)
		_, _ = io.WriteString(hasher, entry.SHA256)
		_ = writeDigestSeparator(hasher)
	}
	return "sha256:" + hex.EncodeToString(hasher.Sum(nil))
}
