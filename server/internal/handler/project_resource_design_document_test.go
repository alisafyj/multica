package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// A design_document resource references an existing design_document row by id.
// Unlike document (an external link), the id must resolve inside the same
// workspace — the validator is where a typoed or foreign id is caught.
func TestProjectResourceDesignDocumentValidation(t *testing.T) {
	cases := []struct {
		name    string
		ref     any
		wantBad bool
		wantID  string
	}{
		{"missing id", map[string]any{}, true, ""},
		{"empty id", map[string]any{"design_document_id": ""}, true, ""},
		{"blank id", map[string]any{"design_document_id": "   "}, true, ""},
		{"non-uuid id", map[string]any{"design_document_id": "not-a-uuid"}, true, ""},
		{"valid canonical uuid", map[string]any{"design_document_id": "1a1a1a1a-1a1a-4a1a-8a1a-1a1a1a1a1a1a"}, false, "1a1a1a1a-1a1a-4a1a-8a1a-1a1a1a1a1a1a"},
		// Alternate UUID spelling is accepted but normalized before storage, so
		// uniqueness and deletion compare one canonical representation.
		{"valid undashed uuid", map[string]any{"design_document_id": "1a1a1a1a1a1a4a1a8a1a1a1a1a1a1a1a"}, false, "1a1a1a1a-1a1a-4a1a-8a1a-1a1a1a1a1a1a"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := json.Marshal(tc.ref)
			if err != nil {
				t.Fatalf("marshal fixture: %v", err)
			}
			normalized, err := validateAndNormalizeResourceRef(projectResourceTypeDesignDocument, raw)
			if tc.wantBad {
				if err == nil {
					t.Fatalf("expected validation error, got %s", normalized)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected validation error: %v", err)
			}
			var ref designDocumentResourceRef
			if err := json.Unmarshal(normalized, &ref); err != nil {
				t.Fatalf("decode normalized ref: %v", err)
			}
			if ref.DesignDocumentID != tc.wantID {
				t.Fatalf("normalized id = %q, want %q", ref.DesignDocumentID, tc.wantID)
			}
		})
	}
}

// Attaching a design_document resource whose id resolves to a real document in
// the same workspace succeeds and stores a normalized ref.
func TestProjectResourceDesignDocumentAttach(t *testing.T) {
	fixture := createDesignDocumentRevisionFixture(t)
	document := fixture.Document
	projectID := uuidToString(document.ProjectID)

	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/projects/"+projectID+"/resources", map[string]any{
		"resource_type": "design_document",
		"resource_ref": map[string]any{
			"design_document_id": "  " + uuidToString(document.ID) + "  ",
		},
	})
	req = withURLParam(req, "id", projectID)
	testHandler.CreateProjectResource(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var created ProjectResourceResponse
	if err := json.NewDecoder(w.Body).Decode(&created); err != nil {
		t.Fatalf("decode: %v", err)
	}
	var ref struct {
		DesignDocumentID string `json:"design_document_id"`
	}
	if err := json.Unmarshal(created.ResourceRef, &ref); err != nil {
		t.Fatalf("decode resource_ref: %v", err)
	}
	if ref.DesignDocumentID != uuidToString(document.ID) {
		t.Errorf("design_document_id not trimmed: %q", ref.DesignDocumentID)
	}

	// The same document cannot be attached twice to the same project — the
	// UNIQUE(project_id, resource_type, resource_ref) constraint reads as 409.
	w = httptest.NewRecorder()
	req = newRequest("POST", "/api/projects/"+projectID+"/resources", map[string]any{
		"resource_type": "design_document",
		"resource_ref":  map[string]any{"design_document_id": uuidToString(document.ID)},
	})
	req = withURLParam(req, "id", projectID)
	testHandler.CreateProjectResource(w, req)
	if w.Code != http.StatusConflict {
		t.Errorf("duplicate attach: expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

// An id that parses as a UUID but points at no design_document row (or at one
// in another workspace) is rejected — the reference would render as a dead
// card on the project.
func TestProjectResourceDesignDocumentMustExist(t *testing.T) {
	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/projects?workspace_id="+testWorkspaceID, map[string]any{
		"title": "Design doc existence project",
	})
	testHandler.CreateProject(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateProject: %d %s", w.Code, w.Body.String())
	}
	var project ProjectResponse
	if err := json.NewDecoder(w.Body).Decode(&project); err != nil {
		t.Fatalf("decode CreateProject: %v", err)
	}
	defer func() {
		r := newRequest("DELETE", "/api/projects/"+project.ID, nil)
		r = withURLParam(r, "id", project.ID)
		testHandler.DeleteProject(httptest.NewRecorder(), r)
	}()

	w = httptest.NewRecorder()
	req = newRequest("POST", "/api/projects/"+project.ID+"/resources", map[string]any{
		"resource_type": "design_document",
		"resource_ref":  map[string]any{"design_document_id": "1a1a1a1a-1a1a-4a1a-8a1a-1a1a1a1a1a1a"},
	})
	req = withURLParam(req, "id", project.ID)
	testHandler.CreateProjectResource(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for nonexistent design document, got %d: %s", w.Code, w.Body.String())
	}
}

// Deleting a design document must remove every project_resource row that
// references it — the reference carries no foreign key, so without this the
// project would keep a dead card pointing at a gone document. References can
// live in ANY project of the workspace (that is the whole point of the create
// modal's picker), so the cleanup read is workspace-scoped, not per-project.
func TestDeleteDesignDocumentRemovesProjectReferences(t *testing.T) {
	fixture := createDesignDocumentRevisionFixture(t)
	document := fixture.Document

	// A SECOND project, distinct from the document's own: the create-modal
	// flow attaches another project's document to the project being created.
	w := httptest.NewRecorder()
	req := newRequest("POST", "/api/projects?workspace_id="+testWorkspaceID, map[string]any{
		"title": "Referencing project",
	})
	testHandler.CreateProject(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("CreateProject: %d %s", w.Code, w.Body.String())
	}
	var referencingProject ProjectResponse
	if err := json.NewDecoder(w.Body).Decode(&referencingProject); err != nil {
		t.Fatalf("decode CreateProject: %v", err)
	}
	defer func() {
		r := newRequest("DELETE", "/api/projects/"+referencingProject.ID, nil)
		r = withURLParam(r, "id", referencingProject.ID)
		testHandler.DeleteProject(httptest.NewRecorder(), r)
	}()

	// Attach the reference from BOTH the owning project and the other one.
	for _, projectID := range []string{uuidToString(document.ProjectID), referencingProject.ID} {
		w := httptest.NewRecorder()
		req := newRequest("POST", "/api/projects/"+projectID+"/resources", map[string]any{
			"resource_type": "design_document",
			"resource_ref":  map[string]any{"design_document_id": uuidToString(document.ID)},
		})
		req = withURLParam(req, "id", projectID)
		testHandler.CreateProjectResource(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("attach to %s: expected 201, got %d: %s", projectID, w.Code, w.Body.String())
		}
	}

	if recorder := deleteDesignDocumentRequest(t, uuidToString(document.ID)); recorder.Code != http.StatusNoContent {
		t.Fatalf("delete document: status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	var references int
	if err := testPool.QueryRow(context.Background(),
		`SELECT count(*) FROM project_resource WHERE resource_type = 'design_document' AND resource_ref->>'design_document_id' = $1`,
		uuidToString(document.ID),
	).Scan(&references); err != nil {
		t.Fatalf("count references: %v", err)
	}
	if references != 0 {
		t.Fatalf("project references left behind = %d, want 0", references)
	}
}
