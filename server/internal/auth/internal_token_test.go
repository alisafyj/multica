package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestParseInternalToken(t *testing.T) {
	expiresAt := time.Now().Add(time.Hour).Truncate(time.Second)
	claims := jwt.MapClaims{
		"sub":         "user-id",
		"email":       "employee@soyoung.com",
		"auth_source": "sso",
		"exp":         expiresAt.Unix(),
	}
	sign := func(method jwt.SigningMethod, claims jwt.MapClaims) string {
		t.Helper()
		raw, err := jwt.NewWithClaims(method, claims).SignedString(JWTSecret())
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	identity, err := ParseInternalToken(sign(jwt.SigningMethodHS256, claims))
	if err != nil {
		t.Fatal(err)
	}
	if identity.UserID != "user-id" || identity.Email != "employee@soyoung.com" || identity.Source != "sso" || !identity.ExpiresAt.Equal(expiresAt) {
		t.Fatalf("identity = %#v", identity)
	}

	tests := []struct {
		name   string
		method jwt.SigningMethod
		claims jwt.MapClaims
	}{
		{"rejects HS512", jwt.SigningMethodHS512, claims},
		{"requires sub", jwt.SigningMethodHS256, jwt.MapClaims{"auth_source": "sso", "exp": expiresAt.Unix()}},
		{"requires auth source", jwt.SigningMethodHS256, jwt.MapClaims{"sub": "user-id", "exp": expiresAt.Unix()}},
		{"rejects non-SSO source", jwt.SigningMethodHS256, jwt.MapClaims{"sub": "user-id", "auth_source": "legacy", "exp": expiresAt.Unix()}},
		{"requires expiry", jwt.SigningMethodHS256, jwt.MapClaims{"sub": "user-id", "auth_source": "sso"}},
		{"rejects expired", jwt.SigningMethodHS256, jwt.MapClaims{"sub": "user-id", "auth_source": "sso", "exp": time.Now().Add(-time.Minute).Unix()}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := ParseInternalToken(sign(test.method, test.claims)); err == nil {
				t.Fatal("ParseInternalToken() error = nil")
			}
		})
	}
}
