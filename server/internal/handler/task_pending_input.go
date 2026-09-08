package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/middleware"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

const (
	pendingInputBodyLimit           = 32 << 10
	pendingInputMaxQuestions        = 3
	pendingInputMaxOptions          = 8
	pendingInputMaxQuestionRunes    = 2000
	pendingInputMaxHeaderRunes      = 120
	pendingInputMaxLabelRunes       = 200
	pendingInputMaxDescriptionRunes = 500
	maxPendingInputAnswerRunes      = 2000
	pendingInputMaxPerClaim         = 32
)

var pendingInputRequestKeyPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

type taskPendingInputOption struct {
	Label       string `json:"label"`
	Description string `json:"description"`
}

type taskPendingInputQuestion struct {
	ID          string                   `json:"id"`
	Header      string                   `json:"header"`
	Question    string                   `json:"question"`
	Options     []taskPendingInputOption `json:"options"`
	AllowOther  bool                     `json:"allow_other"`
	MultiSelect bool                     `json:"multi_select"`
	Secret      bool                     `json:"secret,omitempty"`
}

type taskPendingInputAnswer struct {
	Answers []string `json:"answers"`
}

type registerTaskPendingInputRequest struct {
	Version         int32                      `json:"version"`
	RequestKey      string                     `json:"request_key"`
	Blocking        bool                       `json:"blocking"`
	Questions       []taskPendingInputQuestion `json:"questions"`
	ClaimGeneration int64                      `json:"claim_generation"`
}

type answerTaskPendingInputRequest struct {
	IdempotencyKey string                            `json:"idempotency_key"`
	Answers        map[string]taskPendingInputAnswer `json:"answers"`
}

type taskPendingInputResponse struct {
	ID                string                            `json:"id"`
	TaskID            string                            `json:"task_id"`
	IssueID           string                            `json:"issue_id"`
	QuestionCommentID string                            `json:"question_comment_id"`
	State             string                            `json:"state"`
	Version           int32                             `json:"version"`
	Questions         []taskPendingInputQuestion        `json:"questions"`
	Answers           map[string]taskPendingInputAnswer `json:"answers"`
	CreatedAt         string                            `json:"created_at"`
	AnsweredAt        *string                           `json:"answered_at"`
	AckedAt           *string                           `json:"acked_at"`
}

func decodePendingInputJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, pendingInputBodyLimit))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return false
	}
	return true
}

func normalizePendingInputQuestions(questions []taskPendingInputQuestion) ([]taskPendingInputQuestion, error) {
	if len(questions) == 0 || len(questions) > pendingInputMaxQuestions {
		return nil, fmt.Errorf("questions must contain between 1 and %d entries", pendingInputMaxQuestions)
	}
	normalized := make([]taskPendingInputQuestion, len(questions))
	seenQuestions := make(map[string]struct{}, len(questions))
	for i, question := range questions {
		if !canonicalPendingInputText(question.ID, false, pendingInputMaxLabelRunes) {
			return nil, fmt.Errorf("question %d has an invalid id", i+1)
		}
		if _, exists := seenQuestions[question.ID]; exists {
			return nil, fmt.Errorf("question ids must be unique")
		}
		seenQuestions[question.ID] = struct{}{}
		if question.Secret {
			return nil, fmt.Errorf("secret questions are not supported")
		}
		if !canonicalPendingInputText(question.Header, true, pendingInputMaxHeaderRunes) {
			return nil, fmt.Errorf("question %q header is too long", question.ID)
		}
		if !canonicalPendingInputText(question.Question, false, pendingInputMaxQuestionRunes) {
			return nil, fmt.Errorf("question %q text is invalid", question.ID)
		}
		if len(question.Options) > pendingInputMaxOptions {
			return nil, fmt.Errorf("question %q has too many options", question.ID)
		}
		if len(question.Options) == 0 && !question.AllowOther {
			return nil, fmt.Errorf("question %q requires an option or allow_other", question.ID)
		}
		seenLabels := make(map[string]struct{}, len(question.Options))
		for j := range question.Options {
			option := &question.Options[j]
			if !canonicalPendingInputText(option.Label, false, pendingInputMaxLabelRunes) {
				return nil, fmt.Errorf("question %q has an invalid option label", question.ID)
			}
			if !canonicalPendingInputText(option.Description, true, pendingInputMaxDescriptionRunes) {
				return nil, fmt.Errorf("question %q has an option description that is too long", question.ID)
			}
			if _, exists := seenLabels[option.Label]; exists {
				return nil, fmt.Errorf("question %q option labels must be unique", question.ID)
			}
			seenLabels[option.Label] = struct{}{}
		}
		if question.Options == nil {
			question.Options = []taskPendingInputOption{}
		}
		normalized[i] = question
	}
	return normalized, nil
}

