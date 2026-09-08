//go:build !windows

package daemon

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunTaskMapsIssueCompletionCapabilityToRuntimeBrief(t *testing.T) {
	for _, version := range []int{0, 1, 2} {
		t.Run(fmt.Sprintf("version=%d", version), func(t *testing.T) {
			d, _, cleanup := newLeaderReuseTestDaemon(t)
			defer cleanup()
			task := leaderReuseTestTask("task-completion-brief")
			task.IsLeaderTask = false
			task.IssueCompletionContractVersion = version
			result, err := d.runTask(context.Background(), task, "claude", 0, d.logger)
			if err != nil || result.Status != "completed" {
				t.Fatalf("runTask status=%s error=%v", result.Status, err)
			}
			body, err := os.ReadFile(filepath.Join(result.WorkDir, "CLAUDE.md"))
			if err != nil {
				t.Fatal(err)
			}
			brief := string(body)
			if strings.Contains(brief, "Final results MUST be delivered") != (version != 1) {
				t.Fatalf("runtime brief used wrong final delivery rule for version %d", version)
			}
			if strings.Contains(brief, "## Final Issue Delivery") != (version == 1) {
				t.Fatalf("runtime brief used wrong completion capability for version %d", version)
			}
		})
	}
}
