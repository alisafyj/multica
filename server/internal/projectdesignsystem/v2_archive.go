package projectdesignsystem

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
)

type v2ArchiveError struct {
	code    string
	path    string
	message string
}

func (err *v2ArchiveError) Error() string {
	return err.code + ": " + err.message
}

func SnapshotDigest(raw json.RawMessage) (string, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return "", fmt.Errorf("decode input snapshot: %w", err)
	}
	if err := requireJSONEOF(decoder); err != nil {
		return "", err
	}
	canonical, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("encode input snapshot: %w", err)
	}
	return sha256String(canonical), nil
}

func CollectV2Directory(root string, binding PackageBinding) (CollectedV2Package, error) {
	if err := validateV2Binding(binding); err != nil {
		return CollectedV2Package{}, err
	}
	rootInfo, err := os.Lstat(root)
	if err != nil {
		return CollectedV2Package{}, fmt.Errorf("inspect V2 package root: %w", err)
	}
	if rootInfo.Mode()&fs.ModeSymlink != 0 || !rootInfo.IsDir() {
		return CollectedV2Package{}, errors.New("V2 package root must be a real directory")
	}

	files := make(map[string][]byte)
	index := make([]ArtifactIndexEntry, 0)
	seenFileInfo := make([]fs.FileInfo, 0)
	var totalBytes int64
	err = filepath.WalkDir(root, func(filePath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if filePath == root {
			return nil
		}
		relative, err := filepath.Rel(root, filePath)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(relative)
		if entry.Type()&fs.ModeSymlink != 0 {
			return archiveV2Error("archive_link_forbidden", name, "links are not allowed in a V2 package")
		}
		if entry.IsDir() {
			return validateV2DirectoryPath(name)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return archiveV2Error("archive_type_forbidden", name, "only regular files are allowed")
		}
		if hasMultipleHardlinks(info) {
			return archiveV2Error("archive_hardlink_forbidden", name, "hardlinks are not allowed")
		}
		for _, previous := range seenFileInfo {
			if os.SameFile(previous, info) {
				return archiveV2Error("archive_hardlink_forbidden", name, "hardlinks are not allowed")
			}
		}
		seenFileInfo = append(seenFileInfo, info)

		role, mediaType, limit, err := classifyV2Artifact(name)
		if err != nil {
			return err
		}
		if info.Size() > limit {
			return archiveV2Error("archive_file_too_large", name, "file exceeds its size limit")
		}
		contents, err := readBoundedFile(filePath, limit)
		if err != nil {
			return err
		}
		totalBytes += int64(len(contents))
		if totalBytes > maxV2TotalBytes {
			return archiveV2Error("archive_total_too_large", name, "package exceeds its uncompressed size limit")
		}
		files[name] = contents
		index = append(index, ArtifactIndexEntry{
			Path:      name,
			Role:      role,
			MediaType: mediaType,
			SizeBytes: int64(len(contents)),
			SHA256:    sha256Hex(contents),
		})
		if len(index)+1 > maxV2Files {
			return archiveV2Error("archive_file_count_exceeded", name, "package contains too many files")
		}
		return nil
	})
	if err != nil {
		return CollectedV2Package{}, err
	}
	sort.Slice(index, func(left, right int) bool { return index[left].Path < index[right].Path })
	previewTargets, err := DiscoverV2PreviewTargets(index)
	if err != nil {
		return CollectedV2Package{}, err
	}
	contentDigest := digestV2ArtifactIndex(index)
	audit := auditV2Package(files, index, binding, contentDigest, previewTargets)
	manifest := ManifestV2{
		SchemaVersion:  PackageSchemaV2,
		Binding:        binding,
		ContentDigest:  contentDigest,
		Files:          nonNilV2Files(index),
		PreviewTargets: nonNilPreviewTargets(previewTargets),
		Sections:       nonNilSections(audit.sections),
		TokenGroups:    nonNilTokenGroups(audit.tokenGroups),
		Locators:       nonNilLocators(audit.locators),
	}
	collected := CollectedV2Package{Manifest: manifest, Audit: audit.report}
	if !audit.report.Passed {
		return collected, v2AuditError(audit.report)
	}
	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		return CollectedV2Package{}, fmt.Errorf("encode V2 manifest: %w", err)
	}
	files["manifest.json"] = manifestJSON
	archive, err := buildDeterministicV2Archive(files)
	if err != nil {
		return CollectedV2Package{}, err
	}
	if len(archive) > maxV2ArchiveBytes {
		return CollectedV2Package{}, archiveV2Error("archive_compressed_too_large", "", "archive exceeds its compressed size limit")
	}
	validated, err := ValidateV2Archive(archive, binding)
	if err != nil {
		return CollectedV2Package{}, err
	}
	return CollectedV2Package{Archive: archive, Manifest: validated.Manifest, Audit: validated.Audit}, nil
}

