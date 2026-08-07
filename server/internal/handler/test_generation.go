package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/logger"
	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

// testGenerationPlanVersion is stamped into every generated plan so a future
// change to the plan shape can be detected rather than guessed at.
const testGenerationPlanVersion = "1.0"

// defaultTestGenerationExpectedCaseTypes biases a run toward business coverage.
// The whole point of the feature is that generated cases are not only
// code-level, so the default scope contract says so explicitly.
var defaultTestGenerationExpectedCaseTypes = []string{
	"business_flow", "permission", "data_consistency", "boundary", "exception",
}

var validTestGenerationJobStatuses = []string{"queued", "running", "completed", "failed", "cancelled"}

// TestGenerationPlanRepo is one repository the run may read, with the paths it
// should stay inside. Bound by project_resource id for the same reason
// test_case_repo is: URLs change, resource ids do not.
type TestGenerationPlanRepo struct {
	ProjectResourceID string   `json:"project_resource_id"`
	Alias             string   `json:"alias"`
	URL               string   `json:"url,omitempty"`
	Ref               string   `json:"ref,omitempty"`
	PathGlobs         []string `json:"path_globs"`
}

// TestGenerationPlanPayload is the reviewable scope contract. A human edits and
// approves this before a single token is spent on generation.
type TestGenerationPlanPayload struct {
	Version                 string                   `json:"version"`
	Repos                   []TestGenerationPlanRepo `json:"repos"`
	Issues                  []string                 `json:"issues"`
	Modules                 []string                 `json:"modules"`
	KnowledgeRefs           []string                 `json:"knowledge_refs"`
	AttachmentIDs           []string                 `json:"attachment_ids"`
	ExpectedCaseTypes       []string                 `json:"expected_case_types"`
	ExistingCaseDigestCount int64                    `json:"existing_case_digest_count"`
	Instructions            string                   `json:"instructions"`
}

type TestGenerationJobResponse struct {
	ID          string         `json:"id"`
	WorkspaceID string         `json:"workspace_id"`
	ProjectID   string         `json:"project_id"`
	AgentID     *string        `json:"agent_id"`
	AgentTaskID *string        `json:"agent_task_id"`
	Status      string         `json:"status"`
	Input       map[string]any `json:"input"`
	Result      map[string]any `json:"result"`
	Error       *string        `json:"error"`
	CreatedBy   *string        `json:"created_by"`
	CreatedAt   string         `json:"created_at"`
	UpdatedAt   string         `json:"updated_at"`
}

type TestGenerationPlanResponse struct {
	ID          string         `json:"id"`
	WorkspaceID string         `json:"workspace_id"`
	JobID       string         `json:"job_id"`
	Status      string         `json:"status"`
	Plan        map[string]any `json:"plan"`
	ReviewNotes string         `json:"review_notes"`
	ApprovedBy  *string        `json:"approved_by"`
	ApprovedAt  *string        `json:"approved_at"`
	CreatedBy   *string        `json:"created_by"`
	CreatedAt   string         `json:"created_at"`
	UpdatedAt   string         `json:"updated_at"`
}

type TestCaseProposalResponse struct {
	ID           string         `json:"id"`
	WorkspaceID  string         `json:"workspace_id"`
	JobID        string         `json:"job_id"`
	TargetCaseID string         `json:"target_case_id"`
	Kind         string         `json:"kind"`
	Payload      map[string]any `json:"payload"`
	Rationale    string         `json:"rationale"`
	Status       string         `json:"status"`
	ReviewedBy   *string        `json:"reviewed_by"`
	ReviewedAt   *string        `json:"reviewed_at"`
	CreatedAt    string         `json:"created_at"`
}

func unmarshalJSONObject(raw []byte) map[string]any {
	out := map[string]any{}
	if len(raw) == 0 {
		return out
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		// The column has a NOT NULL DEFAULT and is only ever written by this
		// package, so a decode failure means drift rather than user input.
		// Degrade to an empty object; the UI optional-chains everything.
		slog.Warn("test generation: failed to decode json column", "error", err)
		return map[string]any{}
	}
	return out
}

