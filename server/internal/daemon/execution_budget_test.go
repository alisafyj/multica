package daemon

import (
	"testing"

	"github.com/multica-ai/multica/server/pkg/agent"
)

func TestExecutionBudgetNeverStartsFreshSession(t *testing.T) {
	t.Parallel()
	for _, result := range []agent.Result{
		{Status: "failed", ExecutionBudgetExceeded: true},
		{Status: "failed", ExecutionBudgetExceeded: true, ResumeRejected: true},
		{Status: "failed", ExecutionBudgetExceeded: true, ResumeRejectedTransient: true},
	} {
		if shouldRetryWithFreshSession(result, "prior-session", 0, "claude") {
			t.Fatal("configured spending cap must not trigger another agent launch")
		}
	}
}
