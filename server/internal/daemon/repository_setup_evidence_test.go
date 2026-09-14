package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRepositorySetupRetainsResolvedEvidenceColdAndWarm(t *testing.T) {
	installFakeRepositorySetupTools(t)
	root := t.TempDir()
	workDir := filepath.Join(root, "workdir")
	if err := os.Mkdir(workDir, 0o700); err != nil {
		t.Fatal(err)
	}
	writeRepositorySetupFiles(t, workDir)
	task := repositorySetupTask(t, "trusted", 60, "go_mod_download", "pnpm_install")
	for _, wantHit := range []bool{false, true} {
		result, err := prepareRepositorySetup(t.Context(), task, "codex", workDir, root)
		if err != nil {
			t.Fatal(err)
		}
		if result.CacheHit != wantHit || len(result.ToolEvidence) != 2 {
			t.Fatalf("cache hit=%t, tools=%d", result.CacheHit, len(result.ToolEvidence))
		}
		for index, evidence := range result.ToolEvidence {
			if evidence.Step != []string{"go_mod_download", "pnpm_install"}[index] || evidence.Directory != "" ||
				!filepath.IsAbs(evidence.RealPath) || !repositorySetupDigestName(evidence.SHA256) ||
				!repositorySetupDigestName(evidence.InputSHA256) || len(evidence.InputFiles) != 2 {
				t.Fatalf("incomplete resolved tool evidence: %#v", evidence)
			}
			got, err := digestRepositorySetupExecutable(t.Context(), evidence.RealPath)
			if err != nil || got != evidence.SHA256 {
				t.Fatalf("tool digest mismatch: %v", err)
			}
		}
	}
}

func TestRepositorySetupCompletionEvidenceUsesClaimScopeAndRedactsVersion(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	task := Task{ID: "task", RuntimeID: "runtime", ClaimAttempt: 3, ClaimGeneration: 7}
	setup := repositorySetupResult{
		Steps:    []repositorySetupStepResult{{Name: "pnpm_install", Directory: "web", Status: "completed"}},
		CacheKey: strings.Repeat("a", 64), CacheHit: true, SharedCacheStatus: repositorySetupSharedCacheHit,
		ToolEvidence: []repositorySetupToolEvidence{{Step: "pnpm_install", Directory: "web", RealPath: "/fixed/pnpm", Version: "credential-canary-do-not-log", SHA256: strings.Repeat("b", 64), InputSHA256: strings.Repeat("c", 64), InputFiles: []string{"package.json", "pnpm-lock.yaml"}}},
		Env:          map[string]string{"SECRET_TOKEN": "environment-canary-do-not-log"},
	}
	logRepositorySetupEvidence(context.Background(), logger, task, "codex", t.TempDir(), setup)
	if strings.Contains(output.String(), "canary") {
		t.Fatal("setup evidence leaked unclassified values")
	}
	if strings.Contains(output.String(), "/fixed/pnpm") || strings.Contains(output.String(), "realpath") {
		t.Fatal("setup evidence leaked a global tool path")
	}
	var event struct {
		TaskID          string `json:"task_id"`
		RuntimeID       string `json:"runtime_id"`
		ClaimAttempt    int    `json:"claim_attempt"`
		ClaimGeneration int64  `json:"claim_generation"`
		Evidence        struct {
			Schema        string                        `json:"schema"`
			Scope         string                        `json:"scope"`
			Revision      string                        `json:"revision"`
			RevisionState string                        `json:"revision_state"`
			Tools         []repositorySetupToolEvidence `json:"tools"`
		} `json:"preparation"`
	}
	if err := json.Unmarshal(output.Bytes(), &event); err != nil {
		t.Fatal(err)
	}
	if event.TaskID != task.ID || event.RuntimeID != task.RuntimeID || event.ClaimAttempt != 3 || event.ClaimGeneration != 7 ||
		event.Evidence.Schema != "repository_setup_projection/v1" || event.Evidence.Scope != "claim_preparation" ||
		event.Evidence.Revision != "" || event.Evidence.RevisionState != "unavailable" || len(event.Evidence.Tools) != 1 || event.Evidence.Tools[0].Version != "" {
		t.Fatalf("unexpected bounded preparation event: %s", output.String())
	}
}

