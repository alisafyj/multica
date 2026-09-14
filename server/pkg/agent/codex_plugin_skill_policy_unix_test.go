//go:build darwin || linux

package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func pluginSkillPolicyOptions(t *testing.T) (Config, ExecOptions) {
	t.Helper()
	cfg, opts := pluginMCPTestOptions(t)
	home, root := pluginSkillFixture(t)
	cfg.Env["CODEX_HOME"] = home
	cfg.CLIVersion = "codex-cli 0.153.4"
	opts.RuntimeMCPSelection = nil
	skill := filepath.Join(root, "skills", "custom", "SKILL.md")
	cachePluginWrite(t, skill, "---\nname: custom\ndescription: synthetic policy test\n---\nLocal fixture only.\n")
	resolved, err := ResolveCodexPluginSkillRoot(home, "probe@local")
	if err != nil {
		t.Fatal(err)
	}
	opts.CodexPluginSkillSelections = []CodexPluginSkillSelection{{PluginID: "probe@local", Key: "probe@local:skills/custom"}}
	opts.CodexPluginSkillBindings = []CodexPluginSkillBinding{{PluginID: "probe@local", Key: "probe@local:skills/custom", SkillPath: skill, InstallationDigest: resolved.InstallationDigest}}
	cachePluginWrite(t, filepath.Join(home, "config.toml"), fmt.Sprintf("[plugins.'probe@local']\nenabled = true\n[[skills.config]]\npath = %q\nenabled = false\n", skill))
	return cfg, opts
}

func pluginSkillPolicyScript(t *testing.T, effective map[string]any, fallback bool) string {
	t.Helper()
	script := pluginMCPProtocolScript(t, effective, fallback, false)
	initial := pluginEffectiveFixture()
	if features, ok := effective["features"]; ok {
		initial["features"] = features
	}
	before, err := json.Marshal(map[string]any{"config": initial})
	if err != nil {
		t.Fatal(err)
	}
	after, err := json.Marshal(map[string]any{"config": effective})
	if err != nil {
		t.Fatal(err)
	}
	return strings.Replace(script, shellQuote(string(before)), shellQuote(string(after)), 1)
}

func TestCodexPluginSkillPolicyRejectsInvalidBindingsBeforeLaunch(t *testing.T) {
	for _, variant := range []string{"missing-bindings", "missing-selections", "truncated", "duplicate-binding", "duplicate-selection", "wrong-key", "wrong-path", "wrong-digest", "wrong-version", "diagnostic-version-only", "custom-runtime", "task-config-missing", "task-config-enabled", "cli-skills", "extra-skills", "prefix-skills", "profile"} {
		t.Run(variant, func(t *testing.T) {
			cfg, opts := pluginSkillPolicyOptions(t)
			switch variant {
			case "missing-bindings":
				opts.CodexPluginSkillBindings = nil
			case "missing-selections":
				opts.CodexPluginSkillSelections = nil
			case "truncated":
				opts.CodexPluginSkillSelections = append(opts.CodexPluginSkillSelections, CodexPluginSkillSelection{PluginID: "probe@local", Key: "probe@local:skills/missing"})
			case "duplicate-binding":
				opts.CodexPluginSkillBindings = append(opts.CodexPluginSkillBindings, opts.CodexPluginSkillBindings[0])
			case "duplicate-selection":
				opts.CodexPluginSkillSelections = append(opts.CodexPluginSkillSelections, opts.CodexPluginSkillSelections[0])
			case "wrong-key":
				opts.CodexPluginSkillBindings[0].Key = "probe@local:skills/other"
			case "wrong-path":
				opts.CodexPluginSkillBindings[0].SkillPath = filepath.Join(t.TempDir(), "SKILL.md")
			case "wrong-digest":
				opts.CodexPluginSkillBindings[0].InstallationDigest = "sha256:" + strings.Repeat("0", 64)
			case "wrong-version":
				cfg.CLIVersion = "codex-cli 0.153.3"
			case "diagnostic-version-only":
				cfg.CLIVersion = ""
			case "custom-runtime":
				cfg.BuiltinRuntime = false
			case "task-config-missing":
				cachePluginWrite(t, filepath.Join(cfg.Env["CODEX_HOME"], "config.toml"), "")
			case "task-config-enabled":
				cachePluginWrite(t, filepath.Join(cfg.Env["CODEX_HOME"], "config.toml"), fmt.Sprintf("[[skills.config]]\npath = %q\nenabled = true\n", opts.CodexPluginSkillBindings[0].SkillPath))
			case "cli-skills":
				opts.CustomArgs = []string{"-c", `"skills".config=[]`}
			case "extra-skills":
				opts.ExtraArgs = []string{"--config=skills={config=[]}"}
			case "prefix-skills":
				cfg.LaunchPrefix = []string{"-cskills.config=[]"}
			case "profile":
				opts.CustomArgs = []string{"--profile", "other"}
			}
			result, err, _ := runPluginMCPTest(t, context.Background(), pluginSkillPolicyScript(t, pluginEffectiveFixture(), false), cfg, opts)
			if err == nil || result.Status == "completed" {
				t.Fatalf("invalid selection reached execution: result=%+v err=%v", result, err)
			}
			if _, err := os.Stat(filepath.Join(cfg.Env["CODEX_HOME"], "launches")); !os.IsNotExist(err) {
				t.Fatal("invalid binding launched a provider process")
			}
		})
	}
}

