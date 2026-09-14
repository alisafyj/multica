package agent

import "encoding/json"

const codexThreadConfigurationProjectionSchema = "codex_thread_configuration_projection/v1"

type codexThreadConfigurationProjection struct {
	Schema          string                                    `json:"schema"`
	Scope           string                                    `json:"scope"`
	Completeness    string                                    `json:"completeness"`
	NotProven       []string                                  `json:"not_proven"`
	Model           codexConfigurationDigest                  `json:"model"`
	ModelProvider   codexConfigurationDigest                  `json:"model_provider"`
	ReasoningEffort codexConfigurationEnum                    `json:"reasoning_effort"`
	ServiceTier     codexConfigurationEnum                    `json:"service_tier"`
	ApprovalPolicy  codexConfigurationEnum                    `json:"approval_policy"`
	SandboxPolicy   codexThreadConfigurationSandboxProjection `json:"sandbox_policy"`
}

type codexThreadConfigurationSandboxProjection struct {
	Mode                codexConfigurationEnum  `json:"mode"`
	NetworkAccess       codexConfigurationBool  `json:"network_access"`
	ExcludeTMPDirEnvVar codexConfigurationBool  `json:"exclude_tmpdir_env_var"`
	ExcludeSlashTMP     codexConfigurationBool  `json:"exclude_slash_tmp"`
	WritableRoots       codexConfigurationRoots `json:"writable_roots"`
}

func projectCodexThreadConfiguration(response map[string]any) codexThreadConfigurationProjection {
	sandbox, sandboxPresent := response["sandbox"]
	sandboxTable, sandboxValid := sandbox.(map[string]any)
	rootsTable := map[string]any{}
	if sandboxValid {
		if roots, ok := sandboxTable["writableRoots"]; ok {
			rootsTable["writable_roots"] = roots
		}
	}

	return codexThreadConfigurationProjection{
		Schema:       codexThreadConfigurationProjectionSchema,
		Scope:        "native_thread_setup_response_only",
		Completeness: "partial",
		NotProven: []string{
			"turn_overrides",
			"provider_reported_model",
			"provider_reported_effort",
			"complete_configuration",
		},
		Model:           projectCodexDigest(response, "model", "model"),
		ModelProvider:   projectCodexDigest(response, "modelProvider", "model_provider"),
		ReasoningEffort: projectCodexEnum(response, "reasoningEffort", "none", "minimal", "low", "medium", "high", "xhigh", "max", "ultra"),
		ServiceTier:     projectCodexEnum(response, "serviceTier", "default", "priority", "flex"),
		ApprovalPolicy:  projectCodexEnum(response, "approvalPolicy", "untrusted", "on-failure", "on-request", "never"),
		SandboxPolicy: codexThreadConfigurationSandboxProjection{
			Mode:                projectCodexNestedEnum(sandboxTable, sandboxPresent, sandboxValid, "type", "readOnly", "workspaceWrite", "dangerFullAccess"),
			NetworkAccess:       projectCodexNestedBool(sandboxTable, sandboxPresent, sandboxValid, "networkAccess"),
			ExcludeTMPDirEnvVar: projectCodexNestedBool(sandboxTable, sandboxPresent, sandboxValid, "excludeTmpdirEnvVar"),
			ExcludeSlashTMP:     projectCodexNestedBool(sandboxTable, sandboxPresent, sandboxValid, "excludeSlashTmp"),
			WritableRoots:       projectCodexRoots(rootsTable, sandboxPresent, sandboxValid),
		},
	}
}

func projectCodexNestedEnum(table map[string]any, tablePresent, tableValid bool, key string, allowed ...string) codexConfigurationEnum {
	if !tablePresent {
		return codexConfigurationEnum{State: codexConfigMissing}
	}
	if !tableValid {
		return codexConfigurationEnum{State: codexConfigMalformed}
	}
	return projectCodexEnum(table, key, allowed...)
}

func observeCodexThreadConfigurationResponse(client *codexClient, cwd, method, threadID string, raw []byte) {
	var response map[string]any
	if err := json.Unmarshal(raw, &response); err != nil {
		return
	}
	observeCodexThreadConfiguration(client, cwd, method, threadID, response)
}

func observeCodexThreadConfiguration(client *codexClient, cwd, method, threadID string, response map[string]any) {
	if client == nil || client.cfg.Logger == nil || cwd != client.cfg.WorkDir ||
		client.cfg.TaskID == "" || client.cfg.RuntimeID == "" || client.pid <= 0 || client.attempt <= 0 ||
		(method != "thread/start" && method != "thread/resume") {
		return
	}
	thread := projectCodexDigest(map[string]any{"thread_id": threadID}, "thread_id", "thread_id")
	if thread.State != codexConfigObserved {
		return
	}
	client.cfg.Logger.Info("codex thread configuration observed",
		"task_id", client.cfg.TaskID,
		"runtime_id", client.cfg.RuntimeID,
		"pid", client.pid,
		"attempt", client.attempt,
		"cwd", cwd,
		"method", method,
		"thread_id_digest", thread.Digest,
		"configuration", projectCodexThreadConfiguration(response),
	)
}