func canonicalPendingInputText(value string, allowEmpty bool, maxRunes int) bool {
	if strings.ContainsRune(value, '\x00') || value != strings.TrimSpace(value) || utf8RuneCount(value) > maxRunes {
		return false
	}
	return allowEmpty || value != ""
}

func utf8RuneCount(value string) int {
	return len([]rune(value))
}

func normalizedPendingInputRequest(req registerTaskPendingInputRequest) (registerTaskPendingInputRequest, []byte, error) {
	if req.Version != 1 {
		return req, nil, fmt.Errorf("version must be 1")
	}
	if !req.Blocking {
		return req, nil, fmt.Errorf("blocking must be true")
	}
	if req.RequestKey != strings.TrimSpace(req.RequestKey) || !pendingInputRequestKeyPattern.MatchString(req.RequestKey) {
		return req, nil, fmt.Errorf("request_key must be a sha256 hash")
	}
	if req.ClaimGeneration <= 0 {
		return req, nil, fmt.Errorf("claim_generation must be positive")
	}
	questions, err := normalizePendingInputQuestions(req.Questions)
	if err != nil {
		return req, nil, err
	}
	req.Questions = questions
	canonical, err := json.Marshal(req)
	return req, canonical, err
}

func pendingInputRequestSHA(canonical []byte) string {
	sum := sha256.Sum256(canonical)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func taskPendingInputToResponse(row db.TaskPendingInput, stateOverride string) (taskPendingInputResponse, error) {
	var questions []taskPendingInputQuestion
	if err := json.Unmarshal(row.Questions, &questions); err != nil {
		return taskPendingInputResponse{}, fmt.Errorf("decode pending input questions: %w", err)
	}
	var answers map[string]taskPendingInputAnswer
	if len(row.Answers) > 0 {
		if err := json.Unmarshal(row.Answers, &answers); err != nil {
			return taskPendingInputResponse{}, fmt.Errorf("decode pending input answers: %w", err)
		}
	}
	state := row.State
	if stateOverride != "" {
		state = stateOverride
	}
	return taskPendingInputResponse{
		ID:                uuidToString(row.ID),
		TaskID:            uuidToString(row.TaskID),
		IssueID:           uuidToString(row.IssueID),
		QuestionCommentID: uuidToString(row.QuestionCommentID),
		State:             state,
		Version:           row.Version,
		Questions:         questions,
		Answers:           answers,
		CreatedAt:         timestampToString(row.CreatedAt),
		AnsweredAt:        timestampToPtr(row.AnsweredAt),
		AckedAt:           timestampToPtr(row.AckedAt),
	}, nil
}

func renderPendingInputQuestionComment(questions []taskPendingInputQuestion) string {
	var out strings.Builder
	out.WriteString("I need your input before I can continue.\n")
	for i, question := range questions {
		out.WriteString("\n")
		if question.Header != "" {
			fmt.Fprintf(&out, "%d. **%s**\n", i+1, question.Header)
		} else {
			fmt.Fprintf(&out, "%d. ", i+1)
		}
		out.WriteString(question.Question)
		out.WriteString("\n")
		for _, option := range question.Options {
			fmt.Fprintf(&out, "- %s", option.Label)
			if option.Description != "" {
				fmt.Fprintf(&out, ": %s", option.Description)
			}
			out.WriteString("\n")
		}
	}
	return strings.TrimSpace(out.String())
}

func renderPendingInputAnswerComment(questions []taskPendingInputQuestion, answers map[string]taskPendingInputAnswer) string {
	var out strings.Builder
	out.WriteString("Answers:\n")
	for _, question := range questions {
		values := answers[question.ID].Answers
		label := question.Header
		if label == "" {
			label = question.Question
		}
		fmt.Fprintf(&out, "\n- **%s:** %s", label, strings.Join(values, ", "))
	}
	return out.String()
}

func validatePendingInputAnswers(questions []taskPendingInputQuestion, answers map[string]taskPendingInputAnswer) (map[string]taskPendingInputAnswer, []byte, error) {
	if len(answers) != len(questions) {
		return nil, nil, fmt.Errorf("answers must contain exactly one entry for each question")
	}
	normalized := make(map[string]taskPendingInputAnswer, len(questions))
	for _, question := range questions {
		answer, ok := answers[question.ID]
		if !ok {
			return nil, nil, fmt.Errorf("missing answer for question %q", question.ID)
		}
		if len(answer.Answers) == 0 || (!question.MultiSelect && len(answer.Answers) != 1) || len(answer.Answers) > pendingInputMaxOptions {
			return nil, nil, fmt.Errorf("invalid answer count for question %q", question.ID)
		}
		allowed := make(map[string]struct{}, len(question.Options))
		for _, option := range question.Options {
			allowed[option.Label] = struct{}{}
		}
		seen := make(map[string]struct{}, len(answer.Answers))
		values := make([]string, len(answer.Answers))
		for i, value := range answer.Answers {
			if !canonicalPendingInputText(value, false, maxPendingInputAnswerRunes) {
				return nil, nil, fmt.Errorf("question %q contains an invalid answer", question.ID)
			}
			if _, duplicate := seen[value]; duplicate {
				return nil, nil, fmt.Errorf("question %q contains duplicate answers", question.ID)
			}
			seen[value] = struct{}{}
			if _, listed := allowed[value]; !listed && !question.AllowOther {
				return nil, nil, fmt.Errorf("question %q contains an unsupported answer", question.ID)
			}
			values[i] = value
		}
		normalized[question.ID] = taskPendingInputAnswer{Answers: values}
	}
	canonical, err := json.Marshal(normalized)
	return normalized, canonical, err
}

func pendingInputAnswersEqual(stored []byte, expected map[string]taskPendingInputAnswer) bool {
	var decoded map[string]taskPendingInputAnswer
	return json.Unmarshal(stored, &decoded) == nil && maps.EqualFunc(decoded, expected, func(left, right taskPendingInputAnswer) bool {
		return slices.Equal(left.Answers, right.Answers)
	})
}

func parsePendingInputClaimGeneration(w http.ResponseWriter, r *http.Request) (int64, bool) {
	value, err := strconv.ParseInt(strings.TrimSpace(r.URL.Query().Get("claim_generation")), 10, 64)
	if err != nil || value <= 0 {
		writeError(w, http.StatusBadRequest, "invalid claim_generation")
		return 0, false
	}
	return value, true
}

func taskMatchesPendingClaim(task db.AgentTaskQueue, pending db.TaskPendingInput, runtimeID pgtype.UUID, generation int64) bool {
	return task.Status == "running" && task.DispatchedAt.Valid && task.DispatchedAt.Time.UnixMicro() == generation &&
		uuidToString(task.ID) == uuidToString(pending.TaskID) && task.RuntimeID == runtimeID && pending.RuntimeID == runtimeID &&
		pending.ClaimGeneration == generation
}

func pendingInputClosedState(task db.AgentTaskQueue, pending db.TaskPendingInput, runtimeID pgtype.UUID, generation int64, now time.Time) string {
	if pending.AckedAt.Valid || (pending.State != "open" && pending.State != "answered") {
		return pending.State
	}
	if !taskMatchesPendingClaim(task, pending, runtimeID, generation) {
		return "cancelled"
	}
	if pending.State == "open" && pending.ExpiresAt.Valid && !pending.ExpiresAt.Time.After(now) {
		return "expired"
	}
	return pending.State
}

func (h *Handler) RegisterTaskPendingInput(w http.ResponseWriter, r *http.Request) {
	var request registerTaskPendingInputRequest
	if !decodePendingInputJSON(w, r, &request) {
		return
	}
	request, canonical, err := normalizedPendingInputRequest(request)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	runtimeID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "runtimeId"), "runtime_id")
	if !ok {
		return
	}
	taskID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "taskId"), "task_id")
	if !ok {
		return
	}
	workspaceID, err := parseUUIDQuiet(middleware.DaemonWorkspaceIDFromContext(r.Context()))
	daemonID := middleware.DaemonIDFromContext(r.Context())
	if err != nil || daemonID == "" || middleware.DaemonAuthPathFromContext(r.Context()) != middleware.DaemonAuthPathDaemonToken {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}

	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to begin pending input registration")
		return
	}
	defer tx.Rollback(r.Context())
	qtx := h.Queries.WithTx(tx)
	if _, err := qtx.LockTaskRunEvidenceWorkspace(r.Context(), workspaceID); err != nil {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}
	runtime, err := qtx.LockTaskPendingInputRuntimeForDaemon(r.Context(), db.LockTaskPendingInputRuntimeForDaemonParams{
		RuntimeID: runtimeID, WorkspaceID: workspaceID, DaemonID: pgtype.Text{String: daemonID, Valid: true},
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}
	task, err := qtx.LockTaskRunEvidenceTask(r.Context(), db.LockTaskRunEvidenceTaskParams{TaskID: taskID, RuntimeID: runtime.ID})
	if err != nil || !task.IssueID.Valid {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}
	if task.Status != "running" || !task.DispatchedAt.Valid || task.DispatchedAt.Time.UnixMicro() != request.ClaimGeneration {
		writeError(w, http.StatusConflict, "task claim is no longer active")
		return
	}
	requestSHA := pendingInputRequestSHA(canonical)
	existing, err := qtx.GetTaskPendingInputByRequest(r.Context(), db.GetTaskPendingInputByRequestParams{
		TaskID: task.ID, ClaimGeneration: request.ClaimGeneration, RequestKey: request.RequestKey, WorkspaceID: workspaceID,
	})
	if err == nil {
		if existing.RequestSha256 != requestSHA {
			writeError(w, http.StatusConflict, "request_key already belongs to different input")
			return
		}
		if err := tx.Commit(r.Context()); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to finish pending input registration")
			return
		}
		response, err := taskPendingInputToResponse(existing, "")
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to encode pending input")
			return
		}
		writeJSON(w, http.StatusOK, response)
		return
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusInternalServerError, "failed to inspect pending input")
		return
	}
	capacity, err := qtx.CountTaskPendingInputsForClaim(r.Context(), db.CountTaskPendingInputsForClaimParams{
		TaskID: task.ID, ClaimGeneration: request.ClaimGeneration,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to inspect pending input capacity")
		return
	}
	if capacity.TotalCount >= pendingInputMaxPerClaim {
		writeError(w, http.StatusConflict, "pending input limit reached for task claim")
		return
	}
	if capacity.ActiveCount != 0 {
		writeError(w, http.StatusConflict, "task claim already has an active pending input")
		return
	}
	issue, err := qtx.LockIssueForDescriptionUpdate(r.Context(), db.LockIssueForDescriptionUpdateParams{ID: task.IssueID, WorkspaceID: workspaceID})
	if err != nil {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}
	questionsJSON, _ := json.Marshal(request.Questions)
	commentID := dbid.NewV7()
	createdComment, err := qtx.CreateComment(r.Context(), db.CreateCommentParams{
		ID: commentID, IssueID: task.IssueID, WorkspaceID: workspaceID,
		AuthorType: "agent", AuthorID: task.AgentID, Content: renderPendingInputQuestionComment(request.Questions),
		Type: "comment", SourceTaskID: task.ID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create pending input question")
		return
	}
	row, err := qtx.CreateTaskPendingInput(r.Context(), db.CreateTaskPendingInputParams{
		ID: dbid.NewV7(), WorkspaceID: workspaceID, IssueID: task.IssueID, TaskID: task.ID,
		AgentID: task.AgentID, RuntimeID: runtimeID, ClaimGeneration: request.ClaimGeneration,
		RequestKey: request.RequestKey, RequestSha256: requestSHA, Questions: questionsJSON, QuestionCommentID: commentID,
		QuestionIssueRevision: pgtype.Int8{Int64: createdComment.IssueRevision, Valid: true},
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to register pending input")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to commit pending input registration")
		return
	}
	comment := createdComment.Comment()
	h.publish(protocol.EventCommentCreated, uuidToString(workspaceID), "agent", uuidToString(task.AgentID), map[string]any{
		"comment": commentToResponse(comment, nil, nil), "issue_title": issue.Title,
		"issue_assignee_type": textToPtr(issue.AssigneeType), "issue_assignee_id": uuidToPtr(issue.AssigneeID),
		"issue_status": issue.Status, "issue_revision": createdComment.IssueRevision,
	})
	response, err := taskPendingInputToResponse(row, "")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to encode pending input")
		return
	}
	writeJSON(w, http.StatusCreated, response)
}

