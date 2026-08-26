package handler

import (
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/multica-ai/multica/server/internal/service"
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

// An adjustment's agent reads one attachments directory holding two different
// things: what the document was made from, and what this request wants looked
// at. Order is the only thing that separates them, and the prompt tells the
// agent to read it that way — so the document's own references come first and
// this turn's come last, always.
func TestDesignDocumentRunAttachmentsKeepsThisTurnLast(t *testing.T) {
	document := []service.DesignDocumentTaskAttachment{{ID: "doc-1"}, {ID: "doc-2"}}
	turn := []designDocumentAttachmentSnapshot{{AttachmentID: "turn-1"}}

	merged := designDocumentRunAttachments(document, turn)

	var ids []string
	for _, attachment := range merged {
		ids = append(ids, attachment.ID)
	}
	if strings.Join(ids, ",") != "doc-1,doc-2,turn-1" {
		t.Fatalf("run attachments = %v, want the document's own first", ids)
	}
	// The document's frozen list must not grow a turn's reference: it is read
	// back out of the stored snapshot on every later adjustment.
	if len(document) != 2 {
		t.Fatalf("the document's own list was appended to in place: %v", document)
	}
}

// Neither side present is the ordinary case and must produce an empty list
// rather than a nil the encoder writes as null.
func TestDesignDocumentRunAttachmentsIsEmptyWithoutAny(t *testing.T) {
	if merged := designDocumentRunAttachments(nil, nil); merged == nil || len(merged) != 0 {
		t.Fatalf("run attachments = %v, want an empty list", merged)
	}
}
