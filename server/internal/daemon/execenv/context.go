package execenv

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/multica-ai/multica/server/internal/opendesign"
	"github.com/multica-ai/multica/server/internal/projectdesignsystem"
	skillpkg "github.com/multica-ai/multica/server/internal/skill"
)

// writeContextFiles renders and writes .agent_context/issue_context.md and
// skills into the appropriate provider-native location.
//
// Claude:      skills → {workDir}/.claude/skills/{name}/SKILL.md  (native discovery)
// Codex:       skills → handled separately in Prepare via codex-home
// Copilot:     skills → {workDir}/.github/skills/{name}/SKILL.md  (native project-level discovery)
// OpenCode:    skills → {workDir}/.opencode/skills/{name}/SKILL.md  (native discovery)
// OpenClaw:    skills → {workDir}/skills/{name}/SKILL.md  (native discovery — paired with a per-task synthesized openclaw-config.json that pins agents.defaults.workspace to workDir; see openclaw_config.go)
// Pi:          skills → {workDir}/.pi/skills/{name}/SKILL.md  (native discovery)
// Cursor:      skills → {workDir}/.cursor/skills/{name}/SKILL.md  (native discovery)
// Kimi:        skills → {workDir}/.kimi/skills/{name}/SKILL.md  (native discovery)
// Kiro:        skills → {workDir}/.kiro/skills/{name}/SKILL.md  (native discovery)
// Antigravity: skills → {workDir}/.agents/skills/{name}/SKILL.md  (native discovery — see https://antigravity.google/docs/gcli-migration "Workspace skills")
// Default:     skills → {workDir}/.agent_context/skills/{name}/SKILL.md
//
// manifest, when non-nil, is populated with every file we created and every
// intermediate directory we had to MkdirAll (skipping any that pre-existed).
// CleanupSidecars uses it to roll the workdir back to its pre-Prepare
// state for local_directory tasks. Callers that don't need cleanup —
// cloud-mode tasks whose envRoot is wiped wholesale by the GC loop — may
// pass nil to skip the bookkeeping entirely.
func writeContextFiles(workDir, provider string, ctx TaskContextForEnv, manifest *sidecarManifest) error {
	contextDir := filepath.Join(workDir, ".agent_context")
	if err := recordMkdirAll(contextDir, 0o755, manifest); err != nil {
		return fmt.Errorf("create .agent_context dir: %w", err)
	}

	content := renderIssueContext(provider, ctx)
	path := filepath.Join(contextDir, "issue_context.md")
	if err := recordWriteFile(path, []byte(content), 0o644, manifest); err != nil {
		// A pre-existing path means the user already owns
		// .agent_context/issue_context.md — either they created it
		// themselves or it survived from a crashed prior run we can't
		// safely distinguish from intentional content. Refusing the
		// write is the correct call: the runtime brief (CLAUDE.md /
		// AGENTS.md / GEMINI.md) already carries every fact this file
		// would, so the agent runs fine without the sidecar copy.
		// Anything else is a real failure.
		if !errors.Is(err, errPathPreExists) {
			return fmt.Errorf("write issue_context.md: %w", err)
		}
	}

	if err := writeProjectDesignSystemContext(workDir, ctx, manifest); err != nil {
		return fmt.Errorf("write project design system context: %w", err)
	}

	if len(ctx.AgentSkills) > 0 {
		skillsDir, err := resolveSkillsDir(workDir, provider, manifest)
		if err != nil {
			return fmt.Errorf("resolve skills dir: %w", err)
		}
		// Codex skills are written to codex-home in Prepare; skip here.
		if provider != "codex" {
			if err := writeSkillFiles(skillsDir, ctx.AgentSkills, manifest); err != nil {
				return fmt.Errorf("write skill files: %w", err)
			}
		}
	}

	// Project resources are best-effort: a write failure logs but does not
	// block task startup. Missing resources surface as the agent simply not
	// seeing the file, which matches the "scoped, not dumped" design (the
	// meta skill content always lists what the agent should expect).
	if err := writeProjectResources(workDir, ctx, manifest); err != nil {
		// Caller logs warnings; avoid noisy returns for non-fatal context.
		return fmt.Errorf("write project resources: %w", err)
	}

	return nil
}

