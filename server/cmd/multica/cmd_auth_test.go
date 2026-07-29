package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/spf13/cobra"
)

func testCmd() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.PersistentFlags().String("profile", "", "")
	return cmd
}

func TestResolveAppURL(t *testing.T) {
	cmd := testCmd()
	t.Setenv("MULTICA_APP_URL", "http://localhost:14000")
	t.Setenv("FRONTEND_ORIGIN", "http://localhost:13000")
	if got := resolveAppURL(cmd); got != "http://localhost:14000" {
		t.Fatalf("resolveAppURL() = %q", got)
	}
}

func TestLoginFlagsOnlyExposeServiceAccountEscapeHatch(t *testing.T) {
	if loginCmd.Flags().Lookup("token") != nil {
		t.Fatal("human PAT --token flag must be removed")
	}
	if loginCmd.Flags().Lookup("service-token") == nil {
		t.Fatal("ai_work --service-token flag is missing")
	}
}

func TestBuildSSOAuthorizeURLUsesPKCEWithoutToken(t *testing.T) {
	raw := buildSSOAuthorizeURL(
		"https://api.example.test",
		"http://127.0.0.1:4321/callback",
		"state-1",
		"challenge-1",
	)
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	q := parsed.Query()
	if parsed.Path != "/auth/sso/authorize" || q.Get("client_id") != "cli" {
		t.Fatalf("unexpected authorize URL: %s", raw)
	}
	if q.Get("code_challenge") != "challenge-1" || q.Get("code_challenge_method") != "S256" {
		t.Fatalf("PKCE parameters missing: %s", raw)
	}
	if q.Get("state") != "state-1" || q.Get("redirect_uri") != "http://127.0.0.1:4321/callback" {
		t.Fatalf("callback state mismatch: %s", raw)
	}
	if q.Has("token") {
		t.Fatalf("authorize URL leaked a token: %s", raw)
	}
}

func TestExchangeSSOCodePostsVerifier(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/auth/sso/token" || r.Method != http.MethodPost {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["code"] != "code-1" || body["code_verifier"] != "verifier-1" || body["client_id"] != "cli" || body["grant_type"] != "authorization_code" {
			t.Fatalf("unexpected exchange body: %#v", body)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"token":      "jwt-1",
			"expires_at": "2026-07-28T18:00:00Z",
			"user": map[string]string{
				"name": "Alice", "email": "alice@soyoung.com",
			},
		})
	}))
	t.Cleanup(srv.Close)

	got, err := exchangeSSOCode(context.Background(), srv.URL, "code-1", "verifier-1", "http://127.0.0.1:4321/callback")
	if err != nil {
		t.Fatal(err)
	}
	if got.Token != "jwt-1" || got.User.Email != "alice@soyoung.com" {
		t.Fatalf("unexpected exchange response: %#v", got)
	}
}

func TestNormalizeAPIBaseURL(t *testing.T) {
	if got := normalizeAPIBaseURL("ws://localhost:18106/ws"); got != "http://localhost:18106" {
		t.Fatalf("normalizeAPIBaseURL() = %q", got)
	}
}
