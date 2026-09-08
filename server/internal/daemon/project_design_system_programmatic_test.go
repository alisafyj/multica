package daemon

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/daemon/execenv"
	"github.com/multica-ai/multica/server/internal/designpreview"
	"github.com/multica-ai/multica/server/internal/projectdesignsystem"
	"github.com/multica-ai/multica/server/internal/service"
)

func TestProgrammaticFirstProjectDesignSystemTaskPredicateIsFailClosed(t *testing.T) {
	valid := service.ProjectDesignSystemTaskContext{
		Type: service.ProjectDesignSystemTaskContextType, Operation: service.ProjectDesignSystemGenerate,
		ExecutionMode:     service.ProjectDesignSystemExecutionModeProgrammaticFirst,
		ProjectResourceID: "repository-1", PackageSchema: projectdesignsystem.PackageSchemaV2,
	}
	encode := func(value service.ProjectDesignSystemTaskContext) Task {
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		return Task{ProjectDesignSystemContext: raw}
	}
	if !isProgrammaticFirstProjectDesignSystemTask(encode(valid)) {
		t.Fatal("valid programmatic-first context was not recognized")
	}
	settingsRepository := valid
	settingsRepository.ProjectResourceID = ""
	settingsRepository.WorkspaceRepositoryID = "repository-settings-1"
	if !isProgrammaticFirstProjectDesignSystemTask(encode(settingsRepository)) {
		t.Fatal("settings-repository programmatic context was not recognized")
	}
	regenerate := valid
	regenerate.Operation = service.ProjectDesignSystemRegenerate
	if !isProgrammaticFirstProjectDesignSystemTask(encode(regenerate)) {
		t.Fatal("programmatic-first regeneration was not recognized")
	}
	cases := map[string]service.ProjectDesignSystemTaskContext{
		"ordinary agent generate": func() service.ProjectDesignSystemTaskContext { value := valid; value.ExecutionMode = ""; return value }(),
		"adjustment": func() service.ProjectDesignSystemTaskContext {
			value := valid
			value.Operation = service.ProjectDesignSystemAdjust
			return value
		}(),
		"project scope": func() service.ProjectDesignSystemTaskContext {
			value := valid
			value.ProjectResourceID = ""
			return value
		}(),
		"legacy package": func() service.ProjectDesignSystemTaskContext { value := valid; value.PackageSchema = ""; return value }(),
	}
	for name, value := range cases {
		t.Run(name, func(t *testing.T) {
			if isProgrammaticFirstProjectDesignSystemTask(encode(value)) {
				t.Fatalf("%s unexpectedly entered the no-model path", name)
			}
		})
	}
}

func TestProgrammaticTaskContextCarriesNoAgentPromptSkillsOrMCPState(t *testing.T) {
	raw := json.RawMessage(`{"type":"project_design_system_task","operation":"generate","execution_mode":"programmatic_first"}`)
	task := Task{
		ID: "task-1", AgentID: "agent-1", ProjectID: "project-1", ProjectTitle: "Clinic",
		ProjectDesignSystemContext: raw,
		ProjectResources: []ProjectResourceData{{
			ID: "repository-1", ResourceType: "github_repo",
			ResourceRef: json.RawMessage(`{"url":"https://example.test/clinic.git","ref":"release"}`),
		}},
	}
	context := programmaticTaskContextForEnv(task)
	if context.ProjectDesignSystemContext != string(raw) || len(context.ProjectResources) != 1 {
		t.Fatalf("programmatic context lost its fixed task input: %+v", context)
	}
	if context.AgentInstructions != "" || len(context.AgentSkills) != 0 || len(context.DisabledRuntimeSkills) != 0 || len(context.ConnectedApps) != 0 {
		t.Fatalf("programmatic context leaked Agent runtime state: %+v", context)
	}

	name, url, ref := selectedProgrammaticResource(task.ProjectResources)
	if name != "" || url != "https://example.test/clinic.git" || ref != "release" {
		t.Fatalf("selected resource = %q %q %q", name, url, ref)
	}
}

func TestPrepareProgrammaticRepositoryRequiresExactClaimedResource(t *testing.T) {
	d := &Daemon{}
	_, _, _, _, err := d.prepareProgrammaticRepository(context.Background(), Task{
		ProjectResources: []ProjectResourceData{{ID: "other", ResourceType: "github_repo", ResourceRef: json.RawMessage(`{"url":"https://example.test/other.git"}`)}},
	}, service.ProjectDesignSystemTaskContext{ProjectResourceID: "selected"}, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "selected repository resource is missing") {
		t.Fatalf("error = %v", err)
	}
}