func writeProjectDesignSystemContext(workDir string, ctx TaskContextForEnv, manifest *sidecarManifest) error {
	if strings.TrimSpace(ctx.ProjectDesignSystemContext) == "" {
		return nil
	}

	var task map[string]json.RawMessage
	if err := json.Unmarshal([]byte(ctx.ProjectDesignSystemContext), &task); err != nil {
		return fmt.Errorf("decode task context: %w", err)
	}
	var discriminator string
	if err := json.Unmarshal(task["type"], &discriminator); err != nil || discriminator != "project_design_system_task" {
		return fmt.Errorf("invalid task context type")
	}
	var operation string
	if err := json.Unmarshal(task["operation"], &operation); err != nil {
		return fmt.Errorf("decode operation: %w", err)
	}

	root := filepath.Join(workDir, ".agent_context", "project_design_system")
	if err := recordMkdirAll(root, 0o755, manifest); err != nil {
		return err
	}

	// The V2 native agent chain (pinPackageSchema == PackageSchemaV2 in the
	// task context) is materialized into a read-only bounded sidecar
	// layout: context/, reference/, and base/ each at 0o555 with files
	// stamped 0o444 so the agent can read but not mutate the inputs.
	// The legacy Open Design flow (no package_schema) keeps its previous
	// single-task.json layout untouched so already-queued tasks still
	// parse through the Open Design supervisor.
	if isV2ProjectDesignSystemTask(task) {
		return writeV2ProjectDesignSystemContext(root, task, operation, manifest)
	}

	if operation == "adjust" || operation == "regenerate" {
		var base map[string]json.RawMessage
		if err := json.Unmarshal(task["base_package"], &base); err != nil {
			return fmt.Errorf("decode base package: %w", err)
		}
		var schema string
		if rawSchema, ok := base["schema"]; ok {
			if err := json.Unmarshal(rawSchema, &schema); err != nil {
				return fmt.Errorf("decode base package schema: %w", err)
			}
		}
		if schema == opendesign.BasePackageReferenceSchema {
			var reference opendesign.BasePackageReference
			if err := json.Unmarshal(task["base_package"], &reference); err != nil {
				return fmt.Errorf("decode Open Design base package reference: %w", err)
			}
			if err := opendesign.ValidateBasePackageReference(reference); err != nil {
				return fmt.Errorf("validate Open Design base package reference: %w", err)
			}
		} else {
			if schema != "" {
				return fmt.Errorf("unsupported base package schema %q", schema)
			}
			baseDir := filepath.Join(root, "base")
			if err := recordMkdirAll(baseDir, 0o755, manifest); err != nil {
				return err
			}
			files := []struct {
				key  string
				name string
			}{
				{key: "design_md", name: "DESIGN.md"},
				{key: "tokens_css", name: "tokens.css"},
				{key: "components_html", name: "components.html"},
			}
			for _, file := range files {
				var contents string
				if err := json.Unmarshal(base[file.key], &contents); err != nil {
					return fmt.Errorf("decode base %s: %w", file.name, err)
				}
				if err := recordWriteFile(filepath.Join(baseDir, file.name), []byte(contents), 0o644, manifest); err != nil {
					return err
				}
				delete(base, file.key)
			}
			baseMetadata, err := json.Marshal(base)
			if err != nil {
				return fmt.Errorf("encode base metadata: %w", err)
			}
			task["base_package"] = baseMetadata
		}
	} else if operation == "generate" {
		delete(task, "base_package")
	} else {
		return fmt.Errorf("unsupported operation %q", operation)
	}

	taskJSON, err := json.MarshalIndent(task, "", "  ")
	if err != nil {
		return fmt.Errorf("encode task context: %w", err)
	}
	if err := recordWriteFile(filepath.Join(root, "task.json"), taskJSON, 0o644, manifest); err != nil {
		return err
	}
	return nil
}

// isV2ProjectDesignSystemTask reports whether the task context was
// stamped with the V2 native agent package schema. The V2 marker is
// the sole signal that triggers the bounded read-only sidecar layout;
// historical Open Design contexts that lack the marker continue to
// flow through the legacy single-file task.json path.
func isV2ProjectDesignSystemTask(task map[string]json.RawMessage) bool {
	rawSchema, ok := task["package_schema"]
	if !ok {
		return false
	}
	var schema string
	if err := json.Unmarshal(rawSchema, &schema); err != nil {
		return false
	}
	return schema == projectdesignsystem.PackageSchemaV2
}