func ValidateV2Archive(archive []byte, expected PackageBinding) (ValidatedV2Package, error) {
	if err := validateV2Binding(expected); err != nil {
		return invalidValidatedV2("binding_invalid", "manifest.json", err.Error(), "")
	}
	if len(archive) == 0 || len(archive) > maxV2ArchiveBytes {
		return invalidValidatedV2("archive_compressed_too_large", "", "archive is empty or exceeds its compressed size limit", "")
	}
	files, index, manifestJSON, err := readAndIndexV2Archive(archive)
	if err != nil {
		var archiveErr *v2ArchiveError
		if errors.As(err, &archiveErr) {
			return invalidValidatedV2(archiveErr.code, archiveErr.path, archiveErr.message, "")
		}
		return ValidatedV2Package{}, err
	}
	var manifest ManifestV2
	if err := decodeStrictJSON(manifestJSON, &manifest); err != nil {
		return invalidValidatedV2("manifest_invalid", "manifest.json", err.Error(), "")
	}
	result := ValidatedV2Package{Manifest: manifest}
	if manifest.SchemaVersion != PackageSchemaV2 {
		return invalidValidatedV2("manifest_schema_invalid", "manifest.json", "manifest schema is not V2", manifest.ContentDigest)
	}
	if manifest.Binding != expected {
		return invalidValidatedV2("manifest_binding_mismatch", "manifest.json", "manifest does not match the expected task binding", manifest.ContentDigest)
	}
	if err := validateV2Binding(manifest.Binding); err != nil {
		return invalidValidatedV2("manifest_binding_invalid", "manifest.json", err.Error(), manifest.ContentDigest)
	}
	if !reflect.DeepEqual(manifest.Files, index) {
		return invalidValidatedV2("manifest_index_mismatch", "manifest.json", "manifest file index does not exactly match archive contents", manifest.ContentDigest)
	}
	contentDigest := digestV2ArtifactIndex(index)
	if manifest.ContentDigest != contentDigest {
		return invalidValidatedV2("content_digest_mismatch", "manifest.json", "manifest content digest does not match the recomputed index", contentDigest)
	}
	previewTargets, err := DiscoverV2PreviewTargets(index)
	if err != nil {
		return invalidValidatedV2("preview_targets_invalid", "manifest.json", err.Error(), contentDigest)
	}
	if !reflect.DeepEqual(manifest.PreviewTargets, previewTargets) {
		return invalidValidatedV2("manifest_preview_targets_mismatch", "manifest.json", "manifest Preview targets do not match archive contents", contentDigest)
	}
	audit := auditV2Package(files, index, manifest.Binding, contentDigest, previewTargets)
	result.Audit = audit.report
	if !audit.report.Passed {
		return result, v2AuditError(audit.report)
	}
	if !reflect.DeepEqual(manifest.Sections, nonNilSections(audit.sections)) ||
		!reflect.DeepEqual(manifest.TokenGroups, nonNilTokenGroups(audit.tokenGroups)) ||
		!reflect.DeepEqual(manifest.Locators, nonNilLocators(audit.locators)) {
		return invalidValidatedV2("manifest_audit_index_mismatch", "manifest.json", "manifest derived indexes do not match audited artifacts", contentDigest)
	}
	return result, nil
}

