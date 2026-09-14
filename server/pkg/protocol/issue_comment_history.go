package protocol

// EmptyIssueCommentHistory attests an empty history at an accepted start
// revision. Consumers must validate all bindings and the expiration.
type EmptyIssueCommentHistory struct {
	Version         int    `json:"version"`
	TaskID          string `json:"task_id"`
	WorkspaceID     string `json:"workspace_id"`
	IssueID         string `json:"issue_id"`
	ClaimGeneration int64  `json:"claim_generation"`
	Revision        int64  `json:"revision"`
	CapturedAt      string `json:"captured_at"`
	ExpiresAt       string `json:"expires_at"`
}