// writeV2ProjectDesignSystemContext materializes the V2 native agent
// workspace under {root}: a read-only context/task.json + optional
// context/repository-analysis.json, a read-only reference/index.json
// summarising the brief and references, and an optional read-only
// base/ tree populated for adjust / regenerate tasks. All three
// sub-directories are stamped 0o555; all files are stamped 0o444 so
// the agent can read but not mutate the inputs. The output area
// (envRoot/output/project-design-system) is intentionally not touched
// here — it stays writable for the agent's final package.
func writeV2ProjectDesignSystemContext(root string, task map[string]json.RawMessage, operation string, manifest *sidecarManifest) error {
	if operation == "repository_analysis" {
		return writeV2RepositoryAnalysisContext(root, task, manifest)
	}
	if operation != "generate" && operation != "adjust" && operation != "regenerate" {
		return fmt.Errorf("unsupported V2 operation %q", operation)
	}

	contextDir := filepath.Join(root, "context")
	if err := recordMkdirAll(contextDir, 0o755, manifest); err != nil {
		return err
	}
	referenceDir := filepath.Join(root, "reference")
	if err := recordMkdirAll(referenceDir, 0o755, manifest); err != nil {
		return err
	}

	taskJSON, err := json.MarshalIndent(task, "", "  ")
	if err != nil {
		return fmt.Errorf("encode V2 task context: %w", err)
	}
	if err := recordWriteFile(filepath.Join(contextDir, "task.json"), taskJSON, 0o444, manifest); err != nil {
		return err
	}

	// repository-analysis.json is optional. It is only emitted when the
	// task context carries a non-empty repository_analysis block; we copy
	// the block verbatim so the agent can read the source material
	// without us redacting or reformatting it.
	if rawAnalysis, ok := task["repository_analysis"]; ok && len(rawAnalysis) > 0 && string(rawAnalysis) != "null" {
		var probe any
		if err := json.Unmarshal(rawAnalysis, &probe); err == nil && probe != nil {
			if err := recordWriteFile(filepath.Join(contextDir, "repository-analysis.json"), rawAnalysis, 0o444, manifest); err != nil {
				return err
			}
		}
	}

	indexJSON, err := buildV2ReferenceIndex(task)
	if err != nil {
		return err
	}
	if err := recordWriteFile(filepath.Join(referenceDir, "index.json"), indexJSON, 0o444, manifest); err != nil {
		return err
	}

	if operation == "adjust" || operation == "regenerate" {
		if err := writeV2BaseDirectory(root, task, manifest); err != nil {
			return err
		}
	}

	if err := stampV2ReadOnly(contextDir, referenceDir); err != nil {
		return err
	}
	return nil
}

// writeV2RepositoryAnalysisContext emits the minimal V2 sidecar layout
// for the repository_analysis operation: a read-only context/task.json
// and reference/index.json. The repository-analysis payload stays inline
// in task.json; the base/ tree is intentionally absent because there is
// no base package to consult.
func writeV2RepositoryAnalysisContext(root string, task map[string]json.RawMessage, manifest *sidecarManifest) error {
	contextDir := filepath.Join(root, "context")
	if err := recordMkdirAll(contextDir, 0o755, manifest); err != nil {
		return err
	}
	referenceDir := filepath.Join(root, "reference")
	if err := recordMkdirAll(referenceDir, 0o755, manifest); err != nil {
		return err
	}
	taskJSON, err := json.MarshalIndent(task, "", "  ")
	if err != nil {
		return fmt.Errorf("encode V2 repository analysis task context: %w", err)
	}
	if err := recordWriteFile(filepath.Join(contextDir, "task.json"), taskJSON, 0o444, manifest); err != nil {
		return err
	}
	indexJSON, err := buildV2ReferenceIndex(task)
	if err != nil {
		return err
	}
	if err := recordWriteFile(filepath.Join(referenceDir, "index.json"), indexJSON, 0o444, manifest); err != nil {
		return err
	}
	if err := stampV2ReadOnly(contextDir, referenceDir); err != nil {
		return err
	}
	return nil
}

// stampV2ReadOnly tightens the V2 sidecar directories to 0o555 *after*
// the file writes finish, so the agent can read and traverse the inputs
// but cannot mutate or replace them. Files inside stay at 0o444. We
// chmod the directories directly (rather than recording them under
// 0o555 from the start) because recordMkdirAll at 0o555 would prevent
// the subsequent recordWriteFile calls from creating files inside.
func stampV2ReadOnly(dirs ...string) error {
	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		if err := os.Chmod(dir, 0o555); err != nil {
			return fmt.Errorf("stamp V2 read-only on %s: %w", dir, err)
		}
	}
	return nil
}

// buildV2ReferenceIndex summarises the task input the agent is supposed
// to read as evidence. The summary intentionally omits the full
// reference payloads and the full repository-analysis block — those
// stay in the canonical task.json. The index is the agent's
// "what is in this task at a glance" surface.
func buildV2ReferenceIndex(task map[string]json.RawMessage) ([]byte, error) {
	type referenceSummary struct {
		Kind  string `json:"kind"`
		Label string `json:"label,omitempty"`
	}
	type repositoryAnalysisSummary struct {
		FactsCount                   int `json:"facts_count"`
		SourceFilesCount             int `json:"source_files_count"`
		RepresentativeWorkflowsCount int `json:"representative_workflows_count"`
	}
	index := map[string]any{
		"schema_version": projectdesignsystem.SourceIndexSchemaV1,
	}
	if rawBrief, ok := task["brief"]; ok {
		var brief string
		if err := json.Unmarshal(rawBrief, &brief); err == nil {
			index["brief"] = brief
		}
	}
	if rawPlatform, ok := task["platform"]; ok {
		var platform string
		if err := json.Unmarshal(rawPlatform, &platform); err == nil {
			index["platform"] = platform
		}
	}
	if rawRefs, ok := task["references"]; ok && len(rawRefs) > 0 {
		var refs []referenceSummary
		if err := json.Unmarshal(rawRefs, &refs); err == nil {
			index["references"] = refs
		}
	}
	if rawAnalysis, ok := task["repository_analysis"]; ok && len(rawAnalysis) > 0 && string(rawAnalysis) != "null" {
		var analysis struct {
			Facts                   []map[string]any `json:"facts"`
			SourceFiles             []map[string]any `json:"source_files"`
			RepresentativeWorkflows []map[string]any `json:"representative_workflows"`
		}
		if err := json.Unmarshal(rawAnalysis, &analysis); err == nil {
			index["repository_analysis"] = repositoryAnalysisSummary{
				FactsCount:                   len(analysis.Facts),
				SourceFilesCount:             len(analysis.SourceFiles),
				RepresentativeWorkflowsCount: len(analysis.RepresentativeWorkflows),
			}
		}
	}
	return json.MarshalIndent(index, "", "  ")
}

