package daemon

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	issueTaskSnapshotSchemaVersion  = 1
	issueTaskSnapshotScopeIssueBody = "issue_body"
	issueTaskSnapshotMaxLifetime    = 10 * time.Minute
)

type IssueTaskTriggerSnapshot struct {
	CommentID                 string `json:"comment_id,omitempty"`
	ThreadID                  string `json:"thread_id,omitempty"`
	ContentEmbedded           bool   `json:"content_embedded,omitempty"`
	CoalescedCommentsEmbedded int    `json:"coalesced_comments_embedded,omitempty"`
	NewCommentsSince          string `json:"new_comments_since,omitempty"`
	NewCommentCount           int    `json:"new_comment_count,omitempty"`
}

type IssueTaskSnapshot struct {
	SchemaVersion        int                       `json:"schema_version"`
	Scope                string                    `json:"scope"`
	Complete             bool                      `json:"complete"`
	CapturedAt           string                    `json:"captured_at"`
	ExpiresAt            string                    `json:"expires_at"`
	IssueID              string                    `json:"issue_id"`
	Title                string                    `json:"title"`
	Description          *string                   `json:"description"`
	Status               string                    `json:"status"`
	Metadata             map[string]any            `json:"metadata"`
	Revision             int64                     `json:"revision"`
	ETag                 string                    `json:"etag"`
	CreatedAt            string                    `json:"created_at"`
	UpdatedAt            string                    `json:"updated_at"`
	TitleTruncated       bool                      `json:"title_truncated"`
	DescriptionTruncated bool                      `json:"description_truncated"`
	Trigger              *IssueTaskTriggerSnapshot `json:"trigger,omitempty"`
}

func buildAuthoritativeIssueSnapshotBlock(task Task) string {
	snapshot := task.IssueSnapshot
	if !issueTaskSnapshotUsableAt(task, snapshot, time.Now().UTC()) {
		return ""
	}
	raw, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return ""
	}

	var b strings.Builder
	b.WriteString("## Authoritative Issue Body Snapshot\n\n")
	b.WriteString("The server captured this authorized v1 snapshot at claim time; an accepted guarded start revalidates its body and updates its state. `complete` applies only to the issue body fields shown (title, description, status, metadata, revision, and timestamps); it does not include prior comment history, attachments, properties, or source context. It satisfies only workflow step 1's initial issue-body read for this turn, so do not call `multica issue get` solely to reload those fields. Fetch omitted detail only when the task refers to it. This snapshot does not satisfy the mandatory comment-history catch-up step; follow this turn's bounded comment-read instruction, which may use a validated resume delta or a separately verified empty-history result instead of repeating a roots scan. If a write reports a revision conflict, or other evidence shows the issue changed after this snapshot, refresh the issue before proceeding.\n\n")
	b.WriteString("```json\n")
	b.Write(raw)
	b.WriteString("\n```\n\n")
	return b.String()
}

func issueTaskSnapshotUsableAt(task Task, snapshot *IssueTaskSnapshot, now time.Time) bool {
	if snapshot == nil || snapshot.SchemaVersion != issueTaskSnapshotSchemaVersion || snapshot.Scope != issueTaskSnapshotScopeIssueBody || !snapshot.Complete ||
		snapshot.IssueID == "" || snapshot.IssueID != task.IssueID || snapshot.Title == "" || snapshot.Status == "" ||
		snapshot.Metadata == nil || snapshot.Revision <= 0 || snapshot.ETag != issueTaskSnapshotETag(snapshot.IssueID, snapshot.Revision) ||
		snapshot.TitleTruncated || snapshot.DescriptionTruncated {
		return false
	}
	capturedAt, capturedErr := time.Parse(time.RFC3339, snapshot.CapturedAt)
	expiresAt, expiresErr := time.Parse(time.RFC3339, snapshot.ExpiresAt)
	createdAt, createdErr := time.Parse(time.RFC3339, snapshot.CreatedAt)
	updatedAt, updatedErr := time.Parse(time.RFC3339, snapshot.UpdatedAt)
	if capturedErr != nil || expiresErr != nil || createdErr != nil || updatedErr != nil ||
		expiresAt.Before(capturedAt) || expiresAt.Sub(capturedAt) > issueTaskSnapshotMaxLifetime ||
		now.Before(capturedAt.Add(-time.Minute)) || !now.Before(expiresAt) || updatedAt.Before(createdAt) ||
		updatedAt.After(capturedAt.Add(time.Minute)) {
		return false
	}
	if task.TriggerCommentID == "" {
		return snapshot.Trigger == nil || snapshot.Trigger.CommentID == ""
	}
	return snapshot.Trigger != nil && snapshot.Trigger.CommentID == task.TriggerCommentID &&
		snapshot.Trigger.ThreadID == task.TriggerThreadID && snapshot.Trigger.ContentEmbedded &&
		task.TriggerCommentContent != "" && snapshot.Trigger.CoalescedCommentsEmbedded == len(task.CoalescedComments) &&
		snapshot.Trigger.NewCommentsSince == task.NewCommentsSince && snapshot.Trigger.NewCommentCount == task.NewCommentCount
}

