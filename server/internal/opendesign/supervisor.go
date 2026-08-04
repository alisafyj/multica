package opendesign

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrPreflightFailed    = errors.New("Open Design preflight failed")
	ErrWorkerRunFailed    = errors.New("Open Design worker run failed")
	ErrWorkerCanceled     = errors.New("Open Design worker run canceled")
	ErrPackageAuditFailed = errors.New("Open Design package audit failed")
	ErrPreviewFailed      = errors.New("Open Design Preview verification failed")
	errEventCursorMissing = errors.New("Open Design worker event cursor is missing")
)

var defaultEventStreamRetrySchedule = []time.Duration{
	250 * time.Millisecond,
	1 * time.Second,
	2 * time.Second,
}

type WorkerAPI interface {
	PrepareWorkspace(context.Context, WorkerWorkspaceRequest) (WorkerWorkspace, error)
	StartRun(context.Context, WorkerStartRunRequest) (string, error)
	StreamRunEvents(context.Context, string, int64, func(RunEvent) error) error
	GetRun(context.Context, string) (WorkerRunStatus, error)
	GetResultPackage(context.Context, string) (json.RawMessage, error)
	GetProjectExportManifest(context.Context, string) (json.RawMessage, error)
	GetProjectArchive(context.Context, string) ([]byte, error)
	GetProjectPackageAudit(context.Context, string) (PackageAudit, error)
	GetProjectPreviewURL(context.Context, string, PreviewTarget) (PreviewURL, error)
	CancelRun(context.Context, string) (WorkerRunStatus, error)
}

type PreflightProber interface {
	Probe(context.Context, ExpectedPreflight, string) (PreflightReport, error)
}

type LifecycleCallbacks interface {
	ReportOpenDesignPreflight(context.Context, string, PreflightReport) error
	StartOpenDesignRun(context.Context, string, string) error
	ReportOpenDesignRunEvent(context.Context, string, string, RunEvent) error
	UploadOpenDesignRunArchive(context.Context, string, string, string, []byte) (string, error)
	ReportOpenDesignRunResult(context.Context, string, string, CollectedRunResult) error
	ReportOpenDesignRunAudit(context.Context, string, string, PackageAuditReceipt) error
	ReportOpenDesignRunPreview(context.Context, string, string, PreviewVerificationReceipt) error
	FinalizeOpenDesignRun(context.Context, string, string, RunStatus, json.RawMessage) error
}

type ArtifactVerifier func(string, EngineIdentity) (ArtifactVerification, error)

type SupervisorConfig struct {
	ArtifactRoot   string
	Worker         WorkerAPI
	Probe          PreflightProber
	Callbacks      LifecycleCallbacks
	Preview        PreviewVerifier
	VerifyArtifact ArtifactVerifier
	// EventStreamRetrySchedule is nil for the bounded production default. An
	// explicit empty slice disables reconnects.
	EventStreamRetrySchedule []time.Duration
}

type Supervisor struct {
	artifactRoot             string
	worker                   WorkerAPI
	probe                    PreflightProber
	callbacks                LifecycleCallbacks
	preview                  PreviewVerifier
	verifyArtifact           ArtifactVerifier
	eventStreamRetrySchedule []time.Duration
}

type SupervisorRunRequest struct {
	TaskID      string
	Context     TaskRunContext
	ScratchRoot string
	ProjectName string
	Prompt      string
	Provenance  WorkerWorkspaceProvenance
}

type SupervisorRunResult struct {
	Status      RunStatus
	WorkerRunID string
}

func NewSupervisor(config SupervisorConfig) (*Supervisor, error) {
	if strings.TrimSpace(config.ArtifactRoot) == "" {
		return nil, errors.New("Open Design worker artifact root is required")
	}
	if config.Worker == nil || config.Probe == nil || config.Callbacks == nil || config.Preview == nil {
		return nil, errors.New("Open Design supervisor dependencies are required")
	}
	retrySchedule := config.EventStreamRetrySchedule
	if retrySchedule == nil {
		retrySchedule = defaultEventStreamRetrySchedule
	}
	for _, delay := range retrySchedule {
		if delay < 0 {
			return nil, errors.New("Open Design event stream retry delays cannot be negative")
		}
	}
	verifyArtifact := config.VerifyArtifact
	if verifyArtifact == nil {
		verifyArtifact = VerifyWorkerArtifact
	}
	return &Supervisor{
		artifactRoot:             strings.TrimSpace(config.ArtifactRoot),
		worker:                   config.Worker,
		probe:                    config.Probe,
		callbacks:                config.Callbacks,
		preview:                  config.Preview,
		verifyArtifact:           verifyArtifact,
		eventStreamRetrySchedule: append([]time.Duration(nil), retrySchedule...),
	}, nil
}

