package handler

import "net/http"

// Test execution capability surface: which phone, browser or desktop a run can
// actually drive, and how a run is bound to one.

func (h *Handler) ListTestCapabilities(w http.ResponseWriter, r *http.Request) { notImplemented(w) }
func (h *Handler) ListTestRunCapabilities(w http.ResponseWriter, r *http.Request) {
	notImplemented(w)
}
func (h *Handler) RequestRuntimeCapabilityScan(w http.ResponseWriter, r *http.Request) {
	notImplemented(w)
}
func (h *Handler) ReportRuntimeCapabilities(w http.ResponseWriter, r *http.Request) {
	notImplemented(w)
}
