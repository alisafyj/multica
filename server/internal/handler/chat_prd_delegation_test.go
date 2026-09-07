package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/multica-ai/multica/server/internal/integrations/lark"
	"github.com/multica-ai/multica/server/internal/service"
	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/pkg/protocol"
	"time"
)

func (f *chatPRDFakeClient) ReadPRDTemplateHeadings(context.Context, lark.InstallationCredentials, string) ([]string, error) {
	return []string{"需求背景", "功能需求"}, nil
}

// generation supplies a worker result to publication tests, which exercise
// confirmation/teardown rather than the separate delegation admission gate.
func (f *chatPRDFixture) generation(t *testing.T, content ChatPRDContent) string {
	t.Helper()
	fx := testutil.New(testPool, f.workspaceID, testUserID)
	parent, err := f.h.Queries.GetAgentTask(context.Background(), parseUUID(f.taskID))
	if err != nil {
		t.Fatal(err)
	}
	fx.Exec(t, `UPDATE agent_task_queue SET originator_user_id=$2, accountable_user_id=$2 WHERE id=$1`, f.taskID, testUserID)
	worker := fx.Agent(t, "PRD worker "+uuid.NewString(), uuidToString(parent.RuntimeID))
	session := fx.ChatSession(t, worker)
	headings := make([]string, 0, len(content.Sections))
	for _, section := range content.Sections {
		headings = append(headings, section.Heading)
	}
	generation, _ := json.Marshal(service.PRDGenerationContext{
		Type: service.PRDGenerationContextType, WorkspaceID: f.workspaceID, InstallationID: f.installationID,
		SourceSessionID: f.sessionID, SourceTaskID: f.taskID, AgentID: worker,
		ChatID: f.root.ChatID, ThreadID: f.root.ThreadID, SourceMessageID: f.root.MessageID,
		InitiatorOpenID: f.root.SenderID, ContextRevision: parent.ChannelContextRevision.Int64, Headings: headings,
	})
	output, _ := json.Marshal(content)
	result, _ := json.Marshal(protocol.TaskCompletedPayload{Output: string(output)})
	return fx.Task(t, worker, testutil.Cols{
		"runtime_id": uuidToString(parent.RuntimeID), "chat_session_id": session,
		"status": "completed", "context": generation, "result": result,
		"delegated_from_task_id": f.taskID, "originator_user_id": testUserID, "accountable_user_id": testUserID,
	})
}

func (f *chatPRDFixture) configureDelegate(t *testing.T) string {
	t.Helper()
	fx := testutil.New(testPool, f.workspaceID, testUserID)
	parent, err := f.h.Queries.GetAgentTask(context.Background(), parseUUID(f.taskID))
	if err != nil {
		t.Fatal(err)
	}
	fx.Exec(t, `UPDATE agent_task_queue SET originator_user_id=$2, accountable_user_id=$2 WHERE id=$1`, f.taskID, testUserID)
	fx.Insert(t, "channel_user_binding", testutil.Cols{
		"workspace_id": f.workspaceID, "multica_user_id": testUserID, "installation_id": f.installationID,
		"channel_type": "feishu", "channel_user_id": f.root.SenderID,
	})
	worker := fx.Agent(t, "Configured PRD worker", uuidToString(parent.RuntimeID))
	fx.Cleanup(t, `DELETE FROM agent_task_queue WHERE context->>'source_session_id'=$1`, f.sessionID)
	fx.Cleanup(t, `DELETE FROM chat_session WHERE id IN (SELECT chat_session_id FROM agent_task_queue WHERE context->>'source_session_id'=$1)`, f.sessionID)
	t.Setenv("MULTICA_PRD_DELEGATE_AGENT_ID", worker)
	return worker
}