func TestRepositorySetupCompletionEvidenceSkipsUnclaimedPreparation(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	logRepositorySetupEvidence(t.Context(), logger, Task{}, "codex", t.TempDir(), repositorySetupResult{
		Steps: []repositorySetupStepResult{{Name: "go_mod_download", Status: "completed"}},
	})
	if output.Len() != 0 {
		t.Fatal("unclaimed preparation emitted formal claim evidence")
	}
}

func TestRepositorySetupCompletionEvidenceBindsExistingGitRevision(t *testing.T) {
	workDir := t.TempDir()
	for _, args := range [][]string{
		{"init", "--quiet"},
		{"-c", "user.name=Setup Fixture", "-c", "user.email=fixture@example.invalid", "-c", "commit.gpgsign=false", "commit", "--allow-empty", "-m", "fixture", "--quiet"},
	} {
		if _, err := runGitCommandContext(t.Context(), workDir, 5*time.Second, args...); err != nil {
			t.Fatal(err)
		}
	}
	want, err := runGitCommandContext(t.Context(), workDir, 5*time.Second, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	before := time.Now()
	logRepositorySetupEvidence(t.Context(), slog.New(slog.NewJSONHandler(&output, nil)),
		Task{ID: "task", RuntimeID: "runtime", ClaimAttempt: 1, ClaimGeneration: 1}, "codex", workDir,
		repositorySetupResult{Steps: []repositorySetupStepResult{{Name: "go_mod_download", Status: "completed"}}})
	var event struct {
		Preparation repositorySetupProjection `json:"preparation"`
	}
	if err := json.Unmarshal(output.Bytes(), &event); err != nil {
		t.Fatal(err)
	}
	completed, err := time.Parse(time.RFC3339Nano, event.Preparation.CompletedAt)
	if err != nil || completed.Before(before) || completed.After(time.Now()) || event.Preparation.RevisionState != "observed" || event.Preparation.Revision != strings.TrimSpace(want) {
		t.Fatalf("revision/time binding missing: %s", output.String())
	}
}

func TestRepositorySetupEvidenceRetainsStepDirectory(t *testing.T) {
	installFakeRepositorySetupTools(t)
	root := t.TempDir()
	workDir := filepath.Join(root, "workdir")
	webDir := filepath.Join(workDir, "web")
	if err := os.MkdirAll(webDir, 0o700); err != nil {
		t.Fatal(err)
	}
	writeRepositorySetupFiles(t, webDir)
	task := setupTaskDirectories(t, repositorySetupTask(t, "trusted", 60, "pnpm_install"), map[string]string{"pnpm_install": "web"})
	result, err := prepareRepositorySetup(t.Context(), task, "codex", workDir, root)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.ToolEvidence) != 1 || result.ToolEvidence[0].Directory != "web" || result.ToolEvidence[0].Step != "pnpm_install" {
		t.Fatalf("step identity lost: %#v", result.ToolEvidence)
	}
}

func TestRepositorySetupVersionEvidenceIsBounded(t *testing.T) {
	for _, test := range []struct {
		step, version, want string
	}{
		{"go_mod_download", "go version go1.27.0 darwin/arm64", "go version go1.27.0 darwin/arm64"},
		{"go_mod_download", "go version go1.28rc1 linux/amd64", "go version go1.28rc1 linux/amd64"},
		{"pnpm_install", "10.28.2", "10.28.2"},
		{"pnpm_install", "10.28.2-beta.1", "10.28.2-beta.1"},
		{"pnpm_install", "10.28.2\nsecret=value", ""},
		{"pnpm_install", strings.Repeat("1", 129), ""},
		{"unknown", "10.28.2", ""},
	} {
		if got := boundedRepositorySetupVersion(test.step, test.version); got != test.want {
			t.Errorf("version classification failed for step %q", test.step)
		}
	}
}
