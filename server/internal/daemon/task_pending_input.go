package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/multica-ai/multica/server/pkg/agent"
)

const pendingInputAnswerPollInterval = 5 * time.Second

var pendingInputRetrySchedule = []time.Duration{250 * time.Millisecond, time.Second, 2 * time.Second}

func taskSupportsPendingInput(task Task, provider string) bool {
	if provider != "claude" && provider != "codex" {
		return false
	}
	if task.IssueID == "" || task.ClaimGeneration <= 0 || task.IssueCompletionContractVersion != 1 || task.RemoteMCPDaemonToken == "" {
		return false
	}
	return task.QuickCreatePrompt == "" && task.AutopilotRunID == "" && len(task.PMOSyncContext) == 0 &&
		len(task.UIDraftCreateContext) == 0 && len(task.DesignRestoreContext) == 0 &&
		!testingContextPresent(task.TestGenerationContext) && !testingContextPresent(task.TestRunContext) &&
		len(task.DesignSystemProfileAnalyzeContext) == 0 && len(task.TemplateBlueprintAnalyzeContext) == 0 &&
		len(task.ProjectDesignSystemContext) == 0 && len(task.DesignDocumentContext) == 0
}

type pendingInputDTO struct {
	ID      string                              `json:"id"`
	State   string                              `json:"state"`
	Answers map[string]pendingInputAnswerValues `json:"answers,omitempty"`
}

type pendingInputAnswerValues struct {
	Answers []string `json:"answers"`
}

type pendingInputTerminalError struct {
	err error
}

func (e pendingInputTerminalError) Error() string { return e.err.Error() }
func (e pendingInputTerminalError) Unwrap() error { return e.err }

type pendingInputBroker struct {
	client          *Client
	runtimeID       string
	taskID          string
	claimGeneration int64
	pollInterval    time.Duration
}

func newPendingInputBroker(client *Client, runtimeID, taskID string, claimGeneration int64, pollInterval time.Duration) pendingInputBroker {
	if pollInterval <= 0 {
		pollInterval = pendingInputAnswerPollInterval
	}
	return pendingInputBroker{
		client: client, runtimeID: runtimeID, taskID: taskID,
		claimGeneration: claimGeneration, pollInterval: pollInterval,
	}
}

func (b pendingInputBroker) Request(ctx context.Context, request agent.PendingInputRequest) (agent.PendingInputAnswer, error) {
	for index := range request.Questions {
		if request.Questions[index].Options == nil {
			request.Questions[index].Options = []agent.PendingInputOption{}
		}
	}
	if err := agent.ValidatePendingInputRequest(request); err != nil {
		return agent.PendingInputAnswer{}, err
	}
	basePath := fmt.Sprintf("/api/daemon/runtimes/%s/tasks/%s/pending-inputs",
		url.PathEscape(b.runtimeID), url.PathEscape(b.taskID))
	registration := struct {
		agent.PendingInputRequest
		ClaimGeneration int64 `json:"claim_generation"`
	}{PendingInputRequest: request, ClaimGeneration: b.claimGeneration}
	registrationJSON, err := json.Marshal(registration)
	if err != nil {
		return agent.PendingInputAnswer{}, fmt.Errorf("marshal pending user input registration: %w", err)
	}
	if len(registrationJSON) > agent.PendingInputMaxBodyBytes {
		return agent.PendingInputAnswer{}, fmt.Errorf("pending user input registration exceeds %d bytes", agent.PendingInputMaxBodyBytes)
	}
	var pending pendingInputDTO
	if err := b.client.postJSONWithRetry(ctx, basePath, registration, &pending, pendingInputRetrySchedule); err != nil {
		var requestErr *requestError
		if errors.As(err, &requestErr) && requestErr.StatusCode == 404 {
			return agent.PendingInputAnswer{}, fmt.Errorf("%w: pending-input API is unavailable; update the Multica server", agent.ErrPendingInputUnsupported)
		}
		return agent.PendingInputAnswer{}, fmt.Errorf("register pending user input: %w", err)
	}
	if pending.ID == "" {
		return agent.PendingInputAnswer{}, errors.New("register pending user input: server returned no id")
	}

	pendingPath := basePath + "/" + url.PathEscape(pending.ID)
	ticker := time.NewTicker(b.pollInterval)
	defer ticker.Stop()
	for {
		answer, done, err := b.poll(ctx, pendingPath, request)
		if err != nil {
			var terminal pendingInputTerminalError
			if errors.As(err, &terminal) {
				return agent.PendingInputAnswer{}, terminal.err
			}
			if !isTransientError(err) {
				return agent.PendingInputAnswer{}, err
			}
		}
		if done {
			pendingID := pending.ID
			answer.OnDelivered = func(deliveryCtx context.Context) error {
				return b.ack(deliveryCtx, basePath, pendingID)
			}
			return answer, nil
		}
		select {
		case <-ctx.Done():
			return agent.PendingInputAnswer{}, ctx.Err()
		case <-ticker.C:
		}
	}
}

func (b pendingInputBroker) poll(ctx context.Context, pendingPath string, request agent.PendingInputRequest) (agent.PendingInputAnswer, bool, error) {
	path := pendingPath + "?claim_generation=" + url.QueryEscape(strconv.FormatInt(b.claimGeneration, 10))
	var pending pendingInputDTO
	if err := b.client.getJSON(ctx, path, &pending); err != nil {
		var requestErr *requestError
		if errors.As(err, &requestErr) && requestErr.StatusCode == 409 {
			return agent.PendingInputAnswer{}, false, pendingInputTerminalError{err: errors.New("pending user input is no longer active; it was cancelled or expired")}
		}
		return agent.PendingInputAnswer{}, false, fmt.Errorf("poll pending user input: %w", err)
	}
	switch pending.State {
	case "open":
		return agent.PendingInputAnswer{}, false, nil
	case "answered":
		answer := agent.PendingInputAnswer{Answers: make(map[string][]string, len(pending.Answers))}
		for id, values := range pending.Answers {
			answer.Answers[id] = append([]string(nil), values.Answers...)
		}
		if err := agent.ValidatePendingInputAnswer(request, answer); err != nil {
			return agent.PendingInputAnswer{}, false, pendingInputTerminalError{err: fmt.Errorf("server returned invalid pending user input answer: %w", err)}
		}
		return answer, true, nil
	case "cancelled":
		return agent.PendingInputAnswer{}, false, pendingInputTerminalError{err: errors.New("pending user input was cancelled before an answer was delivered")}
	case "expired":
		return agent.PendingInputAnswer{}, false, pendingInputTerminalError{err: errors.New("pending user input expired before an answer was delivered")}
	default:
		return agent.PendingInputAnswer{}, false, pendingInputTerminalError{err: fmt.Errorf("server returned unknown pending user input state %q", pending.State)}
	}
}

func (b pendingInputBroker) ack(ctx context.Context, basePath, pendingID string) error {
	path := basePath + "/" + url.PathEscape(pendingID) + "/ack"
	var pending pendingInputDTO
	if err := b.client.postJSONWithRetry(ctx, path, struct {
		ClaimGeneration int64 `json:"claim_generation"`
	}{ClaimGeneration: b.claimGeneration}, &pending, pendingInputRetrySchedule); err != nil {
		return fmt.Errorf("acknowledge delivered user input: %w", err)
	}
	if pending.State != "answered" {
		return fmt.Errorf("acknowledge delivered user input: server returned state %q", pending.State)
	}
	return nil
}
