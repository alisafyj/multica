package opendesign

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

type supervisorWorkerFake struct {
	status        WorkerRunStatus
	runLookups    []supervisorRunLookup
	runLookupCall int
	resultPackage json.RawMessage
	manifest      json.RawMessage
	archive       []byte
	audit         PackageAudit
	previewURLs   []PreviewURL
	previewCalls  []PreviewTarget
	events        []RunEvent
	streamPlans   []supervisorStreamPlan
	streamAfters  []int64
	startCalls    int
	cancelCalls   int
	prepareErr    error
	auditErr      error
	previewURLErr error
}

type supervisorRunLookup struct {
	status WorkerRunStatus
	err    error
}

type supervisorStreamPlan struct {
	events []RunEvent
	err    error
}

func (f *supervisorWorkerFake) PrepareWorkspace(context.Context, WorkerWorkspaceRequest) (WorkerWorkspace, error) {
	if f.prepareErr != nil {
		return WorkerWorkspace{}, f.prepareErr
	}
	return WorkerWorkspace{ProjectID: "project-1", ConversationID: "conversation-1"}, nil
}

func (f *supervisorWorkerFake) StartRun(context.Context, WorkerStartRunRequest) (string, error) {
	f.startCalls++
	return "11111111-1111-4111-8111-111111111111", nil
}

func (f *supervisorWorkerFake) StreamRunEvents(_ context.Context, _ string, after int64, consume func(RunEvent) error) error {
	call := len(f.streamAfters)
	f.streamAfters = append(f.streamAfters, after)
	events := f.events
	var streamErr error
	if call < len(f.streamPlans) {
		events = f.streamPlans[call].events
		streamErr = f.streamPlans[call].err
	}
	for _, event := range events {
		if err := consume(event); err != nil {
			return err
		}
	}
	return streamErr
}

func (f *supervisorWorkerFake) GetRun(context.Context, string) (WorkerRunStatus, error) {
	if f.runLookupCall < len(f.runLookups) {
		lookup := f.runLookups[f.runLookupCall]
		f.runLookupCall++
		return lookup.status, lookup.err
	}
	return f.status, nil
}

func (f *supervisorWorkerFake) GetResultPackage(context.Context, string) (json.RawMessage, error) {
	return f.resultPackage, nil
}

func (f *supervisorWorkerFake) GetProjectExportManifest(context.Context, string) (json.RawMessage, error) {
	return f.manifest, nil
}

func (f *supervisorWorkerFake) GetProjectArchive(context.Context, string) ([]byte, error) {
	return f.archive, nil
}

func (f *supervisorWorkerFake) GetProjectPackageAudit(context.Context, string) (PackageAudit, error) {
	return f.audit, f.auditErr
}

func (f *supervisorWorkerFake) GetProjectPreviewURL(_ context.Context, _ string, target PreviewTarget) (PreviewURL, error) {
	f.previewCalls = append(f.previewCalls, target)
	if f.previewURLErr != nil {
		return PreviewURL{}, f.previewURLErr
	}
	previewURL := PreviewURL{Target: target, URL: "http://127.0.0.1:17456/preview/" + target.ID}
	f.previewURLs = append(f.previewURLs, previewURL)
	return previewURL, nil
}

func (f *supervisorWorkerFake) CancelRun(context.Context, string) (WorkerRunStatus, error) {
	f.cancelCalls++
	return WorkerRunStatus{Status: "canceled"}, nil
}

type supervisorProbeFake struct {
	report PreflightReport
}

func (f supervisorProbeFake) Probe(context.Context, ExpectedPreflight, string) (PreflightReport, error) {
	return f.report, nil
}

type supervisorCallbackFake struct {
	preflight  []PreflightReport
	starts     []string
	events     []RunEvent
	archives   []supervisorArchiveUpload
	results    []CollectedRunResult
	audits     []PackageAuditReceipt
	previews   []PreviewVerificationReceipt
	terminals  []RunTerminalRequest
	lifecycle  []string
	archiveErr error
	resultErr  error
	auditErr   error
	previewErr error
}

type supervisorArchiveUpload struct {
	RunID         string
	ContentDigest string
	Archive       []byte
}

func (f *supervisorCallbackFake) ReportOpenDesignPreflight(_ context.Context, _ string, report PreflightReport) error {
	f.preflight = append(f.preflight, report)
	return nil
}

func (f *supervisorCallbackFake) StartOpenDesignRun(_ context.Context, _, runID string) error {
	f.starts = append(f.starts, runID)
	return nil
}

func (f *supervisorCallbackFake) ReportOpenDesignRunEvent(_ context.Context, _, _ string, event RunEvent) error {
	f.events = append(f.events, event)
	return nil
}

