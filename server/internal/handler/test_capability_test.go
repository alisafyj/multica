package handler

// NOTE: All tests in this package are skipped when the database is
// unreachable (TestMain calls os.Exit(0)). The tests below are compile-time
// guards for the capability matching helpers and resolveRunCapabilities; the
// logic tests that MUST produce --- PASS lines live in
// internal/integrations/testcapability/dispatch_test.go and
// internal/daemon/capabilities_test.go.

import (
	"testing"
)

// ---------------------------------------------------------------------------
// matchCapabilityTarget
// ---------------------------------------------------------------------------

func TestMatchCapabilityTarget_EmptyMatch_AlwaysTrue(t *testing.T) {
	got := matchCapabilityTarget(nil, map[string]string{"browser": "chromium"})
	if !got {
		t.Error("empty match map must match any target")
	}
}

func TestMatchCapabilityTarget_ExactMatch(t *testing.T) {
	match := map[string]string{"browser": "chromium"}
	target := map[string]string{"browser": "chromium", "provider": "playwright"}
	if !matchCapabilityTarget(match, target) {
		t.Error("exact key match must return true")
	}
}

func TestMatchCapabilityTarget_MissingKey_ReturnsFalse(t *testing.T) {
	match := map[string]string{"os": "android"}
	target := map[string]string{"browser": "chromium"}
	if matchCapabilityTarget(match, target) {
		t.Error("missing key in target must return false")
	}
}

func TestMatchCapabilityTarget_WrongValue_ReturnsFalse(t *testing.T) {
	match := map[string]string{"browser": "firefox"}
	target := map[string]string{"browser": "chromium"}
	if matchCapabilityTarget(match, target) {
		t.Error("wrong value in target must return false")
	}
}

// ---------------------------------------------------------------------------
// satisfiesConstraint
// ---------------------------------------------------------------------------

func TestSatisfiesConstraint_Exact(t *testing.T) {
	if !satisfiesConstraint("chromium", "chromium") {
		t.Error("exact match must be true")
	}
	if satisfiesConstraint("firefox", "chromium") {
		t.Error("different value must be false")
	}
}

func TestSatisfiesConstraint_GTE(t *testing.T) {
	if !satisfiesConstraint("131", ">=120") {
		t.Error("131 >= 120 must be true")
	}
	if satisfiesConstraint("100", ">=120") {
		t.Error("100 >= 120 must be false")
	}
	if !satisfiesConstraint("120", ">=120") {
		t.Error("120 >= 120 must be true (equal satisfies >=)")
	}
}

func TestSatisfiesConstraint_GT(t *testing.T) {
	if !satisfiesConstraint("121", ">120") {
		t.Error("121 > 120 must be true")
	}
	if satisfiesConstraint("120", ">120") {
		t.Error("120 > 120 must be false (equal does not satisfy >)")
	}
}

func TestSatisfiesConstraint_LTE(t *testing.T) {
	if !satisfiesConstraint("100", "<=120") {
		t.Error("100 <= 120 must be true")
	}
	if satisfiesConstraint("130", "<=120") {
		t.Error("130 <= 120 must be false")
	}
}

// ---------------------------------------------------------------------------
// compareVersionLike
// ---------------------------------------------------------------------------

func TestCompareVersionLike_SingleSegment(t *testing.T) {
	if got := compareVersionLike("10", "9"); got <= 0 {
		t.Errorf("compareVersionLike(10, 9) = %d, want > 0", got)
	}
	if got := compareVersionLike("9", "10"); got >= 0 {
		t.Errorf("compareVersionLike(9, 10) = %d, want < 0", got)
	}
	if got := compareVersionLike("10", "10"); got != 0 {
		t.Errorf("compareVersionLike(10, 10) = %d, want 0", got)
	}
}

func TestCompareVersionLike_MultiSegment(t *testing.T) {
	if got := compareVersionLike("1.10.0", "1.9.0"); got <= 0 {
		t.Errorf("compareVersionLike(1.10.0, 1.9.0) = %d, want > 0", got)
	}
	if got := compareVersionLike("2.0.0", "1.99.99"); got <= 0 {
		t.Errorf("compareVersionLike(2.0.0, 1.99.99) = %d, want > 0", got)
	}
}

func TestCompareVersionLike_UnequalLengths(t *testing.T) {
	if got := compareVersionLike("1.0", "1"); got != 0 {
		t.Errorf("compareVersionLike(1.0, 1) = %d, want 0 (trailing zero)", got)
	}
}
