package lark

import (
	"context"
	"testing"

	"github.com/multica-ai/multica/server/internal/integrations/channel"
)

type historyPageClient struct {
	APIClient
	pages      map[string][]LarkMessage
	next       map[string]string
	boundaries map[string]LarkMessage
}

func (f *historyPageClient) ListChatMessagesPage(_ context.Context, _ InstallationCredentials, _ ListMessagesParams, cursor string) ([]LarkMessage, string, error) {
	return f.pages[cursor], f.next[cursor], nil
}

func (f *historyPageClient) GetMessage(_ context.Context, _ InstallationCredentials, id string) ([]LarkMessage, error) {
	return []LarkMessage{f.boundaries[id]}, nil
}

func TestReadThreadHistorySourceIDsAndPaging(t *testing.T) {
	root := LarkMessage{MessageID: "om_root", ChatID: "oc_chat", ThreadID: "omt_topic", CreateTime: "1000", SenderID: "ou_human", SenderType: "user", MessageType: "text", Content: `{"text":"request"}`}
	reply := root
	reply.MessageID, reply.CreateTime, reply.SenderID, reply.SenderType = "om_reply", "2000", "cli_app", "app"
	reply.Content = `{"text":"draft"}`
	client := &historyPageClient{pages: map[string][]LarkMessage{"": {reply}, "older": {root}}, next: map[string]string{"": "older"}}
	first, err := ReadThreadHistory(context.Background(), client, InstallationCredentials{AppID: "cli_app"}, "oc_chat", "omt_topic", "ou_bot", channel.HistoryOptions{Limit: 1})
	if err != nil || len(first.Messages) != 1 || first.Messages[0].ID != "om_reply" || first.Messages[0].Role != channel.HistoryRoleAssistant || first.NextCursor != "older" {
		t.Fatalf("first page = %+v, %v", first, err)
	}
	second, err := ReadThreadHistory(context.Background(), client, InstallationCredentials{AppID: "cli_app"}, "oc_chat", "omt_topic", "ou_bot", channel.HistoryOptions{Limit: 1, Before: first.NextCursor})
	if err != nil || len(second.Messages) != 1 || second.Messages[0].ID != "om_root" || second.Messages[0].Text != "request" || second.Messages[0].AuthorID != "ou_human" || second.NextCursor != "" {
		t.Fatalf("older page = %+v, %v", second, err)
	}
}

func TestReadThreadHistoryRejectsForeignTopicMessages(t *testing.T) {
	message := LarkMessage{MessageID: "om_foreign", ChatID: "oc_chat", ThreadID: "omt_sibling", CreateTime: "1000"}
	client := &historyPageClient{pages: map[string][]LarkMessage{"": {message}}}
	page, err := ReadThreadHistory(context.Background(), client, InstallationCredentials{}, "oc_chat", "omt_topic", "", channel.HistoryOptions{})
	if err == nil || len(page.Messages) != 0 {
		t.Fatalf("foreign topic leaked: %+v, %v", page, err)
	}
}

func TestReadThreadHistoryHonorsContextWindow(t *testing.T) {
	message := func(id, ts string) LarkMessage {
		return LarkMessage{MessageID: id, ChatID: "oc_chat", ThreadID: "omt_topic", CreateTime: ts, SenderID: "ou_human", MessageType: "text", Content: `{"text":"context"}`}
	}
	start, end := message("om_start", "2000"), message("om_end", "4000")
	client := &historyPageClient{
		pages:      map[string][]LarkMessage{"": {end, message("om_current", "3000"), start, message("om_ambiguous", "2000"), message("om_old", "1000")}},
		boundaries: map[string]LarkMessage{start.MessageID: start, end.MessageID: end},
	}
	opts := channel.HistoryOptions{After: start.MessageID, Until: end.MessageID}
	page, err := ReadThreadHistory(context.Background(), client, InstallationCredentials{}, "oc_chat", "omt_topic", "", opts)
	if err != nil || len(page.Messages) != 2 || page.Messages[0].ID != start.MessageID || page.Messages[1].ID != "om_current" {
		t.Fatalf("context window = %+v, %v", page, err)
	}
	opts.BoundaryPending = true
	page, err = ReadThreadHistory(context.Background(), client, InstallationCredentials{}, "oc_chat", "omt_topic", "", opts)
	if err != nil || len(page.Messages) != 0 || page.NextCursor != "" {
		t.Fatalf("pending boundary leaked history: %+v, %v", page, err)
	}
	client.boundaries[start.MessageID] = message(start.MessageID, "not-a-time")
	opts.BoundaryPending = false
	if _, err := ReadThreadHistory(context.Background(), client, InstallationCredentials{}, "oc_chat", "omt_topic", "", opts); err == nil {
		t.Fatal("invalid context boundary must fail closed")
	}
}
