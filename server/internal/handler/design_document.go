package handler

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/multica-ai/multica/server/pkg/dbid"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/multica-ai/multica/server/internal/designdocument"
	"github.com/multica-ai/multica/server/internal/designsystemcatalogue"
	"github.com/multica-ai/multica/server/internal/projectdesignsystem"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// The design centre home composer creates page-design tasks (DC-042 / DC-049).
// Project and agent are required; repository and issue are deliberately not
// (DC-053, DC-045) — a user who just wants a page from a description should
// not be blocked on attaching a repo, and linking an issue must never imply
// the task will move it.

const (
	designDocumentMaxBriefBytes    = 16 << 10
	designDocumentMaxTitleBytes    = 200
	designDocumentMaxSnapshotBytes = 256 << 10
	designDocumentDefaultRecipe    = "default"
	// The package protocol this slice produces (P-011).
	designDocumentPackageSchema = "multica.design-document/v1"
)

// designDocumentRecipes are the scenario chips the first home composer ships
// (DC-049). Every one of them produces a prototype; they differ in the recipe
// the agent follows, not in the artifact kind. The template slice widens this
// to template ids without changing the API shape.
var designDocumentRecipes = map[string]struct{}{
	"default":         {},
	"ui-mockup":       {},
	"web-clone":       {},
	"wireframe":       {},
	"mobile-app":      {},
	"figma-migration": {},
}

type CreateDesignDocumentRequest struct {
	ProjectID string `json:"project_id"`
	AgentID   string `json:"agent_id"`
	// Optional. Naming a repository grounds the task against it; omitting it
	// means no grounding at all, and the document says so.
	ProjectResourceID string `json:"project_resource_id"`
	// CreateIssue opens a companion task card for this run when no issue was
	// named. Traceable link only, exactly like IssueID: the design task never
	// moves the issue it created (DC-045).
	CreateIssue bool `json:"create_issue"`
	// Optional traceable link only.
	IssueID string `json:"issue_id"`
	// Optional explicit design system for this run (DC-060). A workspace
	// system id, or the slug of a bundled catalogue system — never both.
	// Unset keeps the repository -> project fallback (DC-053).
	DesignSystemID      string          `json:"design_system_id"`
	BuiltinDesignSystem string          `json:"builtin_design_system"`
	Title               string          `json:"title"`
	Platform            string          `json:"platform"`
	Recipe              string          `json:"recipe"`
	Brief               string          `json:"brief"`
	Attachments         json.RawMessage `json:"attachments"`
}

type DesignDocumentResponse struct {
	ID                string                           `json:"id"`
	WorkspaceID       string                           `json:"workspace_id"`
	ProjectID         string                           `json:"project_id"`
	ProjectResourceID string                           `json:"project_resource_id,omitempty"`
	IssueID           string                           `json:"issue_id,omitempty"`
	Title             string                           `json:"title"`
	Platform          string                           `json:"platform"`
	Recipe            string                           `json:"recipe"`
	Status            string                           `json:"status"`
	DraftRevisionID   string                           `json:"draft_revision_id,omitempty"`
	SavedRevisionID   string                           `json:"saved_revision_id,omitempty"`
	ActiveTask        *ProjectDesignSystemTaskResponse `json:"active_task"`
	InputSnapshot     json.RawMessage                  `json:"input_snapshot"`
	LastError         json.RawMessage                  `json:"last_error"`
	// Whether this run had repository evidence. The UI must not let a user
	// assume the agent read code when it did not.
	RepositoryGrounded bool   `json:"repository_grounded"`
	CreatedAt          string `json:"created_at"`
	UpdatedAt          string `json:"updated_at"`
	SavedAt            string `json:"saved_at,omitempty"`
}

// designDocumentInputSnapshot is the canonical request record a revision's
// input_snapshot_sha256 is computed over. Only fields the user actually chose
// belong here — server-side ids and timestamps would make the digest differ
// between two identical requests.
type designDocumentInputSnapshot struct {
	AgentID           string `json:"agent_id"`
	ProjectResourceID string `json:"project_resource_id,omitempty"`
	IssueID           string `json:"issue_id,omitempty"`
	// The design system the user named for this run, frozen with the rest of
	// the inputs so a regeneration reruns under the same choice (DC-060).
	DesignSystemID      string          `json:"design_system_id,omitempty"`
	BuiltinDesignSystem string          `json:"builtin_design_system,omitempty"`
	Platform            string          `json:"platform"`
	Recipe              string          `json:"recipe"`
	Brief               string          `json:"brief"`
	Attachments         json.RawMessage `json:"attachments,omitempty"`
}

