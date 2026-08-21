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

// A pointer that resolves to no task at all is the longest-lived version of
// the same wedge: the task row is gone (retention, a purged workspace, a
// hand-deleted row) so there is nothing left to mark terminal, and a status
// that trusts the bare pointer reports 生成中 for as long as the document
// exists. Real occurrence: a document sat at 生成中 for two days pointing at a
// task row that no longer existed.
func TestDesignDocumentStatusDoesNotReportRunningForAPointerWithNoTask(t *testing.T) {
	document := db.DesignDocument{ActiveTaskID: ddUUID(), DraftRevisionID: ddUUID()}
	if got := designDocumentStatus(document, nil); got == "running" {
		t.Fatal("a pointer that resolves to no task still reported the document as running")
	}
	if got := designDocumentStatus(document, nil); got != "draft" {
		t.Fatalf("status with an unresolvable pointer = %q, want the draft it actually has", got)
	}
}

// The display lean must stay the opposite of the guard lean. designDocumentStatus
// treats an unreadable run as finished (a stale label self-corrects on the next
// poll); designDocumentRunIsLive treats one as live (a guard that guesses wrong
// destroys work). Pinning both here keeps a later "consistency" cleanup from
// collapsing them into one default.
func TestDesignDocumentStatusAndGuardLeanOppositeWays(t *testing.T) {
	document := db.DesignDocument{ActiveTaskID: ddUUID()}
	if got := designDocumentStatus(document, nil); got == "running" {
		t.Fatal("status must not claim a run it could not resolve")
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
