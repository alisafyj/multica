package agent

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"path/filepath"
	"sort"
	"strings"
)

const (
	codexConfigurationProjectionSchema = "codex_configuration_projection/v1"
	codexConfigObserved                = "observed"
	codexConfigMissing                 = "missing"
	codexConfigMalformed               = "malformed"
)

type codexConfigurationProjection struct {
	Schema          string                              `json:"schema"`
	Scope           string                              `json:"scope"`
	Completeness    string                              `json:"completeness"`
	NotProven       []string                            `json:"not_proven"`
	Model           codexConfigurationDigest            `json:"model"`
	ModelProvider   codexConfigurationDigest            `json:"model_provider"`
	ReasoningEffort codexConfigurationEnum              `json:"reasoning_effort"`
	ServiceTier     codexConfigurationEnum              `json:"service_tier"`
	ApprovalPolicy  codexConfigurationEnum              `json:"approval_policy"`
	SandboxMode     codexConfigurationEnum              `json:"sandbox_mode"`
	WorkspaceWrite  codexConfigurationWorkspaceWrite    `json:"workspace_write"`
	Features        codexConfigurationFeatureProjection `json:"features"`
}

type codexConfigurationDigest struct {
	State  string `json:"state"`
	Digest string `json:"digest,omitempty"`
}

type codexConfigurationEnum struct {
	State string `json:"state"`
	Value string `json:"value,omitempty"`
}

type codexConfigurationBool struct {
	State string `json:"state"`
	Value *bool  `json:"value,omitempty"`
}

type codexConfigurationRoots struct {
	State  string `json:"state"`
	Count  *int   `json:"count,omitempty"`
	Digest string `json:"digest,omitempty"`
}

type codexConfigurationWorkspaceWrite struct {
	NetworkAccess       codexConfigurationBool  `json:"network_access"`
	ExcludeTMPDirEnvVar codexConfigurationBool  `json:"exclude_tmpdir_env_var"`
	ExcludeSlashTMP     codexConfigurationBool  `json:"exclude_slash_tmp"`
	WritableRoots       codexConfigurationRoots `json:"writable_roots"`
}

type codexConfigurationFeatureProjection struct {
	DefaultModeRequestUserInput codexConfigurationBool `json:"default_mode_request_user_input"`
	FastMode                    codexConfigurationBool `json:"fast_mode"`
	Memories                    codexConfigurationBool `json:"memories"`
	MultiAgent                  codexConfigurationBool `json:"multi_agent"`
	Plugins                     codexConfigurationBool `json:"plugins"`
	ResponsesWebsockets         codexConfigurationBool `json:"responses_websockets"`
}

func projectCodexConfiguration(config map[string]any) codexConfigurationProjection {
	workspace, workspacePresent := config["sandbox_workspace_write"]
	workspaceTable, workspaceValid := workspace.(map[string]any)
	features, featuresPresent := config["features"]
	featureTable, featuresValid := features.(map[string]any)

	return codexConfigurationProjection{
		Schema:       codexConfigurationProjectionSchema,
		Scope:        "explicit_config_read_values_only",
		Completeness: "partial",
		NotProven: []string{
			"native_defaults",
			"layer_provenance",
			"complete_configuration",
			"provider_reported_model",
			"thread_permission_confirmation",
		},
		Model:         projectCodexDigest(config, "model", "model"),
		ModelProvider: projectCodexDigest(config, "model_provider", "model_provider"),
		ReasoningEffort: projectCodexEnum(config, "model_reasoning_effort",
			"none", "minimal", "low", "medium", "high", "xhigh", "max", "ultra"),
		ServiceTier:    projectCodexEnum(config, "service_tier", "default", "priority", "flex"),
		ApprovalPolicy: projectCodexEnum(config, "approval_policy", "untrusted", "on-failure", "on-request", "never"),
		SandboxMode:    projectCodexEnum(config, "sandbox_mode", "read-only", "workspace-write", "danger-full-access"),
		WorkspaceWrite: codexConfigurationWorkspaceWrite{
			NetworkAccess:       projectCodexNestedBool(workspaceTable, workspacePresent, workspaceValid, "network_access"),
			ExcludeTMPDirEnvVar: projectCodexNestedBool(workspaceTable, workspacePresent, workspaceValid, "exclude_tmpdir_env_var"),
			ExcludeSlashTMP:     projectCodexNestedBool(workspaceTable, workspacePresent, workspaceValid, "exclude_slash_tmp"),
			WritableRoots:       projectCodexRoots(workspaceTable, workspacePresent, workspaceValid),
		},
		Features: codexConfigurationFeatureProjection{
			DefaultModeRequestUserInput: projectCodexNestedBool(featureTable, featuresPresent, featuresValid, codexRequestUserInputFeature),
			FastMode:                    projectCodexNestedBool(featureTable, featuresPresent, featuresValid, codexFastModeFeature),
			Memories:                    projectCodexNestedBool(featureTable, featuresPresent, featuresValid, "memories"),
			MultiAgent:                  projectCodexNestedBool(featureTable, featuresPresent, featuresValid, "multi_agent"),
			Plugins:                     projectCodexNestedBool(featureTable, featuresPresent, featuresValid, "plugins"),
			ResponsesWebsockets:         projectCodexNestedBool(featureTable, featuresPresent, featuresValid, "responses_websockets"),
		},
	}
}

