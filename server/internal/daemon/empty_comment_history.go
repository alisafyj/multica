package daemon

import (
	"fmt"
	"time"
)

func buildVerifiedEmptyCommentHistoryPrompt(task Task, now time.Time) string {
	proof := task.emptyCommentHistory
	if proof == nil || proof.Version != 1 || task.IssueStartContractVersion != 1 ||
		task.ID == "" || task.WorkspaceID == "" || task.ClaimGeneration <= 0 ||
		proof.TaskID != task.ID || proof.WorkspaceID != task.WorkspaceID || proof.IssueID != task.IssueID ||
		proof.ClaimGeneration != task.ClaimGeneration ||
		!issueTaskSnapshotUsableAt(task, task.IssueSnapshot, now) || proof.Revision != task.IssueSnapshot.Revision ||
		task.TriggerCommentID != "" || len(task.CoalescedComments) != 0 || task.NewCommentCount != 0 {
		return ""
	}
	captured, captureErr := time.Parse(time.RFC3339Nano, proof.CapturedAt)
	expires, expiryErr := time.Parse(time.RFC3339Nano, proof.ExpiresAt)
	claimTime, _ := time.Parse(time.RFC3339Nano, task.IssueSnapshot.CapturedAt)
	if captureErr != nil || expiryErr != nil || !expires.After(captured) || expires.Sub(captured) > 5*time.Minute ||
		captured.Before(claimTime) || now.Before(captured.Add(-time.Minute)) || !now.Before(expires) {
		return ""
	}
	return fmt.Sprintf("## Verified Empty Comment History\n\nThe server verified zero comments for this issue at task start (%s). This separately satisfies the initial comment-history catch-up step; do not scan roots just to confirm it again. It does not cover later comments: process new triggers or other evidence of changed instructions through the normal bounded comment reads.\n\n", proof.CapturedAt)
}