func ReadV2Artifact(archive []byte, index []ArtifactIndexEntry, name string) ([]byte, error) {
	if _, _, _, err := classifyV2Artifact(name); err != nil {
		return nil, err
	}
	files, actualIndex, manifestJSON, err := readAndIndexV2Archive(archive)
	if err != nil {
		return nil, err
	}
	var manifest ManifestV2
	if err := decodeStrictJSON(manifestJSON, &manifest); err != nil {
		return nil, err
	}
	validated, err := ValidateV2Archive(archive, manifest.Binding)
	if err != nil {
		return nil, err
	}
	if !reflect.DeepEqual(index, actualIndex) || !reflect.DeepEqual(index, validated.Manifest.Files) {
		return nil, errors.New("V2 artifact index does not match the archive")
	}
	contents, exists := files[name]
	if !exists {
		return nil, fs.ErrNotExist
	}
	return append([]byte(nil), contents...), nil
}

func readAndIndexV2Archive(archive []byte) (map[string][]byte, []ArtifactIndexEntry, []byte, error) {
	if err := preflightV2ArchiveEOCD(archive); err != nil {
		return nil, nil, nil, err
	}
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return nil, nil, nil, archiveV2Error("archive_invalid", "", "archive is not a valid ZIP")
	}
	if len(reader.File) > maxV2Files {
		return nil, nil, nil, archiveV2Error("archive_file_count_exceeded", "", "archive contains too many entries")
	}
	artifactCapacity := len(reader.File)
	if artifactCapacity > 0 {
		artifactCapacity--
	}
	files := make(map[string][]byte, artifactCapacity)
	index := make([]ArtifactIndexEntry, 0, artifactCapacity)
	seen := make(map[string]struct{}, len(reader.File))
	var manifestJSON []byte
	var totalBytes int64
	for _, entry := range reader.File {
		name, err := validateV2ArchivePath(entry.Name)
		if err != nil {
			return nil, nil, nil, err
		}
		if _, exists := seen[name]; exists {
			return nil, nil, nil, archiveV2Error("archive_duplicate_path", name, "archive contains a duplicate path")
		}
		seen[name] = struct{}{}
		mode := entry.Mode()
		if entry.FileInfo().IsDir() || mode&fs.ModeSymlink != 0 || !mode.IsRegular() {
			return nil, nil, nil, archiveV2Error("archive_type_forbidden", name, "archive entries must be regular files")
		}
		limit := int64(MaxDesignMDBytes)
		role := "manifest"
		mediaType := "application/json"
		if name != "manifest.json" {
			role, mediaType, limit, err = classifyV2Artifact(name)
			if err != nil {
				return nil, nil, nil, err
			}
		}
		if entry.UncompressedSize64 > uint64(limit) {
			return nil, nil, nil, archiveV2Error("archive_file_too_large", name, "archive entry exceeds its size limit")
		}
		stream, err := entry.Open()
		if err != nil {
			return nil, nil, nil, archiveV2Error("archive_entry_unreadable", name, "archive entry cannot be opened")
		}
		contents, readErr := io.ReadAll(io.LimitReader(stream, limit+1))
		closeErr := stream.Close()
		if readErr != nil || closeErr != nil {
			return nil, nil, nil, archiveV2Error("archive_entry_unreadable", name, "archive entry cannot be read")
		}
		if int64(len(contents)) > limit || uint64(len(contents)) != entry.UncompressedSize64 {
			return nil, nil, nil, archiveV2Error("archive_file_too_large", name, "archive entry has an invalid expanded size")
		}
		totalBytes += int64(len(contents))
		if totalBytes > maxV2TotalBytes {
			return nil, nil, nil, archiveV2Error("archive_total_too_large", name, "archive exceeds its uncompressed size limit")
		}
		if name == "manifest.json" {
			manifestJSON = contents
			continue
		}
		files[name] = contents
		index = append(index, ArtifactIndexEntry{
			Path:      name,
			Role:      role,
			MediaType: mediaType,
			SizeBytes: int64(len(contents)),
			SHA256:    sha256Hex(contents),
		})
	}
	if manifestJSON == nil {
		return nil, nil, nil, archiveV2Error("manifest_missing", "manifest.json", "archive has no generated manifest")
	}
	sort.Slice(index, func(left, right int) bool { return index[left].Path < index[right].Path })
	return files, index, manifestJSON, nil
}