func (h *Handler) CreateDesignDocument(w http.ResponseWriter, r *http.Request) {
	var req CreateDesignDocumentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProjectDesignSystemError(w, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}
	req.ProjectID = strings.TrimSpace(req.ProjectID)
	req.AgentID = strings.TrimSpace(req.AgentID)
	req.ProjectResourceID = strings.TrimSpace(req.ProjectResourceID)
	req.IssueID = strings.TrimSpace(req.IssueID)
	req.DesignSystemID = strings.TrimSpace(req.DesignSystemID)
	req.BuiltinDesignSystem = strings.TrimSpace(req.BuiltinDesignSystem)
	req.Title = strings.TrimSpace(req.Title)
	req.Platform = strings.TrimSpace(req.Platform)
	req.Recipe = strings.TrimSpace(req.Recipe)
	req.Brief = strings.TrimSpace(req.Brief)

	if req.ProjectID == "" {
		writeProjectDesignSystemError(w, http.StatusBadRequest, "project_id_required", "project_id is required")
		return
	}
	if req.AgentID == "" {
		writeProjectDesignSystemError(w, http.StatusBadRequest, "agent_id_required", "agent_id is required")
		return
	}
	if req.Brief == "" {
		writeProjectDesignSystemError(w, http.StatusBadRequest, "brief_required", "brief is required")
		return
	}
	if len(req.Brief) > designDocumentMaxBriefBytes {
		writeProjectDesignSystemError(w, http.StatusRequestEntityTooLarge, "brief_too_large", "brief exceeds the size limit")
		return
	}
	if len(req.Title) > designDocumentMaxTitleBytes {
		writeProjectDesignSystemError(w, http.StatusBadRequest, "title_too_large", "title exceeds the size limit")
		return
	}
	if req.Platform == "" {
		writeProjectDesignSystemError(w, http.StatusBadRequest, "platform_required", "platform is required")
		return
	}
	if !validProjectDesignSystemPlatform(req.Platform) {
		writeProjectDesignSystemError(w, http.StatusBadRequest, "platform_invalid", "platform must be web, mobile, or cross_platform")
		return
	}
	// One design system, or none. Accepting both would leave the agent to
	// guess which visual language actually governs the run.
	if req.DesignSystemID != "" && req.BuiltinDesignSystem != "" {
		writeProjectDesignSystemError(w, http.StatusBadRequest, "design_system_ambiguous", "choose either a workspace design system or a built-in one, not both")
		return
	}
	workspaceUUID, requesterUUID, ok := h.projectDesignSystemRequestScope(w, r)
	if !ok {
		return
	}
	// A recipe is either one of the composer's built-in chips or a published
	// catalogue entry this workspace can see. Resolving it here means a
	// document can never record a recipe that does not exist.
	resolvedRecipe, ok := h.resolveDesignDocumentRecipe(r, w, workspaceUUID, req.Recipe)
	if !ok {
		return
	}
	req.Recipe = resolvedRecipe
	projectUUID, ok := parseUUIDOrBadRequest(w, req.ProjectID, "project_id")
	if !ok {
		return
	}
	project, err := h.Queries.GetProjectInWorkspace(r.Context(), db.GetProjectInWorkspaceParams{
		ID: projectUUID, WorkspaceID: workspaceUUID,
	})
	if err != nil {
		writeProjectDesignSystemError(w, http.StatusNotFound, "project_not_found", "project not found")
		return
	}

	// Repository is optional; when present it must be a github_repo under
	// this project, same check the design system scope uses.
	scope, ok := h.projectDesignSystemScopeFromBody(r.Context(), w, workspaceUUID, projectUUID, req.ProjectResourceID)
	if !ok {
		return
	}

	issueUUID, ok := h.resolveOptionalDesignDocumentIssue(r, w, workspaceUUID, projectUUID, req.IssueID)
	if !ok {
		return
	}

	agentUUID, ok := parseUUIDOrBadRequest(w, req.AgentID, "agent_id")
	if !ok {
		return
	}

	title := req.Title
	if title == "" {
		title = project.Title
	}

	// The launcher can open a task card next to the design run. The issue is a
	// traceable companion, never a driver: nothing in the design task moves it
	// (DC-045). It is created before the document so the document row can carry
	// its id, and deleted again if the document fails — this repo resolves
	// dependent cleanup in application code, not with cascades.
	createdIssueID := pgtype.UUID{}
	if req.CreateIssue && !issueUUID.Valid {
		created, issueErr := h.IssueService.Create(r.Context(), service.IssueCreateParams{
			WorkspaceID:  workspaceUUID,
			Title:        title,
			Description:  pgtype.Text{String: req.Brief, Valid: strings.TrimSpace(req.Brief) != ""},
			Status:       "todo",
			Priority:     "none",
			AssigneeType: pgtype.Text{String: "agent", Valid: true},
			AssigneeID:   agentUUID,
			CreatorType:  "member",
			CreatorID:    requesterUUID,
			ProjectID:    projectUUID,
		}, service.IssueCreateOpts{
			ActorID: uuidToString(requesterUUID),
			// The agent assignee is a readable trace of who is doing the work,
			// not a dispatch instruction: creating the issue must not ALSO start
			// an independent agent run that would race the design task for the
			// same local directory (that run is what actually wedged the design
			// task behind an unrelated one before this flag existed).
			SuppressAssigneeRun: true,
		})
		if issueErr != nil {
			writeProjectDesignSystemError(w, http.StatusInternalServerError, "issue_create_failed", "failed to create the companion task")
			return
		}
		issueUUID = created.Issue.ID
		createdIssueID = created.Issue.ID
	}

	// Reference attachments are resolved and pinned here, once: the frozen
	// input records what they are and the exact bytes they were, so the run
	// (and any later adjustment carrying them forward) cannot see a different
	// file under the same id.
	attachments, attachmentErr := h.resolveDesignDocumentAttachments(r.Context(), workspaceUUID, req.Attachments)
	if attachmentErr != nil {
		writeProjectDesignSystemRequestError(w, attachmentErr)
		return
	}
	attachmentsJSON, err := json.Marshal(attachments)
	if err != nil {
		writeProjectDesignSystemError(w, http.StatusInternalServerError, "context_failed", "failed to encode attachments")
		return
	}

	input := designDocumentInputSnapshot{
		AgentID:             req.AgentID,
		ProjectResourceID:   uuidToString(scope.ProjectResourceID),
		IssueID:             uuidToString(issueUUID),
		DesignSystemID:      req.DesignSystemID,
		BuiltinDesignSystem: req.BuiltinDesignSystem,
		Platform:            req.Platform,
		Recipe:              req.Recipe,
		Brief:               req.Brief,
		Attachments:         attachmentsJSON,
	}
	inputJSON, err := json.Marshal(input)
	if err != nil || len(inputJSON) > designDocumentMaxSnapshotBytes {
		writeProjectDesignSystemError(w, http.StatusRequestEntityTooLarge, "input_snapshot_too_large", "design inputs exceed the size limit")
		return
	}

	document, task, err := h.createDesignDocumentTask(
		r.Context(), workspaceUUID, requesterUUID, projectUUID, scope, issueUUID, agentUUID, title, input, inputJSON, attachments,
	)
	if err != nil {
		if createdIssueID.Valid {
			// The companion issue only exists for a document that was created.
			if deleteErr := h.Queries.DeleteIssue(r.Context(), db.DeleteIssueParams{
				ID:          createdIssueID,
				WorkspaceID: workspaceUUID,
			}); deleteErr != nil {
				slog.Error("design document: companion issue left behind after a failed create",
					"issue_id", uuidToString(createdIssueID), "error", deleteErr)
			}
		}
		writeProjectDesignSystemRequestError(w, err)
		return
	}
	h.TaskService.NotifyTaskEnqueued(r.Context(), task)
	writeJSON(w, http.StatusCreated, designDocumentResponse(document, &task))
}