func (h *Handler) loadDaemonPendingInputForUpdate(ctx context.Context, qtx *db.Queries, r *http.Request, runtimeID, taskID pgtype.UUID, pendingID pgtype.UUID, generation int64) (db.AgentTaskQueue, db.TaskPendingInput, db.AgentRuntime, int, error) {
	workspaceID, err := parseUUIDQuiet(middleware.DaemonWorkspaceIDFromContext(r.Context()))
	daemonID := middleware.DaemonIDFromContext(r.Context())
	if err != nil || daemonID == "" || middleware.DaemonAuthPathFromContext(r.Context()) != middleware.DaemonAuthPathDaemonToken {
		return db.AgentTaskQueue{}, db.TaskPendingInput{}, db.AgentRuntime{}, http.StatusNotFound, errors.New("pending input not found")
	}
	if _, err := qtx.LockTaskRunEvidenceWorkspace(ctx, workspaceID); err != nil {
		return db.AgentTaskQueue{}, db.TaskPendingInput{}, db.AgentRuntime{}, http.StatusNotFound, err
	}
	runtime, err := qtx.LockTaskPendingInputRuntimeForDaemon(ctx, db.LockTaskPendingInputRuntimeForDaemonParams{
		RuntimeID: runtimeID, WorkspaceID: workspaceID, DaemonID: pgtype.Text{String: daemonID, Valid: true},
	})
	if err != nil {
		return db.AgentTaskQueue{}, db.TaskPendingInput{}, runtime, http.StatusNotFound, err
	}
	task, err := qtx.LockTaskRunEvidenceTask(ctx, db.LockTaskRunEvidenceTaskParams{TaskID: taskID, RuntimeID: runtime.ID})
	if err != nil {
		return task, db.TaskPendingInput{}, runtime, http.StatusNotFound, err
	}
	pending, err := qtx.LockTaskPendingInput(ctx, db.LockTaskPendingInputParams{ID: pendingID, WorkspaceID: workspaceID})
	if err != nil {
		return task, pending, runtime, http.StatusNotFound, err
	}
	if pending.WorkspaceID != workspaceID || task.RuntimeID != runtimeID || pending.RuntimeID != runtimeID {
		return task, pending, runtime, http.StatusNotFound, errors.New("pending input not found")
	}
	if uuidToString(pending.TaskID) != uuidToString(taskID) || pending.ClaimGeneration != generation {
		return task, pending, runtime, http.StatusNotFound, errors.New("pending input not found")
	}
	return task, pending, runtime, 0, nil
}

