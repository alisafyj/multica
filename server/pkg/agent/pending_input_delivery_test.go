package agent

import (
	"context"
	"testing"
	"time"
)

func TestPendingInputDeliverySurvivesExecutionCancellationWithBound(t *testing.T) {
	type contextKey struct{}
	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), contextKey{}, "claim"))
	cancel()
	var deliveredContext context.Context
	answer := PendingInputAnswer{OnDelivered: func(ctx context.Context) error {
		deliveredContext = ctx
		if ctx.Err() != nil || ctx.Value(contextKey{}) != "claim" {
			t.Fatal("execution cancellation cancelled delivery or lost claim context")
		}
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) <= 0 || time.Until(deadline) > 5*time.Second {
			t.Fatal("delivery must have an independent bounded deadline")
		}
		return nil
	}}
	if err := answer.MarkDelivered(ctx); err != nil {
		t.Fatal(err)
	}
	if deliveredContext.Err() != context.Canceled {
		t.Fatal("delivery context must be released when callback returns")
	}
}