func testGenerationJobToResponse(job db.TestGenerationJob) TestGenerationJobResponse {
	return TestGenerationJobResponse{
		ID:          uuidToString(job.ID),
		WorkspaceID: uuidToString(job.WorkspaceID),
		ProjectID:   uuidToString(job.ProjectID),
		AgentID:     uuidToPtr(job.AgentID),
		AgentTaskID: uuidToPtr(job.AgentTaskID),
		Status:      job.Status,
		Input:       unmarshalJSONObject(job.Input),
		Result:      unmarshalJSONObject(job.Result),
		Error:       textToPtr(job.Error),
		CreatedBy:   uuidToPtr(job.CreatedBy),
		CreatedAt:   timestampToString(job.CreatedAt),
		UpdatedAt:   timestampToString(job.UpdatedAt),
	}
}

func testGenerationPlanToResponse(plan db.TestGenerationPlan) TestGenerationPlanResponse {
	return TestGenerationPlanResponse{
		ID:          uuidToString(plan.ID),
		WorkspaceID: uuidToString(plan.WorkspaceID),
		JobID:       uuidToString(plan.JobID),
		Status:      plan.Status,
		Plan:        unmarshalJSONObject(plan.Plan),
		ReviewNotes: plan.ReviewNotes,
		ApprovedBy:  uuidToPtr(plan.ApprovedBy),
		ApprovedAt:  timestampToPtr(plan.ApprovedAt),
		CreatedBy:   uuidToPtr(plan.CreatedBy),
		CreatedAt:   timestampToString(plan.CreatedAt),
		UpdatedAt:   timestampToString(plan.UpdatedAt),
	}
}

func testCaseProposalToResponse(proposal db.TestCaseProposal) TestCaseProposalResponse {
	return TestCaseProposalResponse{
		ID:           uuidToString(proposal.ID),
		WorkspaceID:  uuidToString(proposal.WorkspaceID),
		JobID:        uuidToString(proposal.JobID),
		TargetCaseID: uuidToString(proposal.TargetCaseID),
		Kind:         proposal.Kind,
		Payload:      unmarshalJSONObject(proposal.Payload),
		Rationale:    proposal.Rationale,
		Status:       proposal.Status,
		ReviewedBy:   uuidToPtr(proposal.ReviewedBy),
		ReviewedAt:   timestampToPtr(proposal.ReviewedAt),
		CreatedAt:    timestampToString(proposal.CreatedAt),
	}
}

func (h *Handler) writeTestGenerationWriteError(w http.ResponseWriter, r *http.Request, err error, action string) {
	if isCheckViolation(err) {
		writeError(w, http.StatusBadRequest, "test generation job "+action+" rejected: a field value failed a database constraint")
		return
	}
	slog.Error("test generation job "+action+" failed", append(logger.RequestAttrs(r), "error", err)...)
	writeError(w, http.StatusInternalServerError, "failed to "+action+" test generation job")
}

// loadTestGenerationJobForUser resolves the {id} path param inside the caller's
// workspace. Every write below uses the returned job.ID, never the raw param.
func (h *Handler) loadTestGenerationJobForUser(w http.ResponseWriter, r *http.Request) (db.TestGenerationJob, pgtype.UUID, bool) {
	wsUUID, ok := parseUUIDOrBadRequest(w, h.resolveWorkspaceID(r), "workspace_id")
	if !ok {
		return db.TestGenerationJob{}, pgtype.UUID{}, false
	}
	idUUID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "id"), "test generation job id")
	if !ok {
		return db.TestGenerationJob{}, pgtype.UUID{}, false
	}
	job, err := h.Queries.GetTestGenerationJobInWorkspace(r.Context(), db.GetTestGenerationJobInWorkspaceParams{
		ID:          idUUID,
		WorkspaceID: wsUUID,
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "test generation job not found")
		return db.TestGenerationJob{}, pgtype.UUID{}, false
	}
	return job, wsUUID, true
}