func (s *Supervisor) Run(ctx context.Context, request SupervisorRunRequest) (SupervisorRunResult, error) {
	if err := validateSupervisorRunRequest(request); err != nil {
		return SupervisorRunResult{}, err
	}
	expected := ExpectedPreflight{
		Engine:    request.Context.Engine,
		AdapterID: request.Context.Agent.AdapterID,
		Model:     request.Context.Agent.Model,
	}
	if _, err := s.verifyArtifact(s.artifactRoot, expected.Engine); err != nil {
		report := failedPreflightReport(expected, "artifact verification failed: "+err.Error())
		if callbackErr := s.callbacks.ReportOpenDesignPreflight(ctx, request.TaskID, report); callbackErr != nil {
			return SupervisorRunResult{Status: RunStatusPreflightFailed}, errors.Join(err, callbackErr)
		}
		return SupervisorRunResult{Status: RunStatusPreflightFailed}, fmt.Errorf("%w: %v", ErrPreflightFailed, err)
	}
	report, probeErr := s.probe.Probe(ctx, expected, PluginsDisabled)
	if probeErr != nil {
		report = failedPreflightReport(expected, "worker probe failed: "+probeErr.Error())
	}
	if err := s.callbacks.ReportOpenDesignPreflight(ctx, request.TaskID, report); err != nil {
		return SupervisorRunResult{Status: RunStatusPreflightFailed}, err
	}
	if probeErr != nil {
		return SupervisorRunResult{Status: RunStatusPreflightFailed}, fmt.Errorf("%w: %v", ErrPreflightFailed, probeErr)
	}
	if err := ValidatePreflight(expected, report); err != nil {
		return SupervisorRunResult{Status: RunStatusPreflightFailed}, fmt.Errorf("%w: %v", ErrPreflightFailed, err)
	}

	workspace, err := s.worker.PrepareWorkspace(ctx, WorkerWorkspaceRequest{
		ScratchRoot: request.ScratchRoot,
		Name:        request.ProjectName,
		Provenance:  request.Provenance,
	})
	if err != nil {
		return s.failBeforeWorkerRun(ctx, request.TaskID, "open_design_workspace_prepare_failed", err)
	}
	workerRunID, err := s.worker.StartRun(ctx, WorkerStartRunRequest{
		Workspace: workspace,
		Agent:     request.Context.Agent,
		Prompt:    request.Prompt,
	})
	if err != nil {
		return s.failBeforeWorkerRun(ctx, request.TaskID, "open_design_worker_start_failed", err)
	}
	result := SupervisorRunResult{Status: RunStatusRunning, WorkerRunID: workerRunID}
	if err := s.callbacks.StartOpenDesignRun(ctx, request.TaskID, workerRunID); err != nil {
		s.cancelWorkerBestEffort(ctx, workerRunID)
		failure := openDesignFailure("open_design_start_callback_failed", err)
		_ = s.callbacks.FinalizeOpenDesignRun(context.WithoutCancel(ctx), request.TaskID, "", RunStatusAgentFailed, failure)
		result.Status = RunStatusAgentFailed
		return result, err
	}

	status, failureCode, streamErr := s.monitorWorkerRun(ctx, request.TaskID, workerRunID)
	if ctx.Err() != nil {
		return s.cancel(ctx, request.TaskID, workerRunID)
	}
	if streamErr != nil {
		s.cancelWorkerBestEffort(ctx, workerRunID)
		failure := openDesignFailure(failureCode, streamErr)
		if callbackErr := s.callbacks.FinalizeOpenDesignRun(context.WithoutCancel(ctx), request.TaskID, workerRunID, RunStatusAgentFailed, failure); callbackErr != nil {
			streamErr = errors.Join(streamErr, callbackErr)
		}
		result.Status = RunStatusAgentFailed
		return result, fmt.Errorf("%w: %v", ErrWorkerRunFailed, streamErr)
	}
	switch status.Status {
	case "succeeded":
		resultPackage, err := s.worker.GetResultPackage(ctx, workerRunID)
		if err != nil {
			return s.failActiveWorkerRun(ctx, request.TaskID, workerRunID, "open_design_result_package_failed", err)
		}
		if err := validateWorkerResultPackage(resultPackage, workerRunID); err != nil {
			return s.failActiveWorkerRun(ctx, request.TaskID, workerRunID, "open_design_result_package_invalid", err)
		}
		manifest, err := s.worker.GetProjectExportManifest(ctx, workspace.ProjectID)
		if err != nil {
			return s.failActiveWorkerRun(ctx, request.TaskID, workerRunID, "open_design_export_manifest_failed", err)
		}
		archive, err := s.worker.GetProjectArchive(ctx, workspace.ProjectID)
		if err != nil {
			return s.failActiveWorkerRun(ctx, request.TaskID, workerRunID, "open_design_archive_failed", err)
		}
		collected, err := CollectWorkerRunResult(resultPackage, manifest, archive, workerRunID, workspace.ProjectID)
		if err != nil {
			return s.failActiveWorkerRun(ctx, request.TaskID, workerRunID, "open_design_result_collection_failed", err)
		}
		archiveObjectKey, err := s.callbacks.UploadOpenDesignRunArchive(ctx, request.TaskID, workerRunID, collected.ContentDigest, archive)
		if err != nil {
			return s.failActiveWorkerRun(ctx, request.TaskID, workerRunID, "open_design_archive_upload_failed", err)
		}
		collected.ArchiveObjectKey = strings.TrimSpace(archiveObjectKey)
		if collected.ArchiveObjectKey == "" {
			return s.failActiveWorkerRun(ctx, request.TaskID, workerRunID, "open_design_archive_upload_failed", errors.New("archive upload returned an empty object key"))
		}
		if err := s.callbacks.ReportOpenDesignRunResult(ctx, request.TaskID, workerRunID, collected); err != nil {
			return s.failActiveWorkerRun(ctx, request.TaskID, workerRunID, "open_design_result_callback_failed", err)
		}
		audit, err := s.worker.GetProjectPackageAudit(ctx, workspace.ProjectID)
		if err != nil {
			return s.failPackageAudit(ctx, request.TaskID, workerRunID, "open_design_package_audit_request_failed", err)
		}
		auditReceipt, err := NewPackageAuditReceipt(request.Context.Engine, collected.ContentDigest, audit)
		if err != nil {
			return s.failPackageAudit(ctx, request.TaskID, workerRunID, "open_design_package_audit_invalid", err)
		}
		if err := s.callbacks.ReportOpenDesignRunAudit(ctx, request.TaskID, workerRunID, auditReceipt); err != nil {
			return s.failPackageAudit(ctx, request.TaskID, workerRunID, "open_design_package_audit_callback_failed", err)
		}
		if !audit.OK {
			result.Status = RunStatusAuditFailed
			return result, fmt.Errorf("%w: upstream audit returned %d error(s) and %d warning(s)", ErrPackageAuditFailed, len(audit.Errors), len(audit.Warnings))
		}
		previewTargets, err := DiscoverPreviewTargets(archive)
		if err != nil {
			return s.failPreview(ctx, request.TaskID, workerRunID, "open_design_preview_targets_invalid", err)
		}
		previewURLs := make([]PreviewURL, 0, len(previewTargets))
		for _, target := range previewTargets {
			previewURL, err := s.worker.GetProjectPreviewURL(ctx, workspace.ProjectID, target)
			if err != nil {
				return s.failPreview(ctx, request.TaskID, workerRunID, "open_design_preview_url_failed", err)
			}
			previewURLs = append(previewURLs, previewURL)
		}
		verification, err := s.preview.Verify(ctx, previewURLs)
		if err != nil {
			return s.failPreview(ctx, request.TaskID, workerRunID, "open_design_preview_verification_failed", err)
		}
		if err := ValidatePreviewVerificationTargetSet(verification, previewTargets); err != nil {
			return s.failPreview(ctx, request.TaskID, workerRunID, "open_design_preview_verification_invalid", err)
		}
		previewReceipt, err := NewPreviewVerificationReceipt(request.Context.Engine, collected.ContentDigest, verification)
		if err != nil {
			return s.failPreview(ctx, request.TaskID, workerRunID, "open_design_preview_verification_invalid", err)
		}
		if err := s.callbacks.ReportOpenDesignRunPreview(ctx, request.TaskID, workerRunID, previewReceipt); err != nil {
			return s.failPreview(ctx, request.TaskID, workerRunID, "open_design_preview_callback_failed", err)
		}
		if !verification.Passed {
			result.Status = RunStatusPreviewFailed
			return result, fmt.Errorf("%w: one or more declared surfaces did not render", ErrPreviewFailed)
		}
		result.Status = RunStatusSucceeded
		return result, nil
	case "canceled", "cancelled":
		failure := workerFailure(status, "open_design_worker_canceled")
		if err := s.callbacks.FinalizeOpenDesignRun(ctx, request.TaskID, workerRunID, RunStatusCanceled, failure); err != nil {
			return result, err
		}
		result.Status = RunStatusCanceled
		return result, ErrWorkerCanceled
	case "failed":
		failure := workerFailure(status, "open_design_agent_failed")
		if err := s.callbacks.FinalizeOpenDesignRun(ctx, request.TaskID, workerRunID, RunStatusAgentFailed, failure); err != nil {
			return result, err
		}
		result.Status = RunStatusAgentFailed
		return result, ErrWorkerRunFailed
	default:
		err := fmt.Errorf("worker event stream ended with non-terminal status %q", status.Status)
		return s.failActiveWorkerRun(ctx, request.TaskID, workerRunID, "open_design_worker_non_terminal", err)
	}
}

