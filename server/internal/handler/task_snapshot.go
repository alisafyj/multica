package handler

import (
	"encoding/json"
	"fmt"
	"time"
	"unicode/utf8"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const (
	issueTaskSnapshotSchemaVersion       = 1
	issueTaskSnapshotScopeIssueBody      = "issue_body"
	issueTaskSnapshotTitleMaxBytes       = 8 << 10
	issueTaskSnapshotDescriptionMaxBytes = 128 << 10
	issueTaskSnapshotFreshness           = 5 * time.Minute
)

// IssueTaskTriggerSnapshot records which bounded comment input was already
// delivered with the claim. Bodies remain in the established trigger fields so
// old daemons and delivery receipts keep one source of truth.
type IssueTaskTriggerSnapshot struct {
	CommentID                 string `json:"comment_id,omitempty"`
	ThreadID                  string `json:"thread_id,omitempty"`
	ContentEmbedded           bool   `json:"content_embedded,omitempty"`
	CoalescedCommentsEmbedded int    `json:"coalesced_comments_embedded,omitempty"`
	NewCommentsSince          string `json:"new_comments_since,omitempty"`
	NewCommentCount           int    `json:"new_comment_count,omitempty"`
}

// IssueTaskSnapshot is a bounded, versioned issue-body view captured after the
// claim's workspace authorization checks. It deliberately excludes prior
// comment history. Complete is false whenever critical issue-body or trigger
// delivery metadata was truncated or unavailable, forcing upgraded daemons to
// retain the legacy issue API-read workflow.
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

func newIssueTaskSnapshot(issue db.Issue, resp AgentTaskResponse, capturedAt time.Time) *IssueTaskSnapshot {
	capturedAt = capturedAt.UTC().Truncate(time.Second)
	issueID := uuidToString(issue.ID)
	title, titleTruncated := truncateIssueSnapshotUTF8(issue.Title, issueTaskSnapshotTitleMaxBytes)

	var description *string
	descriptionTruncated := false
	if issue.Description.Valid {
		value, truncated := truncateIssueSnapshotUTF8(issue.Description.String, issueTaskSnapshotDescriptionMaxBytes)
		description = &value
		descriptionTruncated = truncated
	}
	metadata := make(map[string]any)
	metadataValid := len(issue.Metadata) == 0 || json.Unmarshal(issue.Metadata, &metadata) == nil
	if metadata == nil {
		metadata = make(map[string]any)
	}

	snapshot := &IssueTaskSnapshot{
		SchemaVersion:        issueTaskSnapshotSchemaVersion,
		Scope:                issueTaskSnapshotScopeIssueBody,
		CapturedAt:           capturedAt.Format(time.RFC3339),
		ExpiresAt:            capturedAt.Add(issueTaskSnapshotFreshness).Format(time.RFC3339),
		IssueID:              issueID,
		Title:                title,
		Description:          description,
		Status:               issue.Status,
		Metadata:             metadata,
		Revision:             issue.Revision,
		ETag:                 issueTaskSnapshotETag(issueID, issue.Revision),
		CreatedAt:            timestampToString(issue.CreatedAt),
		UpdatedAt:            timestampToString(issue.UpdatedAt),
		TitleTruncated:       titleTruncated,
		DescriptionTruncated: descriptionTruncated,
	}

	triggerComplete := true
	if resp.TriggerCommentID != nil && *resp.TriggerCommentID != "" {
		snapshot.Trigger = &IssueTaskTriggerSnapshot{
			CommentID:                 *resp.TriggerCommentID,
			ThreadID:                  resp.TriggerThreadID,
			ContentEmbedded:           resp.TriggerCommentContent != "",
			CoalescedCommentsEmbedded: len(resp.CoalescedComments),
			NewCommentsSince:          resp.NewCommentsSince,
			NewCommentCount:           resp.NewCommentCount,
		}
		triggerComplete = snapshot.Trigger.ContentEmbedded
	}

	snapshot.Complete = issueID != "" && issueID == resp.IssueID && issue.Title != "" && issue.Status != "" && issue.Revision > 0 && metadataValid &&
		issue.CreatedAt.Valid && issue.UpdatedAt.Valid && !titleTruncated && !descriptionTruncated && triggerComplete
	return snapshot
}

func issueTaskSnapshotETag(issueID string, revision int64) string {
	if issueID == "" || revision <= 0 {
		return ""
	}
	return fmt.Sprintf(`W/"issue:%s:%d"`, issueID, revision)
}

func truncateIssueSnapshotUTF8(value string, maxBytes int) (string, bool) {
	if maxBytes < 0 {
		maxBytes = 0
	}
	if len(value) <= maxBytes {
		return value, false
	}
	end := maxBytes
	for end > 0 && !utf8.RuneStart(value[end]) {
		end--
	}
	return value[:end], true
}
