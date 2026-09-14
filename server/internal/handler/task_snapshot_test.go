package handler

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestNewIssueTaskSnapshotCarriesVersionedAuthoritativeState(t *testing.T) {
	now := time.Date(2026, 9, 6, 8, 0, 0, 0, time.UTC)
	created := now.Add(-time.Hour)
	updated := now.Add(-time.Minute)
	description := "Implement the bounded snapshot."
	issue := db.Issue{
		ID:          parseUUID("11111111-1111-4111-8111-111111111111"),
		Title:       "Snapshot issue",
		Description: pgtype.Text{String: description, Valid: true},
		Status:      "in_progress",
		Revision:    9,
		Metadata:    []byte(`{"pipeline_status":"waiting"}`),
		CreatedAt:   pgtype.Timestamptz{Time: created, Valid: true},
		UpdatedAt:   pgtype.Timestamptz{Time: updated, Valid: true},
	}
	resp := AgentTaskResponse{
		IssueID:               uuidToString(issue.ID),
		TriggerCommentID:      stringPtr("22222222-2222-4222-8222-222222222222"),
		TriggerThreadID:       "33333333-3333-4333-8333-333333333333",
		TriggerCommentContent: "trigger body",
		CoalescedComments: []CoalescedCommentData{{
			ID: "44444444-4444-4444-8444-444444444444", Content: "earlier body",
		}},
		NewCommentsSince: "2026-09-06T07:00:00Z",
		NewCommentCount:  2,
	}

	snapshot := newIssueTaskSnapshot(issue, resp, now)
	if snapshot.SchemaVersion != issueTaskSnapshotSchemaVersion || !snapshot.Complete {
		t.Fatalf("snapshot version/complete = %d/%v", snapshot.SchemaVersion, snapshot.Complete)
	}
	if snapshot.Scope != issueTaskSnapshotScopeIssueBody {
		t.Fatalf("snapshot scope = %q, want %q", snapshot.Scope, issueTaskSnapshotScopeIssueBody)
	}
	if snapshot.IssueID != resp.IssueID || snapshot.Title != issue.Title || snapshot.Description == nil || *snapshot.Description != description || snapshot.Status != issue.Status {
		t.Fatalf("snapshot issue fields = %+v", snapshot)
	}
	if snapshot.Revision != 9 || snapshot.ETag != `W/"issue:11111111-1111-4111-8111-111111111111:9"` {
		t.Fatalf("snapshot revision/etag = %d/%q", snapshot.Revision, snapshot.ETag)
	}
	if snapshot.Metadata["pipeline_status"] != "waiting" {
		t.Fatalf("snapshot metadata = %#v", snapshot.Metadata)
	}
	if snapshot.CreatedAt != created.Format(time.RFC3339) || snapshot.UpdatedAt != updated.Format(time.RFC3339) || snapshot.CapturedAt != now.Format(time.RFC3339) || snapshot.ExpiresAt == "" {
		t.Fatalf("snapshot timestamps = %+v", snapshot)
	}
	if snapshot.Trigger == nil || snapshot.Trigger.CommentID != *resp.TriggerCommentID || snapshot.Trigger.ThreadID != resp.TriggerThreadID || !snapshot.Trigger.ContentEmbedded || snapshot.Trigger.CoalescedCommentsEmbedded != 1 || snapshot.Trigger.NewCommentCount != 2 {
		t.Fatalf("snapshot trigger = %+v", snapshot.Trigger)
	}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"trigger body", "earlier body", "workspace_id", "author_name", "author_id"} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatalf("minimal snapshot leaked %q: %s", forbidden, raw)
		}
	}
	if snapshot.Trigger == nil || snapshot.Trigger.CoalescedCommentsEmbedded != 1 {
		t.Fatalf("snapshot must declare embedded trigger counts without claiming comment bodies: %+v", snapshot.Trigger)
	}
}