type CreateTestGenerationJobRequest struct {
	ProjectID     string   `json:"project_id"`
	IssueIDs      []string `json:"issue_ids"`
	Modules       []string `json:"modules"`
	AttachmentIDs []string `json:"attachment_ids"`
	Instructions  string   `json:"instructions"`
}

func (h *Handler) CreateTestGenerationJob(w http.ResponseWriter, r *http.Request) {
	workspaceID := h.resolveWorkspaceID(r)
	wsUUID, ok := parseUUIDOrBadRequest(w, workspaceID, "workspace_id")
	if !ok {
		return
	}
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	userUUID, ok := parseUUIDOrBadRequest(w, userID, "user id")
	if !ok {
		return
	}
	var req CreateTestGenerationJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	projectUUID, ok := parseUUIDOrBadRequest(w, strings.TrimSpace(req.ProjectID), "project_id")
	if !ok {
		return
	}
	// A generation run without a project has no repositories, no documents and
	// no project description — writeProjectContext early-returns and the agent
	// would be working from workspace context alone.
	project, err := h.Queries.GetProjectInWorkspace(r.Context(), db.GetProjectInWorkspaceParams{
		ID:          projectUUID,
		WorkspaceID: wsUUID,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, "project not found")
		return
	}

	// Idempotent create: an in-flight run for this project is returned rather
	// than starting a second one that would duplicate its output.
	if existing, err := h.Queries.GetReusableTestGenerationJob(r.Context(), db.GetReusableTestGenerationJobParams{
		WorkspaceID: wsUUID,
		ProjectID:   project.ID,
	}); err == nil {
		writeJSON(w, http.StatusOK, testGenerationJobToResponse(existing))
		return
	} else if err != pgx.ErrNoRows {
		h.writeTestGenerationWriteError(w, r, err, "create")
		return
	}

	input := map[string]any{
		"issue_ids":      defaultStringSlice(req.IssueIDs),
		"modules":        defaultStringSlice(req.Modules),
		"attachment_ids": defaultStringSlice(req.AttachmentIDs),
		"instructions":   req.Instructions,
	}
	job, err := h.Queries.CreateTestGenerationJob(r.Context(), db.CreateTestGenerationJobParams{
		WorkspaceID: wsUUID,
		ProjectID:   project.ID,
		Status:      "queued",
		Input:       marshalJSONColumn(input, "{}"),
		CreatedBy:   userUUID,
	})
	if err != nil {
		h.writeTestGenerationWriteError(w, r, err, "create")
		return
	}
	resp := testGenerationJobToResponse(job)
	h.publish(protocol.EventTestGenerationJobUpdated, workspaceID, "member", userID, map[string]any{"job": resp})
	writeJSON(w, http.StatusCreated, resp)
}

func defaultStringSlice(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func (h *Handler) ListTestGenerationJobs(w http.ResponseWriter, r *http.Request) {
	wsUUID, ok := parseUUIDOrBadRequest(w, h.resolveWorkspaceID(r), "workspace_id")
	if !ok {
		return
	}
	var projectFilter pgtype.UUID
	if raw := strings.TrimSpace(r.URL.Query().Get("project_id")); raw != "" {
		projectFilter, ok = parseUUIDOrBadRequest(w, raw, "project_id")
		if !ok {
			return
		}
	}
	var statusFilter pgtype.Text
	if s := strings.TrimSpace(r.URL.Query().Get("status")); s != "" {
		if !validateTestCaseEnum(w, "status", s, validTestGenerationJobStatuses) {
			return
		}
		statusFilter = pgtype.Text{String: s, Valid: true}
	}
	limit := int32(50)
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			writeError(w, http.StatusBadRequest, "limit must be a positive integer")
			return
		}
		if parsed > 200 {
			parsed = 200
		}
		limit = int32(parsed)
	}
	jobs, err := h.Queries.ListTestGenerationJobs(r.Context(), db.ListTestGenerationJobsParams{
		WorkspaceID: wsUUID,
		ProjectID:   projectFilter,
		Status:      statusFilter,
		Limit:       limit,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list test generation jobs")
		return
	}
	resp := make([]TestGenerationJobResponse, len(jobs))
	for i, job := range jobs {
		resp[i] = testGenerationJobToResponse(job)
	}
	writeJSON(w, http.StatusOK, map[string]any{"jobs": resp, "total": len(resp)})
}

