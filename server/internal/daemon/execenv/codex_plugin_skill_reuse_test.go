package execenv

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestReuseRejectsFailedCodexPluginSkillPolicyRefresh(t *testing.T) {
	for _, failure := range []string{"home", "skill-policy"} {
		t.Run(failure, func(t *testing.T) {
			fixture, shared, install := codexPluginPolicyFixture(t)
			config, err := os.ReadFile(filepath.Join(fixture, "config.toml"))
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(shared, "config.toml"), config, 0o600); err != nil {
				t.Fatal(err)
			}
			params := PrepareParams{
				WorkspacesRoot: t.TempDir(), WorkspaceID: "plugin-reuse", TaskID: "45a7565e-7c9b-4ea5-88d8-d4a6468b9592",
				Provider: "codex", CodexVersion: "0.153.4",
				Task: TaskContextForEnv{IssueID: "plugin-reuse", DisabledRuntimeSkills: []RuntimeSkillRefForEnv{
					{Root: "plugin", Plugin: "review@local", Key: "review@local:skills/folder"},
				}},
			}
			env, err := Prepare(params, testLogger())
			if err != nil {
				t.Fatal(err)
			}
			defer env.Cleanup(true)
			if len(env.CodexPluginSkillBindings) != 1 {
				t.Fatal("prepared environment lost selected plugin installation binding")
			}
			encoded, err := json.Marshal(env)
			if err != nil {
				t.Fatal(err)
			}
			var decoded Environment
			if err := json.Unmarshal(encoded, &decoded); err != nil || !reflect.DeepEqual(env.CodexPluginSkillBindings, decoded.CodexPluginSkillBindings) {
				t.Fatal("environment IPC lost the installation binding")
			}
			path := filepath.Join(shared, "config.toml")
			contents := "[malformed"
			if failure == "skill-policy" {
				path = filepath.Join(install, ".codex-plugin", "plugin.json")
				contents = "{malformed"
			}
			if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
				t.Fatal(err)
			}
			if reused := Reuse(ReuseParams{
				WorkspacesRoot: params.WorkspacesRoot, WorkDir: env.WorkDir, Provider: "codex",
				CodexVersion: params.CodexVersion, Task: params.Task,
			}, testLogger()); reused != nil {
				t.Fatal("Reuse returned an environment after losing a required skill policy")
			}
			params.TaskID = "33147dde-d92f-4949-92ef-ed20ad9e8846"
			fresh, err := Prepare(params, testLogger())
			if fresh != nil {
				fresh.Cleanup(true)
			}
			if err == nil || fresh != nil {
				t.Fatal("fresh preparation bypassed the failed skill policy")
			}
		})
	}
}