func TestNewIssueTaskSnapshotMarksTruncationAndPreservesUTF8(t *testing.T) {
	now := time.Now().UTC()
	issue := db.Issue{
		ID:          parseUUID("11111111-1111-4111-8111-111111111111"),
		Title:       strings.Repeat("界", issueTaskSnapshotTitleMaxBytes),
		Description: pgtype.Text{String: strings.Repeat("描", issueTaskSnapshotDescriptionMaxBytes), Valid: true},
		Status:      "todo",
		Revision:    1,
		CreatedAt:   pgtype.Timestamptz{Time: now, Valid: true},
		UpdatedAt:   pgtype.Timestamptz{Time: now, Valid: true},
	}

	snapshot := newIssueTaskSnapshot(issue, AgentTaskResponse{IssueID: uuidToString(issue.ID)}, now)
	if snapshot.Complete || !snapshot.TitleTruncated || !snapshot.DescriptionTruncated {
		t.Fatalf("truncation flags = complete:%v title:%v description:%v", snapshot.Complete, snapshot.TitleTruncated, snapshot.DescriptionTruncated)
	}
	if len(snapshot.Title) > issueTaskSnapshotTitleMaxBytes || snapshot.Description == nil || len(*snapshot.Description) > issueTaskSnapshotDescriptionMaxBytes {
		t.Fatalf("snapshot exceeded byte bounds: title=%d description=%d", len(snapshot.Title), len(*snapshot.Description))
	}
	if !json.Valid([]byte(`"`+snapshot.Title+`"`)) || !json.Valid([]byte(`"`+*snapshot.Description+`"`)) {
		t.Fatal("snapshot truncation split a UTF-8 sequence")
	}
}

func TestAgentTaskResponseOmitsAbsentIssueSnapshot(t *testing.T) {
	raw, err := json.Marshal(AgentTaskResponse{ID: "task-1"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "issue_snapshot") {
		t.Fatalf("legacy response gained empty snapshot: %s", raw)
	}
}

func TestNewIssueTaskSnapshotFailsClosedWhenTriggerBodyIsMissing(t *testing.T) {
	now := time.Now().UTC()
	issueID := "11111111-1111-4111-8111-111111111111"
	triggerID := "22222222-2222-4222-8222-222222222222"
	issue := db.Issue{
		ID:        parseUUID(issueID),
		Title:     "Snapshot issue",
		Status:    "todo",
		Revision:  1,
		CreatedAt: pgtype.Timestamptz{Time: now.Add(-time.Hour), Valid: true},
		UpdatedAt: pgtype.Timestamptz{Time: now, Valid: true},
	}
	snapshot := newIssueTaskSnapshot(issue, AgentTaskResponse{
		IssueID:          issueID,
		TriggerCommentID: &triggerID,
	}, now)
	if snapshot.Complete || snapshot.Trigger == nil || snapshot.Trigger.ContentEmbedded {
		t.Fatalf("missing trigger body did not fail closed: %+v", snapshot)
	}
}

func TestNewIssueTaskSnapshotFailsClosedOnInvalidMetadata(t *testing.T) {
	now := time.Now().UTC()
	issueID := "11111111-1111-4111-8111-111111111111"
	issue := db.Issue{
		ID:        parseUUID(issueID),
		Title:     "Snapshot issue",
		Status:    "todo",
		Revision:  1,
		Metadata:  []byte(`not-json`),
		CreatedAt: pgtype.Timestamptz{Time: now.Add(-time.Hour), Valid: true},
		UpdatedAt: pgtype.Timestamptz{Time: now, Valid: true},
	}
	snapshot := newIssueTaskSnapshot(issue, AgentTaskResponse{IssueID: issueID}, now)
	if snapshot.Complete {
		t.Fatalf("invalid metadata produced complete snapshot: %+v", snapshot)
	}
}

func stringPtr(value string) *string { return &value }