func (f *supervisorCallbackFake) UploadOpenDesignRunArchive(_ context.Context, _, runID, contentDigest string, archive []byte) (string, error) {
	f.lifecycle = append(f.lifecycle, "archive")
	f.archives = append(f.archives, supervisorArchiveUpload{
		RunID:         runID,
		ContentDigest: contentDigest,
		Archive:       append([]byte(nil), archive...),
	})
	if f.archiveErr != nil {
		return "", f.archiveErr
	}
	return "workspaces/workspace-1/design-systems/design-system-1/open-design-runs/task-1/archive.zip", nil
}

func (f *supervisorCallbackFake) ReportOpenDesignRunResult(_ context.Context, _, _ string, result CollectedRunResult) error {
	f.lifecycle = append(f.lifecycle, "result")
	f.results = append(f.results, result)
	return f.resultErr
}

func (f *supervisorCallbackFake) ReportOpenDesignRunAudit(_ context.Context, _, _ string, receipt PackageAuditReceipt) error {
	f.lifecycle = append(f.lifecycle, "audit")
	f.audits = append(f.audits, receipt)
	return f.auditErr
}

func (f *supervisorCallbackFake) ReportOpenDesignRunPreview(_ context.Context, _, _ string, receipt PreviewVerificationReceipt) error {
	f.lifecycle = append(f.lifecycle, "preview")
	f.previews = append(f.previews, receipt)
	return f.previewErr
}

func (f *supervisorCallbackFake) FinalizeOpenDesignRun(_ context.Context, _, runID string, status RunStatus, failure json.RawMessage) error {
	f.terminals = append(f.terminals, RunTerminalRequest{OpenDesignRunID: runID, Status: status, Failure: failure})
	return nil
}

func successfulPreflight(context TaskRunContext) PreflightReport {
	return PreflightReport{
		Schema:     PreflightSchema,
		Engine:     context.Engine,
		AdapterID:  context.Agent.AdapterID,
		Model:      context.Agent.Model,
		Binary:     ProbeResult{Status: ProbePassed, Version: "1.0.0"},
		Auth:       ProbeResult{Status: ProbePassed, Required: true},
		ModelProbe: ProbeResult{Status: ProbePassed},
		Plugins:    PluginPreflight{Policy: PluginsDisabled},
	}
}

type supervisorPreviewVerifierFake struct {
	verification PreviewVerification
	err          error
	targets      []PreviewURL
}

func (f *supervisorPreviewVerifierFake) Verify(_ context.Context, targets []PreviewURL) (PreviewVerification, error) {
	f.targets = append([]PreviewURL(nil), targets...)
	return f.verification, f.err
}