func (h *Handler) GetTestGenerationJob(w http.ResponseWriter, r *http.Request) {
	job, _, ok := h.loadTestGenerationJobForUser(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, testGenerationJobToResponse(job))
}

// buildDefaultTestGenerationPlan derives the initial scope contract from the
// project's own durable resources: every github_repo becomes a readable
// repository, every document becomes a business-knowledge reference. A human
// then narrows it before approving.
func (h *Handler) buildDefaultTestGenerationPlan(
	ctx context.Context,
	job db.TestGenerationJob,
) (TestGenerationPlanPayload, error) {
	payload := TestGenerationPlanPayload{
		Version:           testGenerationPlanVersion,
		Repos:             []TestGenerationPlanRepo{},
		Issues:            []string{},
		Modules:           []string{},
		KnowledgeRefs:     []string{},
		AttachmentIDs:     []string{},
		ExpectedCaseTypes: append([]string{}, defaultTestGenerationExpectedCaseTypes...),
	}

	input := unmarshalJSONObject(job.Input)
	payload.Issues = stringsFromAny(input["issue_ids"])
	payload.Modules = stringsFromAny(input["modules"])
	payload.AttachmentIDs = stringsFromAny(input["attachment_ids"])
	if instructions, isString := input["instructions"].(string); isString {
		payload.Instructions = instructions
	}

	resources, err := h.Queries.ListProjectResources(ctx, job.ProjectID)
	if err != nil {
		return payload, fmt.Errorf("list project resources: %w", err)
	}
	for _, resource := range resources {
		ref := unmarshalJSONObject(resource.ResourceRef)
		switch resource.ResourceType {
		case "github_repo":
			url, _ := ref["url"].(string)
			gitRef, _ := ref["ref"].(string)
			alias := strings.TrimSpace(resource.Label.String)
			if alias == "" {
				alias = repoAliasFromURL(url)
			}
			payload.Repos = append(payload.Repos, TestGenerationPlanRepo{
				ProjectResourceID: uuidToString(resource.ID),
				Alias:             alias,
				URL:               url,
				Ref:               gitRef,
				PathGlobs:         []string{},
			})
		case "document":
			if url, isString := ref["url"].(string); isString && url != "" {
				payload.KnowledgeRefs = append(payload.KnowledgeRefs, url)
			}
		}
	}

	existing, err := h.Queries.ListTestCases(ctx, db.ListTestCasesParams{
		WorkspaceID: job.WorkspaceID,
		ProjectID:   job.ProjectID,
	})
	if err != nil {
		return payload, fmt.Errorf("count existing test cases: %w", err)
	}
	payload.ExistingCaseDigestCount = int64(len(existing))

	return payload, nil
}

// repoAliasFromURL derives a short, step-referencable name from a repo URL.
// Display only — the durable binding is the project_resource id.
func repoAliasFromURL(url string) string {
	trimmed := strings.TrimSuffix(strings.TrimRight(url, "/"), ".git")
	if i := strings.LastIndex(trimmed, "/"); i >= 0 {
		trimmed = trimmed[i+1:]
	}
	if trimmed == "" {
		return "repo"
	}
	return trimmed
}

func stringsFromAny(value any) []string {
	items, isSlice := value.([]any)
	if !isSlice {
		return []string{}
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if s, isString := item.(string); isString && strings.TrimSpace(s) != "" {
			out = append(out, s)
		}
	}
	return out
}