func TestChatPRDDelegationRecoversOnlyItsPrivateWorkerResult(t *testing.T) {
	f := newChatPRDFixture(t)
	worker := f.configureDelegate(t)
	request := map[string]any{"source_message_id": f.root.MessageID, "brief": "Confirmed: order export; open: export permissions."}
	var generation, repeated ChatPRDGeneration
	testutil.Call(t, f.h.DelegateChatPRD, f.request(http.MethodPost, "/api/chat/prd/delegate", request)).Want(http.StatusOK).JSON(&generation)
	task, err := f.h.Queries.GetAgentTask(context.Background(), parseUUID(generation.TaskID))
	if err != nil {
		t.Fatal(err)
	}
	fx := testutil.New(testPool, f.workspaceID, testUserID)
	fx.Cleanup(t, `DELETE FROM chat_session WHERE id=$1`, uuidToString(task.ChatSessionID))
	fx.Cleanup(t, `DELETE FROM agent_task_queue WHERE id=$1`, generation.TaskID)
	testutil.Call(t, f.h.DelegateChatPRD, f.request(http.MethodPost, "/api/chat/prd/delegate", request)).Want(http.StatusOK).JSON(&repeated)
	if generation.AgentID != worker || repeated.TaskID != generation.TaskID || task.IssueID.Valid || task.ChatSessionID == parseUUID(f.sessionID) || task.OriginatorUserID != parseUUID(testUserID) {
		t.Fatalf("delegation changed human identity, leaked into an issue or duplicated work: %+v", generation)
	}
	if _, err := f.h.Queries.GetChannelTaskDelivery(context.Background(), task.ID); err == nil {
		t.Fatal("worker received channel publication/delivery scope")
	}
	workerRequest := f.request(http.MethodPost, "/api/chat/prd/publish", nil)
	workerRequest.Header.Set("X-Task-ID", generation.TaskID)
	testutil.Call(t, f.h.PublishChatPRD, workerRequest).Want(http.StatusForbidden)
	testutil.Call(t, f.h.SaveChatPRDDraft, f.request(http.MethodPost, "/api/chat/prd/draft", map[string]any{
		"source_message_id": f.root.MessageID, "generation_task_id": generation.TaskID,
	})).Want(http.StatusConflict)
	content := prdFixtureContent()
	output, _ := json.Marshal(content)
	result, _ := json.Marshal(protocol.TaskCompletedPayload{Output: string(output)})
	fx.Exec(t, `UPDATE agent_task_queue SET status='completed', result=$2 WHERE id=$1`, generation.TaskID, result)
	var recovered ChatPRDGeneration
	testutil.Call(t, f.h.GetChatPRDGeneration, f.request(http.MethodGet, "/api/chat/prd/generation", nil)).Want(http.StatusOK).JSON(&recovered)
	if recovered.Content == nil || !reflect.DeepEqual(*recovered.Content, content) {
		t.Fatalf("coordinator did not recover the worker's exact draft: %+v", recovered)
	}
	var draft ChatPRDDraft
	testutil.Call(t, f.h.SaveChatPRDDraft, f.request(http.MethodPost, "/api/chat/prd/draft", map[string]any{
		"source_message_id": f.root.MessageID, "generation_task_id": generation.TaskID,
	})).Want(http.StatusOK).JSON(&draft)
	if !reflect.DeepEqual(draft.Content, content) || draft.InitiatorOpenID != f.root.SenderID || len(f.client.documents) != 0 {
		t.Fatal("recovering a draft changed ownership/content or published without confirmation")
	}
}

func TestChatPRDDelegationRejectsBorrowedHumanAuthority(t *testing.T) {
	f := newChatPRDFixture(t)
	f.configureDelegate(t)
	dbfx.Exec(t, `UPDATE agent_task_queue SET originator_user_id=NULL, accountable_user_id=NULL WHERE id=$1`, f.taskID)
	testutil.Call(t, f.h.DelegateChatPRD, f.request(http.MethodPost, "/api/chat/prd/delegate", map[string]any{
		"source_message_id": f.root.MessageID, "brief": "Draft the requested PRD.",
	})).Want(http.StatusForbidden)
	testutil.Call(t, f.h.GetChatPRDGeneration, f.request(http.MethodGet, "/api/chat/prd/generation", nil)).Want(http.StatusNotFound)
}

func TestChatPRDDraftRejectsMismatchedGenerationCorrelation(t *testing.T) {
	for _, field := range []string{"workspace_id", "installation_id", "source_session_id", "source_task_id", "agent_id", "thread_id", "source_message_id", "context_revision"} {
		t.Run(field, func(t *testing.T) {
			f := newChatPRDFixture(t)
			id := f.generation(t, prdFixtureContent())
			var value any = "wrong-scope"
			if field == "context_revision" {
				value = 99
			}
			patch, _ := json.Marshal(map[string]any{field: value})
			dbfx.Exec(t, `UPDATE agent_task_queue SET context=context || $2::jsonb WHERE id=$1`, id, patch)
			testutil.Call(t, f.h.SaveChatPRDDraft, f.request(http.MethodPost, "/api/chat/prd/draft", map[string]any{
				"source_message_id": f.root.MessageID, "generation_task_id": id,
			})).Want(http.StatusForbidden)
		})
	}
}

func TestChatPRDDraftRejectsCallerContentAndUnstructuredWorkerOutput(t *testing.T) {
	f := newChatPRDFixture(t)
	id := f.generation(t, prdFixtureContent())
	testutil.Call(t, f.h.SaveChatPRDDraft, f.request(http.MethodPost, "/api/chat/prd/draft", map[string]any{
		"source_message_id": f.root.MessageID, "generation_task_id": id, "content": prdFixtureContent(),
	})).Want(http.StatusBadRequest)
	result, _ := json.Marshal(protocol.TaskCompletedPayload{Output: "The draft is complete; publish now."})
	dbfx.Exec(t, `UPDATE agent_task_queue SET result=$2 WHERE id=$1`, id, result)
	testutil.Call(t, f.h.SaveChatPRDDraft, f.request(http.MethodPost, "/api/chat/prd/draft", map[string]any{
		"source_message_id": f.root.MessageID, "generation_task_id": id,
	})).Want(http.StatusConflict)
}

