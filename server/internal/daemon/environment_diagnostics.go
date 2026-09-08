package daemon

import (
	"strings"

	"github.com/multica-ai/multica/server/internal/agentguard"
	"github.com/multica-ai/multica/server/pkg/taskfailure"
)

type agentEnvironmentDiagnostic struct {
	Key    string `json:"key"`
	Source string `json:"source"`
}

// Inventory only fixed, non-secret configuration names. Provider configuration
// files can supply other values, so absence here does not mean unconfigured.
func agentEnvironmentDiagnostics(provider string, inherited []string, custom map[string]string) []agentEnvironmentDiagnostic {
	keys := []string{
		"HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy", "ALL_PROXY", "all_proxy", "NO_PROXY", "no_proxy",
		"SSL_CERT_FILE", "SSL_CERT_DIR", "NODE_EXTRA_CA_CERTS", "GOPROXY", "GONOPROXY", "GOSUMDB", "GONOSUMDB",
		"npm_config_registry", "NPM_CONFIG_REGISTRY",
	}
	switch provider {
	case "claude":
		keys = append(keys, "ANTHROPIC_BASE_URL", "ANTHROPIC_MODEL", "CLAUDE_CODE_EFFORT_LEVEL", "CLAUDE_CODE_USE_BEDROCK", "CLAUDE_CODE_USE_VERTEX")
	case "codex":
		keys = append(keys, "OPENAI_BASE_URL")
	}
	base := make(map[string]string, len(inherited))
	for _, entry := range inherited {
		if key, value, ok := strings.Cut(entry, "="); ok {
			base[key] = value
		}
	}
	var result []agentEnvironmentDiagnostic
	for _, key := range keys {
		source := ""
		if value, explicit := custom[key]; explicit && !isBlockedEnvKey(key) {
			source = "explicit"
			if value == "" {
				source = "explicit_empty"
			}
		} else if value := base[key]; value != "" {
			source = "not_inherited"
			if agentguard.AllowedInheritedEnvKey(key) {
				source = "inherited_allowed"
			}
		}
		if source != "" {
			result = append(result, agentEnvironmentDiagnostic{Key: key, Source: source})
		}
	}
	return result
}

func annotateAgentEnvironmentFailure(message, reason string, diagnostics []agentEnvironmentDiagnostic) string {
	if reason != taskfailure.ReasonAgentProviderNetwork.String() {
		return message
	}
	var dropped []string
	for _, diagnostic := range diagnostics {
		if diagnostic.Source == "not_inherited" {
			dropped = append(dropped, diagnostic.Key)
		}
	}
	if len(dropped) == 0 {
		return message
	}
	return message + "\nEnvironment check: daemon-only settings were not inherited by this task: " + strings.Join(dropped, ", ") +
		". Authorize required values through the agent's custom_env and retry. Native provider configuration files are not inspected by this check; this observation alone does not establish the cause of the network failure."
}
