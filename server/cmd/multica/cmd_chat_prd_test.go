package main

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestChatPRDDraftPreservesFileContent(t *testing.T) {
	t.Chdir(t.TempDir())
	content := map[string]any{
		"title": "PRD-订单导出",
		"sections": []any{map[string]any{
			"heading": "需求背景", "body": "保留 `code`、$(not-a-command)、\"引号\"、C:\\exports 和换行\n【待确认】权限。",
		}},
	}
	raw, err := json.MarshalIndent(content, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("prd-draft.json", raw, 0600); err != nil {
		t.Fatal(err)
	}
	chatSessionCommandTestServer(t, func(t *testing.T, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/chat/prd/draft" {
			t.Fatalf("unexpected draft request: %s %s", r.Method, r.URL.Path)
		}
		var body struct {
			SourceMessageID string         `json:"source_message_id"`
			Content         map[string]any `json:"content"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.SourceMessageID != "om_real_topic_root" || !reflect.DeepEqual(body.Content, content) {
			t.Fatalf("file-first draft changed source or Unicode/shell-like content: %+v", body)
		}
	})
	cmd := newChatSessionTestCmd()
	cmd.Flags().String("source-message", "om_real_topic_root", "")
	cmd.Flags().String("content-file", "./prd-draft.json", "")
	if _, err := captureStdout(t, func() error { return runChatPRDDraft(cmd, nil) }); err != nil {
		t.Fatal(err)
	}
}

func TestChatPRDDraftRejectsInvalidFiles(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []byte
	}{
		{"Markdown wrapper", []byte("```json\n{}\n```")},
		{"multiple values", []byte("{} {}")},
		{"array", []byte("[]")},
		{"null", []byte("null")},
		{"invalid UTF-8", []byte{'{', '"', 'x', '"', ':', '"', 0xff, '"', '}'}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			if err := os.WriteFile("prd-draft.json", tc.body, 0600); err != nil {
				t.Fatal(err)
			}
			chatSessionCommandTestServer(t, func(t *testing.T, _ *http.Request) {
				t.Error("invalid draft reached the API instead of failing file validation")
			})
			cmd := newChatSessionTestCmd()
			cmd.Flags().String("source-message", "om_real_topic_root", "")
			cmd.Flags().String("content-file", "./prd-draft.json", "")
			if err := runChatPRDDraft(cmd, nil); err == nil {
				t.Fatal("invalid JSON draft was accepted")
			}
		})
	}
}

func TestChatPRDDraftRejectsAnotherWorkdirFile(t *testing.T) {
	outside := filepath.Join(t.TempDir(), "prd-draft.json")
	if err := os.WriteFile(outside, []byte(`{"title":"Other task","sections":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(t.TempDir())
	chatSessionCommandTestServer(t, func(t *testing.T, _ *http.Request) {
		t.Error("another task's draft reached the API")
	})
	cmd := newChatSessionTestCmd()
	cmd.Flags().String("source-message", "om_real_topic_root", "")
	cmd.Flags().String("content-file", outside, "")
	if err := runChatPRDDraft(cmd, nil); err == nil {
		t.Fatal("draft read another task's file outside its working directory")
	}
}
