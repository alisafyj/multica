package execenv

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ValidatePrimaryRepositoryContext runs while the env-root claim is held and
// before context writers run. Repository symlinks cannot redirect sidecar writes.
func ValidatePrimaryRepositoryContext(workDir, provider string) error {
	root, err := os.OpenRoot(workDir)
	if err != nil {
		return err
	}
	defer root.Close()
	paths := []string{".git/info", ".agent_context", ".multica"}
	if dir := skillsDirPath(workDir, provider); dir != "" {
		rel, err := filepath.Rel(workDir, dir)
		if err != nil {
			return err
		}
		paths = append(paths, rel)
	}
	for _, path := range paths {
		current := ""
		for _, part := range strings.Split(filepath.ToSlash(path), "/") {
			current = filepath.Join(current, part)
			info, err := root.Lstat(current)
			if errors.Is(err, os.ErrNotExist) {
				break
			}
			if err != nil {
				return err
			}
			if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("primary repository context path %q must be a real directory", current)
			}
		}
	}
	return nil
}

// ExcludePrimaryRepositorySidecars excludes only new platform files, not whole
// native settings directories or repository-owned rules. Primary checkouts use
// their own .git directory, so these patterns never affect another checkout.
func ExcludePrimaryRepositorySidecars(envRoot, workDir string) error {
	_, manifest, err := readSidecarManifest(envRoot)
	if err != nil || manifest == nil {
		return err
	}
	root, err := os.OpenRoot(workDir)
	if err != nil {
		return err
	}
	defer root.Close()
	const excludePath = ".git/info/exclude"
	if info, err := root.Lstat(excludePath); err == nil && !info.Mode().IsRegular() {
		return fmt.Errorf("primary repository Git exclude must be a regular file")
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	file, err := root.OpenFile(excludePath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	existing, err := io.ReadAll(io.LimitReader(file, 1<<20+1))
	if err != nil {
		return err
	}
	if len(existing) > 1<<20 {
		return fmt.Errorf("primary repository Git exclude exceeds size limit")
	}
	patterns := make(map[string]bool)
	for _, line := range strings.Split(string(existing), "\n") {
		patterns[line] = true
	}
	escape := strings.NewReplacer("\\", "\\\\", "*", "\\*", "?", "\\?", "[", "\\[", "]", "\\]", " ", "\\ ")
	var additions strings.Builder
	for _, path := range manifest.Files {
		rel, err := filepath.Rel(workDir, path)
		if err != nil || !filepath.IsLocal(rel) || strings.ContainsAny(rel, "\r\n") {
			return fmt.Errorf("invalid primary repository sidecar path")
		}
		pattern := "/" + escape.Replace(filepath.ToSlash(rel))
		if !patterns[pattern] {
			additions.WriteString("\n" + pattern)
			patterns[pattern] = true
		}
	}
	if additions.Len() == 0 {
		return nil
	}
	_, err = file.WriteString(additions.String() + "\n")
	return err
}
