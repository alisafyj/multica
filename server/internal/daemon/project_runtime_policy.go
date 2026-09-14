package daemon

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/multica-ai/multica/server/internal/agentguard"
)

const (
	projectConfigurationRestricted = "restricted"
	projectConfigurationTrusted    = "trusted"
	maxProjectMCPConfigBytes       = 1 << 20
	maxProjectMCPServerNames       = 64
	maxProjectMCPServerNameBytes   = 128
)

type projectRuntimePolicy struct {
	ConfigurationPolicy string
	MCPConfig           json.RawMessage
	MCPDiagnostics      []projectMCPDiagnostic
}

type projectMCPDiagnostic struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

type projectRepositoryPolicyRef struct {
	URL                 string                 `json:"url"`
	Ref                 string                 `json:"ref,omitempty"`
	ConfigurationPolicy string                 `json:"configuration_policy,omitempty"`
	MCPServers          []string               `json:"mcp_servers,omitempty"`
	Setup               *repositorySetupConfig `json:"setup,omitempty"`
}

// deriveProjectRuntimePolicy reads policy only from the managed project
// resource that also matches the repository authorized in the task claim.
// MCPConfig is secret-bearing local data; MCPDiagnostics is the only safe
// projection for logs or API responses.
func deriveProjectRuntimePolicy(task Task, provider, workDir string) (projectRuntimePolicy, error) {
	result := projectRuntimePolicy{}
	authorized, err := primaryRepositoryForTask(task)
	if err != nil {
		result.ConfigurationPolicy = projectConfigurationRestricted
		return result, err
	}
	if authorized == nil {
		return result, nil
	}

	var selected *projectRepositoryPolicyRef
	for _, resource := range task.ProjectResources {
		if resource.ResourceType != "github_repo" {
			continue
		}
		var ref projectRepositoryPolicyRef
		if err := json.Unmarshal(resource.ResourceRef, &ref); err != nil {
			result.ConfigurationPolicy = projectConfigurationRestricted
			return result, fmt.Errorf("project runtime policy: invalid github repository resource: %w", err)
		}
		ref.URL = strings.TrimSpace(ref.URL)
		ref.Ref = strings.TrimSpace(ref.Ref)
		if ref.URL == authorized.URL && ref.Ref == authorized.Ref {
			copy := ref
			selected = &copy
			break
		}
	}
	if selected == nil {
		result.ConfigurationPolicy = projectConfigurationRestricted
		return result, nil
	}

	result.ConfigurationPolicy = projectConfigurationRestricted
	policy := strings.TrimSpace(selected.ConfigurationPolicy)
	if policy != "" && policy != projectConfigurationRestricted && policy != projectConfigurationTrusted {
		return result, fmt.Errorf("project runtime policy: invalid configuration policy")
	}
	if err := validateProjectMCPServerNames(selected.MCPServers); err != nil {
		return result, err
	}
	if policy != projectConfigurationTrusted {
		for _, name := range selected.MCPServers {
			result.MCPDiagnostics = append(result.MCPDiagnostics, projectMCPDiagnostic{Name: name, Status: "trust_required"})
		}
		return result, nil
	}
	result.ConfigurationPolicy = projectConfigurationTrusted
	if len(selected.MCPServers) == 0 {
		return result, nil
	}
	diagnosticIndex := make(map[string]int, len(selected.MCPServers))
	result.MCPDiagnostics = make([]projectMCPDiagnostic, len(selected.MCPServers))
	for i, name := range selected.MCPServers {
		diagnosticIndex[name] = i
		result.MCPDiagnostics[i] = projectMCPDiagnostic{Name: name, Status: "missing"}
	}

	var relativePath, key, format string
	switch provider {
	case "claude":
		relativePath, key, format = ".mcp.json", "mcpServers", "json"
	case "codex":
		relativePath, key, format = filepath.Join(".codex", "config.toml"), "mcp_servers", "toml"
	default:
		for i := range result.MCPDiagnostics {
			result.MCPDiagnostics[i].Status = "unsupported_provider"
		}
		return result, nil
	}

	raw, found, err := readProjectConfigurationFile(workDir, relativePath)
	if err != nil {
		return result, err
	}
	if !found {
		return result, nil
	}
	document, err := unmarshalRuntimeMcpConfig(raw, format)
	if err != nil {
		return result, err
	}
	configured, _ := nestedRuntimeMcpMap(document, key)
	candidates := make(map[string]any)
	for _, name := range selected.MCPServers {
		entry, ok := configured[name]
		if !ok {
			continue
		}
		if _, ok := entry.(map[string]any); !ok {
			return result, fmt.Errorf("selected project MCP server %q has invalid configuration", name)
		}
		if projectMCPServerDisabled(entry) {
			result.MCPDiagnostics[diagnosticIndex[name]].Status = "disabled"
			continue
		}
		candidates[name] = normalizeRuntimeMcpEntry(provider, entry)
		result.MCPDiagnostics[diagnosticIndex[name]].Status = "selected"
	}

	candidateRaw, err := json.Marshal(map[string]any{"mcpServers": candidates})
	if err != nil {
		return result, fmt.Errorf("marshal selected project MCP config: %w", err)
	}
	filteredRaw, _, err := agentguard.FilterMCPConfig(candidateRaw)
	if err != nil {
		return result, err
	}
	var filteredDocument map[string]any
	if err := json.Unmarshal(filteredRaw, &filteredDocument); err != nil {
		return result, fmt.Errorf("parse privacy-filtered project MCP config: %w", err)
	}
	filtered, _ := nestedRuntimeMcpMap(filteredDocument, "mcpServers")

	for name := range candidates {
		if _, kept := filtered[name]; !kept {
			result.MCPDiagnostics[diagnosticIndex[name]].Status = "blocked_privacy"
		}
	}
	if len(filtered) > 0 {
		result.MCPConfig = filteredRaw
	}
	return result, nil
}