// resolveOptionalDesignDocumentIssue accepts an empty issue, and otherwise
// requires the issue to live in this project. A design document linked to an
// issue from a different project would be untraceable in the project tab.
func (h *Handler) resolveOptionalDesignDocumentIssue(
	r *http.Request,
	w http.ResponseWriter,
	workspaceID pgtype.UUID,
	projectID pgtype.UUID,
	raw string,
) (pgtype.UUID, bool) {
	if raw == "" {
		return pgtype.UUID{}, true
	}
	issueUUID, ok := parseUUIDOrBadRequest(w, raw, "issue_id")
	if !ok {
		return pgtype.UUID{}, false
	}
	issue, err := h.Queries.GetIssueInWorkspace(r.Context(), db.GetIssueInWorkspaceParams{
		ID: issueUUID, WorkspaceID: workspaceID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		writeProjectDesignSystemError(w, http.StatusNotFound, "issue_not_found", "issue not found")
		return pgtype.UUID{}, false
	}
	if err != nil {
		writeProjectDesignSystemError(w, http.StatusInternalServerError, "issue_lookup_failed", "failed to load issue")
		return pgtype.UUID{}, false
	}
	if issue.ProjectID != projectID {
		writeProjectDesignSystemError(w, http.StatusBadRequest, "issue_project_mismatch", "issue belongs to another project")
		return pgtype.UUID{}, false
	}
	return issueUUID, true
}

// ListDesignDocuments returns every document under a project, most recently
// touched first — the order the project tab lists them in (DC-042).
func (h *Handler) ListDesignDocuments(w http.ResponseWriter, r *http.Request) {
	workspaceUUID, _, ok := h.projectDesignSystemRequestScope(w, r)
	if !ok {
		return
	}
	projectUUID, ok := parseUUIDOrBadRequest(w, strings.TrimSpace(r.URL.Query().Get("project_id")), "project_id")
	if !ok {
		return
	}
	if _, err := h.Queries.GetProjectInWorkspace(r.Context(), db.GetProjectInWorkspaceParams{
		ID: projectUUID, WorkspaceID: workspaceUUID,
	}); err != nil {
		writeProjectDesignSystemError(w, http.StatusNotFound, "project_not_found", "project not found")
		return
	}
	documents, err := h.Queries.ListDesignDocumentsByProject(r.Context(), db.ListDesignDocumentsByProjectParams{
		WorkspaceID: workspaceUUID,
		ProjectID:   projectUUID,
	})
	if err != nil {
		writeProjectDesignSystemError(w, http.StatusInternalServerError, "lookup_failed", "failed to load design documents")
		return
	}
	responses := make([]DesignDocumentResponse, 0, len(documents))
	for _, document := range documents {
		// The active task decides whether "running" is still true, so the list
		// has to load it too. Without this a document whose task died holding
		// the pointer lists as 生成中 indefinitely — the detail view would
		// disagree with the list it was opened from.
		var activeTask *db.AgentTaskQueue
		if document.ActiveTaskID.Valid {
			task, err := h.Queries.GetAgentTask(r.Context(), document.ActiveTaskID)
			if err == nil {
				activeTask = &task
			} else if !errors.Is(err, pgx.ErrNoRows) {
				writeProjectDesignSystemError(w, http.StatusInternalServerError, "lookup_failed", "failed to load design documents")
				return
			}
		}
		responses = append(responses, designDocumentResponse(document, activeTask))
	}
	writeJSON(w, http.StatusOK, map[string]any{"documents": responses})
}

// designDocumentStatus derives the user-visible state from the pointers, so
// there is no status column that can disagree with them.
// isTerminalTaskStatus reports whether an agent task has finished for good.
// Anything else — queued, dispatched, running, waiting, deferred — is still
// on its way to one of these.
func isTerminalTaskStatus(status string) bool {
	switch status {
	case "completed", "failed", "cancelled":
		return true
	default:
		return false
	}
}

// designDocumentStatus derives the status from the document's own pointers.
//
// `task` is the row `active_task_id` points at. It is consulted only to
// disprove "running": a task that already reached a terminal state cannot
// still be generating, and treating the pointer alone as proof of work is what
// leaves a document reading "生成中" forever after a task dies without
// releasing it.
//
// nil means the pointer resolves to no task — the caller had none to resolve,
// or the row is gone. Neither is a running generation, so nil never reads as
// running. That is deliberately the opposite lean from designDocumentRunIsLive
// below, which assumes a run it could not read is live: a guard that guesses
// wrong destroys work, while a status that guesses wrong is corrected by the
// next poll. The dangerous direction differs, so the default does too.
// designDocumentRunIsLive reports whether a document's active_task_id still
// points at a task that can actually finish.
//
// The pointer alone is not the answer. A task that failed, was cancelled, or
// completed without clearing the pointer leaves it set forever, and a guard
// written as `ActiveTaskID.Valid` then locks the document out of every
// operation it protects — the same wedge behind "生成中 forever" that
// designDocumentStatus below already resolves by looking the task up. Guards
// must ask this, not the pointer, or a dead run becomes a permanent dead end.
//
// `queries` is the handle to ask on: guards that re-check inside the
// enqueue transaction must read the task through that transaction, not
// through h.Queries, or they read around their own row lock.
func (h *Handler) designDocumentRunIsLive(
	ctx context.Context,
	queries *db.Queries,
	document db.DesignDocument,
) bool {
	if !document.ActiveTaskID.Valid {
		return false
	}
	task, err := queries.GetAgentTaskInWorkspace(ctx, db.GetAgentTaskInWorkspaceParams{
		ID: document.ActiveTaskID, WorkspaceID: document.WorkspaceID,
	})
	switch {
	// The task row is gone: nothing is going to complete into this document,
	// so the pointer is a leftover and must not lock the document.
	case errors.Is(err, pgx.ErrNoRows):
		return false
	// Any other failure means we could not read the task, not that there is
	// none. A caller guarding a destructive step has to assume the run is
	// live and let the user retry, rather than act on an unread row.
	case err != nil:
		return true
	}
	return !isTerminalTaskStatus(task.Status)
}

func designDocumentStatus(document db.DesignDocument, task *db.AgentTaskQueue) string {
	activeTaskRunning := document.ActiveTaskID.Valid &&
		task != nil && !isTerminalTaskStatus(task.Status)
	switch {
	case activeTaskRunning:
		return "running"
	// The pointer outlived its task. Surface what the document actually has —
	// the failure, or the draft the failed run never replaced — instead of a
	// generation that is not happening.
	case document.ActiveTaskID.Valid && len(document.LastError) > 0 && string(document.LastError) != "null":
		return "failed"
	case document.SavedRevisionID.Valid && document.DraftRevisionID.Valid &&
		document.DraftRevisionID != document.SavedRevisionID:
		return "draft_ahead_of_saved"
	case document.SavedRevisionID.Valid:
		return "saved"
	case document.DraftRevisionID.Valid:
		return "draft"
	case len(document.LastError) > 0 && string(document.LastError) != "null":
		return "failed"
	default:
		return "empty"
	}
}

func designDocumentResponse(document db.DesignDocument, task *db.AgentTaskQueue) DesignDocumentResponse {
	response := DesignDocumentResponse{
		ID:                 uuidToString(document.ID),
		WorkspaceID:        uuidToString(document.WorkspaceID),
		ProjectID:          uuidToString(document.ProjectID),
		ProjectResourceID:  uuidToString(document.ProjectResourceID),
		IssueID:            uuidToString(document.IssueID),
		Title:              document.Title,
		Platform:           document.Platform,
		Recipe:             document.Recipe,
		Status:             designDocumentStatus(document, task),
		DraftRevisionID:    uuidToString(document.DraftRevisionID),
		SavedRevisionID:    uuidToString(document.SavedRevisionID),
		InputSnapshot:      jsonOrDefault(document.InputSnapshot, `{}`),
		LastError:          jsonOrDefault(document.LastError, `null`),
		RepositoryGrounded: document.ProjectResourceID.Valid,
	}
	if document.CreatedAt.Valid {
		response.CreatedAt = document.CreatedAt.Time.UTC().Format(time.RFC3339Nano)
	}
	if document.UpdatedAt.Valid {
		response.UpdatedAt = document.UpdatedAt.Time.UTC().Format(time.RFC3339Nano)
	}
	if document.SavedAt.Valid {
		response.SavedAt = document.SavedAt.Time.UTC().Format(time.RFC3339Nano)
	}
	if task != nil {
		var taskContext service.DesignDocumentTaskContext
		operation := ""
		if json.Unmarshal(task.Context, &taskContext) == nil {
			operation = string(taskContext.Operation)
		}
		response.ActiveTask = &ProjectDesignSystemTaskResponse{
			ID:            uuidToString(task.ID),
			AgentID:       uuidToString(task.AgentID),
			Status:        task.Status,
			Operation:     operation,
			Error:         textToPtr(task.Error),
			FailureReason: textToPtr(task.FailureReason),
			WaitReason:    textToPtr(task.WaitReason),
			CreatedAt:     timestampToString(task.CreatedAt),
			DispatchedAt:  timestampToPtr(task.DispatchedAt),
			StartedAt:     timestampToPtr(task.StartedAt),
			CompletedAt:   timestampToPtr(task.CompletedAt),
		}
	}
	return response
}

// createDesignDocumentTask creates the document and its first task in one
// transaction. A document without a task would show as permanently empty with
// nothing running, and a task without a document would have nowhere to
// deliver its package.
func (h *Handler) createDesignDocumentTask(
	ctx context.Context,
	workspaceID pgtype.UUID,
	requesterID pgtype.UUID,
	projectID pgtype.UUID,
	scope projectDesignSystemScope,
	issueID pgtype.UUID,
	agentID pgtype.UUID,
	title string,
	input designDocumentInputSnapshot,
	inputJSON []byte,
	attachments []designDocumentAttachmentSnapshot,
) (db.DesignDocument, db.AgentTaskQueue, error) {
	tx, err := h.TxStarter.Begin(ctx)
	if err != nil {
		return db.DesignDocument{}, db.AgentTaskQueue{}, projectDesignSystemInternalError("transaction_failed", "failed to start design generation")
	}
	defer tx.Rollback(ctx)
	queries := h.Queries.WithTx(tx)

	document, err := queries.CreateDesignDocument(ctx, db.CreateDesignDocumentParams{
		WorkspaceID:       workspaceID,
		ProjectID:         projectID,
		ProjectResourceID: scope.ProjectResourceID,
		IssueID:           issueID,
		Title:             title,
		Platform:          input.Platform,
		Recipe:            input.Recipe,
		CurrentAgentID:    agentID,
		InputSnapshot:     inputJSON,
		CreatedBy:         requesterID,
	})
	if err != nil {
		return db.DesignDocument{}, db.AgentTaskQueue{}, projectDesignSystemInternalError("create_failed", "failed to create design document")
	}
	agent, err := queries.GetAgent(ctx, agentID)
	if err != nil || agent.WorkspaceID != workspaceID {
		return db.DesignDocument{}, db.AgentTaskQueue{}, &projectDesignSystemRequestError{status: http.StatusNotFound, code: "agent_not_found", message: "agent not found"}
	}
	verdict, err := service.AgentReadiness(ctx, queries, agent)
	if err != nil {
		return db.DesignDocument{}, db.AgentTaskQueue{}, projectDesignSystemInternalError("agent_check_failed", "failed to check agent readiness")
	}
	if !verdict.Ready() {
		return db.DesignDocument{}, db.AgentTaskQueue{}, &projectDesignSystemRequestError{status: http.StatusConflict, code: "agent_unavailable", message: verdict.Detail}
	}

	contextJSON, err := h.designDocumentGenerateTaskContext(
		ctx, queries, requesterID, workspaceID, projectID, scope.ProjectResourceID, issueID, document.ID, agent.ID, input, inputJSON, attachments,
	)
	if err != nil {
		return db.DesignDocument{}, db.AgentTaskQueue{}, err
	}

	task, err := queries.CreateQuickCreateTask(ctx, db.CreateQuickCreateTaskParams{
		ID:        dbid.NewV7(),
		AgentID:   agent.ID,
		RuntimeID: agent.RuntimeID,
		Priority:  0,
		Context:   contextJSON,
	})
	if err != nil {
		return db.DesignDocument{}, db.AgentTaskQueue{}, projectDesignSystemInternalError("enqueue_failed", "failed to enqueue design generation")
	}

	document, err = queries.UpdateDesignDocumentActiveTask(ctx, db.UpdateDesignDocumentActiveTaskParams{
		ID:              document.ID,
		WorkspaceID:     workspaceID,
		CurrentAgentID:  agent.ID,
		ActiveTaskID:    task.ID,
		ActiveOperation: pgtype.Text{String: string(service.DesignDocumentGenerate), Valid: true},
		InputSnapshot:   inputJSON,
	})
	if err != nil {
		return db.DesignDocument{}, db.AgentTaskQueue{}, projectDesignSystemInternalError("update_failed", "failed to attach the design task")
	}
	if err := tx.Commit(ctx); err != nil {
		return db.DesignDocument{}, db.AgentTaskQueue{}, projectDesignSystemInternalError("transaction_failed", "failed to commit design generation")
	}
	return document, task, nil
}

// designDocumentGenerateTaskContext builds the envelope of a generation run —
// the first one, or a rerun after a first attempt that never produced a
// revision. Everything derives from the frozen composer snapshot so a rerun
// sees exactly what the first run saw (bar the agent, which the caller may
// have replaced). The design system is re-resolved from the frozen CHOICE, not
// copied: its digest is pinned here so the run itself stays deterministic.
func (h *Handler) designDocumentGenerateTaskContext(
	ctx context.Context,
	queries *db.Queries,
	requesterID pgtype.UUID,
	workspaceID pgtype.UUID,
	projectID pgtype.UUID,
	projectResourceID pgtype.UUID,
	issueID pgtype.UUID,
	documentID pgtype.UUID,
	agentID pgtype.UUID,
	input designDocumentInputSnapshot,
	inputJSON []byte,
	attachments []designDocumentAttachmentSnapshot,
) ([]byte, error) {
	// Pin the design system the agent must design under: the user's explicit
	// choice when there is one (DC-060), otherwise the repository -> project
	// fallback (DC-052).
	designSystemUUID := pgtype.UUID{}
	if input.DesignSystemID != "" {
		parsed, err := util.ParseUUID(input.DesignSystemID)
		if err != nil {
			return nil, &projectDesignSystemRequestError{
				status: http.StatusBadRequest, code: "design_system_invalid", message: "design_system_id is invalid",
			}
		}
		designSystemUUID = parsed
	}
	var builtinContext *service.BuiltinDesignContext
	if input.BuiltinDesignSystem != "" {
		detail, found, err := designsystemcatalogue.Get(input.BuiltinDesignSystem)
		if err != nil {
			return nil, projectDesignSystemInternalError("design_context_failed", "failed to load the built-in design system")
		}
		if !found {
			return nil, &projectDesignSystemRequestError{
				status: http.StatusNotFound, code: "design_system_not_found", message: "built-in design system not found",
			}
		}
		builtinContext = &service.BuiltinDesignContext{
			Slug:           detail.Slug,
			Name:           detail.Name,
			Category:       detail.Category,
			DesignMarkdown: detail.DesignMarkdown,
			TokensCSS:      detail.TokensCSS,
		}
	}
	designContext, err := (service.ProjectDesignContextResolver{
		Store:        queries,
		AllowedHosts: h.projectDesignSystemAllowedHosts(),
	}).Resolve(ctx, service.ResolveProjectDesignContextParams{
		WorkspaceID:       workspaceID,
		ProjectID:         projectID,
		ProjectResourceID: projectResourceID,
		DesignSystemID:    designSystemUUID,
		Builtin:           builtinContext,
	})
	if err != nil {
		if errors.Is(err, service.ErrSavedDesignContextInvalid) {
			return nil, &projectDesignSystemRequestError{status: http.StatusUnprocessableEntity, code: "design_context_invalid", message: "saved design system is invalid"}
		}
		return nil, projectDesignSystemInternalError("design_context_failed", "failed to resolve design context")
	}
	designContextJSON, err := json.Marshal(designContext)
	if err != nil {
		return nil, projectDesignSystemInternalError("design_context_failed", "failed to encode design context")
	}

	inputDigest, err := projectdesignsystem.SnapshotDigest(inputJSON)
	if err != nil {
		return nil, projectDesignSystemInternalError("input_digest_failed", "failed to digest design inputs")
	}
	project, err := queries.GetProjectInWorkspace(ctx, db.GetProjectInWorkspaceParams{
		ID: projectID, WorkspaceID: workspaceID,
	})
	if err != nil {
		return nil, &projectDesignSystemRequestError{status: http.StatusNotFound, code: "project_not_found", message: "project not found"}
	}
	projectJSON, err := json.Marshal(map[string]string{
		"id":          uuidToString(projectID),
		"title":       project.Title,
		"description": project.Description.String,
	})
	if err != nil {
		return nil, projectDesignSystemInternalError("context_failed", "failed to build agent task context")
	}

	contextJSON, err := json.Marshal(service.DesignDocumentTaskContext{
		Type:                service.DesignDocumentTaskContextType,
		Operation:           service.DesignDocumentGenerate,
		RequesterID:         uuidToString(requesterID),
		WorkspaceID:         uuidToString(workspaceID),
		ProjectID:           uuidToString(projectID),
		ProjectResourceID:   uuidToString(projectResourceID),
		IssueID:             uuidToString(issueID),
		DesignDocumentID:    uuidToString(documentID),
		AgentID:             uuidToString(agentID),
		Project:             projectJSON,
		Platform:            input.Platform,
		Recipe:              input.Recipe,
		Brief:               input.Brief,
		Attachments:         input.Attachments,
		DesignContext:       designContextJSON,
		DesignSystemDigest:  designContext.Digest,
		PackageSchema:       designDocumentPackageSchema,
		InputSnapshotSHA256: inputDigest,
		ExecutionReady:      true,
		Input:               designDocumentGenerateInput(projectResourceID.Valid, attachments),
	})
	if err != nil {
		return nil, projectDesignSystemInternalError("context_failed", "failed to build agent task context")
	}
	return contextJSON, nil
}

// designDocumentGenerateInput is the grounding envelope of a first generation
// (DC-053): a repository was attached, so the daemon checks it out and grounds
// the run against it; or none was, so the daemon records explicitly that no
// code was read and the agent designs from the requirement alone.
func designDocumentGenerateInput(repositoryAttached bool, attachments []designDocumentAttachmentSnapshot) service.DesignDocumentTaskInput {
	mode := service.DesignDocumentGroundingUnavailable
	if repositoryAttached {
		mode = service.DesignDocumentGroundingPending
	}
	return service.DesignDocumentTaskInput{
		SchemaVersion:       service.DesignDocumentInputSchema,
		RepositoryGrounding: mode,
		Attachments:         designDocumentTaskAttachments(attachments),
	}
}

// designDocumentPinnedInput is the grounding envelope of an adjustment. The
// daemon does not re-read code for an adjustment: it reuses a pinned receipt.
// Grounding receipts are not yet persisted per revision, so the pinned receipt
// states honestly that this run carries no repository evidence of its own —
// the immutable base package it starts from is where the first generation's
// grounding already landed.
func designDocumentPinnedInput() (service.DesignDocumentTaskInput, error) {
	receipt, err := json.Marshal(designdocument.RepositoryGrounding{
		SchemaVersion: designdocument.GroundingSchemaVersion,
		Status:        designdocument.GroundingUnavailable,
		Repositories:  []designdocument.GroundedRepository{},
		Facts:         []designdocument.GroundingFact{},
		Conflicts:     []designdocument.GroundingObservation{},
		Missing:       []designdocument.GroundingObservation{},
		Warnings:      []string{"This adjustment re-reads no repository; it builds on the immutable base revision, which already carries the first generation's repository evidence."},
	})
	if err != nil {
		return service.DesignDocumentTaskInput{}, err
	}
	return service.DesignDocumentTaskInput{
		SchemaVersion:       service.DesignDocumentInputSchema,
		RepositoryGrounding: service.DesignDocumentGroundingPinned,
		Repository:          receipt,
	}, nil
}

// jsonOrDefault keeps a nil or empty JSONB column from serialising as null
// where the API contract promises an object.
func jsonOrDefault(raw []byte, fallback string) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage(fallback)
	}
	return json.RawMessage(raw)
}