func issueTaskSnapshotETag(issueID string, revision int64) string {
	if issueID == "" || revision <= 0 {
		return ""
	}
	return fmt.Sprintf(`W/"issue:%s:%d"`, issueID, revision)
}

func buildIssueCompletionDeliveryPrompt(task Task) string {
	return buildIssueCompletionDeliveryPromptWithOutcome(task, taskSupportsIssueOutcomeArtifact(task))
}

// Reuse the same capability decision when a caller also emits status guidance:
// a second wall-clock check could cross the snapshot's expiry mid-prompt.
func buildIssueCompletionDeliveryPromptWithOutcome(task Task, outcomeAvailable bool) string {
	if task.IssueID == "" || task.IssueCompletionContractVersion != issueTaskSnapshotSchemaVersion {
		return ""
	}
	prompt := "## Final Issue Delivery\n\nReturn your final response normally. After successful provider completion, Multica posts that exact final response to the issue and to every covered trigger thread. Do not run `multica issue comment add` for this final response, and do not post routine progress updates or plans along the way; streamed run progress is already visible. The default `delivered` outcome does not move issue status. This explicit completion-capable rule supersedes generic runtime instructions that require posting a final issue comment yourself; all other comment guardrails remain in force.\n"
	prompt += "For a missing user decision, use the native `request_user_input` tool in Codex or `AskUserQuestion` in Claude Code when available, and wait for its answer before continuing the same task and session. Do not write a blocked outcome while waiting, end the turn with only the question, or post the question as a final issue comment. This clarification rule takes precedence over generic instructions to stop when input is missing. If the native question tool is unavailable or fails, report that limitation and the unresolved question as the final blocker; do not guess the answer or wait indefinitely without a working question channel.\n"
	if outcomeAvailable {
		prompt += "Write `{\"version\":1,\"outcome\":\"review_ready\"}` to the file path exposed as `MULTICA_ISSUE_OUTCOME_FILE` only when the requested work is complete and its required verification has passed. If required input cannot be obtained through the clarification channel above or verification cannot be completed, write `{\"version\":1,\"outcome\":\"blocked\"}` instead. The file must contain exactly one JSON object with no other fields or records. Use one tool call to write the exact outcome artifact directly to the supplied `MULTICA_ISSUE_OUTCOME_FILE` path. The runtime validates it, so do not separately discover its path, inspect the environment, probe CLI or filesystem capabilities, or read the file back solely for the outcome. If the write fails, report the failure and do not claim the outcome succeeded. The runtime supplies the final comment and guarded issue revision itself. If you do not write this artifact, the outcome is `delivered` and status does not move. The platform does not infer review readiness or blockage from your prose, tool results, or a successful exit.\n"
		prompt += "Use this artifact instead of a separate status or review command for the same final transition; do not do both. Mid-turn status changes and explicitly requested CLI status or review commands remain available. If you already applied the final transition through the CLI, omit the artifact and return the final response normally.\n"
	} else {
		prompt += "Existing explicit status or review commands remain available when you deliberately choose them.\n"
	}
	if taskIsSquadLeader(task) && task.TriggerCommentID != "" {
		prompt += "If the squad `no_action` call succeeds, it is the complete recorded outcome: return no user-facing final body so automatic delivery remains silent. If that call fails, return exactly one short failure result for automatic delivery.\n"
	}
	return prompt + "\n"
}
