package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/integrations/testcapability"
	"github.com/multica-ai/multica/server/internal/logger"
	obsmetrics "github.com/multica-ai/multica/server/internal/metrics"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

type DispatchTestRunRequest struct {
	AgentID string `json:"agent_id"`
	Prompt  string `json:"prompt"`
}

// runRequiredCapabilities collects what the round needs from the frozen case
// snapshots rather than from the live cases: the run executes what it froze, so
// it must be bound to the devices that snapshot asked for.
func runRequiredCapabilities(runCases []db.TestRunCase) []TestCapabilityRequirement {
	seen := make(map[string]struct{})
	out := make([]TestCapabilityRequirement, 0)
	for _, rc := range runCases {
		var snapshot struct {
			RequiredCapabilities []TestCapabilityRequirement `json:"required_capabilities"`
		}
		if err := json.Unmarshal(rc.CaseSnapshot, &snapshot); err != nil {
			continue
		}
		for _, req := range snapshot.RequiredCapabilities {
			if strings.TrimSpace(req.Kind) == "" {
				continue
			}
			// Dedupe on kind plus its constraints: two cases needing the same
			// kind of device share one binding.
			key := req.Kind + "\x00" + fmt.Sprintf("%v", req.Match)
			if _, dup := seen[key]; dup {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, req)
		}
	}
	return out
}

// deviceKindRequired names the first required phone kind, if the round has
// one. Those kinds may only be bound on a machine designated as a test host
// (agent_runtime.test_host_enabled): a shared laptop that happens to run a
// device hub must not become a lab by accident.
func deviceKindRequired(requirements []TestCapabilityRequirement) (string, bool) {
	for _, req := range requirements {
		if req.Optional {
			continue
		}
		switch req.Kind {
		case "android_device", "ios_device":
			return req.Kind, true
		}
	}
	return "", false
}

