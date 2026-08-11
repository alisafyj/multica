package handler

import "testing"

func TestParseTestCaseNumber(t *testing.T) {
	cases := []struct {
		in   string
		want int32
		ok   bool
	}{
		{"TC-42", 42, true},
		{"tc-42", 42, true},
		{"  TC-7  ", 7, true},
		{"TC-0", 0, false},
		{"TC--1", 0, false},
		{"TC-", 0, false},
		{"TC-abc", 0, false},
		{"42", 0, false},
		{"MUL-42", 0, false},
		{"", 0, false},
		{"00000000-0000-0000-0000-000000000000", 0, false},
	}
	for _, c := range cases {
		got, ok := parseTestCaseNumber(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("parseTestCaseNumber(%q) = (%d, %v), want (%d, %v)", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestFormatTestCaseKey(t *testing.T) {
	if got := formatTestCaseKey(42); got != "TC-42" {
		t.Errorf("formatTestCaseKey(42) = %q, want \"TC-42\"", got)
	}
}