func preflightV2ArchiveEOCD(archive []byte) error {
	const (
		eocdSignature       = 0x06054b50
		zip64Locator        = 0x07064b50
		eocdSize            = 22
		maximumCommentBytes = 65535
	)
	if len(archive) < eocdSize {
		return archiveV2Error("archive_invalid", "", "archive is not a valid ZIP")
	}
	searchStart := len(archive) - (eocdSize + maximumCommentBytes)
	if searchStart < 0 {
		searchStart = 0
	}
	eocdOffset := -1
	for offset := len(archive) - eocdSize; offset >= searchStart; offset-- {
		if binary.LittleEndian.Uint32(archive[offset:offset+4]) != eocdSignature {
			continue
		}
		commentLength := int(binary.LittleEndian.Uint16(archive[offset+20 : offset+22]))
		if offset+eocdSize+commentLength == len(archive) {
			eocdOffset = offset
			break
		}
	}
	if eocdOffset < 0 {
		return archiveV2Error("archive_invalid", "", "archive is not a valid ZIP")
	}
	eocd := archive[eocdOffset : eocdOffset+eocdSize]
	diskNumber := binary.LittleEndian.Uint16(eocd[4:6])
	centralDirectoryDisk := binary.LittleEndian.Uint16(eocd[6:8])
	entriesOnDisk := binary.LittleEndian.Uint16(eocd[8:10])
	totalEntries := binary.LittleEndian.Uint16(eocd[10:12])
	centralDirectorySize := binary.LittleEndian.Uint32(eocd[12:16])
	centralDirectoryOffset := binary.LittleEndian.Uint32(eocd[16:20])
	if diskNumber == ^uint16(0) || centralDirectoryDisk == ^uint16(0) ||
		entriesOnDisk == ^uint16(0) || totalEntries == ^uint16(0) ||
		centralDirectorySize == ^uint32(0) || centralDirectoryOffset == ^uint32(0) ||
		(eocdOffset >= 20 && binary.LittleEndian.Uint32(archive[eocdOffset-20:eocdOffset-16]) == zip64Locator) {
		return archiveV2Error("archive_invalid", "", "ZIP64 archives are not supported")
	}
	if diskNumber != 0 || centralDirectoryDisk != 0 || entriesOnDisk != totalEntries {
		return archiveV2Error("archive_invalid", "", "multi-disk ZIP archives are not supported")
	}
	if totalEntries > maxV2Files {
		return archiveV2Error("archive_file_count_exceeded", "", "archive contains too many entries")
	}
	if uint64(centralDirectoryOffset)+uint64(centralDirectorySize) > uint64(eocdOffset) {
		return archiveV2Error("archive_invalid", "", "archive central directory is out of bounds")
	}
	return nil
}