// writeV2BaseDirectory materialises the read-only base/ tree for
// adjust / regenerate operations. The base package must already be a
// V2 native package (no legacy Open Design envelope); the integrity
// SHA-256 carried in the base must match the base_package_sha256
// stamped onto the task context, otherwise the task context and the
// on-disk base disagree and we refuse the workspace.
func writeV2BaseDirectory(root string, task map[string]json.RawMessage, manifest *sidecarManifest) error {
	rawBase, ok := task["base_package"]
	if !ok {
		return fmt.Errorf("V2 %s task missing base_package", taskOperation(task))
	}
	var base map[string]json.RawMessage
	if err := json.Unmarshal(rawBase, &base); err != nil {
		return fmt.Errorf("decode V2 base package: %w", err)
	}
	if rawSchema, ok := base["schema"]; ok {
		var schema string
		if err := json.Unmarshal(rawSchema, &schema); err == nil && schema == opendesign.BasePackageReferenceSchema {
			return fmt.Errorf("V2 base package uses Open Design reference schema; V2 adjust / regenerate requires a native base package")
		}
	}
	var baseDigest string
	if rawDigest, ok := base["integrity_sha256"]; ok {
		if err := json.Unmarshal(rawDigest, &baseDigest); err != nil {
			return fmt.Errorf("decode V2 base integrity_sha256: %w", err)
		}
	}
	if rawDeclared, ok := task["base_package_sha256"]; ok {
		var declared string
		if err := json.Unmarshal(rawDeclared, &declared); err != nil {
			return fmt.Errorf("decode V2 base_package_sha256: %w", err)
		}
		if declared != "" && baseDigest != "" && declared != baseDigest {
			return fmt.Errorf("V2 base package digest mismatch: task context claims %q, base integrity_sha256 is %q", declared, baseDigest)
		}
	}
	if baseDigest == "" {
		return fmt.Errorf("V2 base package missing integrity_sha256")
	}

	baseDir := filepath.Join(root, "base")
	if err := recordMkdirAll(baseDir, 0o755, manifest); err != nil {
		return err
	}
	files := []struct {
		key  string
		name string
	}{
		{key: "design_md", name: "DESIGN.md"},
		{key: "tokens_css", name: "tokens.css"},
		{key: "components_html", name: "components.html"},
	}
	for _, file := range files {
		var contents string
		if err := json.Unmarshal(base[file.key], &contents); err != nil {
			return fmt.Errorf("decode V2 base %s: %w", file.name, err)
		}
		if err := recordWriteFile(filepath.Join(baseDir, file.name), []byte(contents), 0o444, manifest); err != nil {
			return err
		}
	}
	return stampV2ReadOnly(baseDir)
}

func taskOperation(task map[string]json.RawMessage) string {
	if raw, ok := task["operation"]; ok {
		var op string
		if err := json.Unmarshal(raw, &op); err == nil {
			return op
		}
	}
	return ""
}

// projectResourceFile is the on-disk JSON written into the agent's working
// directory. Schema is intentionally a thin pass-through of the API response
// so consumers (skills, future tooling) don't need a separate parser.
type projectResourceFile struct {
	ProjectID    string                  `json:"project_id,omitempty"`
	ProjectTitle string                  `json:"project_title,omitempty"`
	Resources    []ProjectResourceForEnv `json:"resources"`
}

// MarshalJSON renders the resource_ref field as raw JSON instead of a base64
// blob. The struct's other fields are simple strings.
func (p ProjectResourceForEnv) MarshalJSON() ([]byte, error) {
	type alias struct {
		ID           string          `json:"id"`
		ResourceType string          `json:"resource_type"`
		ResourceRef  json.RawMessage `json:"resource_ref"`
		Label        string          `json:"label,omitempty"`
	}
	ref := p.ResourceRef
	if len(ref) == 0 {
		ref = json.RawMessage("{}")
	}
	return json.Marshal(alias{
		ID:           p.ID,
		ResourceType: p.ResourceType,
		ResourceRef:  ref,
		Label:        p.Label,
	})
}