func parseUUIDQuiet(value string) (pgtype.UUID, error) {
	var parsed pgtype.UUID
	err := parsed.Scan(value)
	return parsed, err
}

func (h *Handler) persistPendingInputClosedState(ctx context.Context, qtx *db.Queries, task db.AgentTaskQueue, pending db.TaskPendingInput, runtimeID pgtype.UUID, generation int64) (db.TaskPendingInput, string, error) {
	state := pendingInputClosedState(task, pending, runtimeID, generation, time.Now())
	if state == pending.State {
		return pending, state, nil
	}
	closed, err := qtx.CloseTaskPendingInput(ctx, db.CloseTaskPendingInputParams{ID: pending.ID, WorkspaceID: pending.WorkspaceID, State: state})
	if errors.Is(err, pgx.ErrNoRows) {
		return pending, state, nil
	}
	return closed, state, err
}

func (h *Handler) GetTaskPendingInputForDaemon(w http.ResponseWriter, r *http.Request) {
	h.handleTaskPendingInputDaemonRead(w, r, false)
}

func (h *Handler) AckTaskPendingInput(w http.ResponseWriter, r *http.Request) {
	h.handleTaskPendingInputDaemonRead(w, r, true)
}

func (h *Handler) handleTaskPendingInputDaemonRead(w http.ResponseWriter, r *http.Request, ack bool) {
	if ack {
		var request struct {
			ClaimGeneration int64 `json:"claim_generation"`
		}
		if !decodePendingInputJSON(w, r, &request) {
			return
		}
		r.URL.RawQuery = "claim_generation=" + strconv.FormatInt(request.ClaimGeneration, 10)
	}
	generation, ok := parsePendingInputClaimGeneration(w, r)
	if !ok {
		return
	}
	runtimeID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "runtimeId"), "runtime_id")
	if !ok {
		return
	}
	taskID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "taskId"), "task_id")
	if !ok {
		return
	}
	pendingID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "pendingId"), "pending_input_id")
	if !ok {
		return
	}
	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to begin pending input read")
		return
	}
	defer tx.Rollback(r.Context())
	qtx := h.Queries.WithTx(tx)
	task, pending, _, status, err := h.loadDaemonPendingInputForUpdate(r.Context(), qtx, r, runtimeID, taskID, pendingID, generation)
	if err != nil {
		writeError(w, status, "pending input not found")
		return
	}
	pending, state, err := h.persistPendingInputClosedState(r.Context(), qtx, task, pending, runtimeID, generation)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update pending input")
		return
	}
	if state == "cancelled" || state == "expired" {
		if err := tx.Commit(r.Context()); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to close pending input")
			return
		}
		writeError(w, http.StatusConflict, "pending input is no longer active")
		return
	}
	if ack {
		if state != "answered" {
			writeError(w, http.StatusConflict, "pending input is not answered")
			return
		}
		if taskMatchesPendingClaim(task, pending, runtimeID, generation) {
			rows, err := qtx.RecordTaskPendingInputDelivery(r.Context(), db.RecordTaskPendingInputDeliveryParams{
				PendingID: pending.ID, WorkspaceID: pending.WorkspaceID,
			})
			if err != nil {
				writeError(w, http.StatusInternalServerError, "failed to record pending input delivery")
				return
			}
			if rows != 1 {
				writeError(w, http.StatusConflict, "pending input is no longer active")
				return
			}
		}
		pending, err = qtx.AckTaskPendingInput(r.Context(), db.AckTaskPendingInputParams{ID: pending.ID, WorkspaceID: pending.WorkspaceID})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to acknowledge pending input")
			return
		}
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to finish pending input request")
		return
	}
	response, err := taskPendingInputToResponse(pending, "")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to encode pending input")
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) ListTaskPendingInputsForIssue(w http.ResponseWriter, r *http.Request) {
	issue, ok := h.loadIssueForUser(w, r, chi.URLParam(r, "issueId"))
	if !ok {
		return
	}
	rows, err := h.Queries.ListTaskPendingInputsForIssue(r.Context(), db.ListTaskPendingInputsForIssueParams{WorkspaceID: issue.WorkspaceID, IssueID: issue.ID})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list pending inputs")
		return
	}
	responses := make([]taskPendingInputResponse, 0, len(rows))
	for _, row := range rows {
		response, err := taskPendingInputToResponse(row.TaskPendingInput, row.CurrentState)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to encode pending inputs")
			return
		}
		responses = append(responses, response)
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": responses})
}