func classifyV2Artifact(name string) (string, string, int64, error) {
	if _, err := validateV2ArchivePath(name); err != nil {
		return "", "", 0, err
	}
	switch name {
	case "DESIGN.md":
		return "design", "text/markdown; charset=utf-8", MaxDesignMDBytes, nil
	case "tokens.css":
		return "tokens", "text/css; charset=utf-8", MaxTokensCSSBytes, nil
	case "source/index.json":
		return "source_index", "application/json", 256 << 10, nil
	case "USAGE.md":
		return "usage", "text/markdown; charset=utf-8", 256 << 10, nil
	case "design-tokens.json":
		return "design_tokens", "application/json", 512 << 10, nil
	case "components.manifest.json":
		return "component_index", "application/json", 512 << 10, nil
	case "ui-kit/index.html":
		return "ui_kit", "text/html; charset=utf-8", maxV2FileBytes, nil
	}
	if strings.HasPrefix(name, "preview/") && path.Dir(name) == "preview" && path.Ext(name) == ".html" {
		return "preview", "text/html; charset=utf-8", maxV2FileBytes, nil
	}
	if strings.HasPrefix(name, "assets/") && path.Dir(name) != "." {
		return "asset", v2AssetMediaType(strings.ToLower(path.Ext(name))), maxV2FileBytes, nil
	}
	if strings.HasPrefix(name, "fonts/") && path.Dir(name) != "." {
		return "font", v2FontMediaType(strings.ToLower(path.Ext(name))), maxV2FileBytes, nil
	}
	return "", "", 0, archiveV2Error("archive_path_undeclared", name, "file is outside the V2 package contract")
}

func v2AssetMediaType(extension string) string {
	types := map[string]string{
		".avif": "image/avif", ".gif": "image/gif", ".ico": "image/x-icon",
		".jpeg": "image/jpeg", ".jpg": "image/jpeg", ".png": "image/png",
		".svg": "image/svg+xml", ".webp": "image/webp",
	}
	if value, ok := types[extension]; ok {
		return value
	}
	return "application/octet-stream"
}

func v2FontMediaType(extension string) string {
	types := map[string]string{
		".otf": "font/otf", ".ttf": "font/ttf", ".woff": "font/woff", ".woff2": "font/woff2",
	}
	if value, ok := types[extension]; ok {
		return value
	}
	return "application/octet-stream"
}

func validateV2ArchivePath(value string) (string, error) {
	if value == "" || value != strings.TrimSpace(value) || strings.Contains(value, "\\") ||
		strings.HasPrefix(value, "/") || !fs.ValidPath(value) || path.Clean(value) != value || value == "." {
		return "", archiveV2Error("archive_path_invalid", value, "path must be a normalized relative slash path")
	}
	return value, nil
}

func validateV2DirectoryPath(name string) error {
	if _, err := validateV2ArchivePath(name); err != nil {
		return err
	}
	switch name {
	case "source", "ui-kit", "preview", "assets", "fonts":
		return nil
	}
	if strings.HasPrefix(name, "assets/") || strings.HasPrefix(name, "fonts/") {
		return nil
	}
	return archiveV2Error("archive_path_undeclared", name, "directory is outside the V2 package contract")
}

func validateV2Binding(binding PackageBinding) error {
	values := []string{binding.WorkspaceID, binding.ProjectID, binding.DesignSystemID, binding.TaskID, binding.AgentID}
	for _, value := range values {
		if value == "" || value != strings.TrimSpace(value) || strings.IndexFunc(value, func(r rune) bool { return r < 0x20 }) >= 0 {
			return errors.New("V2 package binding contains an invalid identity")
		}
	}
	if !validSHA256Reference(binding.InputSnapshotSHA256) {
		return errors.New("V2 package binding has an invalid input snapshot digest")
	}
	switch binding.Operation {
	case "generate":
		if binding.BasePackageSHA256 != "" {
			return errors.New("generate binding cannot include a base package digest")
		}
	case "adjust", "regenerate":
		if !validSHA256Reference(binding.BasePackageSHA256) {
			return errors.New("adjust and regenerate bindings require a valid base package digest")
		}
	default:
		return errors.New("V2 package binding operation is invalid")
	}
	return nil
}

func validSHA256Reference(value string) bool {
	if len(value) != 71 || !strings.HasPrefix(value, "sha256:") {
		return false
	}
	for _, character := range value[len("sha256:"):] {
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f')) {
			return false
		}
	}
	return true
}

