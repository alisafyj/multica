package agent

import (
	"encoding/json"
	"log/slog"
	"testing"
)

func TestACPFileEditPermissionTreatsReplacementAsData(t *testing.T) {
	for _, tc := range []struct {
		name        string
		path        string
		kind        string
		tool        string
		previewPath string
		want        string
	}{
		{name: "report text contains slash and example path", path: "./reply.md", kind: "edit", tool: "write_file", previewPath: "./reply.md", want: "allow_once"},
		{name: "outside destination remains denied", path: "/etc/report.md", kind: "edit", tool: "write_file", previewPath: "/etc/report.md", want: "deny"},
		{name: "outside preview remains denied", path: "./reply.md", kind: "edit", tool: "write_file", previewPath: "/etc/report.md", want: "deny"},
		{name: "execute payload is not literal data", path: "./reply.md", kind: "execute", tool: "terminal", previewPath: "./reply.md", want: "deny"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := "累计输入 / 输出 Token：16200 / 550。示例路径 /reports/daily 并非本次文件操作。"
			params := map[string]any{
				"sessionId": "ses_1",
				"options":   []map[string]string{{"optionId": "allow_once", "kind": "allow_once"}, {"optionId": "deny", "kind": "reject_once"}},
				"toolCall": map[string]any{
					"toolCallId": "edit-1", "title": "Approve edit: " + tc.path, "kind": tc.kind,
					"rawInput": map[string]any{"tool": tc.tool, "arguments": map[string]any{"path": tc.path, "content": body}},
					"content":  []map[string]any{{"type": "diff", "path": tc.previewPath, "oldText": nil, "newText": body}},
				},
			}
			w := &bufferWriter{}
			c := &hermesClient{cfg: Config{Logger: slog.Default(), WorkDir: "/tmp/repo"}, stdin: w, pending: make(map[int]*pendingRPC)}
			request, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "session/request_permission", "params": params})
			if err != nil {
				t.Fatal(err)
			}
			c.handleLine(string(request))
			var response struct {
				Result struct {
					Outcome struct {
						OptionID string `json:"optionId"`
					} `json:"outcome"`
				} `json:"result"`
			}
			if err := json.Unmarshal([]byte(w.String()), &response); err != nil {
				t.Fatal(err)
			}
			if response.Result.Outcome.OptionID != tc.want {
				t.Fatalf("permission = %q, want %q", response.Result.Outcome.OptionID, tc.want)
			}
		})
	}
}
