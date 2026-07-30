package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"runtime"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/multica-ai/multica/server/internal/cli"
)

func TestLoginFlagsExposeBothAuthModes(t *testing.T) {
	for _, name := range []string{"token", "service-token", callbackHostFlag} {
		if loginCmd.Flags().Lookup(name) == nil {
			t.Fatalf("loginCmd is missing --%s", name)
		}
	}
	if got := loginCmd.Flags().Lookup("token").NoOptDefVal; got != tokenPromptSentinel {
		t.Fatalf("--token NoOptDefVal = %q, want prompt sentinel", got)
	}
}

func TestFetchUseSySSO(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want bool
	}{
		{name: "legacy", body: `{"use_sy_sso":false}`, want: false},
		{name: "SSO", body: `{"use_sy_sso":true}`, want: true},
		{name: "old server defaults to legacy", body: `{}`, want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/config" {
					t.Fatalf("unexpected request path %s", r.URL.Path)
				}
				_, _ = w.Write([]byte(tc.body))
			}))
			t.Cleanup(srv.Close)

			got, err := fetchUseSySSO(context.Background(), srv.URL)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("fetchUseSySSO() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestRunAuthLoginUsesServerAuthMode(t *testing.T) {
	newServer := func(t *testing.T, useSySSO bool) *httptest.Server {
		t.Helper()
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/config":
				_ = json.NewEncoder(w).Encode(map[string]bool{"use_sy_sso": useSySSO})
			case "/api/me":
				if useSySSO {
					http.Error(w, "rejected for test", http.StatusUnauthorized)
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]string{"name": "Alice", "email": "alice@example.com"})
			default:
				t.Fatalf("unexpected request path %s", r.URL.Path)
			}
		}))
	}

	t.Run("legacy accepts PAT", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())
		srv := newServer(t, false)
		t.Cleanup(srv.Close)
		t.Setenv("MULTICA_SERVER_URL", srv.URL)
		cmd := testAuthLoginCmd()
		if err := cmd.Flags().Set("token", tokenPromptSentinel); err != nil {
			t.Fatal(err)
		}

		if err := runAuthLogin(cmd, []string{"mul_test"}); err != nil {
			t.Fatal(err)
		}
		cfg, err := cli.LoadCLIConfig()
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Token != "mul_test" || cfg.ServiceTokenKeychainAccount != "" {
			t.Fatalf("unexpected saved auth config: %#v", cfg)
		}
	})

	t.Run("SSO accepts service token flag", func(t *testing.T) {
		srv := newServer(t, true)
		t.Cleanup(srv.Close)
		t.Setenv("MULTICA_SERVER_URL", srv.URL)
		cmd := testAuthLoginCmd()
		if err := cmd.Flags().Set("service-token", "msa_test"); err != nil {
			t.Fatal(err)
		}

		err := runAuthLogin(cmd, nil)
		want := "invalid service token"
		if runtime.GOOS != "darwin" {
			want = "supported only on macOS"
		}
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("runAuthLogin() error = %v, want %q", err, want)
		}
	})

	t.Run("SSO rejects legacy token flag", func(t *testing.T) {
		srv := newServer(t, true)
		t.Cleanup(srv.Close)
		t.Setenv("MULTICA_SERVER_URL", srv.URL)
		cmd := testAuthLoginCmd()
		_ = cmd.Flags().Set("token", "mul_test")

		err := runAuthLogin(cmd, nil)
		if err == nil || !strings.Contains(err.Error(), "--token") {
			t.Fatalf("runAuthLogin() error = %v, want incompatible --token error", err)
		}
	})

	t.Run("legacy rejects service token flag", func(t *testing.T) {
		srv := newServer(t, false)
		t.Cleanup(srv.Close)
		t.Setenv("MULTICA_SERVER_URL", srv.URL)
		cmd := testAuthLoginCmd()
		_ = cmd.Flags().Set("service-token", "msa_test")

		err := runAuthLogin(cmd, nil)
		if err == nil || !strings.Contains(err.Error(), "--service-token") {
			t.Fatalf("runAuthLogin() error = %v, want incompatible --service-token error", err)
		}
	})
}

func TestRunAuthLoginDoesNotDowngradeWhenConfigFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("MULTICA_SERVER_URL", srv.URL)

	err := runAuthLogin(testAuthLoginCmd(), nil)
	if err == nil || !strings.Contains(err.Error(), "fetch server auth mode") {
		t.Fatalf("runAuthLogin() error = %v, want config fetch error", err)
	}
}

func testAuthLoginCmd() *cobra.Command {
	cmd := testCmd()
	cmd.Flags().String("token", "", "")
	cmd.Flags().String("service-token", "", "")
	cmd.Flags().String(callbackHostFlag, "", "")
	return cmd
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
