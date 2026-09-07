package service

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
)

const PRDGenerationContextType = "prd_generation"

// PRDGenerationContext correlates a private draft-only chat task with the
// coordinator's immutable channel scope. It is never supplied by the worker.
type PRDGenerationContext struct {
	Type            string   `json:"type"`
	WorkspaceID     string   `json:"workspace_id"`
	InstallationID  string   `json:"installation_id"`
	SourceSessionID string   `json:"source_session_id"`
	SourceTaskID    string   `json:"source_task_id"`
	AgentID         string   `json:"agent_id"`
	ChatID          string   `json:"chat_id"`
	ThreadID        string   `json:"thread_id"`
	SourceMessageID string   `json:"source_message_id"`
	InitiatorOpenID string   `json:"initiator_open_id"`
	ContextRevision int64    `json:"context_revision"`
	RequestDigest   string   `json:"request_digest"`
	Headings        []string `json:"headings"`
}

// EnqueuePRDGenerationInTx reuses the normal chat queue, sealed input batch,
// daemon execution and durable task result. No issue or channel delivery is
// created, and no user credentials or project repositories are inherited.
// The caller owns source-session locking and the original-human invoke gate;
// FinalizeChatTaskEnqueue must run only after its transaction commits.
func (s *TaskService) EnqueuePRDGenerationInTx(ctx context.Context, tx pgx.Tx, parent db.AgentTaskQueue, agent db.Agent, generation PRDGenerationContext, prompt string) (db.AgentTaskQueue, error) {
	if !parent.OriginatorUserID.Valid || !parent.ChatSessionID.Valid || parent.AgentID == agent.ID {
		return db.AgentTaskQueue{}, errors.New("PRD delegation requires a human-originated coordinator task and a different agent")
	}
	qtx := s.Queries.WithTx(tx)
	current, err := qtx.GetAgentForClaimUpdate(ctx, agent.ID)
	if err != nil {
		return db.AgentTaskQueue{}, err
	}
	if current.ArchivedAt.Valid {
		return db.AgentTaskQueue{}, ErrChatTaskAgentArchived
	}
	if !current.RuntimeID.Valid {
		return db.AgentTaskQueue{}, ErrChatTaskAgentNoRuntime
	}
	session, err := qtx.CreateChatSession(ctx, db.CreateChatSessionParams{
		ID: dbid.NewV7(), WorkspaceID: current.WorkspaceID, AgentID: current.ID,
		CreatorID: parent.OriginatorUserID, Title: "PRD draft generation",
	})
	if err != nil {
		return db.AgentTaskQueue{}, err
	}
	task, err := qtx.CreateChatTask(ctx, db.CreateChatTaskParams{
		ID: dbid.NewV7(), AgentID: current.ID, RuntimeID: current.RuntimeID,
		Priority: 2, ChatSessionID: session.ID,
		InitiatorUserID: parent.OriginatorUserID, OriginatorUserID: parent.OriginatorUserID,
		AccountableUserID:   parent.OriginatorUserID,
		ForceFreshSession:   pgtype.Bool{Bool: true, Valid: true},
		ConciseMode:         pgtype.Bool{Bool: parent.ConciseMode, Valid: true},
		OriginatorSource:    pgtype.Text{String: "delegation", Valid: true},
		TriggerEvidenceKind: pgtype.Text{String: "chat", Valid: true}, TriggerEvidenceRefID: parent.ChatSessionID,
	})
	if err != nil {
		return db.AgentTaskQueue{}, err
	}
	raw, err := json.Marshal(generation)
	if err != nil {
		return db.AgentTaskQueue{}, err
	}
	// One recorded generation has one result identity. Automatic retry would
	// create another task id behind the coordinator's back; retries are explicit.
	if _, err := tx.Exec(ctx, `UPDATE agent_task_queue SET context=$2, delegated_from_task_id=$3, max_attempts=1 WHERE id=$1`, task.ID, raw, parent.ID); err != nil {
		return db.AgentTaskQueue{}, err
	}
	task, err = qtx.SetChatTaskInputOwnerSelf(ctx, task.ID)
	if err != nil {
		return db.AgentTaskQueue{}, err
	}
	_, err = qtx.CreateChatMessage(ctx, db.CreateChatMessageParams{
		ID: dbid.NewV7(), ChatSessionID: session.ID, TaskID: task.ID, Role: "user", Content: prompt,
	})
	return task, err
}