func observeCodexConfiguration(client *codexClient, cwd string, purpose codexConfigurationReadPurpose, config map[string]any) {
	if purpose != codexConfigurationReadPurposeTaskEffective || client == nil ||
		client.cfg.Logger == nil || cwd != client.cfg.WorkDir ||
		client.cfg.TaskID == "" || client.cfg.RuntimeID == "" || client.pid <= 0 || client.attempt <= 0 {
		return
	}
	client.cfg.Logger.Info("codex configuration observed",
		"task_id", client.cfg.TaskID,
		"runtime_id", client.cfg.RuntimeID,
		"pid", client.pid,
		"attempt", client.attempt,
		"cwd", cwd,
		"configuration", projectCodexConfiguration(config),
	)
}

func projectCodexDigest(config map[string]any, key, domain string) codexConfigurationDigest {
	value, ok := config[key]
	if !ok {
		return codexConfigurationDigest{State: codexConfigMissing}
	}
	text, ok := value.(string)
	if !ok || !validCodexConfigurationIdentifier(text) {
		return codexConfigurationDigest{State: codexConfigMalformed}
	}
	return codexConfigurationDigest{State: codexConfigObserved, Digest: codexProjectionDigest(domain, []string{text})}
}

func projectCodexEnum(config map[string]any, key string, allowed ...string) codexConfigurationEnum {
	value, ok := config[key]
	if !ok {
		return codexConfigurationEnum{State: codexConfigMissing}
	}
	text, ok := value.(string)
	if ok {
		for _, candidate := range allowed {
			if text == candidate {
				return codexConfigurationEnum{State: codexConfigObserved, Value: text}
			}
		}
	}
	return codexConfigurationEnum{State: codexConfigMalformed}
}

func projectCodexNestedBool(table map[string]any, tablePresent, tableValid bool, key string) codexConfigurationBool {
	if !tablePresent {
		return codexConfigurationBool{State: codexConfigMissing}
	}
	if !tableValid {
		return codexConfigurationBool{State: codexConfigMalformed}
	}
	value, ok := table[key]
	if !ok {
		return codexConfigurationBool{State: codexConfigMissing}
	}
	boolean, ok := value.(bool)
	if !ok {
		return codexConfigurationBool{State: codexConfigMalformed}
	}
	return codexConfigurationBool{State: codexConfigObserved, Value: &boolean}
}

func projectCodexRoots(table map[string]any, tablePresent, tableValid bool) codexConfigurationRoots {
	if !tablePresent {
		return codexConfigurationRoots{State: codexConfigMissing}
	}
	if !tableValid {
		return codexConfigurationRoots{State: codexConfigMalformed}
	}
	value, ok := table["writable_roots"]
	if !ok {
		return codexConfigurationRoots{State: codexConfigMissing}
	}
	var roots []string
	switch list := value.(type) {
	case []string:
		roots = append(roots, list...)
	case []any:
		for _, item := range list {
			root, ok := item.(string)
			if !ok {
				return codexConfigurationRoots{State: codexConfigMalformed}
			}
			roots = append(roots, root)
		}
	default:
		return codexConfigurationRoots{State: codexConfigMalformed}
	}
	canonical := make([]string, 0, len(roots))
	seen := make(map[string]struct{}, len(roots))
	for _, root := range roots {
		if !validCodexConfigurationRoot(root) {
			return codexConfigurationRoots{State: codexConfigMalformed}
		}
		if _, exists := seen[root]; exists {
			continue
		}
		seen[root] = struct{}{}
		canonical = append(canonical, root)
	}
	sort.Strings(canonical)
	count := len(canonical)
	return codexConfigurationRoots{
		State: codexConfigObserved, Count: &count,
		Digest: codexProjectionDigest("writable_roots", canonical),
	}
}

func validCodexConfigurationIdentifier(value string) bool {
	if len(value) == 0 || len(value) > 256 || strings.Contains(value, "://") {
		return false
	}
	for i := 0; i < len(value); i++ {
		character := value[i]
		if (character >= 'a' && character <= 'z') ||
			(character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') ||
			strings.ContainsRune("._:/-", rune(character)) {
			continue
		}
		return false
	}
	return true
}

func validCodexConfigurationRoot(root string) bool {
	if len(root) == 0 || len(root) > 4096 || !filepath.IsAbs(root) {
		return false
	}
	for i := 0; i < len(root); i++ {
		if root[i] < 0x20 || root[i] == 0x7f {
			return false
		}
	}
	return true
}

func codexProjectionDigest(domain string, values []string) string {
	hash := sha256.New()
	hash.Write([]byte(codexConfigurationProjectionSchema))
	hash.Write([]byte{0})
	hash.Write([]byte(domain))
	for _, value := range values {
		var size [8]byte
		binary.BigEndian.PutUint64(size[:], uint64(len(value)))
		hash.Write(size[:])
		hash.Write([]byte(value))
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil))
}
