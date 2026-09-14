package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestPendingInputRequestKeyIsStableAndOpaque(t *testing.T) {
	t.Parallel()

	got := NewPendingInputRequestKey("codex", "thread-secret", "turn-secret", "item-secret")
	if got != NewPendingInputRequestKey("codex", "thread-secret", "turn-secret", "item-secret") {
		t.Fatal("request key is not stable")
	}
	if len(got) != 71 {
		t.Fatalf("request key length = %d, want 71", len(got))
	}
	for _, raw := range []string{"thread-secret", "turn-secret", "item-secret"} {
		if strings.Contains(got, raw) {
			t.Fatalf("request key exposes provider identity %q", raw)
		}
	}
}

func TestPendingInputRequestJSONUsesPublicSnakeCaseShape(t *testing.T) {
	t.Parallel()

	data, err := json.Marshal(PendingInputRequest{
		Version: PendingInputVersion1, RequestKey: NewPendingInputRequestKey("test"), Blocking: true,
		Questions: []PendingInputQuestion{{
			ID: "q1", Header: "Header", Question: "Question?", AllowOther: true, MultiSelect: true,
			Options: []PendingInputOption{{Label: "A", Description: "First"}},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{`"request_key"`, `"allow_other"`, `"multi_select"`} {
		if !strings.Contains(string(data), field) {
			t.Fatalf("missing %s in %s", field, data)
		}
	}
}

func TestValidatePendingInputRequestLimits(t *testing.T) {
	t.Parallel()

	valid := PendingInputRequest{
		Version:    PendingInputVersion1,
		RequestKey: NewPendingInputRequestKey("claude", "session", "request"),
		Blocking:   true,
		Questions: []PendingInputQuestion{{
			ID: "q1", Header: "Choice", Question: "Choose?",
			Options: []PendingInputOption{{Label: "A", Description: "First"}},
		}},
	}
	if err := ValidatePendingInputRequest(valid); err != nil {
		t.Fatalf("valid request rejected: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*PendingInputRequest)
	}{
		{"nonblocking", func(r *PendingInputRequest) { r.Blocking = false }},
		{"too many questions", func(r *PendingInputRequest) {
			r.Questions = append(r.Questions, r.Questions[0], r.Questions[0], r.Questions[0])
		}},
		{"question too long", func(r *PendingInputRequest) {
			r.Questions[0].Question = strings.Repeat("q", PendingInputMaxQuestionRunes+1)
		}},
		{"too many options", func(r *PendingInputRequest) { r.Questions[0].Options = make([]PendingInputOption, 9) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := valid
			req.Questions = append([]PendingInputQuestion(nil), valid.Questions...)
			tt.mutate(&req)
			if err := ValidatePendingInputRequest(req); err == nil {
				t.Fatal("invalid request accepted")
			}
		})
	}
}

func TestPendingInputAnswerDeliveredHook(t *testing.T) {
	t.Parallel()

	called := false
	answer := PendingInputAnswer{
		Answers: map[string][]string{"q1": {"A"}},
		OnDelivered: func(context.Context) error {
			called = true
			return nil
		},
	}
	if err := answer.MarkDelivered(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("delivery hook was not called")
	}
}

func TestPendingInputCoordinatorSuppressesReplaysAndConflicts(t *testing.T) {
	t.Parallel()

	coordinator := newPendingInputCoordinator()
	payload := pendingInputPayloadHash(PendingInputRequest{Questions: []PendingInputQuestion{{ID: "q1", Question: "First?"}}})
	changedPayload := pendingInputPayloadHash(PendingInputRequest{Questions: []PendingInputQuestion{{ID: "q1", Question: "Changed?"}}})
	if got := coordinator.begin("native-1", "key-1", payload); got != pendingInputAccepted {
		t.Fatalf("first begin = %v", got)
	}
	if got := coordinator.begin("native-1", "key-1", payload); got != pendingInputDuplicate {
		t.Fatalf("same replay = %v", got)
	}
	if got := coordinator.begin("native-1", "key-1", changedPayload); got != pendingInputConflict {
		t.Fatalf("same-key changed-payload replay = %v", got)
	}
	if got := coordinator.begin("native-1", "key-2", payload); got != pendingInputConflict {
		t.Fatalf("conflicting replay = %v", got)
	}
	coordinator.finish("native-1")
	if got := coordinator.begin("native-1", "key-1", payload); got != pendingInputDuplicate {
		t.Fatalf("completed replay = %v", got)
	}
	coordinator.wait()
}

func TestPendingInputCoordinatorBoundsActiveAndCompletedRequests(t *testing.T) {
	t.Parallel()

	active := newPendingInputCoordinator()
	for i := 0; i < pendingInputMaxActiveRequests; i++ {
		id := fmt.Sprintf("native-%d", i)
		if got := active.begin(id, "key-"+id, pendingInputPayloadHash(PendingInputRequest{RequestKey: id})); got != pendingInputAccepted {
			t.Fatalf("active begin %d = %v", i, got)
		}
	}
	if got := active.begin("native-overflow", "key-overflow", pendingInputPayloadHash(PendingInputRequest{RequestKey: "overflow"})); got != pendingInputLimitReached {
		t.Fatalf("active overflow = %v", got)
	}
	for i := 0; i < pendingInputMaxActiveRequests; i++ {
		active.finish(fmt.Sprintf("native-%d", i))
	}
	active.wait()

	completed := newPendingInputCoordinator()
	for i := 0; i < pendingInputMaxCompletedRequests; i++ {
		id := fmt.Sprintf("completed-%d", i)
		if got := completed.begin(id, "key-"+id, pendingInputPayloadHash(PendingInputRequest{RequestKey: id})); got != pendingInputAccepted {
			t.Fatalf("completed begin %d = %v", i, got)
		}
		completed.finish(id)
	}
	if got := completed.begin("completed-overflow", "key-completed-overflow", pendingInputPayloadHash(PendingInputRequest{RequestKey: "overflow"})); got != pendingInputLimitReached {
		t.Fatalf("completed overflow = %v", got)
	}
	completed.wait()
}

func TestValidatePendingInputRequestRejectsNonCanonicalAndNUL(t *testing.T) {
	t.Parallel()

	base := PendingInputRequest{
		Version: PendingInputVersion1, RequestKey: NewPendingInputRequestKey("test"), Blocking: true,
		Questions: []PendingInputQuestion{{ID: "q1", Question: "Choose?", Options: []PendingInputOption{{Label: "A"}}}},
	}
	for _, mutate := range []func(*PendingInputRequest){
		func(r *PendingInputRequest) { r.Questions[0].ID = " q1" },
		func(r *PendingInputRequest) { r.Questions[0].Options[0].Label = "A " },
		func(r *PendingInputRequest) { r.Questions[0].Question = "Choose?\x00" },
	} {
		req := base
		req.Questions = append([]PendingInputQuestion(nil), base.Questions...)
		req.Questions[0].Options = append([]PendingInputOption(nil), base.Questions[0].Options...)
		mutate(&req)
		if err := ValidatePendingInputRequest(req); err == nil {
			t.Fatal("non-canonical request accepted")
		}
	}
}

func TestPendingInputCallbackCancellationCanBeJoined(t *testing.T) {
	t.Parallel()

	coordinator := newPendingInputCoordinator()
	if got := coordinator.begin("native", "key", pendingInputPayloadHash(PendingInputRequest{RequestKey: "key"})); got != pendingInputAccepted {
		t.Fatalf("begin = %v", got)
	}
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	go func() {
		defer coordinator.finish("native")
		_, _ = runPendingInputCallback(ctx, func(ctx context.Context, _ PendingInputRequest) (PendingInputAnswer, error) {
			close(started)
			<-ctx.Done()
			return PendingInputAnswer{}, ctx.Err()
		}, PendingInputRequest{}, func() {})
	}()
	<-started
	cancel()
	joined := make(chan struct{})
	go func() {
		coordinator.wait()
		close(joined)
	}()
	select {
	case <-joined:
	case <-time.After(time.Second):
		t.Fatal("cancelled pending input callback did not join")
	}
}