func TestCodexPluginSkillPolicyPinsEffectiveStartResumeAndFallback(t *testing.T) {
	for _, mode := range []string{"start", "resume", "fallback"} {
		t.Run(mode, func(t *testing.T) {
			cfg, opts := pluginSkillPolicyOptions(t)
			if mode != "start" {
				opts.ResumeSessionID = "prior-thread"
			}
			opts.ThinkingLevel = "high"
			selected := opts.CodexPluginSkillBindings[0].SkillPath
			other := filepath.Join(cfg.Env["CODEX_HOME"], "unrelated", "SKILL.md")
			effective := pluginEffectiveFixture()
			effective["skills"] = map[string]any{"config": []any{
				map[string]any{"path": other, "enabled": true},
				map[string]any{"path": selected, "enabled": true},
			}}
			result, err, _ := runPluginMCPTest(t, context.Background(), pluginSkillPolicyScript(t, effective, mode == "fallback"), cfg, opts)
			if err != nil || result.Status != "completed" {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			raw, err := os.ReadFile(filepath.Join(cfg.Env["CODEX_HOME"], "test-rpc"))
			if err != nil {
				t.Fatal(err)
			}
			threads, reads := 0, 0
			for _, row := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
				_, packet, _ := strings.Cut(row, " ")
				var request struct {
					Method string `json:"method"`
					Params struct {
						Config map[string]any `json:"config"`
					} `json:"params"`
				}
				if json.Unmarshal([]byte(packet), &request) != nil {
					t.Fatal("invalid synthetic RPC")
				}
				if request.Method == "config/read" {
					reads++
				}
				if request.Method != "thread/start" && request.Method != "thread/resume" {
					continue
				}
				threads++
				skills, ok := request.Params.Config["skills"].(map[string]any)
				entries, entriesOK := skills["config"].([]any)
				if !ok || !entriesOK || len(entries) != 2 || request.Params.Config["model_reasoning_effort"] != "high" {
					t.Fatalf("thread policy missing or replaced unrelated configuration: %v", request.Params.Config)
				}
				for _, value := range entries {
					entry := value.(map[string]any)
					want := entry["path"] == other
					if entry["enabled"] != want {
						t.Fatal("effective selected skill was not disabled, or unrelated skill was changed")
					}
				}
			}
			wantThreads := 1
			if mode == "fallback" {
				wantThreads = 2
			}
			if threads != wantThreads || reads < threads {
				t.Fatalf("thread/read counts=%d/%d", threads, reads)
			}
		})
	}
}

