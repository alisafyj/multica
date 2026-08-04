package opendesign

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func ExtractProjectArchive(archive []byte, contentDigest, destination string) error {
	if err := ValidateProjectArchiveContentDigest(archive, contentDigest); err != nil {
		return err
	}
	if strings.TrimSpace(destination) == "" || !filepath.IsAbs(destination) {
		return errors.New("Open Design archive destination must be absolute")
	}
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return fmt.Errorf("parse Open Design project archive: %w", err)
	}

	targets := make(map[string]string, len(reader.File))
	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}
		name, err := validateArchivePath(file.Name)
		if err != nil {
			return fmt.Errorf("invalid Open Design archive entry %q: %w", file.Name, err)
		}
		if file.Mode()&fs.ModeType != 0 || !file.Mode().IsRegular() {
			return fmt.Errorf("Open Design archive entry %q is not a regular file", name)
		}
		first, _, _ := strings.Cut(name, "/")
		if first == ".agent_context" {
			return fmt.Errorf("Open Design archive entry %q uses a reserved path", name)
		}
		target := filepath.Join(destination, filepath.FromSlash(name))
		relative, err := filepath.Rel(destination, target)
		if err != nil || relative == "." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || relative == ".." {
			return fmt.Errorf("Open Design archive entry %q escapes the destination", name)
		}
		if _, exists := targets[target]; exists {
			return fmt.Errorf("Open Design archive entry %q is duplicated", name)
		}
		if _, err := os.Lstat(target); err == nil {
			return fmt.Errorf("Open Design archive destination %q already exists", name)
		} else if !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("inspect Open Design archive destination %q: %w", name, err)
		}
		targets[target] = name
	}

	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}
		name := file.Name
		target := filepath.Join(destination, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("create Open Design archive directory for %q: %w", name, err)
		}
		input, err := file.Open()
		if err != nil {
			return fmt.Errorf("open Open Design archive entry %q: %w", name, err)
		}
		output, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err != nil {
			_ = input.Close()
			return fmt.Errorf("create Open Design archive destination %q: %w", name, err)
		}
		written, copyErr := io.Copy(output, io.LimitReader(input, maxCollectedArchiveFileBytes+1))
		closeErr := errors.Join(input.Close(), output.Close())
		if copyErr != nil || closeErr != nil || written > maxCollectedArchiveFileBytes || uint64(written) != file.UncompressedSize64 {
			_ = os.Remove(target)
			return fmt.Errorf("extract Open Design archive entry %q: %w", name, errors.Join(copyErr, closeErr))
		}
	}
	return nil
}
