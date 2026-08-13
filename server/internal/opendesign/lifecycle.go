package opendesign

type RunStatus string

const (
	RunStatusPreflightPending RunStatus = "preflight_pending"
	RunStatusPreflightFailed  RunStatus = "preflight_failed"
	RunStatusReady            RunStatus = "ready"
	RunStatusRunning          RunStatus = "running"
	RunStatusRunSucceeded     RunStatus = "run_succeeded"
	RunStatusCanceled         RunStatus = "canceled"
	RunStatusAgentFailed      RunStatus = "agent_failed"
	RunStatusAuditFailed      RunStatus = "audit_failed"
	RunStatusPreviewFailed    RunStatus = "preview_failed"
	RunStatusSucceeded        RunStatus = "succeeded"
)

func IsSupervisorTerminalRunStatus(status RunStatus) bool {
	switch status {
	case RunStatusCanceled, RunStatusAgentFailed, RunStatusAuditFailed, RunStatusPreviewFailed:
		return true
	default:
		return false
	}
}