func (h *Handler) GetTestGenerationPlan(w http.ResponseWriter, r *http.Request) {
	job, wsUUID, ok := h.loadTestGenerationJobForUser(w, r)
	if !ok {
		return
	}
	plan, err := h.Queries.GetTestGenerationPlanByJob(r.Context(), db.GetTestGenerationPlanByJobParams{
		JobID:       job.ID,
		WorkspaceID: wsUUID,
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "test generation plan not found")
		return
	}
	writeJSON(w, http.StatusOK, testGenerationPlanToResponse(plan))
}

func (h *Handler) GenerateTestGenerationPlan(w http.ResponseWriter, r *http.Request) {
	job, wsUUID, ok := h.loadTestGenerationJobForUser(w, r)
	if !ok {
		return
	}
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	userUUID, ok := parseUUIDOrBadRequest(w, userID, "user id")
	if !ok {
		return
	}

	payload, err := h.buildDefaultTestGenerationPlan(r.Context(), job)
	if err != nil {
		slog.Error("build test generation plan failed", append(logger.RequestAttrs(r), "error", err)...)
		writeError(w, http.StatusInternalServerError, "failed to build the generation plan")
		return
	}

	existing, err := h.Queries.GetTestGenerationPlanByJob(r.Context(), db.GetTestGenerationPlanByJobParams{
		JobID:       job.ID,
		WorkspaceID: wsUUID,
	})
	switch {
	case err == nil && existing.Status != "draft":
		writeError(w, http.StatusConflict, "the generation plan is already "+existing.Status+" and can no longer be regenerated")
		return
	case err == nil:
		updated, updateErr := h.Queries.UpdateTestGenerationPlan(r.Context(), db.UpdateTestGenerationPlanParams{
			ID:          existing.ID,
			WorkspaceID: wsUUID,
			Plan:        marshalJSONColumn(payload, "{}"),
		})
		if updateErr != nil {
			h.writeTestGenerationWriteError(w, r, updateErr, "update")
			return
		}
		writeJSON(w, http.StatusOK, testGenerationPlanToResponse(updated))
		return
	case err != pgx.ErrNoRows:
		h.writeTestGenerationWriteError(w, r, err, "read")
		return
	}

	created, err := h.Queries.CreateTestGenerationPlan(r.Context(), db.CreateTestGenerationPlanParams{
		WorkspaceID: wsUUID,
		JobID:       job.ID,
		Status:      "draft",
		Plan:        marshalJSONColumn(payload, "{}"),
		CreatedBy:   userUUID,
	})
	if err != nil {
		h.writeTestGenerationWriteError(w, r, err, "create")
		return
	}
	writeJSON(w, http.StatusCreated, testGenerationPlanToResponse(created))
}

type UpdateTestGenerationPlanRequest struct {
	Plan        *TestGenerationPlanPayload `json:"plan"`
	ReviewNotes *string                    `json:"review_notes"`
}

func (h *Handler) UpdateTestGenerationPlan(w http.ResponseWriter, r *http.Request) {
	job, wsUUID, ok := h.loadTestGenerationJobForUser(w, r)
	if !ok {
		return
	}
	var req UpdateTestGenerationPlanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	plan, err := h.Queries.GetTestGenerationPlanByJob(r.Context(), db.GetTestGenerationPlanByJobParams{
		JobID:       job.ID,
		WorkspaceID: wsUUID,
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "test generation plan not found")
		return
	}
	if plan.Status != "draft" {
		writeError(w, http.StatusConflict, "the generation plan is already "+plan.Status+" and can no longer be edited")
		return
	}

	params := db.UpdateTestGenerationPlanParams{ID: plan.ID, WorkspaceID: wsUUID}
	if req.Plan != nil {
		if req.Plan.Version == "" {
			req.Plan.Version = testGenerationPlanVersion
		}
		params.Plan = marshalJSONColumn(req.Plan, "{}")
	}
	if req.ReviewNotes != nil {
		params.ReviewNotes = pgtype.Text{String: *req.ReviewNotes, Valid: true}
	}
	updated, err := h.Queries.UpdateTestGenerationPlan(r.Context(), params)
	if err != nil {
		h.writeTestGenerationWriteError(w, r, err, "update")
		return
	}
	writeJSON(w, http.StatusOK, testGenerationPlanToResponse(updated))
}

