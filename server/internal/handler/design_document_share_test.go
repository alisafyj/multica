package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// A fixture whose revision has been saved: saving copies the draft pointer into
// saved_revision_id, which is what makes a revision shareable — a draft is not
// a promise (P-011 / DC-034).
func createSharedDesignDocumentFixture(t *testing.T) designDocumentRevisionFixture {
	t.Helper()
	fixture := createDesignDocumentRevisionFixture(t)
	saved, err := db.New(testPool).SaveDesignDocumentDraft(context.Background(), db.SaveDesignDocumentDraftParams{
		ID: fixture.Document.ID, WorkspaceID: parseUUID(testWorkspaceID), ExpectedDraftRevisionID: fixture.Revision.ID,
	})
	if err != nil {
		t.Fatalf("save draft revision: %v", err)
	}
	fixture.Document = saved
	return fixture
}

func performDesignDocumentShareCreate(t *testing.T, fixture designDocumentRevisionFixture, revisionID string) *httptest.ResponseRecorder {
	t.Helper()
	documentID := uuidToString(fixture.Document.ID)
	recorder := httptest.NewRecorder()
	request := withURLParams(newRequest(http.MethodPost, "/api/design-documents/"+documentID+"/revisions/"+revisionID+"/share", nil),
		"id", documentID, "revisionId", revisionID)
	testHandler.CreateDesignDocumentRevisionShare(recorder, request)
	return recorder
}

func performDesignDocumentShareList(t *testing.T, fixture designDocumentRevisionFixture) *httptest.ResponseRecorder {
	t.Helper()
	documentID := uuidToString(fixture.Document.ID)
	recorder := httptest.NewRecorder()
	testHandler.ListDesignDocumentShares(recorder, withURLParam(newRequest(http.MethodGet, "/api/design-documents/"+documentID+"/shares", nil), "id", documentID))
	return recorder
}

func performDesignDocumentShareRevoke(t *testing.T, fixture designDocumentRevisionFixture, shareID string) *httptest.ResponseRecorder {
	t.Helper()
	documentID := uuidToString(fixture.Document.ID)
	recorder := httptest.NewRecorder()
	request := withURLParams(newRequest(http.MethodDelete, "/api/design-documents/"+documentID+"/shares/"+shareID, nil),
		"id", documentID, "shareId", shareID)
	testHandler.RevokeDesignDocumentShare(recorder, request)
	return recorder
}

func performDesignDocumentShareExchange(t *testing.T, token string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	testHandler.GetDesignDocumentShareExchange(recorder, withURLParam(httptest.NewRequest(http.MethodGet, "/api/design-shares/"+token, nil), "token", token))
	return recorder
}

