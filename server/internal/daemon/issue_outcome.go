package daemon

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

const (
	IssueOutcomeFileEnv       = "MULTICA_ISSUE_OUTCOME_FILE"
	issueOutcomeArtifactBytes = 1024
)

type issueOutcomeArtifact struct {
	Version int                    `json:"version"`
	Outcome IssueCompletionOutcome `json:"outcome"`
}

func issueStartReportForTask(task Task) (IssueStartReport, bool) {
	snapshot := task.IssueSnapshot
	if task.IssueStartContractVersion != 1 || !issueTaskSnapshotUsableAt(task, snapshot, time.Now().UTC()) ||
		task.ClaimGeneration <= 0 {
		return IssueStartReport{}, false
	}
	return IssueStartReport{
		ClaimGeneration: task.ClaimGeneration,
		Intent: IssueStartIntent{
			Version: 1, BaseRevision: snapshot.Revision, BaseETag: snapshot.ETag, BaseStatus: snapshot.Status,
		},
	}, true
}

func applyIssueStartState(task *Task, state *IssueStartState) {
	if task == nil {
		return
	}
	task.emptyCommentHistory = nil
	if state == nil || !state.BaselineAccepted {
		// The start response contains no body with which to repair a rejected snapshot.
		task.IssueSnapshot = nil
		return
	}
	if task.IssueSnapshot == nil {
		return
	}
	task.IssueSnapshot.Status = state.Status
	task.IssueSnapshot.Revision = state.Revision
	task.IssueSnapshot.ETag = state.ETag
	if state.UpdatedAt != "" {
		task.IssueSnapshot.UpdatedAt = state.UpdatedAt
		updatedAt, updateErr := time.Parse(time.RFC3339, state.UpdatedAt)
		capturedAt, captureErr := time.Parse(time.RFC3339, task.IssueSnapshot.CapturedAt)
		if state.Version == 1 && updateErr == nil && captureErr == nil && updatedAt.After(capturedAt) {
			// Guarded start revalidated the body revision; preserve the original expiry ceiling.
			task.IssueSnapshot.CapturedAt = state.UpdatedAt
		}
	}
	if state.Version == 1 && state.EmptyCommentHistory != nil {
		proof := *state.EmptyCommentHistory
		task.emptyCommentHistory = &proof
	}
}

func taskSupportsIssueOutcomeArtifact(task Task) bool {
	_, ok := issueStartReportForTask(task)
	return ok && task.IssueCompletionContractVersion == 1
}

func resetIssueOutcomeArtifact(path string) error {
	if path == "" {
		return nil
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove stale issue outcome artifact: %w", err)
	}
	return nil
}

func readIssueOutcomeArtifact(path string, task Task, output string) (*IssueCompletionIntent, error) {
	if path == "" {
		return nil, nil
	}
	file, err := openIssueOutcomeArtifact(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open issue outcome artifact: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("inspect opened issue outcome artifact: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("issue outcome artifact must be a bounded regular file")
	}
	raw, err := io.ReadAll(io.LimitReader(file, issueOutcomeArtifactBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read issue outcome artifact: %w", err)
	}
	if len(raw) > issueOutcomeArtifactBytes {
		return nil, errors.New("issue outcome artifact must be a bounded regular file")
	}

	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	var artifact issueOutcomeArtifact
	if err := decoder.Decode(&artifact); err != nil {
		return nil, fmt.Errorf("decode issue outcome artifact: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, errors.New("issue outcome artifact must contain exactly one JSON object")
		}
		return nil, fmt.Errorf("decode trailing issue outcome artifact data: %w", err)
	}

	comment := strings.TrimSpace(output)
	snapshot := task.IssueSnapshot
	if artifact.Version != 1 || (artifact.Outcome != IssueCompletionOutcomeReviewReady && artifact.Outcome != IssueCompletionOutcomeBlocked) ||
		comment == "" || snapshot == nil || snapshot.IssueID != task.IssueID || snapshot.Revision <= 0 ||
		snapshot.ETag != issueTaskSnapshotETag(task.IssueID, snapshot.Revision) || snapshot.Status == "" {
		return nil, errors.New("invalid issue outcome artifact")
	}
	return &IssueCompletionIntent{
		Version: 1, Outcome: artifact.Outcome, Comment: comment,
		BaseRevision: snapshot.Revision, BaseETag: snapshot.ETag, BaseStatus: snapshot.Status,
	}, nil
}