// validateTestGenerationPlanForApproval refuses a scope that would waste a run:
// no repositories means the agent has nothing to read, and a repository that
// does not belong to this project means the binding is wrong.
func (h *Handler) validateTestGenerationPlanForApproval(
	ctx context.Context,
	w http.ResponseWriter,
	job db.TestGenerationJob,
	plan db.TestGenerationPlan,
) bool {
	var payload TestGenerationPlanPayload
	if err := json.Unmarshal(plan.Plan, &payload); err != nil {
		writeError(w, http.StatusBadRequest, "the generation plan is not valid JSON; regenerate it")
		return false
	}
	if len(payload.Repos) == 0 && len(payload.KnowledgeRefs) == 0 {
		writeError(w, http.StatusBadRequest,
			"the plan covers no repository and no document; attach a project resource before approving")
		return false
	}
	for _, repo := range payload.Repos {
		resourceUUID, ok := parseUUIDOrBadRequest(w, repo.ProjectResourceID, "project_resource_id")
		if !ok {
			return false
		}
		resource, err := h.Queries.GetProjectResourceInWorkspace(ctx, db.GetProjectResourceInWorkspaceParams{
			ID:          resourceUUID,
			WorkspaceID: job.WorkspaceID,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, "project resource not found: "+repo.ProjectResourceID)
			return false
		}
		if resource.ProjectID != job.ProjectID {
			writeError(w, http.StatusBadRequest,
				"project resource "+repo.ProjectResourceID+" belongs to a different project")
			return false
		}
	}
	return true
}

func (h *Handler) ApproveTestGenerationPlan(w http.ResponseWriter, r *http.Request) {
	job, wsUUID, ok := h.loadTestGenerationJobForUser(w, r)
	if !ok {
		return
	}
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	userUUID, ok := parseUUIDOrBadRequest(w, userID, "user id")
	if !ok {
		return
	}
	plan, err := h.Queries.GetTestGenerationPlanByJob(r.Context(), db.GetTestGenerationPlanByJobParams{
		JobID:       job.ID,
		WorkspaceID: wsUUID,
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "test generation plan not found")
		return
	}
	if plan.Status == "dispatched" {
		writeError(w, http.StatusConflict, "the generation plan has already been dispatched")
		return
	}
	if plan.Status != "draft" && plan.Status != "approved" {
		writeError(w, http.StatusConflict, "the generation plan is "+plan.Status+" and cannot be approved")
		return
	}
	if !h.validateTestGenerationPlanForApproval(r.Context(), w, job, plan) {
		return
	}
	approved, err := h.Queries.UpdateTestGenerationPlan(r.Context(), db.UpdateTestGenerationPlanParams{
		ID:          plan.ID,
		WorkspaceID: wsUUID,
		Status:      pgtype.Text{String: "approved", Valid: true},
		ApprovedBy:  userUUID,
		ApprovedAt:  pgtype.Timestamptz{Time: time.Now(), Valid: true},
	})
	if err != nil {
		h.writeTestGenerationWriteError(w, r, err, "approve")
		return
	}
	writeJSON(w, http.StatusOK, testGenerationPlanToResponse(approved))
}

type DispatchTestGenerationJobRequest struct {
	AgentID string `json:"agent_id"`
	Prompt  string `json:"prompt"`
}

