package main

import (
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/multica-ai/multica/server/internal/analytics"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/realtime"
)

// The design document workspace reads revisions through the authenticated API
// and frames prototypes through the capability route that must live outside
// the auth group (an iframe cannot send the Bearer header). Both halves have to
// be registered, on the methods the client uses.
func TestDesignDocumentRevisionRoutesAreRegistered(t *testing.T) {
	router := NewRouter(nil, realtime.NewHub(), events.New(), analytics.NoopClient{}, nil)
	want := map[string]map[string]bool{
		http.MethodGet: {
			"/api/design-documents/{id}/revisions":                                                    false,
			"/api/design-documents/{id}/revisions/{revisionId}":                                       false,
			"/api/design-documents/{id}/revisions/{revisionId}/archive":                               false,
			"/api/design-document-previews/{workspaceId}/{revisionId}/{digest}/{accessToken}/files/*": false,
			"/api/design-systems/builtin/{slug}/showcase/{digest}/{variant}":                          false,
		},
		http.MethodPost: {
			"/api/design-documents/{id}/revisions/{revisionId}/restore": false,
			"/api/design-documents/{id}/adjust":                         false,
			"/api/design-documents/{id}/regenerate":                     false,
			"/api/design-documents/{id}/save":                           false,
			"/api/design-documents/{id}/discard":                        false,
		},
	}
	err := chi.Walk(router, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		if routes, ok := want[method]; ok {
			if _, exists := routes[route]; exists {
				routes[route] = true
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk router: %v", err)
	}
	for method, routes := range want {
		for route, found := range routes {
			if !found {
				t.Errorf("%s %s is not registered", method, route)
			}
		}
	}
}
