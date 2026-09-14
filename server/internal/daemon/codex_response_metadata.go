package daemon

import (
	"errors"
	"strings"
)

func taskCodexResponseMetadata(data *AgentData, provider, version string, builtin bool) (bool, error) {
	if data == nil {
		return false, nil
	}
	value, present := data.CustomEnv["MULTICA_CODEX_RESPONSE_METADATA"]
	if !present {
		return false, nil
	}
	if value != "codex_sse_v0_153_4" || provider != "codex" || !builtin ||
		strings.TrimPrefix(strings.TrimSpace(version), "codex-cli ") != "0.153.4" {
		return false, errors.New("Codex response metadata requires the supported opt-in and built-in CLI 0.153.4")
	}
	return true, nil
}