func (h *Handler) DispatchTestGenerationJob(w http.ResponseWriter, r *http.Request) {
	job, wsUUID, ok := h.loadTestGenerationJobForUser(w, r)
	if !ok {
		return
	}
	workspaceID := h.resolveWorkspaceID(r)
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	userUUID, ok := parseUUIDOrBadRequest(w, userID, "user id")
	if !ok {
		return
	}
	var req DispatchTestGenerationJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// design_restore does not check this and happily mints a second agent task,
	// orphaning the first. The only guard there is a disabled button.
	if job.Status == "running" {
		writeError(w, http.StatusConflict, "this generation job is already running")
		return
	}

	agentUUID, ok := parseUUIDOrBadRequest(w, strings.TrimSpace(req.AgentID), "agent_id")
	if !ok {
		return
	}
	agent, err := h.Queries.GetAgentInWorkspace(r.Context(), db.GetAgentInWorkspaceParams{
		ID:          agentUUID,
		WorkspaceID: wsUUID,
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}
	if agent.ArchivedAt.Valid {
		writeError(w, http.StatusBadRequest, "this agent is archived")
		return
	}
	if !agent.RuntimeID.Valid {
		writeError(w, http.StatusBadRequest, "this agent has no runtime bound; start a daemon for it first")
		return
	}

	plan, err := h.Queries.GetTestGenerationPlanByJob(r.Context(), db.GetTestGenerationPlanByJobParams{
		JobID:       job.ID,
		WorkspaceID: wsUUID,
	})
	if err == pgx.ErrNoRows {
		writeError(w, http.StatusConflict, "a generation plan is required before dispatch")
		return
	}
	if err != nil {
		h.writeTestGenerationWriteError(w, r, err, "read")
		return
	}
	// There is deliberately no skip_plan escape hatch. Review is the feature.
	if plan.Status != "approved" {
		writeError(w, http.StatusConflict, "the generation plan must be approved before dispatch")
		return
	}

	contextPayload := service.TestGenerationContext{
		Type:        service.TestGenerationContextType,
		Prompt:      strings.TrimSpace(req.Prompt),
		RequesterID: userID,
		WorkspaceID: workspaceID,
		ProjectID:   uuidToString(job.ProjectID),
		AgentID:     uuidToString(agent.ID),
		JobID:       uuidToString(job.ID),
		Plan:        json.RawMessage(plan.Plan),
		Input:       json.RawMessage(job.Input),
	}
	contextJSON, err := json.Marshal(contextPayload)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to build the agent task context")
		return
	}

	agentTask, err := h.Queries.CreateQuickCreateTask(r.Context(), db.CreateQuickCreateTaskParams{
		AgentID:   agent.ID,
		RuntimeID: agent.RuntimeID,
		Priority:  0,
		Context:   contextJSON,
		// Attribution provenance, so this path is not a NULL-source enqueue
		// bypass (MUL-4302 §2). design_restore omits these.
		OriginatorUserID:  userUUID,
		AccountableUserID: userUUID,
		OriginatorSource:  pgtype.Text{String: "direct_human", Valid: true},
	})
	if err != nil {
		h.writeTestGenerationWriteError(w, r, err, "dispatch")
		return
	}

	updated, err := h.Queries.UpdateTestGenerationJob(r.Context(), db.UpdateTestGenerationJobParams{
		ID:          job.ID,
		WorkspaceID: wsUUID,
		Status:      pgtype.Text{String: "queued", Valid: true},
		AgentID:     agent.ID,
		AgentTaskID: agentTask.ID,
		Result:      []byte("{}"),
	})
	if err != nil {
		h.writeTestGenerationWriteError(w, r, err, "dispatch")
		return
	}
	if err := h.Queries.ClearTestGenerationJobError(r.Context(), db.ClearTestGenerationJobErrorParams{
		ID:          job.ID,
		WorkspaceID: wsUUID,
	}); err != nil {
		slog.Warn("clear test generation job error failed", append(logger.RequestAttrs(r), "error", err)...)
	}
	if _, err := h.Queries.MarkTestGenerationPlanDispatched(r.Context(), db.MarkTestGenerationPlanDispatchedParams{
		ID:          plan.ID,
		WorkspaceID: wsUUID,
	}); err != nil {
		slog.Warn("mark test generation plan dispatched failed", append(logger.RequestAttrs(r), "error", err)...)
	}

	resp := testGenerationJobToResponse(updated)
	h.publish(protocol.EventTestGenerationJobUpdated, workspaceID, "member", userID, map[string]any{"job": resp})
	writeJSON(w, http.StatusCreated, map[string]any{
		"job":           resp,
		"agent_task_id": uuidToString(agentTask.ID),
	})
}
