package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/multica-ai/multica/server/pkg/agent"
)

func TestHandleLocalSkillListCodexPluginRuntimeCapability(t *testing.T) {
	for _, tt := range []struct {
		name, cached, resolved, provider      string
		healed, missingEntry, missingPath     bool
		profile, indexedProfile, resolveError bool
		want                                  bool
	}{
		{name: "supported_builtin", cached: "0.153.4", want: true},
		{name: "supported_version_prefix", cached: "codex-cli 0.153.4", want: true},
		{name: "unknown_version"},
		{name: "unsupported_version", cached: "0.154.0"},
		{name: "stale_supported_cache", cached: "0.153.4", healed: true, resolved: "0.154.0"},
		{name: "stale_unsupported_cache", cached: "0.154.0", healed: true, resolved: "0.153.4", want: true},
		{name: "unknown_resolved_version", cached: "0.153.4", healed: true},
		{name: "missing_builtin_entry", cached: "0.153.4", missingEntry: true},
		{name: "other_provider_version_only", missingEntry: true},
		{name: "missing_executable", cached: "0.153.4", missingPath: true},
		{name: "unresolved_custom_profile", cached: "0.153.4", profile: true},
		{name: "indexed_custom_launch", cached: "0.153.4", indexedProfile: true},
		{name: "launch_resolution_error", cached: "0.153.4", resolveError: true},
		{name: "claude_unchanged", provider: "claude", want: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			codexHome := filepath.Join(home, ".codex")
			t.Setenv("HOME", home)
			t.Setenv("CODEX_HOME", codexHome)
			provider := tt.provider
			if provider == "" {
				provider = "codex"
			}
			pluginRoot := filepath.Join(codexHome, "plugins", "cache", "local", "review", "1.0.0")
			writeTestLocalSkill(t, pluginRoot, ".codex-plugin", map[string]string{
				"plugin.json": `{"name":"review","skills":"./skills"}`,
			})
			writeTestLocalSkill(t, pluginRoot, "skills/check", map[string]string{"SKILL.md": "---\nname: check\n---\n"})
			if err := os.WriteFile(filepath.Join(codexHome, "config.toml"), []byte("[plugins.'review@local']\nenabled = true\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			if provider == "claude" {
				root := writeTestClaudePlugin(t, home, "review@local", "review", true)
				writeTestLocalSkill(t, root, "skills/check", map[string]string{"SKILL.md": "---\nname: check\n---\n"})
			}
			writeTestLocalSkill(t, filepath.Join(home, "."+provider, "skills"), "personal", map[string]string{"SKILL.md": "---\nname: personal\n---\n"})
			writeTestLocalSkill(t, filepath.Join(home, ".agents", "skills"), "universal", map[string]string{"SKILL.md": "---\nname: universal\n---\n"})
			want, supported, err := listRuntimeLocalSkills(provider)
			if err != nil || !supported || len(want) != 3 {
				t.Fatalf("fixture discovery: count=%d supported=%v err=%v", len(want), supported, err)
			}
			for i := range want {
				if want[i].Root == localSkillRootPlugin {
					want[i].CanDisable = tt.want
				}
			}
			var report struct {
				Status    string                     `json:"status"`
				Supported bool                       `json:"supported"`
				Skills    []runtimeLocalSkillSummary `json:"skills"`
			}
			d, calls := localSkillReportDaemon(t, func(w http.ResponseWriter, r *http.Request) {
				if err := json.NewDecoder(r.Body).Decode(&report); err != nil {
					t.Error(err)
				}
				w.WriteHeader(http.StatusOK)
			})
			binary := filepath.Join(home, "native-fixture.exe")
			if !tt.missingPath {
				writeExecStub(t, binary)
			}
			d.cfg.Agents = map[string]AgentEntry{"claude": {Path: binary}}
			if !tt.missingEntry {
				d.cfg.Agents["codex"] = AgentEntry{Path: binary}
			}
			d.agentVersions = map[string]string{"codex": tt.cached, "claude": "0.153.4"}
			if tt.healed {
				healedPath := filepath.Join(home, "resolved-fixture.exe")
				writeExecStub(t, healedPath)
				d.resolvedPaths = map[string]healedAgent{"codex": {path: healedPath, version: tt.resolved}}
			}
			rt := Runtime{ID: "selected-runtime", Provider: provider}
			if tt.profile {
				rt.ProfileID = "custom-profile"
			}
			if tt.indexedProfile {
				d.runtimeIndex = map[string]Runtime{rt.ID: {ID: rt.ID, Provider: provider, ProfileID: "custom-profile"}}
				d.profileLaunchSpecs = map[string]profileLaunchSpec{"custom-profile": {path: binary, version: "0.153.4"}}
			}
			oldLaunch, oldDetect := executablePathForLaunch, detectAgentVersion
			t.Cleanup(func() { executablePathForLaunch, detectAgentVersion = oldLaunch, oldDetect })
			launchChecks := 0
			executablePathForLaunch = func(path string) (string, bool, error) {
				launchChecks++
				if tt.resolveError {
					return "", true, errors.New("synthetic unresolved launch target")
				}
				return path, false, nil
			}
			detectAgentVersion = func(context.Context, agent.Command) (string, error) {
				t.Error("fixture must not execute an external CLI version probe")
				return "", errors.New("external CLI forbidden")
			}
			d.handleLocalSkillList(context.Background(), rt, "request")
			if atomic.LoadInt32(calls) != 1 || report.Status != "completed" || !report.Supported {
				t.Fatalf("capability gating failed the list: calls=%d status=%s supported=%v", atomic.LoadInt32(calls), report.Status, report.Supported)
			}
			if !reflect.DeepEqual(report.Skills, want) {
				t.Fatalf("skills=%+v, want=%+v", report.Skills, want)
			}
			if (tt.profile || tt.indexedProfile || tt.missingEntry || provider != "codex") && launchChecks != 0 {
				t.Fatalf("unrelated or custom runtime triggered %d builtin resolutions", launchChecks)
			}
		})
	}
}