func digestV2ArtifactIndex(index []ArtifactIndexEntry) string {
	hasher := sha256.New()
	for _, entry := range index {
		writeV2DigestField(hasher, entry.Path)
		writeV2DigestField(hasher, entry.MediaType)
		writeV2DigestField(hasher, strconv.FormatInt(entry.SizeBytes, 10))
		writeV2DigestField(hasher, entry.SHA256)
	}
	return "sha256:" + hex.EncodeToString(hasher.Sum(nil))
}

func writeV2DigestField(hasher hash.Hash, value string) {
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(len(value)))
	_, _ = hasher.Write(length[:])
	_, _ = io.WriteString(hasher, value)
}

func buildDeterministicV2Archive(files map[string][]byte) ([]byte, error) {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	fixedTime := time.Date(1980, time.January, 1, 0, 0, 0, 0, time.UTC)
	for _, name := range names {
		header := &zip.FileHeader{Name: name, Method: zip.Deflate}
		header.SetMode(0o644)
		header.SetModTime(fixedTime)
		entry, err := writer.CreateHeader(header)
		if err != nil {
			return nil, fmt.Errorf("create V2 archive entry %q: %w", name, err)
		}
		if _, err := entry.Write(files[name]); err != nil {
			return nil, fmt.Errorf("write V2 archive entry %q: %w", name, err)
		}
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close V2 archive: %w", err)
	}
	return output.Bytes(), nil
}

func readBoundedFile(name string, limit int64) ([]byte, error) {
	file, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	contents, readErr := io.ReadAll(io.LimitReader(file, limit+1))
	closeErr := file.Close()
	if readErr != nil || closeErr != nil {
		return nil, errors.Join(readErr, closeErr)
	}
	if int64(len(contents)) > limit {
		return nil, archiveV2Error("archive_file_too_large", name, "file exceeds its size limit")
	}
	return contents, nil
}

func hasMultipleHardlinks(info fs.FileInfo) bool {
	value := reflect.ValueOf(info.Sys())
	if !value.IsValid() {
		return false
	}
	if value.Kind() == reflect.Pointer {
		value = value.Elem()
	}
	if !value.IsValid() || value.Kind() != reflect.Struct {
		return false
	}
	field := value.FieldByName("Nlink")
	return field.IsValid() && field.CanUint() && field.Uint() > 1
}

func archiveV2Error(code, filePath, message string) error {
	return &v2ArchiveError{code: code, path: filePath, message: message}
}

func invalidValidatedV2(code, filePath, message, digest string) (ValidatedV2Package, error) {
	report := AuditReport{
		SchemaVersion: AuditSchemaV1,
		Passed:        false,
		ContentDigest: digest,
		Diagnostics:   []Diagnostic{errorDiagnostic(code, filePath, message)},
	}
	return ValidatedV2Package{Audit: report}, fmt.Errorf("%w: %s", ErrInvalidPackage, code)
}

func v2AuditError(report AuditReport) error {
	code := "audit_failed"
	if len(report.Diagnostics) > 0 {
		code = report.Diagnostics[0].Code
	}
	return fmt.Errorf("%w: %s", ErrInvalidPackage, code)
}

func sha256Hex(value []byte) string {
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])
}

func sha256String(value []byte) string {
	return "sha256:" + sha256Hex(value)
}

func decodeStrictJSON(raw []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	return requireJSONEOF(decoder)
}

func requireJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("JSON contains multiple values")
		}
		return err
	}
	return nil
}

func nonNilV2Files(values []ArtifactIndexEntry) []ArtifactIndexEntry {
	if values == nil {
		return []ArtifactIndexEntry{}
	}
	return values
}

func nonNilPreviewTargets(values []PreviewTarget) []PreviewTarget {
	if values == nil {
		return []PreviewTarget{}
	}
	return values
}