func (s *Supervisor) monitorWorkerRun(ctx context.Context, taskID, workerRunID string) (WorkerRunStatus, string, error) {
	var lastEvent RunEvent
	hasLastEvent := false
	lastEventID := int64(0)

	for attempt := 0; ; attempt++ {
		streamErr := s.worker.StreamRunEvents(ctx, workerRunID, lastEventID, func(event RunEvent) error {
			if event.ID == lastEventID && hasLastEvent {
				if !sameRunEvent(lastEvent, event) {
					return fmt.Errorf("%w: event %d changed while reconnecting", errEventCursorMissing, event.ID)
				}
				return nil
			}
			if event.ID != lastEventID+1 {
				return fmt.Errorf("%w: requested after %d but received event %d", errEventCursorMissing, lastEventID, event.ID)
			}
			if err := s.callbacks.ReportOpenDesignRunEvent(ctx, taskID, workerRunID, event); err != nil {
				return err
			}
			lastEvent = RunEvent{ID: event.ID, Event: event.Event, Data: append(json.RawMessage(nil), event.Data...)}
			hasLastEvent = true
			lastEventID = event.ID
			return nil
		})
		if ctx.Err() != nil {
			return WorkerRunStatus{}, "", ctx.Err()
		}
		if errors.Is(streamErr, errEventCursorMissing) {
			return WorkerRunStatus{}, "open_design_event_cursor_missing", streamErr
		}

		status, statusErr := s.worker.GetRun(ctx, workerRunID)
		if statusErr == nil && isTerminalWorkerRunStatus(status.Status) {
			return status, "", nil
		}
		if hasHTTPStatus(streamErr, http.StatusNotFound) || hasHTTPStatus(statusErr, http.StatusNotFound) {
			return WorkerRunStatus{}, "open_design_worker_run_missing", errors.Join(streamErr, statusErr)
		}
		if attempt >= len(s.eventStreamRetrySchedule) {
			cause := errors.Join(streamErr, statusErr)
			if cause == nil {
				cause = fmt.Errorf("worker event stream ended with non-terminal status %q", status.Status)
			}
			return WorkerRunStatus{}, "open_design_event_stream_unavailable", cause
		}
		if err := waitForEventStreamRetry(ctx, s.eventStreamRetrySchedule[attempt]); err != nil {
			return WorkerRunStatus{}, "", err
		}
	}
}

