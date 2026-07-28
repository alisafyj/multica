package daemon

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/multica-ai/multica/server/internal/projectdesignsystem"
)

func attachProjectDesignSystemArtifacts(task Task, result TaskResult) TaskResult {
	if len(task.ProjectDesignSystemContext) == 0 || result.Status != "completed" {
		return result
	}
	if strings.TrimSpace(result.EnvRoot) == "" {
		result.Status = "blocked"
		result.Comment = "project design system artifacts invalid: execution environment root is missing"
		result.FailureReason = "project_design_system_artifacts_invalid"
		return result
	}

	outputDir := filepath.Join(result.EnvRoot, "output", "project-design-system")
	artifacts, err := readProjectDesignSystemArtifacts(outputDir)
	if err != nil {
		result.Status = "blocked"
		result.Comment = "project design system artifacts invalid: " + err.Error()
		result.FailureReason = "project_design_system_artifacts_invalid"
		result.ProjectDesignSystemArtifacts = nil
		return result
	}
	result.ProjectDesignSystemArtifacts = &artifacts
	return result
}

func readProjectDesignSystemArtifacts(outputDir string) (ProjectDesignSystemArtifacts, error) {
	root, err := filepath.Abs(outputDir)
	if err != nil {
		return ProjectDesignSystemArtifacts{}, fmt.Errorf("resolve output directory: %w", err)
	}
	rootInfo, err := os.Lstat(root)
	if err != nil {
		return ProjectDesignSystemArtifacts{}, fmt.Errorf("inspect output directory: %w", err)
	}
	if rootInfo.Mode()&os.ModeSymlink != 0 || !rootInfo.IsDir() {
		return ProjectDesignSystemArtifacts{}, fmt.Errorf("output directory must be a real directory")
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return ProjectDesignSystemArtifacts{}, fmt.Errorf("resolve output directory links: %w", err)
	}

	artifacts := ProjectDesignSystemArtifacts{}
	total := 0
	files := []struct {
		name  string
		limit int
		set   func(string)
	}{
		{name: "DESIGN.md", limit: projectdesignsystem.MaxDesignMDBytes, set: func(value string) { artifacts.DesignMD = value }},
		{name: "tokens.css", limit: projectdesignsystem.MaxTokensCSSBytes, set: func(value string) { artifacts.TokensCSS = value }},
		{name: "components.html", limit: projectdesignsystem.MaxComponentsHTMLBytes, set: func(value string) { artifacts.ComponentsHTML = value }},
	}
	for _, artifact := range files {
		path := filepath.Join(root, artifact.name)
		info, err := os.Lstat(path)
		if err != nil {
			return ProjectDesignSystemArtifacts{}, fmt.Errorf("inspect %s: %w", artifact.name, err)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return ProjectDesignSystemArtifacts{}, fmt.Errorf("%s must be a regular file", artifact.name)
		}
		if info.Size() > int64(artifact.limit) {
			return ProjectDesignSystemArtifacts{}, fmt.Errorf("%s exceeds its size limit", artifact.name)
		}
		resolvedPath, err := filepath.EvalSymlinks(path)
		if err != nil {
			return ProjectDesignSystemArtifacts{}, fmt.Errorf("resolve %s: %w", artifact.name, err)
		}
		if !pathWithinDirectory(resolvedRoot, resolvedPath) {
			return ProjectDesignSystemArtifacts{}, fmt.Errorf("%s resolves outside the output directory", artifact.name)
		}

		file, err := os.Open(path)
		if err != nil {
			return ProjectDesignSystemArtifacts{}, fmt.Errorf("open %s: %w", artifact.name, err)
		}
		openedInfo, statErr := file.Stat()
		if statErr != nil {
			_ = file.Close()
			return ProjectDesignSystemArtifacts{}, fmt.Errorf("inspect opened %s: %w", artifact.name, statErr)
		}
		if !openedInfo.Mode().IsRegular() || !os.SameFile(info, openedInfo) {
			_ = file.Close()
			return ProjectDesignSystemArtifacts{}, fmt.Errorf("%s changed during collection", artifact.name)
		}
		contents, readErr := io.ReadAll(io.LimitReader(file, int64(artifact.limit)+1))
		closeErr := file.Close()
		if readErr != nil {
			return ProjectDesignSystemArtifacts{}, fmt.Errorf("read %s: %w", artifact.name, readErr)
		}
		if closeErr != nil {
			return ProjectDesignSystemArtifacts{}, fmt.Errorf("close %s: %w", artifact.name, closeErr)
		}
		if len(contents) > artifact.limit {
			return ProjectDesignSystemArtifacts{}, fmt.Errorf("%s exceeds its size limit", artifact.name)
		}
		total += len(contents)
		if total > projectdesignsystem.MaxAggregateBytes {
			return ProjectDesignSystemArtifacts{}, fmt.Errorf("artifact package exceeds its aggregate size limit")
		}
		artifact.set(string(contents))
	}
	return artifacts, nil
}

func pathWithinDirectory(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil || filepath.IsAbs(rel) {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}
