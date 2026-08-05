package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// testCaseKeyPrefix is the human-readable key namespace for test cases.
// Unlike issues the prefix is fixed rather than workspace-configurable: a case
// key is only ever resolved inside an already workspace-scoped request, so a
// per-workspace prefix would add ambiguity without adding disambiguation.
const testCaseKeyPrefix = "TC-"

// formatTestCaseKey renders the human-readable key for a case number.
func formatTestCaseKey(number int32) string {
	return fmt.Sprintf("%s%d", testCaseKeyPrefix, number)
}

// parseTestCaseNumber accepts "TC-42" (case-insensitive, surrounding space
// tolerated) and returns the case number. Anything else — a bare number, a
// UUID, another prefix — returns ok=false so the caller falls through to UUID
// resolution.
func parseTestCaseNumber(ref string) (int32, bool) {
	trimmed := strings.TrimSpace(ref)
	if len(trimmed) <= len(testCaseKeyPrefix) {
		return 0, false
	}
	if !strings.EqualFold(trimmed[:len(testCaseKeyPrefix)], testCaseKeyPrefix) {
		return 0, false
	}
	number, err := strconv.Atoi(trimmed[len(testCaseKeyPrefix):])
	if err != nil || number <= 0 {
		return 0, false
	}
	return int32(number), true
}

// loadTestCaseForUser resolves a path param that may be either a TC-42 key or a
// UUID into the requesting workspace's test case. Per the repository UUID rule
// every write that follows must use the returned entity's ID, never the raw ref.
func (h *Handler) loadTestCaseForUser(w http.ResponseWriter, r *http.Request, ref string) (db.TestCase, bool) {
	wsUUID, ok := parseUUIDOrBadRequest(w, h.resolveWorkspaceID(r), "workspace_id")
	if !ok {
		return db.TestCase{}, false
	}
	if number, isKey := parseTestCaseNumber(ref); isKey {
		testCase, err := h.Queries.GetTestCaseByNumber(r.Context(), db.GetTestCaseByNumberParams{
			WorkspaceID: wsUUID,
			CaseNumber:  number,
		})
		if err != nil {
			writeError(w, http.StatusNotFound, "test case not found")
			return db.TestCase{}, false
		}
		return testCase, true
	}
	idUUID, ok := parseUUIDOrBadRequest(w, ref, "test case id")
	if !ok {
		return db.TestCase{}, false
	}
	testCase, err := h.Queries.GetTestCaseInWorkspace(r.Context(), db.GetTestCaseInWorkspaceParams{
		ID:          idUUID,
		WorkspaceID: wsUUID,
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "test case not found")
		return db.TestCase{}, false
	}
	return testCase, true
}