func TestSupervisorPersistsRunEvidenceWithoutDeclaringFinalSuccess(t *testing.T) {
	t.Parallel()

	runContext := TaskRunContext{
		Schema: RunSchema,
		RunID:  "22222222-2222-4222-8222-222222222222",
		Engine: PinnedEngineIdentity(),
		Agent: AgentIdentity{
			MulticaAgentID: "33333333-3333-4333-8333-333333333333",
			AdapterID:      "opencode",
		},
	}
	workerRunID := "11111111-1111-4111-8111-111111111111"
	worker := &supervisorWorkerFake{
		status: WorkerRunStatus{ID: workerRunID, Status: "succeeded", ExitCode: intPointer(0)},
		events: []RunEvent{
			{ID: 1, Event: "start", Data: json.RawMessage(`{"status":"running"}`)},
			{ID: 2, Event: "end", Data: json.RawMessage(`{"status":"succeeded"}`)},
		},
		resultPackage: json.RawMessage(`{"schema":"open-design.run-result-package.v1","run":{"id":"11111111-1111-4111-8111-111111111111"},"workspace":{"storage":{"baseDir":"/private/tmp/workspace"}},"events":{"logPath":"/private/tmp/events.jsonl"}}`),
		manifest:      successfulPreviewExportManifest(),
		archive:       successfulPreviewArchive(t),
		audit:         successfulPackageAudit(),
	}
	callbacks := &supervisorCallbackFake{}
	previewVerifier := &supervisorPreviewVerifierFake{verification: successfulPreviewVerification()}
	supervisor, err := NewSupervisor(SupervisorConfig{
		ArtifactRoot: "/pinned/open-design",
		Worker:       worker,
		Probe:        supervisorProbeFake{report: successfulPreflight(runContext)},
		Callbacks:    callbacks,
		Preview:      previewVerifier,
		VerifyArtifact: func(string, EngineIdentity) (ArtifactVerification, error) {
			return ArtifactVerification{}, nil
		},
	})
	if err != nil {
		t.Fatalf("NewSupervisor: %v", err)
	}
	result, err := supervisor.Run(context.Background(), SupervisorRunRequest{
		TaskID:      "44444444-4444-4444-8444-444444444444",
		Context:     runContext,
		ScratchRoot: t.TempDir(),
		ProjectName: "CRM design system",
		Prompt:      "Create the project design system",
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Status != RunStatusSucceeded || result.WorkerRunID != workerRunID {
		t.Fatalf("result = %+v", result)
	}
	if len(callbacks.preflight) != 1 || len(callbacks.starts) != 1 || len(callbacks.events) != 2 || len(callbacks.results) != 1 || len(callbacks.audits) != 1 || len(callbacks.previews) != 1 {
		t.Fatalf("callbacks = %+v", callbacks)
	}
	if len(callbacks.archives) != 1 || callbacks.archives[0].RunID != workerRunID || callbacks.archives[0].ContentDigest != callbacks.results[0].ContentDigest || string(callbacks.archives[0].Archive) != string(worker.archive) {
		t.Fatalf("archive callback = %+v", callbacks.archives)
	}
	if strings.Join(callbacks.lifecycle, ",") != "archive,result,audit,preview" {
		t.Fatalf("callback lifecycle = %#v, want Preview after archive, result, and audit", callbacks.lifecycle)
	}
	if len(callbacks.terminals) != 0 {
		t.Fatalf("success must not bypass audit/preview with a final terminal callback: %+v", callbacks.terminals)
	}
	if strings.Contains(string(callbacks.results[0].ResultPackage), "/private/tmp") || len(callbacks.results[0].ArtifactIndex) != 4 || callbacks.results[0].ContentDigest == "" {
		t.Fatalf("collected result = %+v", callbacks.results[0])
	}
	if callbacks.audits[0].Schema != PackageAuditReceiptSchema || callbacks.audits[0].ContentDigest != callbacks.results[0].ContentDigest || callbacks.audits[0].Engine != runContext.Engine || !callbacks.audits[0].Audit.OK {
		t.Fatalf("audit receipt = %+v", callbacks.audits[0])
	}
	if len(previewVerifier.targets) != 2 || previewVerifier.targets[0].Target.Path != "preview/colors.html" || previewVerifier.targets[1].Target.Path != previewUIKitPath {
		t.Fatalf("Preview targets = %+v", previewVerifier.targets)
	}
	if callbacks.previews[0].Schema != PreviewVerificationReceiptSchema || callbacks.previews[0].ContentDigest != callbacks.results[0].ContentDigest || !callbacks.previews[0].Verification.Passed {
		t.Fatalf("Preview receipt = %+v", callbacks.previews[0])
	}
}

func TestSupervisorPersistsRejectedPackageAuditWithoutAdvancingToPreview(t *testing.T) {
	t.Parallel()

	runContext := TaskRunContext{
		Schema: RunSchema,
		RunID:  "22222222-2222-4222-8222-222222222222",
		Engine: PinnedEngineIdentity(),
		Agent:  AgentIdentity{MulticaAgentID: "33333333-3333-4333-8333-333333333333", AdapterID: "opencode"},
	}
	workerRunID := "11111111-1111-4111-8111-111111111111"
	worker := &supervisorWorkerFake{
		status:        WorkerRunStatus{ID: workerRunID, Status: "succeeded", ExitCode: intPointer(0)},
		resultPackage: json.RawMessage(`{"schema":"open-design.run-result-package.v1","run":{"id":"11111111-1111-4111-8111-111111111111"}}`),
		manifest: testProjectExportManifest(map[string]testManifestFile{
			"index.html": {MIME: "text/html", Role: "entry", Body: "<main></main>"},
		}),
		archive: testProjectArchive(t, []testArchiveFile{{Path: "index.html", Body: "<main></main>"}}),
		audit: PackageAudit{
			OK:             false,
			FilesInspected: 1,
			Errors: []PackageAuditIssue{{
				Severity: "error",
				Code:     "missing_required_file",
				Message:  "DESIGN.md is required",
				Path:     "DESIGN.md",
			}},
			Warnings: []PackageAuditIssue{},
		},
	}
	callbacks := &supervisorCallbackFake{}
	previewVerifier := &supervisorPreviewVerifierFake{verification: successfulPreviewVerification()}
	supervisor, err := NewSupervisor(SupervisorConfig{
		ArtifactRoot: "/pinned/open-design",
		Worker:       worker,
		Probe:        supervisorProbeFake{report: successfulPreflight(runContext)},
		Callbacks:    callbacks,
		Preview:      previewVerifier,
		VerifyArtifact: func(string, EngineIdentity) (ArtifactVerification, error) {
			return ArtifactVerification{}, nil
		},
	})
	if err != nil {
		t.Fatalf("NewSupervisor: %v", err)
	}
	result, err := supervisor.Run(context.Background(), SupervisorRunRequest{
		TaskID:      "44444444-4444-4444-8444-444444444444",
		Context:     runContext,
		ScratchRoot: t.TempDir(),
		ProjectName: "CRM design system",
		Prompt:      "Create the project design system",
	})
	if !errors.Is(err, ErrPackageAuditFailed) {
		t.Fatalf("Run error = %v, want ErrPackageAuditFailed", err)
	}
	if result.Status != RunStatusAuditFailed || len(callbacks.audits) != 1 || callbacks.audits[0].Audit.OK || len(callbacks.terminals) != 0 {
		t.Fatalf("result = %+v, callbacks = %+v", result, callbacks)
	}
	if strings.Join(callbacks.lifecycle, ",") != "archive,result,audit" {
		t.Fatalf("callback lifecycle = %#v", callbacks.lifecycle)
	}
	if len(previewVerifier.targets) != 0 || len(callbacks.previews) != 0 || len(worker.previewCalls) != 0 {
		t.Fatalf("rejected Audit advanced to Preview: verifier=%+v callbacks=%+v URLs=%+v", previewVerifier.targets, callbacks.previews, worker.previewCalls)
	}
}

func TestSupervisorPersistsWarningOnlyPackageAuditWithoutAdvancingToPreview(t *testing.T) {
	t.Parallel()

	runContext := TaskRunContext{
		Schema: RunSchema,
		RunID:  "22222222-2222-4222-8222-222222222222",
		Engine: PinnedEngineIdentity(),
		Agent:  AgentIdentity{MulticaAgentID: "33333333-3333-4333-8333-333333333333", AdapterID: "opencode"},
	}
	workerRunID := "11111111-1111-4111-8111-111111111111"
	worker := &supervisorWorkerFake{
		status:        WorkerRunStatus{ID: workerRunID, Status: "succeeded", ExitCode: intPointer(0)},
		resultPackage: json.RawMessage(`{"schema":"open-design.run-result-package.v1","run":{"id":"11111111-1111-4111-8111-111111111111"}}`),
		manifest: testProjectExportManifest(map[string]testManifestFile{
			"index.html": {MIME: "text/html", Role: "entry", Body: "<main></main>"},
		}),
		archive: testProjectArchive(t, []testArchiveFile{{Path: "index.html", Body: "<main></main>"}}),
		audit: PackageAudit{
			OK:             false,
			FilesInspected: 39,
			Errors:         []PackageAuditIssue{},
			Warnings: []PackageAuditIssue{{
				Severity: "warning",
				Code:     "readme_missing_product_overview",
				Message:  "README needs a product overview",
				Path:     "README.md",
			}},
		},
	}
	callbacks := &supervisorCallbackFake{}
	previewVerifier := &supervisorPreviewVerifierFake{verification: successfulPreviewVerification()}
	supervisor, err := NewSupervisor(SupervisorConfig{
		ArtifactRoot: "/pinned/open-design",
		Worker:       worker,
		Probe:        supervisorProbeFake{report: successfulPreflight(runContext)},
		Callbacks:    callbacks,
		Preview:      previewVerifier,
		VerifyArtifact: func(string, EngineIdentity) (ArtifactVerification, error) {
			return ArtifactVerification{}, nil
		},
	})
	if err != nil {
		t.Fatalf("NewSupervisor: %v", err)
	}
	result, err := supervisor.Run(context.Background(), SupervisorRunRequest{
		TaskID:      "44444444-4444-4444-8444-444444444444",
		Context:     runContext,
		ScratchRoot: t.TempDir(),
		ProjectName: "CRM design system",
		Prompt:      "Create the project design system",
	})
	if !errors.Is(err, ErrPackageAuditFailed) {
		t.Fatalf("Run error = %v, want ErrPackageAuditFailed", err)
	}
	if result.Status != RunStatusAuditFailed || len(callbacks.audits) != 1 || callbacks.audits[0].Audit.OK || len(callbacks.audits[0].Audit.Warnings) != 1 || len(callbacks.terminals) != 0 {
		t.Fatalf("result = %+v, callbacks = %+v", result, callbacks)
	}
	if strings.Join(callbacks.lifecycle, ",") != "archive,result,audit" {
		t.Fatalf("callback lifecycle = %#v", callbacks.lifecycle)
	}
	if len(previewVerifier.targets) != 0 || len(callbacks.previews) != 0 || len(worker.previewCalls) != 0 {
		t.Fatalf("warning-only Audit advanced to Preview: verifier=%+v callbacks=%+v URLs=%+v", previewVerifier.targets, callbacks.previews, worker.previewCalls)
	}
}

func TestSupervisorPersistsRejectedPreviewWithoutCreatingSuccess(t *testing.T) {
	t.Parallel()

	runContext := TaskRunContext{
		Schema: RunSchema,
		RunID:  "22222222-2222-4222-8222-222222222222",
		Engine: PinnedEngineIdentity(),
		Agent:  AgentIdentity{MulticaAgentID: "33333333-3333-4333-8333-333333333333", AdapterID: "opencode"},
	}
	workerRunID := "11111111-1111-4111-8111-111111111111"
	worker := &supervisorWorkerFake{
		status:        WorkerRunStatus{ID: workerRunID, Status: "succeeded", ExitCode: intPointer(0)},
		resultPackage: json.RawMessage(`{"schema":"open-design.run-result-package.v1","run":{"id":"11111111-1111-4111-8111-111111111111"}}`),
		manifest:      successfulPreviewExportManifest(),
		archive:       successfulPreviewArchive(t),
		audit:         successfulPackageAudit(),
	}
	failedVerification := successfulPreviewVerification()
	failedVerification.Targets[0].Screenshot.Entropy = 0
	failedVerification.Targets[0].Screenshot.MaxChannelStddev = 0
	failedVerification.Targets[0] = EvaluatePreviewCapture(PreviewCapture{
		Target:                    failedVerification.Targets[0].Target,
		DocumentLoaded:            true,
		DOMPresent:                true,
		ComputedVisibilityVisible: true,
		RenderedElementCount:      41,
		VisibleTextLength:         317,
		BodyWidth:                 1425,
		BodyHeight:                1064,
		Screenshot:                failedVerification.Targets[0].Screenshot,
	}, failedVerification.Policy)
	failedVerification.Passed = false
	callbacks := &supervisorCallbackFake{}
	previewVerifier := &supervisorPreviewVerifierFake{verification: failedVerification}
	supervisor, err := NewSupervisor(SupervisorConfig{
		ArtifactRoot: "/pinned/open-design",
		Worker:       worker,
		Probe:        supervisorProbeFake{report: successfulPreflight(runContext)},
		Callbacks:    callbacks,
		Preview:      previewVerifier,
		VerifyArtifact: func(string, EngineIdentity) (ArtifactVerification, error) {
			return ArtifactVerification{}, nil
		},
	})
	if err != nil {
		t.Fatalf("NewSupervisor: %v", err)
	}
	result, err := supervisor.Run(context.Background(), SupervisorRunRequest{
		TaskID:      "44444444-4444-4444-8444-444444444444",
		Context:     runContext,
		ScratchRoot: t.TempDir(),
		ProjectName: "CRM design system",
		Prompt:      "Create the project design system",
	})
	if !errors.Is(err, ErrPreviewFailed) {
		t.Fatalf("Run error = %v, want ErrPreviewFailed", err)
	}
	if result.Status != RunStatusPreviewFailed || len(callbacks.previews) != 1 || callbacks.previews[0].Verification.Passed || len(callbacks.terminals) != 0 {
		t.Fatalf("result = %+v, callbacks = %+v", result, callbacks)
	}
	if strings.Join(callbacks.lifecycle, ",") != "archive,result,audit,preview" {
		t.Fatalf("callback lifecycle = %#v", callbacks.lifecycle)
	}
}

func TestSupervisorFinalizesRunWhenResultCallbackFails(t *testing.T) {
	t.Parallel()

	runContext := TaskRunContext{
		Schema: RunSchema,
		RunID:  "22222222-2222-4222-8222-222222222222",
		Engine: PinnedEngineIdentity(),
		Agent:  AgentIdentity{MulticaAgentID: "33333333-3333-4333-8333-333333333333", AdapterID: "opencode"},
	}
	workerRunID := "11111111-1111-4111-8111-111111111111"
	worker := &supervisorWorkerFake{
		status:        WorkerRunStatus{ID: workerRunID, Status: "succeeded", ExitCode: intPointer(0)},
		resultPackage: json.RawMessage(`{"schema":"open-design.run-result-package.v1","run":{"id":"11111111-1111-4111-8111-111111111111"}}`),
		manifest: testProjectExportManifest(map[string]testManifestFile{
			"index.html": {MIME: "text/html", Role: "entry", Body: "<main></main>"},
		}),
		archive: testProjectArchive(t, []testArchiveFile{{Path: "index.html", Body: "<main></main>"}}),
	}
	callbacks := &supervisorCallbackFake{resultErr: errors.New("server unavailable")}
	previewVerifier := &supervisorPreviewVerifierFake{verification: successfulPreviewVerification()}
	supervisor, err := NewSupervisor(SupervisorConfig{
		ArtifactRoot: "/pinned/open-design",
		Worker:       worker,
		Probe:        supervisorProbeFake{report: successfulPreflight(runContext)},
		Callbacks:    callbacks,
		Preview:      previewVerifier,
		VerifyArtifact: func(string, EngineIdentity) (ArtifactVerification, error) {
			return ArtifactVerification{}, nil
		},
	})
	if err != nil {
		t.Fatalf("NewSupervisor: %v", err)
	}
	result, err := supervisor.Run(context.Background(), SupervisorRunRequest{
		TaskID:      "44444444-4444-4444-8444-444444444444",
		Context:     runContext,
		ScratchRoot: t.TempDir(),
		ProjectName: "CRM design system",
		Prompt:      "Create the project design system",
	})
	if !errors.Is(err, ErrWorkerRunFailed) {
		t.Fatalf("Run error = %v, want ErrWorkerRunFailed", err)
	}
	if result.Status != RunStatusAgentFailed || len(callbacks.terminals) != 1 || callbacks.terminals[0].Status != RunStatusAgentFailed {
		t.Fatalf("result = %+v, terminals = %+v", result, callbacks.terminals)
	}
	if !strings.Contains(string(callbacks.terminals[0].Failure), "open_design_result_callback_failed") {
		t.Fatalf("terminal failure = %s", callbacks.terminals[0].Failure)
	}
}

func TestSupervisorFinalizesRunWhenArchiveUploadFails(t *testing.T) {
	t.Parallel()

	runContext := TaskRunContext{
		Schema: RunSchema,
		RunID:  "22222222-2222-4222-8222-222222222222",
		Engine: PinnedEngineIdentity(),
		Agent:  AgentIdentity{MulticaAgentID: "33333333-3333-4333-8333-333333333333", AdapterID: "opencode"},
	}
	workerRunID := "11111111-1111-4111-8111-111111111111"
	worker := &supervisorWorkerFake{
		status:        WorkerRunStatus{ID: workerRunID, Status: "succeeded", ExitCode: intPointer(0)},
		resultPackage: json.RawMessage(`{"schema":"open-design.run-result-package.v1","run":{"id":"11111111-1111-4111-8111-111111111111"}}`),
		manifest: testProjectExportManifest(map[string]testManifestFile{
			"index.html": {MIME: "text/html", Role: "entry", Body: "<main></main>"},
		}),
		archive: testProjectArchive(t, []testArchiveFile{{Path: "index.html", Body: "<main></main>"}}),
	}
	callbacks := &supervisorCallbackFake{archiveErr: errors.New("object storage unavailable")}
	previewVerifier := &supervisorPreviewVerifierFake{verification: successfulPreviewVerification()}
	supervisor, err := NewSupervisor(SupervisorConfig{
		ArtifactRoot: "/pinned/open-design",
		Worker:       worker,
		Probe:        supervisorProbeFake{report: successfulPreflight(runContext)},
		Callbacks:    callbacks,
		Preview:      previewVerifier,
		VerifyArtifact: func(string, EngineIdentity) (ArtifactVerification, error) {
			return ArtifactVerification{}, nil
		},
	})
	if err != nil {
		t.Fatalf("NewSupervisor: %v", err)
	}
	result, err := supervisor.Run(context.Background(), SupervisorRunRequest{
		TaskID:      "44444444-4444-4444-8444-444444444444",
		Context:     runContext,
		ScratchRoot: t.TempDir(),
		ProjectName: "CRM design system",
		Prompt:      "Create the project design system",
	})
	if !errors.Is(err, ErrWorkerRunFailed) {
		t.Fatalf("Run error = %v, want ErrWorkerRunFailed", err)
	}
	if result.Status != RunStatusAgentFailed || len(callbacks.results) != 0 || len(callbacks.terminals) != 1 {
		t.Fatalf("result = %+v, callbacks = %+v", result, callbacks)
	}
	if !strings.Contains(string(callbacks.terminals[0].Failure), "open_design_archive_upload_failed") {
		t.Fatalf("terminal failure = %s", callbacks.terminals[0].Failure)
	}
}

func TestSupervisorMapsWorkerFailureToAgentFailed(t *testing.T) {
	t.Parallel()

	runContext := TaskRunContext{
		Schema: RunSchema,
		RunID:  "22222222-2222-4222-8222-222222222222",
		Engine: PinnedEngineIdentity(),
		Agent:  AgentIdentity{MulticaAgentID: "33333333-3333-4333-8333-333333333333", AdapterID: "opencode"},
	}
	worker := &supervisorWorkerFake{
		status: WorkerRunStatus{
			ID:              "11111111-1111-4111-8111-111111111111",
			Status:          "failed",
			ErrorCode:       "AGENT_EXECUTION_FAILED",
			FailureCategory: "upstream_unavailable",
			FailureDetail:   "upstream_5xx",
		},
	}
	callbacks := &supervisorCallbackFake{}
	previewVerifier := &supervisorPreviewVerifierFake{verification: successfulPreviewVerification()}
	supervisor, err := NewSupervisor(SupervisorConfig{
		ArtifactRoot: "/pinned/open-design",
		Worker:       worker,
		Probe:        supervisorProbeFake{report: successfulPreflight(runContext)},
		Callbacks:    callbacks,
		Preview:      previewVerifier,
		VerifyArtifact: func(string, EngineIdentity) (ArtifactVerification, error) {
			return ArtifactVerification{}, nil
		},
	})
	if err != nil {
		t.Fatalf("NewSupervisor: %v", err)
	}
	result, err := supervisor.Run(context.Background(), SupervisorRunRequest{
		TaskID:      "44444444-4444-4444-8444-444444444444",
		Context:     runContext,
		ScratchRoot: t.TempDir(),
		ProjectName: "CRM design system",
		Prompt:      "Create the project design system",
	})
	if !errors.Is(err, ErrWorkerRunFailed) {
		t.Fatalf("Run error = %v, want ErrWorkerRunFailed", err)
	}
	if result.Status != RunStatusAgentFailed || len(callbacks.terminals) != 1 || callbacks.terminals[0].Status != RunStatusAgentFailed {
		t.Fatalf("result = %+v, terminals = %+v", result, callbacks.terminals)
	}
	if len(callbacks.results) != 0 {
		t.Fatalf("failed worker run reported a result package: %+v", callbacks.results)
	}
}

func TestSupervisorReconnectsFromLastPersistedEvent(t *testing.T) {
	t.Parallel()

	worker := successfulSupervisorWorker(t)
	worker.streamPlans = []supervisorStreamPlan{
		{
			events: []RunEvent{{ID: 1, Event: "start", Data: json.RawMessage(`{"status":"running"}`)}},
			err:    errors.New("connection reset by peer"),
		},
		{
			events: []RunEvent{{ID: 2, Event: "end", Data: json.RawMessage(`{"status":"succeeded"}`)}},
		},
	}
	worker.runLookups = []supervisorRunLookup{
		{status: WorkerRunStatus{ID: testSupervisorWorkerRunID, Status: "running"}},
		{status: WorkerRunStatus{ID: testSupervisorWorkerRunID, Status: "succeeded", ExitCode: intPointer(0)}},
	}
	callbacks := &supervisorCallbackFake{}

	result, err := runSuccessfulSupervisor(t, worker, callbacks)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Status != RunStatusSucceeded {
		t.Fatalf("result = %+v", result)
	}
	if len(worker.streamAfters) != 2 || worker.streamAfters[0] != 0 || worker.streamAfters[1] != 1 {
		t.Fatalf("event stream cursors = %+v, want [0 1]", worker.streamAfters)
	}
	if len(callbacks.events) != 2 || callbacks.events[0].ID != 1 || callbacks.events[1].ID != 2 {
		t.Fatalf("persisted events = %+v", callbacks.events)
	}
}

func TestSupervisorUsesTerminalStatusAfterEventStreamDisconnect(t *testing.T) {
	t.Parallel()

	worker := successfulSupervisorWorker(t)
	worker.streamPlans = []supervisorStreamPlan{{
		events: []RunEvent{{ID: 1, Event: "start", Data: json.RawMessage(`{"status":"running"}`)}},
		err:    errors.New("connection reset after worker completion"),
	}}
	callbacks := &supervisorCallbackFake{}

	result, err := runSuccessfulSupervisor(t, worker, callbacks)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Status != RunStatusSucceeded || len(worker.streamAfters) != 1 {
		t.Fatalf("result = %+v, stream cursors = %+v", result, worker.streamAfters)
	}
}

func TestSupervisorRejectsMissingEventCursor(t *testing.T) {
	t.Parallel()

	worker := successfulSupervisorWorker(t)
	worker.streamPlans = []supervisorStreamPlan{{
		events: []RunEvent{{ID: 2, Event: "end", Data: json.RawMessage(`{"status":"succeeded"}`)}},
	}}
	callbacks := &supervisorCallbackFake{}

	result, err := runSuccessfulSupervisor(t, worker, callbacks)
	if !errors.Is(err, ErrWorkerRunFailed) || result.Status != RunStatusAgentFailed {
		t.Fatalf("result = %+v, error = %v", result, err)
	}
	if got := terminalFailureCode(t, callbacks); got != "open_design_event_cursor_missing" {
		t.Fatalf("failure code = %q", got)
	}
	if len(callbacks.events) != 0 || worker.cancelCalls != 1 {
		t.Fatalf("events = %+v, cancel calls = %d", callbacks.events, worker.cancelCalls)
	}
}

func TestSupervisorClassifiesMissingWorkerRunAfterRestart(t *testing.T) {
	t.Parallel()

	missingRun := supervisorHTTPStatusError{statusCode: 404, code: "NOT_FOUND", message: "run not found"}
	worker := successfulSupervisorWorker(t)
	worker.streamPlans = []supervisorStreamPlan{{err: missingRun}}
	worker.runLookups = []supervisorRunLookup{{err: missingRun}}
	callbacks := &supervisorCallbackFake{}

	result, err := runSuccessfulSupervisor(t, worker, callbacks)
	if !errors.Is(err, ErrWorkerRunFailed) || result.Status != RunStatusAgentFailed {
		t.Fatalf("result = %+v, error = %v", result, err)
	}
	if got := terminalFailureCode(t, callbacks); got != "open_design_worker_run_missing" {
		t.Fatalf("failure code = %q", got)
	}
	if worker.startCalls != 1 {
		t.Fatalf("worker run starts = %d, want exactly one", worker.startCalls)
	}
}

const testSupervisorWorkerRunID = "11111111-1111-4111-8111-111111111111"

type supervisorHTTPStatusError struct {
	statusCode int
	code       string
	message    string
}

func (e supervisorHTTPStatusError) Error() string {
	return e.message
}

func (e supervisorHTTPStatusError) HTTPStatusCode() int {
	return e.statusCode
}

func successfulSupervisorWorker(t *testing.T) *supervisorWorkerFake {
	t.Helper()
	return &supervisorWorkerFake{
		status:        WorkerRunStatus{ID: testSupervisorWorkerRunID, Status: "succeeded", ExitCode: intPointer(0)},
		resultPackage: json.RawMessage(`{"schema":"open-design.run-result-package.v1","run":{"id":"11111111-1111-4111-8111-111111111111"}}`),
		manifest:      successfulPreviewExportManifest(),
		archive:       successfulPreviewArchive(t),
		audit:         successfulPackageAudit(),
	}
}

func runSuccessfulSupervisor(t *testing.T, worker *supervisorWorkerFake, callbacks *supervisorCallbackFake) (SupervisorRunResult, error) {
	t.Helper()
	runContext := TaskRunContext{
		Schema: RunSchema,
		RunID:  "22222222-2222-4222-8222-222222222222",
		Engine: PinnedEngineIdentity(),
		Agent:  AgentIdentity{MulticaAgentID: "33333333-3333-4333-8333-333333333333", AdapterID: "opencode"},
	}
	supervisor, err := NewSupervisor(SupervisorConfig{
		ArtifactRoot:             "/pinned/open-design",
		Worker:                   worker,
		Probe:                    supervisorProbeFake{report: successfulPreflight(runContext)},
		Callbacks:                callbacks,
		Preview:                  &supervisorPreviewVerifierFake{verification: successfulPreviewVerification()},
		EventStreamRetrySchedule: []time.Duration{0, 0},
		VerifyArtifact: func(string, EngineIdentity) (ArtifactVerification, error) {
			return ArtifactVerification{}, nil
		},
	})
	if err != nil {
		t.Fatalf("NewSupervisor: %v", err)
	}
	return supervisor.Run(context.Background(), SupervisorRunRequest{
		TaskID:      "44444444-4444-4444-8444-444444444444",
		Context:     runContext,
		ScratchRoot: t.TempDir(),
		ProjectName: "CRM design system",
		Prompt:      "Create the project design system",
	})
}

func terminalFailureCode(t *testing.T, callbacks *supervisorCallbackFake) string {
	t.Helper()
	if len(callbacks.terminals) != 1 {
		t.Fatalf("terminal callbacks = %+v", callbacks.terminals)
	}
	var failure struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(callbacks.terminals[0].Failure, &failure); err != nil {
		t.Fatalf("decode terminal failure: %v", err)
	}
	return failure.Code
}

func intPointer(value int) *int {
	return &value
}

func successfulPackageAudit() PackageAudit {
	return PackageAudit{
		OK:             true,
		FilesInspected: 1,
		Errors:         []PackageAuditIssue{},
		Warnings:       []PackageAuditIssue{},
	}
}

func successfulPreviewArchive(t *testing.T) []byte {
	t.Helper()
	return testProjectArchive(t, []testArchiveFile{
		{Path: "DESIGN.md", Body: "# CRM"},
		{Path: previewManifestPath, Body: `{"version":1,"previews":[{"id":"colors","path":"colors.html"}]}`},
		{Path: "preview/colors.html", Body: "<main>Colors</main>"},
		{Path: previewUIKitPath, Body: "<main>UI Kit</main>"},
	})
}

func successfulPreviewExportManifest() json.RawMessage {
	return testProjectExportManifest(map[string]testManifestFile{
		"DESIGN.md":           {MIME: "text/markdown", Role: "source", Body: "# CRM"},
		previewManifestPath:   {MIME: "application/json", Role: "supporting", Body: `{"version":1,"previews":[{"id":"colors","path":"colors.html"}]}`},
		"preview/colors.html": {MIME: "text/html", Role: "artifact", Body: "<main>Colors</main>"},
		previewUIKitPath:      {MIME: "text/html", Role: "artifact", Body: "<main>UI Kit</main>"},
	})
}