func TestCodexPluginSkillPolicyRevalidatesAfterNativeInventoryAndFallback(t *testing.T) {
	for _, stage := range []string{"inventory", "resume", "config-read"} {
		t.Run(stage, func(t *testing.T) {
			cfg, opts := pluginSkillPolicyOptions(t)
			effective := pluginEffectiveFixture()
			if stage == "inventory" {
				_, mcpOpts := pluginMCPTestOptions(t)
				opts.RuntimeMCPSelection = mcpOpts.RuntimeMCPSelection
			} else if stage == "resume" {
				opts.ResumeSessionID = "prior-thread"
			}
			script := pluginSkillPolicyScript(t, effective, stage == "resume")
			root, err := ResolveCodexPluginSkillRoot(cfg.Env["CODEX_HOME"], "probe@local")
			if err != nil {
				t.Fatal(err)
			}
			installed := filepath.Dir(root.Path)
			method := map[string]string{"inventory": "plugin/installed", "resume": "thread/resume", "config-read": "config/read"}[stage]
			needle := `*'"method":"` + method + `"'*) `
			mutation := "mv " + shellQuote(installed) + " " + shellQuote(filepath.Join(filepath.Dir(installed), "2.0.0")) + "; "
			script = strings.Replace(script, needle, needle+mutation, 1)
			result, runErr, _ := runPluginMCPTest(t, context.Background(), script, cfg, opts)
			if runErr == nil && result.Status != "failed" {
				t.Fatalf("cache replacement was accepted: %+v", result)
			}
			raw, err := os.ReadFile(filepath.Join(cfg.Env["CODEX_HOME"], "test-rpc"))
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(raw), `"method":"turn/start"`) || strings.Contains(string(raw), `"method":"thread/start"`) {
				t.Fatal("binding drift reached a fresh thread or turn")
			}
			if stage == "inventory" && strings.Contains(string(raw), `"method":"thread/resume"`) {
				t.Fatal("native inventory bypassed binding gate")
			}
		})
	}
}

func TestCodexPluginSkillPolicyRejectsMalformedEffectiveSkills(t *testing.T) {
	for _, variant := range []string{"null-skills", "unknown-skills-key", "null-entries", "bad-entry", "unknown-entry-key", "missing-enabled", "nonboolean-enabled", "relative-path", "duplicate-path", "rpc-error"} {
		t.Run(variant, func(t *testing.T) {
			cfg, opts := pluginSkillPolicyOptions(t)
			entry := map[string]any{"path": opts.CodexPluginSkillBindings[0].SkillPath, "enabled": false}
			skills := map[string]any{"config": []any{entry}}
			effective := pluginEffectiveFixture()
			effective["skills"] = skills
			switch variant {
			case "null-skills":
				effective["skills"] = nil
			case "unknown-skills-key":
				skills["synthetic-private-value"] = true
			case "null-entries":
				skills["config"] = nil
			case "bad-entry":
				skills["config"] = []any{false}
			case "unknown-entry-key":
				entry["synthetic-private-value"] = true
			case "missing-enabled":
				delete(entry, "enabled")
			case "nonboolean-enabled":
				entry["enabled"] = "false"
			case "relative-path":
				entry["path"] = "SKILL.md"
			case "duplicate-path":
				skills["config"] = []any{entry, entry}
			}
			script := pluginSkillPolicyScript(t, effective, false)
			if variant == "rpc-error" {
				begin := strings.Index(script, `*'"method":"config/read"'*)`)
				end := strings.Index(script[begin:], "\n") + begin
				script = script[:begin] + `*'"method":"config/read"'*) printf '{"id":%s,"error":{"code":1,"message":"synthetic-private-value"}}\n' "$id" ;;` + script[end:]
			}
			result, err, logs := runPluginMCPTest(t, context.Background(), script, cfg, opts)
			if err != nil || result.Status != "failed" {
				t.Fatalf("unsupported effective policy was accepted: %+v %v", result, err)
			}
			if strings.Contains(result.Error+logs, "synthetic-private-value") {
				t.Fatal("raw configuration error escaped")
			}
			raw, err := os.ReadFile(filepath.Join(cfg.Env["CODEX_HOME"], "test-rpc"))
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(raw), `"method":"thread/start"`) || strings.Contains(string(raw), `"method":"turn/start"`) {
				t.Fatal("malformed configuration reached a thread or turn")
			}
		})
	}
}