// writeProjectResources writes .multica/project/resources.json into the
// working directory when the task carries project context. The file is
// always written when a project is attached (even with zero resources) so
// agents can rely on its presence as a signal that a project exists.
//
// manifest, when non-nil, is populated with the .multica/project chain
// of created directories and the resources.json file so CleanupSidecars
// can undo them on local_directory teardown.
func writeProjectResources(workDir string, ctx TaskContextForEnv, manifest *sidecarManifest) error {
	if ctx.ProjectID == "" && len(ctx.ProjectResources) == 0 {
		return nil
	}
	dir := filepath.Join(workDir, ".multica", "project")
	if err := recordMkdirAll(dir, 0o755, manifest); err != nil {
		return err
	}
	resources := ctx.ProjectResources
	if resources == nil {
		resources = []ProjectResourceForEnv{}
	}
	payload := projectResourceFile{
		ProjectID:    ctx.ProjectID,
		ProjectTitle: ctx.ProjectTitle,
		Resources:    resources,
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	if err := recordWriteFile(filepath.Join(dir, "resources.json"), data, 0o644, manifest); err != nil {
		// .multica/project/resources.json is Multica-owned and a
		// pre-existing path is almost certainly user content the
		// manifest must not destroy. The runtime brief already lists
		// every project resource so the agent runs fine without the
		// JSON sidecar — collision degrades to brief-only mode.
		if !errors.Is(err, errPathPreExists) {
			return err
		}
	}
	return nil
}

// resolveSkillsDir returns the directory where skills should be written
// based on the agent provider, creating it. manifest, when non-nil, is
// populated with every intermediate directory we had to MkdirAll so
// CleanupSidecars can rmdir them on local_directory teardown.
func resolveSkillsDir(workDir, provider string, manifest *sidecarManifest) (string, error) {
	skillsDir := skillsDirPath(workDir, provider)
	if err := recordMkdirAll(skillsDir, 0o755, manifest); err != nil {
		return "", err
	}
	return skillsDir, nil
}

// skillsDirPath returns the provider-native skills parent directory under
// workDir WITHOUT creating it or recording anything. resolveSkillsDir wraps
// this with the MkdirAll/manifest bookkeeping; the reuse-path skill rollback
// (removeReusedManagedSkillDirs) needs the bare path with no side effects so
// it can match the managed skill roots the prior manifest recorded.
func skillsDirPath(workDir, provider string) string {
	switch provider {
	case "claude":
		// Claude Code natively discovers skills from .claude/skills/ in the workdir.
		return filepath.Join(workDir, ".claude", "skills")
	case "copilot":
		// GitHub Copilot CLI natively discovers project-level skills from
		// .github/skills/<name>/SKILL.md (takes precedence over user-level
		// skills in ~/.copilot/skills/).
		// See: https://docs.github.com/en/copilot/reference/copilot-cli-reference/cli-config-dir-reference
		return filepath.Join(workDir, ".github", "skills")
	case "opencode":
		// OpenCode natively discovers project skills from .opencode/skills/ in
		// the workdir. ConfigPaths.directories() walks up from the discovery
		// root looking for a bare `.opencode` directory (no opencode.json
		// signal required), then skill/index.ts scans `{skill,skills}/**/SKILL.md`
		// under each match. Discovery is anchored at the task workdir via
		// `opencode run --dir <workDir>` + PWD override in opencodeBackend —
		// without those, OpenCode walks from the daemon's inherited PWD and
		// misses .opencode/skills + AGENTS.md entirely (MUL-2416).
		return filepath.Join(workDir, ".opencode", "skills")
	case "openclaw":
		// OpenClaw's native skill scanner reads <workspaceDir>/skills/. The
		// daemon pairs this with a per-task synthesized openclaw-config.json
		// (see openclaw_config.go) that pins agents.defaults.workspace to
		// workDir, so writing here is what the CLI actually scans. Before
		// MUL-2219 this used to fall back to .agent_context/skills/, which
		// no openclaw scan path ever inspected.
		return filepath.Join(workDir, "skills")
	case "pi":
		// Pi natively discovers skills from .pi/skills/ in the workdir.
		return filepath.Join(workDir, ".pi", "skills")
	case "cursor":
		// Cursor natively discovers skills from .cursor/skills/ in the workdir.
		return filepath.Join(workDir, ".cursor", "skills")
	case "kimi":
		// Kimi Code CLI auto-discovers project-level skills from .kimi/skills/
		// in the workdir. See https://moonshotai.github.io/kimi-cli/en/customization/skills.html
		return filepath.Join(workDir, ".kimi", "skills")
	case "kiro":
		// Kiro CLI auto-discovers project-level skills from .kiro/skills/
		// in the workdir.
		return filepath.Join(workDir, ".kiro", "skills")
	case "antigravity":
		// Antigravity (`agy`) auto-discovers workspace-level skills from
		// .agents/skills/ in the workdir. The CLI inherits Gemini CLI's
		// workspace skill layout; see https://antigravity.google/docs/gcli-migration
		// under "Workspace skills".
		return filepath.Join(workDir, ".agents", "skills")
	default:
		// Fallback: write to .agent_context/skills/ (referenced by meta config).
		return filepath.Join(workDir, ".agent_context", "skills")
	}
}

var nonAlphaNum = regexp.MustCompile(`[^a-z0-9]+`)

// ensureSkillFrontmatter returns SKILL.md content guaranteed to lead with a
// YAML frontmatter block carrying a parseable, non-empty `name` key.
//
// Runtimes like OpenCode silently drop SKILL.md whose frontmatter is missing
// or whose `name` doesn't parse, so we handle three cases:
//
//   - No frontmatter at all → synthesize one with `name: <slug>` (and the DB
//     description when available).
//   - Frontmatter present and already has a non-empty `name` → leave it
//     untouched. The upstream import may have shaped that block deliberately
//     to match a specific runtime, and we don't want to clobber it.
//   - Frontmatter present but missing `name` (e.g. an upstream skill whose
//     YAML only set `description`, with the directory slug filling in for
//     `name` at import time) → prepend `name: <slug>` as the first key of
//     the existing block so OpenCode can still route the skill.
func ensureSkillFrontmatter(content, slug, description string) string {
	fmStart, ok := frontmatterBodyStart(content)
	if !ok {
		var b strings.Builder
		b.WriteString("---\n")
		fmt.Fprintf(&b, "name: %s\n", slug)
		if d := strings.TrimSpace(description); d != "" {
			fmt.Fprintf(&b, "description: %s\n", yamlEscapeInline(d))
		}
		b.WriteString("---\n\n")
		b.WriteString(content)
		return b.String()
	}
	if hasFrontmatterName(content[fmStart:]) {
		return content
	}
	// Frontmatter exists but lacks a parseable `name`. Inject one as the
	// first key of the existing block and keep the rest verbatim (including
	// `description`, body, and any runtime-specific keys the import path
	// preserved).
	return content[:fmStart] + "name: " + slug + "\n" + content[fmStart:]
}

// frontmatterBodyStart returns the byte offset where the YAML body begins
// (just after the opening `---` line) and whether a valid opening delimiter
// was found.
func frontmatterBodyStart(content string) (int, bool) {
	if strings.HasPrefix(content, "---\n") {
		return 4, true
	}
	if strings.HasPrefix(content, "---\r\n") {
		return 5, true
	}
	return 0, false
}

// hasFrontmatterName reports whether the frontmatter body (the slice starting
// just after the opening `---` line) contains a parseable, non-empty `name:`
// scalar before the closing `---`.
func hasFrontmatterName(fmBody string) bool {
	closeIdx := strings.Index(fmBody, "\n---")
	if closeIdx < 0 {
		// Missing close — scan everything we have and fall through. The
		// frontmatter is malformed and OpenCode will reject it anyway, but
		// detecting an existing name keeps us from layering a second one
		// on top.
		closeIdx = len(fmBody)
	}
	for _, line := range strings.Split(fmBody[:closeIdx], "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "name:") {
			continue
		}
		v := strings.TrimSpace(strings.TrimPrefix(line, "name:"))
		v = strings.Trim(v, `"'`)
		if v != "" {
			return true
		}
	}
	return false
}

