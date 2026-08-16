package handler

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func designDocumentRevisionUUID(t *testing.T, value byte) pgtype.UUID {
	t.Helper()
	uuid := pgtype.UUID{Valid: true}
	for index := range uuid.Bytes {
		uuid.Bytes[index] = value
	}
	return uuid
}

// The draft wins over the saved revision. Adjusting the saved revision while a
// draft exists would silently discard the run the user is looking at, and the
// adjustment would land on top of content that is no longer current.
func TestDesignDocumentAdjustPrefersTheDraftOverTheSavedRevision(t *testing.T) {
	draft := designDocumentRevisionUUID(t, 0x11)
	saved := designDocumentRevisionUUID(t, 0x22)

	base, ok := designDocumentAdjustBase(db.DesignDocument{
		DraftRevisionID: draft,
		SavedRevisionID: saved,
	})
	if !ok || base != draft {
		t.Fatalf("base = %v (ok=%v), want the draft %v", base, ok, draft)
	}
}

// With no draft the saved revision is the thing on screen, so it is the base.
func TestDesignDocumentAdjustFallsBackToTheSavedRevision(t *testing.T) {
	saved := designDocumentRevisionUUID(t, 0x22)

	base, ok := designDocumentAdjustBase(db.DesignDocument{SavedRevisionID: saved})
	if !ok || base != saved {
		t.Fatalf("base = %v (ok=%v), want the saved revision %v", base, ok, saved)
	}
}

// An empty or failed document has no revision at all. The handler turns this
// into a conflict; it must never be reported as an adjustable base, because the
// caller would then pin a zero revision id into the task context.
func TestDesignDocumentAdjustHasNoBaseWithoutAnyRevision(t *testing.T) {
	if base, ok := designDocumentAdjustBase(db.DesignDocument{}); ok {
		t.Fatalf("a document with no revision reported base %v", base)
	}
}
