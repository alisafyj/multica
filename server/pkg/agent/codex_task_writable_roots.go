package agent

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
)

type codexTaskWritablePolicy struct {
	workspace map[string]any
}

func resolveCodexTaskWritablePolicy(config map[string]any, roots []string) (*codexTaskWritablePolicy, error) {
	if len(roots) == 0 || config["approval_policy"] != "never" || config["sandbox_mode"] != "workspace-write" {
		return nil, nil
	}
	for _, key := range []string{"default_permissions", "permissions", "sandbox_permissions"} {
		if value := config[key]; value != nil {
			return nil, fmt.Errorf("task cache grants require an unambiguous legacy Codex sandbox policy")
		}
	}
	workspace := map[string]any{"network_access": false, "exclude_tmpdir_env_var": false, "exclude_slash_tmp": false}
	current, ok := config["sandbox_workspace_write"].(map[string]any)
	if config["sandbox_workspace_write"] != nil && !ok {
		return nil, fmt.Errorf("invalid Codex workspace-write configuration")
	}
	var combined []string
	for key, value := range current {
		switch key {
		case "writable_roots":
			var err error
			combined, err = codexWritableRootStrings(value)
			if err != nil {
				return nil, err
			}
		case "network_access", "exclude_tmpdir_env_var", "exclude_slash_tmp":
			if _, ok := value.(bool); !ok {
				return nil, fmt.Errorf("invalid Codex sandbox boolean %s", key)
			}
			workspace[key] = value
		default:
			return nil, fmt.Errorf("unsupported Codex workspace-write setting")
		}
	}
	for _, root := range append(combined, roots...) {
		if !filepath.IsAbs(root) || filepath.Clean(root) != root {
			return nil, fmt.Errorf("Codex task cache grant must be an absolute clean path")
		}
	}
	for _, root := range roots {
		if !slices.Contains(combined, root) {
			combined = append(combined, root)
		}
	}
	workspace["writable_roots"] = combined
	return &codexTaskWritablePolicy{workspace: workspace}, nil
}

func applyCodexTaskWritablePolicy(params map[string]any, policy *codexTaskWritablePolicy) {
	if policy == nil {
		return
	}
	config, _ := params["config"].(map[string]any)
	if config == nil {
		config = make(map[string]any)
	}
	config["sandbox_workspace_write"] = policy.workspace
	params["config"] = config
	params["approvalPolicy"] = "never"
	params["sandbox"] = "workspace-write"
}

func verifyCodexTaskWritablePolicy(raw []byte, policy *codexTaskWritablePolicy) error {
	if policy == nil {
		return nil
	}
	var result struct {
		ApprovalPolicy string         `json:"approvalPolicy"`
		Sandbox        map[string]any `json:"sandbox"`
	}
	if err := json.Unmarshal(raw, &result); err != nil || result.ApprovalPolicy != "never" || result.Sandbox["type"] != "workspaceWrite" {
		return fmt.Errorf("Codex did not confirm the selected task sandbox policy")
	}
	for wire, key := range map[string]string{"networkAccess": "network_access", "excludeTmpdirEnvVar": "exclude_tmpdir_env_var", "excludeSlashTmp": "exclude_slash_tmp"} {
		if result.Sandbox[wire] != policy.workspace[key] {
			return fmt.Errorf("Codex task sandbox confirmation mismatch: %s", wire)
		}
	}
	roots, err := codexWritableRootStrings(result.Sandbox["writableRoots"])
	if err != nil {
		return err
	}
	want := slices.Clone(policy.workspace["writable_roots"].([]string))
	slices.Sort(roots)
	slices.Sort(want)
	if !slices.Equal(slices.Compact(roots), slices.Compact(want)) {
		return fmt.Errorf("Codex did not confirm the exact task writable roots")
	}
	return nil
}

func codexWritableRootStrings(value any) ([]string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("invalid Codex writable roots")
	}
	var roots []string
	if value == nil || json.Unmarshal(raw, &roots) != nil {
		return nil, fmt.Errorf("invalid Codex writable roots")
	}
	return roots, nil
}