// yamlEscapeInline returns a double-quoted YAML scalar that always parses as
// a string. Plain scalars are deliberately avoided: values like `[foo]`,
// `{x: y}`, `false`, `null`, or `2024-01-01` would parse as flow sequences,
// flow mappings, booleans, nulls, or timestamps under YAML 1.2, and
// OpenCode's frontmatter check rejects non-string descriptions outright. We
// flatten newlines (frontmatter values are single-line per key) and escape
// `\` and `"` so any input is a safe inline string.
func yamlEscapeInline(s string) string {
	flat := strings.ReplaceAll(s, "\r\n", " ")
	flat = strings.ReplaceAll(flat, "\n", " ")
	flat = strings.ReplaceAll(flat, "\r", " ")
	escaped := strings.ReplaceAll(flat, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	return `"` + escaped + `"`
}

// sanitizeSkillName converts a skill name to a safe directory name.
func sanitizeSkillName(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = nonAlphaNum.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		s = "skill"
	}
	return s
}

// writeSkillFiles writes skill directories into the given parent directory.
// Each skill gets its own subdirectory containing SKILL.md and supporting
// files. manifest, when non-nil, is populated with every newly-created
// directory and file so CleanupSidecars can remove them on
// local_directory teardown without touching user-owned skill directories
// that happen to live alongside ours under the same skills/ parent.
//
// When a Multica skill's natural slug collides with a user-installed
// skill at the same path, we allocate a collision-free sibling slug
// (e.g. `issue-review-multica`) and write there instead. Provider-native
// discovery still picks it up because every subdir under skillsDir is a
// distinct skill; the user's original directory stays bit-for-bit
// intact. Without this fallback writeSkillFiles would have to either
// overwrite user bytes (the bug PR #3444 review caught) or skip the
// skill entirely (which would silently drop a Multica skill the agent
// expects to see).
func writeSkillFiles(skillsDir string, skills []SkillContextForEnv, manifest *sidecarManifest) error {
	if err := recordMkdirAll(skillsDir, 0o755, manifest); err != nil {
		return fmt.Errorf("create skills dir: %w", err)
	}

	for _, skill := range skills {
		baseSlug := sanitizeSkillName(skill.Name)
		slug, dir, err := allocateCollisionFreeSkillDir(skillsDir, baseSlug)
		if err != nil {
			return fmt.Errorf("allocate skill dir for %q: %w", skill.Name, err)
		}
		if err := recordMkdirAll(dir, 0o755, manifest); err != nil {
			return err
		}

		// ensureSkillFrontmatter synthesises a `name:` value when the
		// upstream skill is missing one. Use the chosen slug (which
		// may differ from baseSlug on collision) so the YAML name
		// matches the directory name; runtimes that key on either
		// stay consistent.
		body := ensureSkillFrontmatter(skill.Content, slug, skill.Description)
		if err := recordWriteFile(filepath.Join(dir, "SKILL.md"), []byte(body), 0o644, manifest); err != nil {
			return err
		}

		// Write supporting files. The skill directory is collision-
		// free by construction, so a recordWriteFile collision under
		// it would mean the skill's bundled files list two entries
		// at the same path — that's an upstream data bug, not a
		// user-content collision, and we surface it.
		//
		// One common data bug is storing SKILL.md as both the primary
		// content (skill.Content) and as a supporting file. Skip the
		// duplicate so the agent still gets every unique file. The check
		// is canonical (see skillpkg.IsReservedContentPath) so a
		// non-canonical spelling like "./SKILL.md" — which filepath.Join
		// resolves onto the same dir/SKILL.md we just wrote — is caught
		// too, instead of colliding and failing prep with errPathPreExists.
		for _, f := range skill.Files {
			if skillpkg.IsReservedContentPath(f.Path) {
				continue
			}
			fpath := filepath.Join(dir, f.Path)
			if err := recordMkdirAll(filepath.Dir(fpath), 0o755, manifest); err != nil {
				return err
			}
			if err := recordWriteFile(fpath, []byte(f.Content), 0o644, manifest); err != nil {
				return err
			}
		}
	}

	return nil
}