func TestCodexPluginSkillPolicyInactiveInstalledAndNilPath(t *testing.T) {
	for _, nilSelection := range []bool{false, true} {
		t.Run(fmt.Sprint(nilSelection), func(t *testing.T) {
			cfg, opts := pluginSkillPolicyOptions(t)
			config := fmt.Sprintf("[plugins.'probe@local']\nenabled = false\n[features]\nplugins = false\n[[skills.config]]\npath = %q\nenabled = false\n", opts.CodexPluginSkillBindings[0].SkillPath)
			cachePluginWrite(t, filepath.Join(cfg.Env["CODEX_HOME"], "config.toml"), config)
			if nilSelection {
				opts.CodexPluginSkillSelections, opts.CodexPluginSkillBindings = nil, nil
				cfg.BuiltinRuntime, cfg.CLIVersion = false, "unsupported"
				opts.CustomArgs = []string{"-c", "skills.config=[]"}
			}
			result, err, _ := runPluginMCPTest(t, context.Background(), pluginSkillPolicyScript(t, pluginEffectiveFixture(), false), cfg, opts)
			if err != nil || result.Status != "completed" {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			raw, err := os.ReadFile(filepath.Join(cfg.Env["CODEX_HOME"], "test-rpc"))
			if err != nil {
				t.Fatal(err)
			}
			if nilSelection && strings.Contains(string(raw), `"method":"config/read"`) {
				t.Fatal("nil selections changed legacy configuration reads")
			}
		})
	}
}

func TestCodexPluginSkillPolicyRejectsCaseAliasSelection(t *testing.T) {
	cfg, opts := pluginSkillPolicyOptions(t)
	binding := &opts.CodexPluginSkillBindings[0]
	binding.Key = "probe@local:skills/CUSTOM"
	binding.SkillPath = filepath.Join(filepath.Dir(filepath.Dir(binding.SkillPath)), "CUSTOM", "SKILL.md")
	opts.CodexPluginSkillSelections[0].Key = binding.Key
	cachePluginWrite(t, filepath.Join(cfg.Env["CODEX_HOME"], "config.toml"), fmt.Sprintf("[[skills.config]]\npath = %q\nenabled = false\n", binding.SkillPath))
	if _, err := (&codexBackend{cfg: cfg}).prepareCodexPluginSkillPolicy(opts); err == nil {
		t.Fatal("case alias does not identify the native-discovered skill path")
	}
}

func TestCodexPluginSkillPolicyFreezesOriginalBindings(t *testing.T) {
	cfg, opts := pluginSkillPolicyOptions(t)
	b := &codexBackend{cfg: cfg}
	prepared, err := b.prepareCodexPluginSkillPolicy(opts)
	if err != nil {
		t.Fatal(err)
	}
	opts.CodexPluginSkillBindings[0].InstallationDigest = "replaced"
	opts.CodexPluginSkillSelections[0].Key = "replaced"
	if err := verifyCodexPluginSkillPolicy(cfg, prepared); err != nil {
		t.Fatal("caller mutated the retained binding")
	}
	cachePluginWrite(t, filepath.Join(cfg.Env["CODEX_HOME"], "plugins", "cache", "local", "probe", "1.0.0", ".codex-plugin", "plugin.json"), `{"name":"probe","version":"2.0.0"}`)
	if err := verifyCodexPluginSkillPolicy(cfg, prepared); err == nil {
		t.Fatal("retained installation binding was refreshed")
	}
}