func validateProjectMCPServerNames(names []string) error {
	if len(names) > maxProjectMCPServerNames {
		return fmt.Errorf("project runtime policy: too many MCP server names")
	}
	seen := make(map[string]struct{}, len(names))
	for _, name := range names {
		if strings.TrimSpace(name) != name || name == "" || len(name) > maxProjectMCPServerNameBytes {
			return fmt.Errorf("project runtime policy: invalid MCP server name")
		}
		if _, exists := seen[name]; exists {
			return fmt.Errorf("project runtime policy: duplicate MCP server name")
		}
		seen[name] = struct{}{}
	}
	return nil
}

func projectMCPServerDisabled(value any) bool {
	entry, ok := value.(map[string]any)
	if !ok {
		return false
	}
	if disabled, ok := entry["disabled"].(bool); ok && disabled {
		return true
	}
	if enabled, ok := entry["enabled"].(bool); ok && !enabled {
		return true
	}
	return false
}

func readProjectConfigurationFile(workDir, relativePath string) ([]byte, bool, error) {
	root, err := os.OpenRoot(workDir)
	if err != nil {
		return nil, false, err
	}
	defer root.Close()

	current := ""
	parts := strings.Split(filepath.ToSlash(relativePath), "/")
	for _, part := range parts {
		current = filepath.Join(current, part)
		info, err := root.Lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		if err != nil {
			return nil, false, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, false, fmt.Errorf("project configuration path %q must not be a symlink", current)
		}
		if current != relativePath && !info.IsDir() {
			return nil, false, fmt.Errorf("project configuration path %q must be a directory", current)
		}
		if current == relativePath && (!info.Mode().IsRegular() || info.Size() > maxProjectMCPConfigBytes) {
			return nil, false, fmt.Errorf("project MCP configuration file is invalid or exceeds size limit")
		}
	}
	file, err := root.Open(relativePath)
	if err != nil {
		return nil, false, err
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, maxProjectMCPConfigBytes+1))
	if err != nil {
		return nil, false, err
	}
	if len(raw) > maxProjectMCPConfigBytes || len(bytes.TrimSpace(raw)) == 0 {
		return nil, false, fmt.Errorf("project MCP configuration file is empty or exceeds size limit")
	}
	return raw, true, nil
}
