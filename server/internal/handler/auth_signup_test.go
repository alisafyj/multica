package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func newTestHandler(cfg Config) *Handler {
	return &Handler{cfg: cfg}
}

func TestSignupGating(t *testing.T) {
	tests := []struct {
		name  string
		cfg   Config
		email string
		isNew bool
		want  error
	}{
		{"allow_signup_true_new", Config{AllowSignup: true}, "a@x.com", true, nil},
		{"allow_signup_false_new", Config{AllowSignup: false}, "a@x.com", true, ErrSignupProhibited},
		{"allow_signup_false_existing", Config{AllowSignup: false}, "a@x.com", false, nil},
		{"domain_allowlist_match", Config{AllowSignup: false, AllowedEmailDomains: []string{"company.com"}}, "user@company.com", true, nil},
		{"domain_allowlist_mismatch_signup_disabled", Config{AllowSignup: false, AllowedEmailDomains: []string{"company.com"}}, "user@other.com", true, ErrSignupProhibited},
		{"domain_allowlist_mismatch_signup_enabled", Config{AllowSignup: true, AllowedEmailDomains: []string{"company.com"}}, "user@other.com", true, ErrEmailNotAllowed},
		{"email_allowlist_match", Config{AllowSignup: false, AllowedEmails: []string{"boss@x.com"}}, "boss@x.com", true, nil},
		{"email_allowlist_mismatch_signup_enabled", Config{AllowSignup: true, AllowedEmails: []string{"boss@x.com"}}, "user@other.com", true, ErrEmailNotAllowed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newTestHandler(tt.cfg)
			err := h.checkSignupAllowed(tt.email, tt.isNew)
			if !errors.Is(err, tt.want) {
				t.Fatalf("got err=%v want=%v", err, tt.want)
			}
		})
	}
}

func TestEmailCodeAllowlistErrors(t *testing.T) {
	for _, path := range []string{"send-code", "verify-code"} {
		t.Run(path, func(t *testing.T) {
			email := path + "-allowlist-regression@example.com"
			h := newTestHandler(Config{AllowSignup: true, AllowedEmailDomains: []string{"company.com"}})
			h.Queries = testHandler.Queries
			dbfx.Cleanup(t, `DELETE FROM "user" WHERE email = $1`, email)
			body := map[string]string{"email": email}
			handler := h.SendCode
			if path == "verify-code" {
				// A code can remain valid after the instance's signup policy changes.
				body["code"] = "123456"
				dbfx.Insert(t, "verification_code", testutil.Cols{
					"email":      email,
					"code":       body["code"],
					"expires_at": testutil.Raw("now() + interval '10 minutes'"),
				})
				handler = h.VerifyCode
			}
			req := testutil.JSONRequest(http.MethodPost, "/auth/"+path, body)
			resp := testutil.Call(t, handler, req).Want(http.StatusForbidden)
			got := resp.Map()
			if got["error"] != ErrEmailNotAllowed.Error() {
				t.Fatalf("expected an actionable allowlist error, got %v", got)
			}
			if _, hasCode := got["code"]; hasCode {
				t.Fatal("email-code errors must retain their existing response shape")
			}
			if len(resp.Result().Cookies()) != 0 {
				t.Fatal("rejected signup must not establish an authenticated session")
			}
			if count := dbfx.Count(t, `SELECT count(*) FROM "user" WHERE email = $1`, email); count != 0 {
				t.Fatalf("rejected signup created %d users", count)
			}
		})
	}
}

func TestFindOrCreateUserGating(t *testing.T) {
	t.Run("new_user_blocked", func(t *testing.T) {
		h := newTestHandler(Config{})
		h.Queries = db.New(&mockDB{getUserErr: pgx.ErrNoRows})

		_, isNew, err := h.findOrCreateUser(context.Background(), "new@blocked.com")
		if err == nil || isNew || !strings.Contains(err.Error(), "registration is disabled") {
			t.Fatalf("findOrCreateUser() = isNew %v, err %v", isNew, err)
		}
	})

	t.Run("existing_user_allowed", func(t *testing.T) {
		h := newTestHandler(Config{})
		h.Queries = db.New(&mockDB{})

		_, isNew, err := h.findOrCreateUser(context.Background(), "existing@test.com")
		if err != nil || isNew {
			t.Fatalf("findOrCreateUser() = isNew %v, err %v", isNew, err)
		}
	})
}

func TestFindOrCreateUserRejectsServiceAccount(t *testing.T) {
	const email = "legacy-auth-service-account@multica.ai"
	ctx := context.Background()
	_, err := testPool.Exec(ctx, `
		INSERT INTO "user" (name, email, account_kind)
		VALUES ('Service Account', $1, 'service')
		ON CONFLICT (email) DO UPDATE SET account_kind = 'service'
	`, email)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = testPool.Exec(context.Background(), `DELETE FROM "user" WHERE email = $1`, email) })

	_, _, err = testHandler.findOrCreateUser(ctx, email)
	if err == nil || !strings.Contains(err.Error(), "service account") {
		t.Fatalf("findOrCreateUser() error = %v", err)
	}
}