func (h *Handler) AnswerTaskPendingInput(w http.ResponseWriter, r *http.Request) {
	issue, ok := h.loadIssueForUser(w, r, chi.URLParam(r, "issueId"))
	if !ok {
		return
	}
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	actorType, actorID := h.resolveActor(r, userID, uuidToString(issue.WorkspaceID))
	if actorType != "member" || actorID != userID {
		writeError(w, http.StatusForbidden, "human member required")
		return
	}
	pendingID, ok := parseUUIDOrBadRequest(w, chi.URLParam(r, "pendingId"), "pending_input_id")
	if !ok {
		return
	}
	var request answerTaskPendingInputRequest
	if !decodePendingInputJSON(w, r, &request) {
		return
	}
	idempotencyID, ok := parseUUIDOrBadRequest(w, request.IdempotencyKey, "idempotency_key")
	if !ok {
		return
	}
	preview, err := h.Queries.GetTaskPendingInput(r.Context(), db.GetTaskPendingInputParams{ID: pendingID, WorkspaceID: issue.WorkspaceID})
	if err != nil || preview.IssueID != issue.ID {
		writeError(w, http.StatusNotFound, "pending input not found")
		return
	}
	agent, err := h.Queries.GetAgentInWorkspace(r.Context(), db.GetAgentInWorkspaceParams{ID: preview.AgentID, WorkspaceID: issue.WorkspaceID})
	if err != nil || !h.canInvokeAgent(r.Context(), agent, "member", userID, userID, uuidToString(issue.WorkspaceID)) {
		writeError(w, http.StatusForbidden, "agent invocation not allowed")
		return
	}

	tx, err := h.TxStarter.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to begin pending input answer")
		return
	}
	defer tx.Rollback(r.Context())
	qtx := h.Queries.WithTx(tx)
	if _, err := qtx.LockTaskRunEvidenceWorkspace(r.Context(), issue.WorkspaceID); err != nil {
		writeError(w, http.StatusConflict, "issue is no longer available")
		return
	}
	if _, err := qtx.LockTaskPendingInputRuntime(r.Context(), db.LockTaskPendingInputRuntimeParams{
		RuntimeID: preview.RuntimeID, WorkspaceID: issue.WorkspaceID,
	}); err != nil {
		writeError(w, http.StatusConflict, "pending input is no longer active")
		return
	}
	task, err := qtx.LockTaskRunEvidenceTask(r.Context(), db.LockTaskRunEvidenceTaskParams{
		TaskID: preview.TaskID, RuntimeID: preview.RuntimeID,
	})
	if err != nil {
		writeError(w, http.StatusConflict, "pending input is no longer active")
		return
	}
	pending, err := qtx.LockTaskPendingInput(r.Context(), db.LockTaskPendingInputParams{ID: pendingID, WorkspaceID: issue.WorkspaceID})
	if err != nil || pending.IssueID != issue.ID || pending.TaskID != task.ID || pending.RuntimeID != preview.RuntimeID {
		writeError(w, http.StatusNotFound, "pending input not found")
		return
	}
	lockedIssue, err := qtx.LockIssueForDescriptionUpdate(r.Context(), db.LockIssueForDescriptionUpdateParams{ID: issue.ID, WorkspaceID: issue.WorkspaceID})
	if err != nil {
		writeError(w, http.StatusConflict, "issue is no longer available")
		return
	}
	pending, state, err := h.persistPendingInputClosedState(r.Context(), qtx, task, pending, pending.RuntimeID, pending.ClaimGeneration)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update pending input")
		return
	}
	var questions []taskPendingInputQuestion
	if err := json.Unmarshal(pending.Questions, &questions); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to decode pending input")
		return
	}
	normalizedAnswers, canonicalAnswers, err := validatePendingInputAnswers(questions, request.Answers)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if state == "answered" {
		if pending.IdempotencyKey == idempotencyID && pendingInputAnswersEqual(pending.Answers, normalizedAnswers) {
			if err := tx.Commit(r.Context()); err != nil {
				writeError(w, http.StatusInternalServerError, "failed to finish pending input answer")
				return
			}
			response, _ := taskPendingInputToResponse(pending, "")
			writeJSON(w, http.StatusOK, response)
			return
		}
		writeError(w, http.StatusConflict, "pending input already has a different answer")
		return
	}
	if state != "open" {
		if err := tx.Commit(r.Context()); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to close pending input")
			return
		}
		writeError(w, http.StatusConflict, "pending input is no longer active")
		return
	}
	memberID := parseUUID(userID)
	createdComment, err := qtx.CreateComment(r.Context(), db.CreateCommentParams{
		ID: dbid.NewV7(), IssueID: issue.ID, WorkspaceID: issue.WorkspaceID,
		AuthorType: "member", AuthorID: memberID, Content: renderPendingInputAnswerComment(questions, normalizedAnswers),
		Type: "comment", ParentID: pending.QuestionCommentID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create pending input answer")
		return
	}
	answered, err := qtx.AnswerTaskPendingInput(r.Context(), db.AnswerTaskPendingInputParams{
		ID: pending.ID, WorkspaceID: pending.WorkspaceID, Answers: canonicalAnswers,
		AnswerCommentID: createdComment.ID, AnsweredBy: memberID, IdempotencyKey: idempotencyID,
		AnswerIssueRevision: pgtype.Int8{Int64: createdComment.IssueRevision, Valid: true},
	})
	if err != nil {
		writeError(w, http.StatusConflict, "pending input is no longer active")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to commit pending input answer")
		return
	}
	comment := createdComment.Comment()
	h.publish(protocol.EventCommentCreated, uuidToString(issue.WorkspaceID), "member", userID, map[string]any{
		"comment": commentToResponse(comment, nil, nil), "issue_title": lockedIssue.Title,
		"issue_assignee_type": textToPtr(lockedIssue.AssigneeType), "issue_assignee_id": uuidToPtr(lockedIssue.AssigneeID),
		"issue_status": lockedIssue.Status, "issue_revision": createdComment.IssueRevision,
	})
	response, err := taskPendingInputToResponse(answered, "")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to encode pending input")
		return
	}
	writeJSON(w, http.StatusOK, response)
}