func createDesignDocumentShareFor(t *testing.T, fixture designDocumentRevisionFixture) DesignDocumentShareResponse {
	t.Helper()
	recorder := performDesignDocumentShareCreate(t, fixture, uuidToString(fixture.Revision.ID))
	if recorder.Code != http.StatusCreated {
		t.Fatalf("create share: status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var share DesignDocumentShareResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &share); err != nil {
		t.Fatal(err)
	}
	return share
}

// Creating is how a link is born, creating twice hands back the same link, the
// list shows exactly the live links, and revocation is terminal: the list
// forgets the link and a second revoke reads as not found.
func TestDesignDocumentShareLifecycleIsCreateOrReturnAndRevokeOnce(t *testing.T) {
	fixture := createSharedDesignDocumentFixture(t)
	created := createDesignDocumentShareFor(t, fixture)
	if !strings.HasPrefix(created.Token, "mds_") {
		t.Fatalf("token = %q, want an mds_ token", created.Token)
	}
	if created.URL != pluginAuthorizeBaseURL()+"/shares/"+created.Token {
		t.Fatalf("url = %q, want the paste-ready share page url", created.URL)
	}
	if created.RevisionID != uuidToString(fixture.Revision.ID) || created.DocumentID != uuidToString(fixture.Document.ID) ||
		created.DocumentTitle != "Orders overview" || created.RevokedAt != nil {
		t.Fatalf("share = %+v", created)
	}

	recorder := performDesignDocumentShareCreate(t, fixture, uuidToString(fixture.Revision.ID))
	if recorder.Code != http.StatusOK {
		t.Fatalf("second create status = %d, want 200 returning the existing link; body = %s", recorder.Code, recorder.Body.String())
	}
	var again DesignDocumentShareResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &again); err != nil {
		t.Fatal(err)
	}
	if again.ShareID != created.ShareID || again.Token != created.Token {
		t.Fatalf("second create minted a second link: %+v vs %+v", again, created)
	}

	recorder = performDesignDocumentShareList(t, fixture)
	if recorder.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var listed ListDesignDocumentSharesResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.Shares) != 1 || listed.Shares[0].Token != created.Token {
		t.Fatalf("shares = %+v, want exactly the one live link", listed.Shares)
	}

	if recorder := performDesignDocumentShareRevoke(t, fixture, created.ShareID); recorder.Code != http.StatusNoContent {
		t.Fatalf("revoke status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	recorder = performDesignDocumentShareList(t, fixture)
	if err := json.Unmarshal(recorder.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.Shares) != 0 {
		t.Fatalf("shares after revoke = %+v, want none", listed.Shares)
	}
	if recorder := performDesignDocumentShareRevoke(t, fixture, created.ShareID); recorder.Code != http.StatusNotFound {
		t.Fatalf("second revoke status = %d, want 404; body = %s", recorder.Code, recorder.Body.String())
	}
}

// A draft revision is refused, malformed ids are bad requests, and revisions
// and shares of other documents are not found rather than cross-reads.
func TestDesignDocumentShareRefusesDraftsStrangersAndBadIds(t *testing.T) {
	draft := createDesignDocumentRevisionFixture(t)
	if recorder := performDesignDocumentShareCreate(t, draft, uuidToString(draft.Revision.ID)); recorder.Code != http.StatusConflict ||
		!strings.Contains(recorder.Body.String(), "share_draft_revision") {
		t.Fatalf("draft share: status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	saved := createSharedDesignDocumentFixture(t)
	if recorder := performDesignDocumentShareCreate(t, saved, "not-a-uuid"); recorder.Code != http.StatusBadRequest {
		t.Fatalf("malformed revision id: status = %d, want 400", recorder.Code)
	}
	if recorder := performDesignDocumentShareCreate(t, saved, "4c4c4c4c-4c4c-4c4c-8c4c-4c4c4c4c4c4c"); recorder.Code != http.StatusNotFound {
		t.Fatalf("unknown revision: status = %d, want 404", recorder.Code)
	}
	if recorder := performDesignDocumentShareRevoke(t, saved, "not-a-uuid"); recorder.Code != http.StatusBadRequest {
		t.Fatalf("malformed share id: status = %d, want 400", recorder.Code)
	}

	other := createSharedDesignDocumentFixture(t)
	if recorder := performDesignDocumentShareCreate(t, saved, uuidToString(other.Revision.ID)); recorder.Code != http.StatusNotFound {
		t.Fatalf("another document's revision: status = %d, want 404", recorder.Code)
	}
	stranger := createDesignDocumentShareFor(t, other)
	if recorder := performDesignDocumentShareRevoke(t, saved, stranger.ShareID); recorder.Code != http.StatusNotFound {
		t.Fatalf("another document's share: status = %d, want 404", recorder.Code)
	}
	// The refused revoke must not have touched the link's real owner.
	recorder := performDesignDocumentShareList(t, other)
	var listed ListDesignDocumentSharesResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.Shares) != 1 || listed.Shares[0].Token != stranger.Token {
		t.Fatalf("other document's shares after the refused revoke = %+v", listed.Shares)
	}
}

// The public exchange trades the token for the page index plus a fresh
// per-visit capability, uncached, that the anonymous file route will accept.
func TestGetDesignDocumentShareExchangeIssuesAVisitCapability(t *testing.T) {
	fixture := createSharedDesignDocumentFixture(t)
	share := createDesignDocumentShareFor(t, fixture)

	recorder := performDesignDocumentShareExchange(t, share.Token)
	if recorder.Code != http.StatusOK {
		t.Fatalf("exchange status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}
	var body DesignDocumentShareExchangeResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.DocumentTitle != "Orders overview" {
		t.Fatalf("document_title = %q", body.DocumentTitle)
	}
	if len(body.Pages) != len(fixture.Package.Manifest.Pages) || body.PrototypeEntry != fixture.Package.Manifest.PrototypeEntry {
		t.Fatalf("pages/entry = %d/%q, want the manifest's %d/%q",
			len(body.Pages), body.PrototypeEntry, len(fixture.Package.Manifest.Pages), fixture.Package.Manifest.PrototypeEntry)
	}
	revisionID := uuidToString(fixture.Revision.ID)
	if !validateDesignDocumentPreviewAccessToken(body.ResourceAccessToken, testWorkspaceID, revisionID, fixture.Revision.ContentDigest, time.Now()) {
		t.Fatalf("issued capability does not validate: %q", body.ResourceAccessToken)
	}
	digestHex := strings.TrimPrefix(fixture.Revision.ContentDigest, "sha256:")
	wantBase := strings.Join([]string{designDocumentPreviewRoutePrefix, testWorkspaceID, revisionID, digestHex, body.ResourceAccessToken, "files"}, "/")
	if body.ResourceBasePath != wantBase {
		t.Fatalf("resource_base_path = %q, want %q", body.ResourceBasePath, wantBase)
	}
	expires, err := time.Parse(time.RFC3339, body.ResourceAccessExpiresAt)
	if err != nil || time.Until(expires) <= 0 || time.Until(expires) > designDocumentPreviewAccessTokenLifetime+time.Minute {
		t.Fatalf("resource_access_expires_at = %q (%v)", body.ResourceAccessExpiresAt, err)
	}

	// The capability is real: the anonymous file route serves the prototype
	// entry with the base path exactly as returned.
	fileRecorder := performDesignDocumentPreviewFileRequest(t, testWorkspaceID, revisionID, digestHex, body.ResourceAccessToken, "prototype/index.html")
	if fileRecorder.Code != http.StatusOK {
		t.Fatalf("file via exchange capability: status = %d, body = %s", fileRecorder.Code, fileRecorder.Body.String())
	}
}

// Every dead link — unknown, revoked, orphaned by a deleted document, or backed
// by a revision whose manifest no longer parses — answers with the same 404 and
// the same body, so a visitor cannot tell which they hit.
func TestGetDesignDocumentShareExchangeAnswersEveryDeadLinkAlike(t *testing.T) {
	unknown := performDesignDocumentShareExchange(t, "mds_"+strings.Repeat("0", 64))
	if unknown.Code != http.StatusNotFound {
		t.Fatalf("unknown token: status = %d, want 404", unknown.Code)
	}

	revokedFixture := createSharedDesignDocumentFixture(t)
	revokedShare := createDesignDocumentShareFor(t, revokedFixture)
	if recorder := performDesignDocumentShareRevoke(t, revokedFixture, revokedShare.ShareID); recorder.Code != http.StatusNoContent {
		t.Fatalf("revoke: status = %d", recorder.Code)
	}
	revoked := performDesignDocumentShareExchange(t, revokedShare.Token)

	orphanFixture := createSharedDesignDocumentFixture(t)
	orphanShare := createDesignDocumentShareFor(t, orphanFixture)
	if _, err := testPool.Exec(context.Background(), `DELETE FROM design_document WHERE id = $1`, orphanFixture.Document.ID); err != nil {
		t.Fatalf("delete shared document: %v", err)
	}
	orphaned := performDesignDocumentShareExchange(t, orphanShare.Token)

	rotFixture := createSharedDesignDocumentFixture(t)
	rotShare := createDesignDocumentShareFor(t, rotFixture)
	if _, err := testPool.Exec(context.Background(), `UPDATE design_document_revision SET manifest = $1 WHERE id = $2`,
		[]byte(`{}`), rotFixture.Revision.ID); err != nil {
		t.Fatalf("rot the revision manifest: %v", err)
	}
	rotted := performDesignDocumentShareExchange(t, rotShare.Token)

	for name, recorder := range map[string]*httptest.ResponseRecorder{
		"revoked": revoked, "orphaned document": orphaned, "rotted manifest": rotted,
	} {
		if recorder.Code != http.StatusNotFound {
			t.Fatalf("%s: status = %d, want 404; body = %s", name, recorder.Code, recorder.Body.String())
		}
		if got, want := recorder.Body.String(), unknown.Body.String(); got != want {
			t.Fatalf("%s: body = %s, want the identical unknown-token body %s", name, got, want)
		}
	}
}
