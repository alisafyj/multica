package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/multica-ai/multica/server/internal/projectdesignsystem"
	"github.com/multica-ai/multica/server/internal/service"
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
	// Optional traceable link only.
	IssueID     string          `json:"issue_id"`
	Title       string          `json:"title"`
	Platform    string          `json:"platform"`
	Recipe      string          `json:"recipe"`
	Brief       string          `json:"brief"`
	Attachments json.RawMessage `json:"attachments"`
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
	AgentID           string          `json:"agent_id"`
	ProjectResourceID string          `json:"project_resource_id,omitempty"`
	IssueID           string          `json:"issue_id,omitempty"`
	Platform          string          `json:"platform"`
	Recipe            string          `json:"recipe"`
	Brief             string          `json:"brief"`
	Attachments       json.RawMessage `json:"attachments,omitempty"`
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

	input := designDocumentInputSnapshot{
		AgentID:           req.AgentID,
		ProjectResourceID: uuidToString(scope.ProjectResourceID),
		IssueID:           uuidToString(issueUUID),
		Platform:          req.Platform,
		Recipe:            req.Recipe,
		Brief:             req.Brief,
		Attachments:       req.Attachments,
	}
	inputJSON, err := json.Marshal(input)
	if err != nil || len(inputJSON) > designDocumentMaxSnapshotBytes {
		writeProjectDesignSystemError(w, http.StatusRequestEntityTooLarge, "input_snapshot_too_large", "design inputs exceed the size limit")
		return
	}

	document, task, err := h.createDesignDocumentTask(
		r.Context(), workspaceUUID, requesterUUID, projectUUID, scope, issueUUID, agentUUID, title, input, inputJSON,
	)
	if err != nil {
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
// `task` is the row `active_task_id` points at, when the caller loaded it. It
// is consulted only to disprove "running": a task that already reached a
// terminal state cannot still be generating, and treating the pointer alone as
// proof of work is what leaves a document reading "生成中" forever after a task
// dies without releasing it. Callers that pass nil get the pointer-only
// reading, which is correct for list endpoints that never claim a live task.
func designDocumentStatus(document db.DesignDocument, task *db.AgentTaskQueue) string {
	activeTaskRunning := document.ActiveTaskID.Valid &&
		(task == nil || !isTerminalTaskStatus(task.Status))
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

	// Pin the design system the agent must design under, resolved through the
	// repository -> project fallback (DC-052). Digest included so the
	// revision records exactly which system constrained it.
	designContext, err := (service.ProjectDesignContextResolver{
		Store:        queries,
		AllowedHosts: h.projectDesignSystemAllowedHosts(),
	}).Resolve(ctx, service.ResolveProjectDesignContextParams{
		WorkspaceID:       workspaceID,
		ProjectID:         projectID,
		ProjectResourceID: scope.ProjectResourceID,
	})
	if err != nil {
		if errors.Is(err, service.ErrSavedDesignContextInvalid) {
			return db.DesignDocument{}, db.AgentTaskQueue{}, &projectDesignSystemRequestError{status: http.StatusUnprocessableEntity, code: "design_context_invalid", message: "saved design system is invalid"}
		}
		return db.DesignDocument{}, db.AgentTaskQueue{}, projectDesignSystemInternalError("design_context_failed", "failed to resolve design context")
	}
	designContextJSON, err := json.Marshal(designContext)
	if err != nil {
		return db.DesignDocument{}, db.AgentTaskQueue{}, projectDesignSystemInternalError("design_context_failed", "failed to encode design context")
	}

	inputDigest, err := projectdesignsystem.SnapshotDigest(inputJSON)
	if err != nil {
		return db.DesignDocument{}, db.AgentTaskQueue{}, projectDesignSystemInternalError("input_digest_failed", "failed to digest design inputs")
	}
	project, err := queries.GetProjectInWorkspace(ctx, db.GetProjectInWorkspaceParams{
		ID: projectID, WorkspaceID: workspaceID,
	})
	if err != nil {
		return db.DesignDocument{}, db.AgentTaskQueue{}, &projectDesignSystemRequestError{status: http.StatusNotFound, code: "project_not_found", message: "project not found"}
	}
	projectJSON, err := json.Marshal(map[string]string{
		"id":          uuidToString(projectID),
		"title":       project.Title,
		"description": project.Description.String,
	})
	if err != nil {
		return db.DesignDocument{}, db.AgentTaskQueue{}, projectDesignSystemInternalError("context_failed", "failed to build agent task context")
	}

	contextJSON, err := json.Marshal(service.DesignDocumentTaskContext{
		Type:                service.DesignDocumentTaskContextType,
		Operation:           service.DesignDocumentGenerate,
		RequesterID:         uuidToString(requesterID),
		WorkspaceID:         uuidToString(workspaceID),
		ProjectID:           uuidToString(projectID),
		ProjectResourceID:   uuidToString(scope.ProjectResourceID),
		IssueID:             uuidToString(issueID),
		DesignDocumentID:    uuidToString(document.ID),
		AgentID:             uuidToString(agent.ID),
		Project:             projectJSON,
		Platform:            input.Platform,
		Recipe:              input.Recipe,
		Brief:               input.Brief,
		Attachments:         input.Attachments,
		DesignContext:       designContextJSON,
		DesignSystemDigest:  designContext.Digest,
		PackageSchema:       designDocumentPackageSchema,
		InputSnapshotSHA256: inputDigest,
	})
	if err != nil {
		return db.DesignDocument{}, db.AgentTaskQueue{}, projectDesignSystemInternalError("context_failed", "failed to build agent task context")
	}

	task, err := queries.CreateQuickCreateTask(ctx, db.CreateQuickCreateTaskParams{
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

// jsonOrDefault keeps a nil or empty JSONB column from serialising as null
// where the API contract promises an object.
func jsonOrDefault(raw []byte, fallback string) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage(fallback)
	}
	return json.RawMessage(raw)
}
