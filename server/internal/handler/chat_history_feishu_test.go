package handler

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"testing"

	"github.com/multica-ai/multica/server/internal/integrations/lark"
	"github.com/multica-ai/multica/server/internal/testutil"
)

type feishuHistoryClient struct {
	*chatPRDFakeClient
	items  []lark.LarkMessage
	calls  int
	params lark.ListMessagesParams
}

func (f *feishuHistoryClient) ListChatMessagesPage(_ context.Context, _ lark.InstallationCredentials, params lark.ListMessagesParams, _ string) ([]lark.LarkMessage, string, error) {
	f.calls++
	f.params = params
	return f.items, "", nil
}

func TestGetChatThread_FeishuBoundTopic(t *testing.T) {
	f := newChatPRDFixture(t)
	client := &feishuHistoryClient{chatPRDFakeClient: f.client, items: []lark.LarkMessage{f.root}}
	f.h.LarkAPIClient, f.h.SlackHistory = client, nil
	var response ChatChannelHistoryResponse
	testutil.Call(t, f.h.GetChatThread, f.request(http.MethodGet, "/api/chat/thread", nil)).Want(http.StatusOK).JSON(&response)
	if response.ChannelType != "feishu" || response.ThreadID != f.root.ThreadID || len(response.Messages) != 1 || response.Messages[0].ID != f.root.MessageID || response.Messages[0].AuthorID != f.root.SenderID {
		t.Fatalf("source topic messages/IDs unavailable: %+v", response)
	}
	if string(client.params.ChatID) != f.root.ChatID || client.params.ThreadID != f.root.ThreadID {
		t.Fatalf("history read left the bound topic: %+v", client.params)
	}
	// Explicit current-topic reads work too; a sibling in the same room must
	// be denied before its messages can reach the platform transport.
	testutil.Call(t, f.h.GetChatThread, f.request(http.MethodGet, "/api/chat/thread?id="+url.QueryEscape(f.root.ThreadID), nil)).Want(http.StatusOK)
	before := client.calls
	testutil.Call(t, f.h.GetChatThread, f.request(http.MethodGet, "/api/chat/thread?id=omt_unrelated", nil)).Want(http.StatusForbidden)
	if client.calls != before {
		t.Fatal("unauthorized topic reached Feishu")
	}
}

func TestGetChatThread_FeishuRequiresImmutableDelivery(t *testing.T) {
	f := newChatPRDFixture(t)
	client := &feishuHistoryClient{chatPRDFakeClient: f.client, items: []lark.LarkMessage{f.root}}
	f.h.LarkAPIClient, f.h.SlackHistory = client, nil
	dbfx.Exec(t, `DELETE FROM channel_task_delivery WHERE task_id=$1`, f.taskID)
	testutil.Call(t, f.h.GetChatThread, f.request(http.MethodGet, "/api/chat/thread", nil)).Want(http.StatusForbidden)
	if client.calls != 0 {
		t.Fatal("missing delivery authority reached Feishu")
	}
}

func TestGetChatThread_FeishuContextIsolation(t *testing.T) {
	f := newChatPRDFixture(t)
	start := f.root
	start.MessageID = "om_fresh_start"
	ts, err := strconv.ParseInt(start.CreateTime, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	start.CreateTime = strconv.FormatInt(ts+1000, 10)
	current := start
	current.MessageID, current.CreateTime = "om_current", strconv.FormatInt(ts+2000, 10)
	staleReply := current
	staleReply.MessageID, staleReply.SenderID, staleReply.SenderType = "om_stale_reply", "ou_fixture_bot", "app"
	f.client.messages[start.MessageID] = start
	client := &feishuHistoryClient{chatPRDFakeClient: f.client, items: []lark.LarkMessage{staleReply, current, start, f.root}}
	f.h.LarkAPIClient, f.h.SlackHistory = client, nil
	dbfx.Exec(t, `UPDATE channel_chat_session_binding SET context_revision=2, history_start_message_id=$2 WHERE id=$1`, f.bindingID, start.MessageID)
	dbfx.Exec(t, `UPDATE agent_task_queue SET channel_context_revision=2 WHERE id=$1`, f.taskID)
	dbfx.InsertNoID(t, "channel_chat_context_generation", testutil.Cols{
		"chat_session_id": f.sessionID, "revision": 2, "history_start_message_id": start.MessageID,
	}, "chat_session_id=$1 AND revision=2", f.sessionID)
	var response ChatChannelHistoryResponse
	testutil.Call(t, f.h.GetChatThread, f.request(http.MethodGet, "/api/chat/thread", nil)).Want(http.StatusOK).JSON(&response)
	if len(response.Messages) != 2 || response.Messages[0].ID != start.MessageID || response.Messages[1].ID != current.MessageID {
		t.Fatalf("old context or delayed unproven bot reply leaked: %+v", response)
	}
	dbfx.Exec(t, `UPDATE channel_chat_session_binding SET context_revision=3 WHERE id=$1`, f.bindingID)
	before := client.calls
	testutil.Call(t, f.h.GetChatThread, f.request(http.MethodGet, "/api/chat/thread", nil)).Want(http.StatusConflict)
	if client.calls != before {
		t.Fatal("older task context reached Feishu")
	}
}
