package daemon

import (
	"context"
	"log/slog"
	"regexp"
	"strings"
	"time"
)

type repositorySetupToolEvidence struct {
	Step        string   `json:"step"`
	Directory   string   `json:"directory,omitempty"`
	RealPath    string   `json:"-"`
	Version     string   `json:"version"`
	SHA256      string   `json:"sha256"`
	InputSHA256 string   `json:"input_sha256"`
	InputFiles  []string `json:"input_files"`
}

type repositorySetupProjection struct {
	Schema            string                        `json:"schema"`
	Scope             string                        `json:"scope"`
	Revision          string                        `json:"revision"`
	RevisionState     string                        `json:"revision_state"`
	CompletedAt       string                        `json:"completed_at"`
	Steps             []repositorySetupStepResult   `json:"steps"`
	Tools             []repositorySetupToolEvidence `json:"tools"`
	CacheKey          string                        `json:"cache_key"`
	ArtifactRestored  bool                          `json:"artifact_restored"`
	SharedCacheStatus string                        `json:"shared_cache_status"`
	Limitations       []string                      `json:"limitations"`
}

var repositorySetupGoVersion = regexp.MustCompile(`^go version go[0-9]+\.[0-9]+(\.[0-9]+)?([a-z]+[0-9]+)? [a-z0-9]+/[a-z0-9]+$`)
var repositorySetupPNPMVersion = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9.]+)?$`)
var repositorySetupGitRevision = regexp.MustCompile(`^([a-f0-9]{40}|[a-f0-9]{64})$`)

func boundedRepositorySetupVersion(step, version string) string {
	if len(version) > 128 {
		return ""
	}
	if step == "go_mod_download" && repositorySetupGoVersion.MatchString(version) ||
		step == "pnpm_install" && repositorySetupPNPMVersion.MatchString(version) {
		return version
	}
	return ""
}

// Preparation belongs to the claim, not to a provider's internal retry counter.
// Keep this best-effort projection out of the task's success/failure decisions.
func logRepositorySetupEvidence(ctx context.Context, logger *slog.Logger, task Task, provider, workDir string, setup repositorySetupResult) {
	if logger == nil || task.ClaimAttempt <= 0 || task.ClaimGeneration <= 0 || len(setup.Steps) == 0 {
		return
	}
	projection := repositorySetupProjection{
		Schema: "repository_setup_projection/v1", Scope: "claim_preparation",
		RevisionState: "unavailable", Steps: setup.Steps,
		CacheKey: setup.CacheKey, ArtifactRestored: setup.CacheHit, SharedCacheStatus: setup.SharedCacheStatus,
		Limitations: []string{"artifact_hit_not_build_cache_warmth", "package_manager_bundle_not_observed", "environment_values_not_recorded"},
	}
	for _, tool := range setup.ToolEvidence {
		tool.Version = boundedRepositorySetupVersion(tool.Step, tool.Version)
		projection.Tools = append(projection.Tools, tool)
	}
	if revision, err := runGitCommandContext(ctx, workDir, 2*time.Second, "rev-parse", "--verify", "HEAD"); err == nil {
		revision = strings.TrimSpace(revision)
		if repositorySetupGitRevision.MatchString(revision) {
			projection.Revision, projection.RevisionState = revision, "observed"
		}
	}
	projection.CompletedAt = time.Now().UTC().Format(time.RFC3339Nano)
	logger.Info("repository setup evidence", "task_id", task.ID, "runtime_id", task.RuntimeID,
		"claim_attempt", task.ClaimAttempt, "claim_generation", task.ClaimGeneration,
		"provider", provider, "cwd", workDir, "preparation", projection)
}
