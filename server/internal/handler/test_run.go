package handler

import "net/http"

// Test plan and execution-run HTTP surface.
//
// Every handler below is registered in server/cmd/server/router.go. The bodies
// are implemented in this file; the list here is the contract the router and
// the CLI both depend on.

func (h *Handler) ListTestPlans(w http.ResponseWriter, r *http.Request)      { notImplemented(w) }
func (h *Handler) CreateTestPlan(w http.ResponseWriter, r *http.Request)     { notImplemented(w) }
func (h *Handler) GetTestPlan(w http.ResponseWriter, r *http.Request)        { notImplemented(w) }
func (h *Handler) UpdateTestPlan(w http.ResponseWriter, r *http.Request)     { notImplemented(w) }
func (h *Handler) DeleteTestPlan(w http.ResponseWriter, r *http.Request)     { notImplemented(w) }
func (h *Handler) ListTestPlanCases(w http.ResponseWriter, r *http.Request)  { notImplemented(w) }
func (h *Handler) AddTestPlanCases(w http.ResponseWriter, r *http.Request)   { notImplemented(w) }
func (h *Handler) RemoveTestPlanCase(w http.ResponseWriter, r *http.Request) { notImplemented(w) }

func (h *Handler) ListTestRuns(w http.ResponseWriter, r *http.Request)     { notImplemented(w) }
func (h *Handler) CreateTestRun(w http.ResponseWriter, r *http.Request)    { notImplemented(w) }
func (h *Handler) GetTestRun(w http.ResponseWriter, r *http.Request)       { notImplemented(w) }
func (h *Handler) StartTestRun(w http.ResponseWriter, r *http.Request)     { notImplemented(w) }
func (h *Handler) DispatchTestRun(w http.ResponseWriter, r *http.Request)  { notImplemented(w) }
func (h *Handler) RetryTestRun(w http.ResponseWriter, r *http.Request)     { notImplemented(w) }
func (h *Handler) ListTestRunCases(w http.ResponseWriter, r *http.Request) { notImplemented(w) }

func (h *Handler) UpdateTestRunCaseResult(w http.ResponseWriter, r *http.Request) { notImplemented(w) }
func (h *Handler) OpenTestRunCaseDefect(w http.ResponseWriter, r *http.Request)   { notImplemented(w) }
func (h *Handler) ListTestCaseResultTimeline(w http.ResponseWriter, r *http.Request) {
	notImplemented(w)
}

// notImplemented is a temporary scaffold while the handlers in this file are
// being filled in. Remove it once every route above has a real body.
func notImplemented(w http.ResponseWriter) {
	writeError(w, http.StatusNotImplemented, "not implemented yet")
}
