package opendesign

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	workerLockfilePath = "pnpm-lock.yaml"
	workerDistPath     = "apps/daemon/dist"
)

type ArtifactVerification struct {
	LockfileSHA256 string
	DistSHA256     string
	FileCount      int
	ByteCount      int64
}

type artifactFile struct {
	absPath      string
	relativePath string
}

func VerifyWorkerArtifact(root string, expected EngineIdentity) (ArtifactVerification, error) {
	if err := expected.Validate(); err != nil {
		return ArtifactVerification{}, fmt.Errorf("validate expected Open Design engine identity: %w", err)
	}
	root = strings.TrimSpace(root)
	if root == "" {
		return ArtifactVerification{}, errors.New("Open Design worker artifact root is required")
	}

	lockfileSHA256, _, err := digestRegularFile(filepath.Join(root, workerLockfilePath))
	if err != nil {
		return ArtifactVerification{}, fmt.Errorf("verify Open Design worker lockfile: %w", err)
	}
	if lockfileSHA256 != expected.LockfileSHA256 {
		return ArtifactVerification{}, fmt.Errorf(
			"Open Design worker lockfile_sha256 mismatch: got %s, want %s",
			lockfileSHA256,
			expected.LockfileSHA256,
		)
	}

	distSHA256, fileCount, byteCount, err := digestDistTree(filepath.Join(root, filepath.FromSlash(workerDistPath)))
	if err != nil {
		return ArtifactVerification{}, fmt.Errorf("verify Open Design worker dist: %w", err)
	}
	if distSHA256 != expected.DistSHA256 {
		return ArtifactVerification{}, fmt.Errorf(
			"Open Design worker dist_sha256 mismatch: got %s, want %s",
			distSHA256,
			expected.DistSHA256,
		)
	}

	return ArtifactVerification{
		LockfileSHA256: lockfileSHA256,
		DistSHA256:     distSHA256,
		FileCount:      fileCount,
		ByteCount:      byteCount,
	}, nil
}

func digestDistTree(root string) (string, int, int64, error) {
	rootInfo, err := os.Lstat(root)
	if err != nil {
		return "", 0, 0, err
	}
	if rootInfo.Mode()&os.ModeSymlink != 0 {
		return "", 0, 0, fmt.Errorf("dist root %q is a symlink", root)
	}
	if !rootInfo.IsDir() {
		return "", 0, 0, fmt.Errorf("dist root %q is not a directory", root)
	}

	files := make([]artifactFile, 0)
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("dist entry %q is a symlink", path)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("dist entry %q is non-regular", path)
		}
		relativePath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, artifactFile{
			absPath:      path,
			relativePath: filepath.ToSlash(relativePath),
		})
		return nil
	})
	if err != nil {
		return "", 0, 0, err
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].relativePath < files[j].relativePath
	})

	hasher := sha256.New()
	var byteCount int64
	for _, file := range files {
		if _, err := io.WriteString(hasher, file.relativePath); err != nil {
			return "", 0, 0, err
		}
		if err := writeDigestSeparator(hasher); err != nil {
			return "", 0, 0, err
		}
		written, err := copyRegularFile(hasher, file.absPath)
		if err != nil {
			return "", 0, 0, fmt.Errorf("hash %q: %w", file.relativePath, err)
		}
		byteCount += written
		if err := writeDigestSeparator(hasher); err != nil {
			return "", 0, 0, err
		}
	}
	return hex.EncodeToString(hasher.Sum(nil)), len(files), byteCount, nil
}

func digestRegularFile(path string) (string, int64, error) {
	hasher := sha256.New()
	written, err := copyRegularFile(hasher, path)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(hasher.Sum(nil)), written, nil
}

func copyRegularFile(destination hash.Hash, path string) (int64, error) {
	pathInfo, err := os.Lstat(path)
	if err != nil {
		return 0, err
	}
	if pathInfo.Mode()&os.ModeSymlink != 0 {
		return 0, fmt.Errorf("%q is a symlink", path)
	}
	if !pathInfo.Mode().IsRegular() {
		return 0, fmt.Errorf("%q is non-regular", path)
	}

	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil {
		return 0, err
	}
	if !openedInfo.Mode().IsRegular() || !os.SameFile(pathInfo, openedInfo) {
		return 0, fmt.Errorf("%q changed while it was opened", path)
	}
	written, err := io.Copy(destination, file)
	if err != nil {
		return 0, err
	}
	if written != openedInfo.Size() {
		return 0, fmt.Errorf("%q changed while it was read", path)
	}
	return written, nil
}

func writeDigestSeparator(destination io.Writer) error {
	_, err := destination.Write([]byte{0})
	return err
}