// renderIssueContext builds the markdown content for issue_context.md.
func renderIssueContext(provider string, ctx TaskContextForEnv) string {
	if ctx.AutopilotRunID != "" {
		return renderAutopilotContext(ctx)
	}
	if ctx.QuickCreatePrompt != "" {
		return renderQuickCreateContext(ctx)
	}
	if ctx.UIDraftCreateContext != "" {
		return renderUIDraftCreateContext(ctx)
	}
	if ctx.DesignRestoreContext != "" {
		return renderDesignRestoreContext(ctx)
	}
	if ctx.DesignSystemProfileAnalyzeContext != "" {
		return renderDesignSystemProfileAnalyzeContext(ctx)
	}
	if ctx.ProjectDesignSystemContext != "" {
		return renderProjectDesignSystemContext()
	}

	var b strings.Builder

	b.WriteString("# Task Assignment\n\n")
	fmt.Fprintf(&b, "**Issue ID:** %s\n\n", ctx.IssueID)

	if ctx.TriggerCommentID != "" {
		b.WriteString("**Trigger:** Comment Reply\n")
		b.WriteString("**Triggering comment ID:** `" + ctx.TriggerCommentID + "`\n\n")
	} else {
		b.WriteString("**Trigger:** New Assignment\n\n")
	}

	b.WriteString("## Quick Start\n\n")
	fmt.Fprintf(&b, "Run `multica issue get %s --output json` to fetch the full issue details.\n\n", ctx.IssueID)

	if len(ctx.AgentSkills) > 0 {
		b.WriteString("## Agent Skills\n\n")
		b.WriteString("The following skills are available to you:\n\n")
		for _, skill := range ctx.AgentSkills {
			fmt.Fprintf(&b, "- **%s**\n", skill.Name)
		}
		b.WriteString("\n")
	}

	return b.String()
}

