package opendesign

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

const (
	EngineIdentitySchema = "multica.open-design-engine-identity/v1"
	RunSchema            = "multica.open-design-run/v1"
	PreflightSchema      = "multica.open-design-preflight/v1"
)

const (
	ProbePassed  = "passed"
	ProbeFailed  = "failed"
	ProbeUnknown = "unknown"
)

const PluginsDisabled = "disabled"

var sha256Pattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

type EngineIdentity struct {
	Schema         string `json:"schema"`
	Release        string `json:"release"`
	Commit         string `json:"commit"`
	LockfileSHA256 string `json:"lockfile_sha256"`
	DistSHA256     string `json:"dist_sha256"`
}

func PinnedEngineIdentity() EngineIdentity {
	return EngineIdentity{
		Schema:         EngineIdentitySchema,
		Release:        "open-design-v0.16.1",
		Commit:         "276b4d8e970bc143d7ad060181a89a834e3d9caf",
		LockfileSHA256: "90bbe1375eb716240bbb79215c2a12a601abd977fe88587c6c6c6b4df31f6f23",
		DistSHA256:     "bc0a56497d56f85f7c807fe742022077bc35b1360de39bf298f07b184db1e7de",
	}
}

func (i EngineIdentity) Validate() error {
	if i.Schema != EngineIdentitySchema {
		return fmt.Errorf("engine schema %q does not match %q", i.Schema, EngineIdentitySchema)
	}
	if strings.TrimSpace(i.Release) == "" {
		return errors.New("engine release is required")
	}
	if !regexp.MustCompile(`^[a-f0-9]{40}$`).MatchString(i.Commit) {
		return errors.New("engine commit must be a lowercase 40-character git hash")
	}
	if !sha256Pattern.MatchString(i.LockfileSHA256) {
		return errors.New("engine lockfile_sha256 must be a lowercase SHA-256")
	}
	if !sha256Pattern.MatchString(i.DistSHA256) {
		return errors.New("engine dist_sha256 must be a lowercase SHA-256")
	}
	return nil
}

var adapterByMulticaProvider = map[string]string{
	"antigravity": "antigravity",
	"claude":      "claude",
	"codex":       "codex",
	"copilot":     "copilot",
	"cursor":      "cursor-agent",
	"hermes":      "hermes",
	"kimi":        "kimi",
	"kiro":        "kiro",
	"opencode":    "opencode",
	"pi":          "pi",
}

func ResolveAdapter(provider string) (string, bool) {
	adapterID, ok := adapterByMulticaProvider[provider]
	return adapterID, ok
}

type AgentIdentity struct {
	MulticaAgentID string `json:"multica_agent_id"`
	AdapterID      string `json:"adapter_id"`
	Model          string `json:"model,omitempty"`
}

type TaskRunContext struct {
	Schema string         `json:"schema"`
	RunID  string         `json:"run_id"`
	Engine EngineIdentity `json:"engine"`
	Agent  AgentIdentity  `json:"agent"`
}

type ProbeResult struct {
	Status   string `json:"status"`
	Required bool   `json:"required,omitempty"`
	Version  string `json:"version,omitempty"`
	Message  string `json:"message,omitempty"`
}

type PluginPreflight struct {
	Policy string   `json:"policy"`
	IDs    []string `json:"ids,omitempty"`
}

type PreflightReport struct {
	Schema     string          `json:"schema"`
	Engine     EngineIdentity  `json:"engine"`
	AdapterID  string          `json:"adapter_id"`
	Model      string          `json:"model,omitempty"`
	Binary     ProbeResult     `json:"binary"`
	Auth       ProbeResult     `json:"auth"`
	ModelProbe ProbeResult     `json:"model_probe"`
	Plugins    PluginPreflight `json:"plugins"`
}

type ExpectedPreflight struct {
	Engine    EngineIdentity
	AdapterID string
	Model     string
}

func ValidatePreflight(expected ExpectedPreflight, report PreflightReport) error {
	if err := expected.Engine.Validate(); err != nil {
		return fmt.Errorf("invalid expected engine identity: %w", err)
	}
	if report.Schema != PreflightSchema {
		return fmt.Errorf("preflight schema %q does not match %q", report.Schema, PreflightSchema)
	}
	if err := report.Engine.Validate(); err != nil {
		return fmt.Errorf("invalid observed engine identity: %w", err)
	}
	if report.Engine.Release != expected.Engine.Release {
		return fmt.Errorf("engine release %q does not match %q", report.Engine.Release, expected.Engine.Release)
	}
	if report.Engine.Commit != expected.Engine.Commit {
		return fmt.Errorf("engine commit %q does not match %q", report.Engine.Commit, expected.Engine.Commit)
	}
	if report.Engine.LockfileSHA256 != expected.Engine.LockfileSHA256 {
		return errors.New("engine lockfile_sha256 does not match the pinned artifact")
	}
	if report.Engine.DistSHA256 != expected.Engine.DistSHA256 {
		return errors.New("engine dist_sha256 does not match the pinned artifact")
	}
	if report.AdapterID != expected.AdapterID {
		return fmt.Errorf("adapter_id %q does not match %q", report.AdapterID, expected.AdapterID)
	}
	if report.Model != expected.Model {
		return fmt.Errorf("model %q does not match %q", report.Model, expected.Model)
	}
	if report.Binary.Status != ProbePassed {
		return fmt.Errorf("binary preflight status is %q", report.Binary.Status)
	}
	if strings.TrimSpace(report.Binary.Version) == "" {
		return errors.New("binary preflight version is required")
	}
	if report.Auth.Status == ProbeFailed || (report.Auth.Required && report.Auth.Status != ProbePassed) {
		return fmt.Errorf("auth preflight status is %q", report.Auth.Status)
	}
	if report.Auth.Status != ProbePassed && report.Auth.Status != ProbeUnknown {
		return fmt.Errorf("auth preflight status %q is invalid", report.Auth.Status)
	}
	if report.ModelProbe.Status != ProbePassed {
		return fmt.Errorf("model preflight status is %q", report.ModelProbe.Status)
	}
	if report.Plugins.Policy != PluginsDisabled || len(report.Plugins.IDs) != 0 {
		return errors.New("plugins must be disabled for the pinned Phase 0 run")
	}
	return nil
}
