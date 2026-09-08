package handler

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/multica-ai/multica/server/internal/testutil"
	"github.com/multica-ai/multica/server/pkg/protocol"
)

func TestR05StartEmptyCommentHistoryResponse(t *testing.T) {
	for _, variant := range []string{"empty", "existing", "changed", "legacy"} {
		t.Run(variant, func(t *testing.T) {
			fx := newIssueStartFixture(t, "R05 "+variant)
			if variant == "existing" {
				dbfx.Comment(t, fx.issueID, "history must never be embedded")
			}
			if variant == "changed" {
				dbfx.Exec(t, "UPDATE issue SET revision=revision+1 WHERE id=$1", fx.issueID)
			}
			var body any = fx.body(fx.claimGeneration, fx.baseRevision, "todo")
			if variant == "legacy" {
				body = nil
			}
			response := fx.start(t, body).Want(http.StatusOK)
			var payload map[string]json.RawMessage
			if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
				t.Fatal(err)
			}
			if variant == "legacy" {
				if _, present := payload["issue_start"]; present {
					t.Fatal("legacy start gained issue proof envelope")
				}
				return
			}
			var start map[string]json.RawMessage
			if err := json.Unmarshal(payload["issue_start"], &start); err != nil {
				t.Fatal(err)
			}
			raw, present := start["empty_comment_history"]
			if variant != "empty" {
				if present {
					t.Fatal("nonempty or unaccepted baseline serialized a proof")
				}
				return
			}
			if !present {
				t.Fatal("accepted empty history proof missing from handler response")
			}
			var proof protocol.EmptyIssueCommentHistory
			var fields map[string]any
			if json.Unmarshal(raw, &proof) != nil || json.Unmarshal(raw, &fields) != nil || len(fields) != 8 {
				t.Fatal("proof wire fields differ from the shared bounded contract")
			}
			if proof.Version != 1 || proof.TaskID != fx.taskID || proof.WorkspaceID != testWorkspaceID || proof.IssueID != fx.issueID || proof.ClaimGeneration != fx.claimGeneration || proof.Revision != fx.baseRevision+1 {
				t.Fatal("serialized proof binding mismatch")
			}
			captured, captureErr := time.Parse(time.RFC3339Nano, proof.CapturedAt)
			expires, expiryErr := time.Parse(time.RFC3339Nano, proof.ExpiresAt)
			if captureErr != nil || expiryErr != nil || expires.Sub(captured) != 5*time.Minute {
				t.Fatal("serialized proof TTL mismatch")
			}
		})
	}
}

func TestR05StartEmptyCommentHistoryRejectsForeignWorkspace(t *testing.T) {
	fx := newIssueStartFixture(t, "R05 workspace access")
	req := newDaemonTokenRequest(http.MethodPost, "/api/daemon/tasks/"+fx.taskID+"/start",
		fx.body(fx.claimGeneration, fx.baseRevision, "todo"), "00000000-0000-4000-8000-000000000005", "r05-foreign")
	req = withURLParam(req, "taskId", fx.taskID)
	testutil.Call(t, testHandler.StartTask, req).Want(http.StatusNotFound)
	assertTaskAndIssueStatus(t, fx, "dispatched", "todo")
}

func TestR05EmptyCommentHistoryWireOmission(t *testing.T) {
	raw, err := json.Marshal(IssueStartResponse{Version: 1})
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	if _, present := fields["empty_comment_history"]; present {
		t.Fatal("absent proof must be omitted, not null")
	}
}