func renderProjectDesignSystemContext() string {
	return "# Project Design System\n\nRead `.agent_context/project_design_system/task.json` before designing. Write the completed package to `$MULTICA_OUTPUT_DIR`.\n"
}

// renderQuickCreateContext renders issue_context.md for quick-create tasks.
// This file carries only task data (user input, skills). Behavioral rules
// and guardrails live in AGENTS.md (runtime config) and the per-turn prompt
// to avoid redundancy and conflicting instructions.
func renderQuickCreateContext(ctx TaskContextForEnv) string {
	var b strings.Builder
	b.WriteString("# Quick Create\n\n")
	b.WriteString("**Trigger:** Quick-create modal\n\n")
	b.WriteString("## User input\n\n")
	b.WriteString("> ")
	b.WriteString(ctx.QuickCreatePrompt)
	b.WriteString("\n\n")
	if len(ctx.AgentSkills) > 0 {
		b.WriteString("## Agent Skills\n\n")
		for _, skill := range ctx.AgentSkills {
			fmt.Fprintf(&b, "- **%s**\n", skill.Name)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func renderUIDraftCreateContext(ctx TaskContextForEnv) string {
	var b strings.Builder
	b.WriteString("# UI Draft Generation\n\n")
	b.WriteString("**Trigger:** Gallery Native template-assisted draft generation\n\n")
	b.WriteString("## Draft context JSON\n\n")
	b.WriteString("```json\n")
	b.WriteString(ctx.UIDraftCreateContext)
	b.WriteString("\n```\n\n")
	if len(ctx.AgentSkills) > 0 {
		b.WriteString("## Agent Skills\n\n")
		for _, skill := range ctx.AgentSkills {
			fmt.Fprintf(&b, "- **%s**\n", skill.Name)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func renderDesignRestoreContext(ctx TaskContextForEnv) string {
	var b strings.Builder
	b.WriteString("## Design Restore Context\n\n")
	b.WriteString("This task was created from a Gallery Native restore task. Use this JSON as the primary design/context payload.\n\n")
	b.WriteString("```json\n")
	b.WriteString(ctx.DesignRestoreContext)
	b.WriteString("\n```\n")
	return b.String()
}

func renderDesignSystemProfileAnalyzeContext(ctx TaskContextForEnv) string {
	var b strings.Builder
	b.WriteString("# Design System Profile Analysis\n\n")
	b.WriteString("**Trigger:** Figma UI specification upload\n\n")
	b.WriteString("## Analysis context JSON\n\n")
	b.WriteString("```json\n")
	b.WriteString(ctx.DesignSystemProfileAnalyzeContext)
	b.WriteString("\n```\n\n")
	if len(ctx.AgentSkills) > 0 {
		b.WriteString("## Agent Skills\n\n")
		for _, skill := range ctx.AgentSkills {
			fmt.Fprintf(&b, "- **%s**\n", skill.Name)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func renderAutopilotContext(ctx TaskContextForEnv) string {
	var b strings.Builder

	b.WriteString("# Autopilot Run\n\n")
	fmt.Fprintf(&b, "**Autopilot run ID:** %s\n\n", ctx.AutopilotRunID)
	if ctx.AutopilotID != "" {
		fmt.Fprintf(&b, "**Autopilot ID:** %s\n\n", ctx.AutopilotID)
	}
	if ctx.AutopilotTitle != "" {
		fmt.Fprintf(&b, "**Title:** %s\n\n", ctx.AutopilotTitle)
	}
	if ctx.AutopilotSource != "" {
		fmt.Fprintf(&b, "**Trigger source:** %s\n\n", ctx.AutopilotSource)
	}
	if ctx.AutopilotTriggerPayload != "" {
		fmt.Fprintf(&b, "## Trigger Payload\n\n```json\n%s\n```\n\n", ctx.AutopilotTriggerPayload)
	}

	b.WriteString("## Quick Start\n\n")
	b.WriteString("This is a run-only autopilot task with no assigned issue. Do not run `multica issue get` unless the autopilot instructions explicitly ask you to create or update an issue.\n\n")
	if ctx.AutopilotID != "" {
		fmt.Fprintf(&b, "Run `multica autopilot get %s --output json` if you need the full autopilot configuration.\n\n", ctx.AutopilotID)
	}
	if strings.TrimSpace(ctx.AutopilotDescription) != "" {
		b.WriteString("## Autopilot Instructions\n\n")
		b.WriteString(ctx.AutopilotDescription)
		b.WriteString("\n\n")
	}

	if len(ctx.AgentSkills) > 0 {
		b.WriteString("## Agent Skills\n\n")
		b.WriteString("The following skills are available to you:\n\n")
		for _, skill := range ctx.AgentSkills {
			fmt.Fprintf(&b, "- **%s**\n", skill.Name)
		}
		b.WriteString("\n")
	}

	return b.String()
}
