package service

import "testing"

func TestDatabaseTestsRequireExplicitURL(t *testing.T) {
	callerContinued := false
	t.Run("unset skips before caller continues", func(t *testing.T) {
		t.Setenv("DATABASE_URL", "")
		_ = testDatabaseURL(t)
		callerContinued = true
	})
	if callerContinued {
		t.Fatal("unset DATABASE_URL returned to the database-test caller")
	}

	t.Run("explicit URL is returned without dialing", func(t *testing.T) {
		const dummyURL = "postgres://explicit.invalid/test"
		t.Setenv("DATABASE_URL", dummyURL)
		if got := testDatabaseURL(t); got != dummyURL {
			t.Fatalf("testDatabaseURL() = %q, want %q", got, dummyURL)
		}
	})
}
