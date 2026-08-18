// @canonical designDocumentStatus boundary matrix.
package handler

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func ddUUID() pgtype.UUID {
	return pgtype.UUID{Bytes: [16]byte{1, 2, 3, 4}, Valid: true}
}

// A document whose task died still holding `active_task_id` must not read as
// generating. This is the wedge behind MUL "生成中 forever": the sweeper marks
// the task failed, and a status derived from the pointer alone never notices.
func TestDesignDocumentStatusReleasesADocumentWhoseTaskAlreadyEnded(t *testing.T) {
	document := db.DesignDocument{
		ActiveTaskID: ddUUID(),
		LastError:    []byte(`{"code":"runtime_offline"}`),
	}
	for _, status := range []string{"failed", "cancelled", "completed"} {
		task := db.AgentTaskQueue{Status: status}
		if got := designDocumentStatus(document, &task); got == "running" {
			t.Fatalf("terminal task status %q still reported the document as running", status)
		}
	}
}

// The pointer alone remains proof of work while the task is genuinely on its
// way to a terminal state — a queued task has not started, but the document is
// legitimately busy and must not offer a second writer.
func TestDesignDocumentStatusKeepsRunningWhileTheTaskIsLive(t *testing.T) {
	document := db.DesignDocument{ActiveTaskID: ddUUID()}
	for _, status := range []string{"queued", "dispatched", "running", "waiting_local_directory", "deferred"} {
		task := db.AgentTaskQueue{Status: status}
		if got := designDocumentStatus(document, &task); got != "running" {
			t.Fatalf("live task status %q reported %q, want running", status, got)
		}
	}
}

// Callers that did not load the task (list endpoints before this fix, and any
// future caller with no task in hand) keep the pointer-only reading rather
// than silently downgrading a running document to idle.
func TestDesignDocumentStatusTrustsThePointerWhenNoTaskWasLoaded(t *testing.T) {
	document := db.DesignDocument{ActiveTaskID: ddUUID()}
	if got := designDocumentStatus(document, nil); got != "running" {
		t.Fatalf("status without a loaded task = %q, want running", got)
	}
}

// Releasing the pointer must not invent a state: what the document actually
// has after a dead task is its recorded failure, or the draft the failed run
// never replaced.
func TestDesignDocumentStatusFallsBackToWhatTheDocumentActuallyHas(t *testing.T) {
	failed := db.AgentTaskQueue{Status: "failed"}

	withError := db.DesignDocument{ActiveTaskID: ddUUID(), LastError: []byte(`{"code":"runtime_offline"}`)}
	if got := designDocumentStatus(withError, &failed); got != "failed" {
		t.Fatalf("dead task with a recorded error = %q, want failed", got)
	}

	// A `null` error is the column's empty value, not a failure.
	withDraft := db.DesignDocument{
		ActiveTaskID:    ddUUID(),
		DraftRevisionID: ddUUID(),
		LastError:       []byte(`null`),
	}
	if got := designDocumentStatus(withDraft, &failed); got != "draft" {
		t.Fatalf("dead task over an existing draft = %q, want draft", got)
	}
}
