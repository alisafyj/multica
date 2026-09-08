package handler

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/multica-ai/multica/server/internal/service"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const issueCompletionContractVersion = 1

type IssueCompletionRequest struct {
	Version      int    `json:"version"`
	Outcome      string `json:"outcome"`
	Comment      string `json:"comment"`
	BaseRevision int64  `json:"base_revision,omitempty"`
	BaseETag     string `json:"base_etag,omitempty"`
	BaseStatus   string `json:"base_status,omitempty"`
}

func issueCompletionClaimGeneration(task db.AgentTaskQueue) int64 {
	if !task.DispatchedAt.Valid {
		return 0
	}
	return task.DispatchedAt.Time.UnixMicro()
}

func ordinaryIssueCompletionSupported(task db.AgentTaskQueue) bool {
	if !task.IssueID.Valid || task.ChatSessionID.Valid || task.AutopilotRunID.Valid {
		return false
	}
	var typed struct {
		Type string `json:"type"`
	}
	return len(task.Context) == 0 || json.Unmarshal(task.Context, &typed) != nil || typed.Type == ""
}

func validateIssueCompletionRequest(task db.AgentTaskQueue, claimGeneration int64, output string, req *IssueCompletionRequest) (*service.IssueCompletion, error) {
	if req == nil {
		return nil, nil
	}
	if !ordinaryIssueCompletionSupported(task) || req.Version != issueCompletionContractVersion || claimGeneration <= 0 {
		return nil, errors.New("invalid issue completion contract")
	}
	comment := strings.TrimSpace(req.Comment)
	if comment == "" {
		return nil, errors.New("issue completion comment is required")
	}
	if strings.TrimSpace(output) != comment {
		return nil, errors.New("issue completion comment must match output")
	}
	completion := &service.IssueCompletion{Outcome: req.Outcome, Comment: comment, ClaimGeneration: claimGeneration}
	switch req.Outcome {
	case service.IssueCompletionOutcomeDelivered:
		if req.BaseRevision != 0 || req.BaseETag != "" || req.BaseStatus != "" {
			return nil, errors.New("delivered outcome must not include status baseline")
		}
	case service.IssueCompletionOutcomeReviewReady, service.IssueCompletionOutcomeBlocked:
		if req.BaseRevision <= 0 || req.BaseETag != issueTaskSnapshotETag(uuidToString(task.IssueID), req.BaseRevision) ||
			strings.TrimSpace(req.BaseStatus) == "" {
			return nil, errors.New("invalid issue completion status baseline")
		}
		completion.BaseRevision = req.BaseRevision
		completion.BaseStatus = req.BaseStatus
	default:
		return nil, errors.New("invalid issue completion outcome")
	}
	return completion, nil
}