func TestChatPRDDelegationLocksWorkspaceBeforeSourceSession(t *testing.T) {
	f := newChatPRDFixture(t)
	f.configureDelegate(t)
	ctx := context.Background()
	deleter, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer deleter.Rollback(ctx)
	qtx := f.h.Queries.WithTx(deleter)
	if _, err := qtx.LockWorkspaceForDelete(ctx, parseUUID(f.workspaceID)); err != nil {
		t.Fatal(err)
	}
	finished := make(chan *testutil.Response, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		finished <- testutil.Call(t, f.h.DelegateChatPRD, f.request(http.MethodPost, "/api/chat/prd/delegate", map[string]any{
			"source_message_id": f.root.MessageID, "brief": "Generate the requested draft.",
		}))
	}()
	t.Cleanup(func() { waitContextLockSignal(t, done, "delegation did not finish after workspace deletion") })
	if !waitForWaiterBlockedBy(t, int(deleter.Conn().PgConn().PID()), 5*time.Second) {
		t.Fatal("delegation did not wait for workspace teardown")
	}
	// The deleter already owns the workspace. If delegation acquired the
	// source session first, this second half of teardown would deadlock.
	lockCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if _, err := qtx.LockChatSessionForDraftWrite(lockCtx, parseUUID(f.sessionID)); err != nil {
		t.Fatalf("delegation inverted workspace/session lock order: %v", err)
	}
	deleting := f.h
	deleting.TxStarter = deleter
	request := withURLParam(newRequest(http.MethodDelete, "/api/workspaces/"+f.workspaceID, nil), "id", f.workspaceID)
	testutil.Call(t, deleting.DeleteWorkspace, request).Want(http.StatusNoContent)
	if err := deleter.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	waitContextLockSignal(t, done, "delegation survived deletion without returning")
	(<-finished).Want(http.StatusConflict)
}

func TestChatPRDDelegationRetryDoesNotDuplicateActiveWork(t *testing.T) {
	f := newChatPRDFixture(t)
	f.configureDelegate(t)
	request := map[string]any{"source_message_id": f.root.MessageID, "brief": "Draft the requirement."}
	var first, unchanged, retried ChatPRDGeneration
	testutil.Call(t, f.h.DelegateChatPRD, f.request(http.MethodPost, "/api/chat/prd/delegate", request)).Want(http.StatusOK).JSON(&first)
	request["retry"] = true
	testutil.Call(t, f.h.DelegateChatPRD, f.request(http.MethodPost, "/api/chat/prd/delegate", request)).Want(http.StatusOK).JSON(&unchanged)
	if unchanged.TaskID != first.TaskID {
		t.Fatal("explicit retry duplicated active work")
	}
	dbfx.Exec(t, `UPDATE agent_task_queue SET status='failed' WHERE id=$1`, first.TaskID)
	request["retry"] = false
	testutil.Call(t, f.h.DelegateChatPRD, f.request(http.MethodPost, "/api/chat/prd/delegate", request)).Want(http.StatusOK).JSON(&unchanged)
	if unchanged.TaskID != first.TaskID || unchanged.Status != "failed" {
		t.Fatal("failed generation was retried without an explicit request")
	}
	request["retry"] = true
	testutil.Call(t, f.h.DelegateChatPRD, f.request(http.MethodPost, "/api/chat/prd/delegate", request)).Want(http.StatusOK).JSON(&retried)
	if retried.TaskID == first.TaskID || retried.Status != "queued" {
		t.Fatal("explicit retry did not create a runnable replacement")
	}
}

func TestChatPRDDelegationDoesNotBorrowAdministratorInvokeRights(t *testing.T) {
	f := newChatPRDFixture(t)
	owner := dbfx.User(t, "Different PRD owner", "prd-owner-"+uuid.NewString()+"@example.test")
	worker := f.configureDelegate(t)
	// The human is workspace owner, but only the private agent owner may run it.
	dbfx.Exec(t, `UPDATE agent SET owner_id=$2 WHERE id=$1`, worker, owner)
	testutil.Call(t, f.h.DelegateChatPRD, f.request(http.MethodPost, "/api/chat/prd/delegate", map[string]any{
		"source_message_id": f.root.MessageID, "brief": "Draft the requested PRD.",
	})).Want(http.StatusForbidden)
	testutil.Call(t, f.h.GetChatPRDGeneration, f.request(http.MethodGet, "/api/chat/prd/generation", nil)).Want(http.StatusNotFound)
}