func TestProgrammaticProviderPreparesV2OutputWithoutProviderHome(t *testing.T) {
	root := t.TempDir()
	taskContext, err := json.Marshal(service.ProjectDesignSystemTaskContext{
		Type: service.ProjectDesignSystemTaskContextType, Operation: service.ProjectDesignSystemGenerate,
		ExecutionMode: service.ProjectDesignSystemExecutionModeProgrammaticFirst,
		PackageSchema: projectdesignsystem.PackageSchemaV2,
	})
	if err != nil {
		t.Fatal(err)
	}
	env, err := execenv.Prepare(execenv.PrepareParams{
		WorkspacesRoot: root, WorkspaceID: "workspace-1", TaskID: "task-1",
		Provider: "programmatic",
		Task:     execenv.TaskContextForEnv{TaskID: "task-1", ProjectDesignSystemContext: string(taskContext)},
	}, slog.Default())
	if err != nil {
		t.Fatalf("prepare programmatic environment: %v", err)
	}
	defer env.Cleanup(true)
	if env.CodexHome != "" || env.ClaudeSettingsPath != "" || env.OutputDir == "" {
		t.Fatalf("programmatic environment = %+v", env)
	}
	if _, err := os.Stat(filepath.Join(env.WorkDir, ".agent_context", "project_design_system", "context", "task.json")); err != nil {
		t.Fatalf("fixed task context is unavailable: %v", err)
	}
}

func TestProgrammaticPackagePassesDaemonAuditPreviewAndUploadGate(t *testing.T) {
	repository := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repository, "styles"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repository, "styles", "theme.css"), []byte(`:root { --brand-primary: #0f766e; --page-background: #ffffff; --text-primary: #172033; --border-color: #d7dee8; } .card { padding: 16px; border-radius: 10px; }`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repository, "components"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repository, "components", "AppointmentCard.tsx"), []byte(`export function AppointmentCard(){ return <article>Appointment</article> }`), 0o644); err != nil {
		t.Fatal(err)
	}

	envRoot := t.TempDir()
	outputDir := filepath.Join(envRoot, "output", "project-design-system")
	inputDigest := "sha256:" + strings.Repeat("a", 64)
	if _, err := projectdesignsystem.GenerateProgrammaticFirstPackage(context.Background(), repository, outputDir, projectdesignsystem.ProgrammaticInput{
		ProjectName: "Clinic", RepositoryName: "clinic-web", CommitSHA: strings.Repeat("b", 40),
		Platform: "web", Brief: "A calm clinic product.", InputSnapshotSHA256: inputDigest,
	}, nil); err != nil {
		t.Fatalf("generate programmatic package: %v", err)
	}

	taskID := "task-programmatic"
	taskContext, err := json.Marshal(service.ProjectDesignSystemTaskContext{
		Type: service.ProjectDesignSystemTaskContextType, Operation: service.ProjectDesignSystemGenerate,
		ExecutionMode: service.ProjectDesignSystemExecutionModeProgrammaticFirst,
		WorkspaceID:   "workspace-1", ProjectID: "project-1", ProjectDesignSystemID: "design-system-1",
		ProjectResourceID: "repository-1", AgentID: "agent-1", PackageSchema: projectdesignsystem.PackageSchemaV2,
		InputSnapshotSHA256: inputDigest,
	})
	if err != nil {
		t.Fatal(err)
	}
	client := newFinalizingClient()
	verifier := newV2VerifyStub(t)
	finalized, err := finalizeProjectDesignSystemResult(context.Background(), Task{
		ID: taskID, AgentID: "agent-1", ProjectDesignSystemContext: taskContext,
	}, TaskResult{Status: "completed", Comment: "programmatic quick draft", EnvRoot: envRoot}, finalizeDeps{
		ResolveBrowserPath: func(string) (string, error) { return "/dev/null/chromium", nil },
		NewVerifier:        func(string, designpreview.Policy) (designpreview.Verifier, error) { return verifier, nil },
		Upload:             client,
	})
	if err != nil {
		t.Fatalf("finalize programmatic package: %v", err)
	}
	if finalized.Status != "completed" || finalized.ProjectDesignSystemPackage == nil || verifier.called != 1 || len(client.uploadedAt) == 0 {
		t.Fatalf("programmatic package did not pass the normal gate: status=%q reason=%q receipt=%+v verifier=%d upload=%d", finalized.Status, finalized.FailureReason, finalized.ProjectDesignSystemPackage, verifier.called, len(client.uploadedAt))
	}
}