func isTerminalWorkerRunStatus(status string) bool {
	switch status {
	case "succeeded", "failed", "canceled", "cancelled":
		return true
	default:
		return false
	}
}

func sameRunEvent(left, right RunEvent) bool {
	if left.ID != right.ID || left.Event != right.Event {
		return false
	}
	var leftData any
	var rightData any
	return json.Unmarshal(left.Data, &leftData) == nil &&
		json.Unmarshal(right.Data, &rightData) == nil &&
		reflect.DeepEqual(leftData, rightData)
}

func hasHTTPStatus(err error, statusCode int) bool {
	if err == nil {
		return false
	}
	var statusErr interface {
		HTTPStatusCode() int
	}
	return errors.As(err, &statusErr) && statusErr.HTTPStatusCode() == statusCode
}

func waitForEventStreamRetry(ctx context.Context, delay time.Duration) error {
	if delay == 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func validateSupervisorRunRequest(request SupervisorRunRequest) error {
	if _, err := uuid.Parse(request.TaskID); err != nil {
		return errors.New("Open Design supervisor task_id must be a UUID")
	}
	if request.Context.Schema != RunSchema {
		return fmt.Errorf("Open Design run schema %q does not match %q", request.Context.Schema, RunSchema)
	}
	if parsed, err := uuid.Parse(request.Context.RunID); err != nil || parsed.String() != request.Context.RunID {
		return errors.New("Open Design context run_id must be a canonical UUID")
	}
	if err := request.Context.Engine.Validate(); err != nil {
		return err
	}
	if request.Context.Engine != PinnedEngineIdentity() {
		return errors.New("Open Design task does not use the pinned engine identity")
	}
	if parsed, err := uuid.Parse(request.Context.Agent.MulticaAgentID); err != nil || parsed.String() != request.Context.Agent.MulticaAgentID {
		return errors.New("Open Design context multica_agent_id must be a canonical UUID")
	}
	if strings.TrimSpace(request.Context.Agent.AdapterID) == "" {
		return errors.New("Open Design context adapter_id is required")
	}
	if strings.TrimSpace(request.ScratchRoot) == "" || strings.TrimSpace(request.ProjectName) == "" || strings.TrimSpace(request.Prompt) == "" {
		return errors.New("Open Design scratch root, project name, and prompt are required")
	}
	return nil
}

func failedPreflightReport(expected ExpectedPreflight, message string) PreflightReport {
	return PreflightReport{
		Schema:     PreflightSchema,
		Engine:     expected.Engine,
		AdapterID:  expected.AdapterID,
		Model:      expected.Model,
		Binary:     ProbeResult{Status: ProbeFailed, Message: truncateFailureMessage(message)},
		Auth:       ProbeResult{Status: ProbeUnknown},
		ModelProbe: ProbeResult{Status: ProbeFailed},
		Plugins:    PluginPreflight{Policy: PluginsDisabled},
	}
}

func (s *Supervisor) failBeforeWorkerRun(ctx context.Context, taskID, code string, cause error) (SupervisorRunResult, error) {
	failure := openDesignFailure(code, cause)
	callbackErr := s.callbacks.FinalizeOpenDesignRun(context.WithoutCancel(ctx), taskID, "", RunStatusAgentFailed, failure)
	if callbackErr != nil {
		cause = errors.Join(cause, callbackErr)
	}
	return SupervisorRunResult{Status: RunStatusAgentFailed}, fmt.Errorf("%w: %v", ErrWorkerRunFailed, cause)
}

func (s *Supervisor) failActiveWorkerRun(ctx context.Context, taskID, workerRunID, code string, cause error) (SupervisorRunResult, error) {
	failure := openDesignFailure(code, cause)
	callbackErr := s.callbacks.FinalizeOpenDesignRun(context.WithoutCancel(ctx), taskID, workerRunID, RunStatusAgentFailed, failure)
	if callbackErr != nil {
		cause = errors.Join(cause, callbackErr)
	}
	return SupervisorRunResult{Status: RunStatusAgentFailed, WorkerRunID: workerRunID}, fmt.Errorf("%w: %v", ErrWorkerRunFailed, cause)
}

func (s *Supervisor) failPackageAudit(ctx context.Context, taskID, workerRunID, code string, cause error) (SupervisorRunResult, error) {
	failure := openDesignFailure(code, cause)
	callbackErr := s.callbacks.FinalizeOpenDesignRun(context.WithoutCancel(ctx), taskID, workerRunID, RunStatusAuditFailed, failure)
	if callbackErr != nil {
		cause = errors.Join(cause, callbackErr)
	}
	return SupervisorRunResult{Status: RunStatusAuditFailed, WorkerRunID: workerRunID}, fmt.Errorf("%w: %v", ErrPackageAuditFailed, cause)
}

func (s *Supervisor) failPreview(ctx context.Context, taskID, workerRunID, code string, cause error) (SupervisorRunResult, error) {
	failure := openDesignFailure(code, cause)
	callbackErr := s.callbacks.FinalizeOpenDesignRun(context.WithoutCancel(ctx), taskID, workerRunID, RunStatusPreviewFailed, failure)
	if callbackErr != nil {
		cause = errors.Join(cause, callbackErr)
	}
	return SupervisorRunResult{Status: RunStatusPreviewFailed, WorkerRunID: workerRunID}, fmt.Errorf("%w: %v", ErrPreviewFailed, cause)
}

func (s *Supervisor) cancel(ctx context.Context, taskID, workerRunID string) (SupervisorRunResult, error) {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
	defer cancel()
	status, cancelErr := s.worker.CancelRun(cleanupCtx, workerRunID)
	failure := workerFailure(status, "open_design_worker_canceled")
	if cancelErr != nil {
		failure = openDesignFailure("open_design_cancel_failed", cancelErr)
	}
	callbackErr := s.callbacks.FinalizeOpenDesignRun(cleanupCtx, taskID, workerRunID, RunStatusCanceled, failure)
	if cancelErr != nil || callbackErr != nil {
		return SupervisorRunResult{Status: RunStatusCanceled, WorkerRunID: workerRunID}, errors.Join(ErrWorkerCanceled, cancelErr, callbackErr)
	}
	return SupervisorRunResult{Status: RunStatusCanceled, WorkerRunID: workerRunID}, ErrWorkerCanceled
}

func (s *Supervisor) cancelWorkerBestEffort(ctx context.Context, workerRunID string) {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
	defer cancel()
	_, _ = s.worker.CancelRun(cleanupCtx, workerRunID)
}

func validateWorkerResultPackage(raw json.RawMessage, workerRunID string) error {
	var result struct {
		Schema string `json:"schema"`
		Run    struct {
			ID string `json:"id"`
		} `json:"run"`
	}
	if len(raw) == 0 || json.Unmarshal(raw, &result) != nil {
		return errors.New("Open Design result package is invalid JSON")
	}
	if result.Schema != RunResultPackageSchema {
		return fmt.Errorf("Open Design result package schema %q does not match %q", result.Schema, RunResultPackageSchema)
	}
	if result.Run.ID != workerRunID {
		return errors.New("Open Design result package run id does not match the active worker run")
	}
	return nil
}

func workerFailure(status WorkerRunStatus, fallbackCode string) json.RawMessage {
	code := strings.TrimSpace(status.ErrorCode)
	if code == "" {
		code = fallbackCode
	}
	parts := make([]string, 0, 3)
	for _, value := range []string{status.FailureCategory, status.FailureDetail, status.Error} {
		if value = strings.TrimSpace(value); value != "" {
			parts = append(parts, value)
		}
	}
	message := strings.Join(parts, ": ")
	if message == "" {
		message = status.Status
	}
	return marshalFailure(code, message)
}

func openDesignFailure(code string, cause error) json.RawMessage {
	message := ""
	if cause != nil {
		message = cause.Error()
	}
	return marshalFailure(code, message)
}

func marshalFailure(code, message string) json.RawMessage {
	payload, _ := json.Marshal(map[string]string{
		"code":    strings.TrimSpace(code),
		"message": truncateFailureMessage(message),
	})
	return payload
}

func truncateFailureMessage(message string) string {
	message = strings.TrimSpace(message)
	if len(message) <= 4<<10 {
		return message
	}
	return message[:4<<10]
}
