package agent

import "testing"

func codexMessagesFrom(t *testing.T, lines ...string) []Message {
	t.Helper()
	c, _, _ := newTestCodexClient(t)
	c.notificationProtocol = "raw"
	var messages []Message
	c.onMessage = func(msg Message) { messages = append(messages, msg) }
	for _, line := range lines {
		c.handleLine(line)
	}
	return messages
}

// Codex reports its thinking as `reasoning` items. Dropping them is what made a
// turn that reasons for minutes look identical to a stalled agent.
func TestCodexReasoningItemBecomesThinking(t *testing.T) {
	t.Parallel()

	messages := codexMessagesFrom(t,
		`{"jsonrpc":"2.0","method":"item/completed","params":{"item":{"type":"reasoning","id":"r-1","text":"  先读取规范，再决定页面结构。  "}}}`)

	if len(messages) != 1 {
		t.Fatalf("expected one message, got %d: %+v", len(messages), messages)
	}
	if messages[0].Type != MessageThinking {
		t.Fatalf("type = %q, want %q", messages[0].Type, MessageThinking)
	}
	if messages[0].Content != "先读取规范，再决定页面结构。" {
		t.Fatalf("content = %q, want the trimmed reasoning text", messages[0].Content)
	}
}

// The protocol carries reasoning as either a flat `text` or a `summary` built
// from parts (item/reasoning/summaryPartAdded), so both shapes must read.
func TestCodexReasoningSummaryShapesAreReadable(t *testing.T) {
	t.Parallel()

	for name, line := range map[string]string{
		"summary string": `{"jsonrpc":"2.0","method":"item/completed","params":{"item":{"type":"reasoning","id":"r","summary":"计划已定"}}}`,
		"summary parts":  `{"jsonrpc":"2.0","method":"item/completed","params":{"item":{"type":"reasoning","id":"r","summary":[{"text":"计划已定"}]}}}`,
	} {
		t.Run(name, func(t *testing.T) {
			messages := codexMessagesFrom(t, line)
			if len(messages) != 1 || messages[0].Type != MessageThinking || messages[0].Content != "计划已定" {
				t.Fatalf("%s produced %+v", name, messages)
			}
		})
	}
}

// An item with nothing readable is dropped: a blank thinking bubble reads as a
// stalled agent, which is the very thing this case exists to prevent.
func TestCodexReasoningWithoutTextIsDropped(t *testing.T) {
	t.Parallel()

	messages := codexMessagesFrom(t,
		`{"jsonrpc":"2.0","method":"item/completed","params":{"item":{"type":"reasoning","id":"r-empty","text":"   "}}}`)
	if len(messages) != 0 {
		t.Fatalf("expected no message, got %+v", messages)
	}
}

// The plan arrives as a turn notification, not a tool call — looking for an
// `update_plan` tool found nothing because it was never there.
func TestCodexTurnPlanUpdatedBecomesATodoList(t *testing.T) {
	t.Parallel()

	messages := codexMessagesFrom(t,
		`{"jsonrpc":"2.0","method":"turn/plan/updated","params":{"explanation":"分四步","plan":[`+
			`{"step":"审计当前视觉层级","status":"completed"},`+
			`{"step":"修正排版与间距","status":"in_progress"},`+
			`{"step":"验证响应式","status":"pending"}]}}`)

	if len(messages) != 1 {
		t.Fatalf("expected one message, got %d: %+v", len(messages), messages)
	}
	msg := messages[0]
	if msg.Type != MessageToolUse || msg.Tool != "todo_write" {
		t.Fatalf("message = (%q, %q), want a todo_write tool use", msg.Type, msg.Tool)
	}
	if msg.Input["explanation"] != "分四步" {
		t.Fatalf("explanation = %v", msg.Input["explanation"])
	}
	todos, ok := msg.Input["todos"].([]map[string]any)
	if !ok || len(todos) != 3 {
		t.Fatalf("todos = %#v, want three normalised rows", msg.Input["todos"])
	}
	if todos[0]["content"] != "审计当前视觉层级" || todos[0]["status"] != "completed" {
		t.Fatalf("first todo = %#v", todos[0])
	}
	if todos[1]["status"] != "in_progress" {
		t.Fatalf("second todo status = %v, want the in-progress row preserved", todos[1]["status"])
	}
}

// A step with no text is dropped rather than rendered blank, and a status the
// protocol adds later survives verbatim instead of silently reading "pending".
func TestCodexPlanStepsDropBlanksAndKeepUnknownStatuses(t *testing.T) {
	t.Parallel()

	messages := codexMessagesFrom(t,
		`{"jsonrpc":"2.0","method":"turn/plan/updated","params":{"plan":[`+
			`{"step":"   ","status":"pending"},`+
			`{"step":"复核","status":"blocked_on_review"},`+
			`{"step":"交付"}]}}`)

	todos := messages[0].Input["todos"].([]map[string]any)
	if len(todos) != 2 {
		t.Fatalf("todos = %#v, want the blank step dropped", todos)
	}
	if todos[0]["status"] != "blocked_on_review" {
		t.Fatalf("unknown status = %v, want it kept verbatim", todos[0]["status"])
	}
	if todos[1]["status"] != "pending" {
		t.Fatalf("missing status = %v, want the pending default", todos[1]["status"])
	}
}

// An empty plan says nothing worth rendering; emitting it would replace a
// useful checklist with an empty one.
func TestCodexEmptyPlanEmitsNothing(t *testing.T) {
	t.Parallel()

	messages := codexMessagesFrom(t, `{"jsonrpc":"2.0","method":"turn/plan/updated","params":{"plan":[]}}`)
	if len(messages) != 0 {
		t.Fatalf("expected no message, got %+v", messages)
	}
}