// DispatchTestRun hands a round to an agent. Capability resolution happens
// here, before anything is queued: a run that has no device to drive is parked
// as blocked with the missing kind named, because a dispatched run with no
// device only reveals itself as broken minutes later, inside the agent.
func (h *Handler) DispatchTestRun(w http.ResponseWriter, r *http.Request) {
	run, wsUUID, ok := h.loadTestRunForUser(w, r)
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
	var req DispatchTestRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if run.Status == "running" {
		writeError(w, http.StatusConflict, "this run is already running")
		return
	}
	if run.AgentTaskID.Valid {
		writeError(w, http.StatusConflict, "this run has already been dispatched")
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

	runCases, err := h.Queries.ListTestRunCases(r.Context(), db.ListTestRunCasesParams{
		RunID:       run.ID,
		WorkspaceID: wsUUID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load the run's cases")
		return
	}
	if len(runCases) == 0 {
		writeError(w, http.StatusBadRequest, "this run has no cases to execute")
		return
	}

	// The overlay is mounted on the agent's runtime, so that is the only
	// daemon whose capabilities can serve this run.
	agentRuntime, err := h.runtimeLookup(obsmetrics.RuntimeLookupSourceTestCapability).Get(r.Context(), agent.RuntimeID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "the agent's runtime is gone; bind it to a running daemon first")
		return
	}

	requirements := runRequiredCapabilities(runCases)
	if kind, needsPhone := deviceKindRequired(requirements); needsPhone && !agentRuntime.TestHostEnabled {
		h.parkTestRunBlocked(w, r, run, wsUUID, workspaceID, userID, kind,
			"the agent's runtime is not a test host; turn on \"Test host\" on its runtime page before dispatching device cases")
		return
	}
	binding, missingKind, resolved := h.resolveRunCapabilities(r.Context(), wsUUID, requirements, effectiveDaemonIDForRuntime(agentRuntime))
	if !resolved {
		// Explicit failure, not a silent downgrade: parking the run tells the
		// user which capability is missing instead of burning an agent run that
		// discovers it has no phone.
		h.parkTestRunBlocked(w, r, run, wsUUID, workspaceID, userID, missingKind,
			"no runtime can provide the required capability: "+missingKind)
		return
	}

	bindingJSON := marshalJSONColumn(binding, "{}")

	// One agent task per case (TS-021): cases run independently, in parallel
	// across phones, and each records its own result. The overlay (browser
	// MCP, device connector) is computed per task with the resolved binding on
	// the context — the queue insert takes it as an argument and nothing
	// recomputes it at claim time. A parallelism cap (M4) queues only the
	// first N now; the per-case completion hooks release the rest.
	limit := len(runCases)
	if run.Parallelism.Valid && run.Parallelism.Int32 > 0 && int(run.Parallelism.Int32) < limit {
		limit = int(run.Parallelism.Int32)
	}
	input := testRunCaseTaskInput{
		run:          run,
		agent:        agent,
		userUUID:     userUUID,
		requesterID:  userID,
		workspaceID:  workspaceID,
		prompt:       strings.TrimSpace(req.Prompt),
		binding:      binding,
		requirements: requirements,
	}
	var firstTaskID pgtype.UUID
	created := 0
	for _, rc := range runCases[:limit] {
		agentTask, err := h.createTestRunCaseTask(r.Context(), h.Queries, input, rc)
		if err != nil {
			slog.Error("dispatch test run failed", append(logger.RequestAttrs(r), "error", err, "created", created, "of", limit)...)
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to dispatch case %d of %d", created+1, limit))
			return
		}
		if !firstTaskID.Valid {
			firstTaskID = agentTask.ID
		}
		created++
	}

	// The agent becomes the run's executor here. UpdateTestRunCaseResult reads
	// run.ExecutorID to attribute an agent-written result, so leaving the
	// creating member on those columns would file the agent's results under a
	// human who never ran them. agent_task_id keeps the first case task so
	// older readers still see the round as dispatched, and so the follow-up
	// dispatch of a capped round can read the requester and prompt back.
	updated, err := h.Queries.UpdateTestRun(r.Context(), db.UpdateTestRunParams{
		ID:                run.ID,
		WorkspaceID:       wsUUID,
		AgentTaskID:       firstTaskID,
		ExecutorType:      pgtype.Text{String: "agent", Valid: true},
		ExecutorID:        agent.ID,
		CapabilityBinding: bindingJSON,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to record the dispatch")
		return
	}
	resp := testRunToResponse(updated)
	h.publish(protocol.EventTestRunUpdated, workspaceID, "member", userID, map[string]any{"test_run": resp})
	writeJSON(w, http.StatusCreated, map[string]any{
		"test_run":      resp,
		"agent_task_id": uuidToString(firstTaskID),
		"case_tasks":    created,
		"cases":         len(runCases),
	})
}

// parkTestRunBlocked records why a round could not be dispatched and answers
// 409 with the missing kind, so the run page can say what to fix.
func (h *Handler) parkTestRunBlocked(w http.ResponseWriter, r *http.Request, run db.TestRun, wsUUID pgtype.UUID, workspaceID, userID, missingKind, reason string) {
	blocked, err := h.Queries.UpdateTestRun(r.Context(), db.UpdateTestRunParams{
		ID:          run.ID,
		WorkspaceID: wsUUID,
		Status:      pgtype.Text{String: "blocked", Valid: true},
		Error:       pgtype.Text{String: reason, Valid: true},
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to record the blocked run")
		return
	}
	resp := testRunToResponse(blocked)
	h.publish(protocol.EventTestRunUpdated, workspaceID, "member", userID, map[string]any{"test_run": resp})
	writeJSON(w, http.StatusConflict, map[string]any{
		"test_run":     resp,
		"missing_kind": missingKind,
		"message":      reason,
	})
}

// testRunCaseTaskInput is everything a per-case task needs besides the case
// itself; the same values serve the initial dispatch and the follow-up
// dispatch of a capped round.
type testRunCaseTaskInput struct {
	run          db.TestRun
	agent        db.Agent
	userUUID     pgtype.UUID
	requesterID  string
	workspaceID  string
	prompt       string
	binding      TestRunCapabilityBinding
	requirements []TestCapabilityRequirement
}

// createTestRunCaseTask queues the agent task for one case and binds the case
// to it. The device connector overlay carries the case's TC key as the lease
// label and the run / case / runtime ids as lease tags, which is how the
// daemon later relays the case's live frame.
func (h *Handler) createTestRunCaseTask(ctx context.Context, q *db.Queries, in testRunCaseTaskInput, rc db.TestRunCase) (db.AgentTaskQueue, error) {
	label := runCaseLabel(rc)
	contextPayload := service.TestRunContext{
		Type:              service.TestRunContextType,
		Prompt:            in.prompt,
		RequesterID:       in.requesterID,
		WorkspaceID:       in.workspaceID,
		ProjectID:         uuidToString(in.run.ProjectID),
		AgentID:           uuidToString(in.agent.ID),
		RunID:             uuidToString(in.run.ID),
		CapabilityBinding: json.RawMessage(marshalJSONColumn(in.binding, "{}")),
		RunCaseID:         uuidToString(rc.ID),
		CaseKey:           label,
		CaseSnapshot:      json.RawMessage(rc.CaseSnapshot),
	}
	contextJSON, err := json.Marshal(contextPayload)
	if err != nil {
		return db.AgentTaskQueue{}, fmt.Errorf("build the agent task context: %w", err)
	}
	tags := map[string]string{
		"run_case_id":  uuidToString(rc.ID),
		"run_id":       uuidToString(in.run.ID),
		"runtime_id":   uuidToString(in.agent.RuntimeID),
		"workspace_id": in.workspaceID,
	}
	octx := testcapability.WithResolvedCapabilities(ctx, capabilityEntriesForOverlay(in.binding, in.requirements, label, tags))
	overlay, connectedApps := h.TaskService.BuildRuntimeMCPOverlayForMerge(octx, in.userUUID, in.agent)
	agentTask, err := q.CreateQuickCreateTask(octx, db.CreateQuickCreateTaskParams{
		ID:                   dbid.NewV7(),
		AgentID:              in.agent.ID,
		RuntimeID:            in.agent.RuntimeID,
		Priority:             0,
		Context:              contextJSON,
		OriginatorUserID:     in.userUUID,
		AccountableUserID:    in.userUUID,
		OriginatorSource:     pgtype.Text{String: "direct_human", Valid: true},
		RuntimeMcpOverlay:    overlay,
		RuntimeConnectedApps: connectedApps,
	})
	if err != nil {
		return db.AgentTaskQueue{}, err
	}
	if _, err := q.UpdateTestRunCaseAgentTask(ctx, db.UpdateTestRunCaseAgentTaskParams{
		ID:          rc.ID,
		WorkspaceID: in.run.WorkspaceID,
		AgentTaskID: agentTask.ID,
	}); err != nil {
		return db.AgentTaskQueue{}, fmt.Errorf("record the case's agent task: %w", err)
	}
	return agentTask, nil
}

// dispatchNextTestRunCases releases the next cases of a capped round once a
// case settles ("finish one, release one"). Uncapped rounds queued everything
// at dispatch and return at once. The requester and prompt come back from the
// first case task's context; the binding is the frozen one on the run.
func (h *Handler) dispatchNextTestRunCases(ctx context.Context, q *db.Queries, run db.TestRun) error {
	if !run.Parallelism.Valid || run.Parallelism.Int32 <= 0 {
		return nil
	}
	if run.Status == "completed" || run.Status == "aborted" || run.Status == "blocked" {
		return nil
	}
	if run.ExecutorType != "agent" || !run.AgentTaskID.Valid {
		return nil
	}
	active, err := q.CountDispatchedActiveTestRunCases(ctx, db.CountDispatchedActiveTestRunCasesParams{RunID: run.ID, WorkspaceID: run.WorkspaceID})
	if err != nil {
		return err
	}
	free := int(run.Parallelism.Int32) - int(active)
	if free <= 0 {
		return nil
	}
	waiting, err := q.ListUndispatchedTestRunCases(ctx, db.ListUndispatchedTestRunCasesParams{
		RunID:       run.ID,
		WorkspaceID: run.WorkspaceID,
		Limit:       int32(free),
	})
	if err != nil {
		return err
	}
	if len(waiting) == 0 {
		return nil
	}
	firstTask, err := q.GetAgentTask(ctx, run.AgentTaskID)
	if err != nil {
		return fmt.Errorf("load the round's first case task: %w", err)
	}
	runCtx, ok := testRunContextForTask(firstTask)
	if !ok {
		return fmt.Errorf("the round's first task carries no test run context")
	}
	userUUID, err := util.ParseUUID(runCtx.RequesterID)
	if err != nil {
		return fmt.Errorf("requester id on the first case task: %w", err)
	}
	agent, err := q.GetAgentInWorkspace(ctx, db.GetAgentInWorkspaceParams{ID: run.ExecutorID, WorkspaceID: run.WorkspaceID})
	if err != nil {
		return fmt.Errorf("load the executing agent: %w", err)
	}
	var binding TestRunCapabilityBinding
	if len(run.CapabilityBinding) > 0 {
		if err := json.Unmarshal(run.CapabilityBinding, &binding); err != nil {
			return fmt.Errorf("decode the frozen capability binding: %w", err)
		}
	}
	if binding.Resolved == nil {
		binding.Resolved = map[string]string{}
	}
	allCases, err := q.ListTestRunCases(ctx, db.ListTestRunCasesParams{RunID: run.ID, WorkspaceID: run.WorkspaceID})
	if err != nil {
		return err
	}
	input := testRunCaseTaskInput{
		run:          run,
		agent:        agent,
		userUUID:     userUUID,
		requesterID:  runCtx.RequesterID,
		workspaceID:  runCtx.WorkspaceID,
		prompt:       runCtx.Prompt,
		binding:      binding,
		requirements: runRequiredCapabilities(allCases),
	}
	for _, rc := range waiting {
		if _, err := h.createTestRunCaseTask(ctx, q, input, rc); err != nil {
			return err
		}
	}
	return nil
}

// runCaseLabel is what a case is called on the phone owner's approval prompt
// and in the hub's audit log: its TC key from the frozen snapshot, else the
// run case id.
func runCaseLabel(rc db.TestRunCase) string {
	var snapshot struct {
		Key string `json:"key"`
	}
	if err := json.Unmarshal(rc.CaseSnapshot, &snapshot); err == nil && strings.TrimSpace(snapshot.Key) != "" {
		return strings.TrimSpace(snapshot.Key)
	}
	return uuidToString(rc.ID)
}

// capabilityEntriesForOverlay turns the frozen binding into the shape the MCP
// overlay provider consumes.
func capabilityEntriesForOverlay(
	binding TestRunCapabilityBinding,
	requirements []TestCapabilityRequirement,
	label string,
	tags map[string]string,
) []testcapability.TestRunCapabilityEntry {
	entries := make([]testcapability.TestRunCapabilityEntry, 0, len(binding.Resolved))
	for _, req := range requirements {
		key, bound := binding.Resolved[req.Kind]
		if !bound {
			continue
		}
		entries = append(entries, testcapability.TestRunCapabilityEntry{
			Kind:   req.Kind,
			Key:    key,
			Target: binding.Targets[req.Kind],
			Match:  req.Match,
			Label:  label,
			Tags:   tags,
		})
	}
	return entries
}
