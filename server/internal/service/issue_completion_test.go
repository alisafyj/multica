package service

import (
	"strings"
	"testing"
)

func TestCanonicalIssueCompletionComment(t *testing.T) {
	if got, want := CanonicalIssueCompletionComment(` line one\nline two `), "line one\nline two"; got != want {
		t.Fatalf("canonical escaped newline = %q, want %q", got, want)
	}
	over := strings.Repeat("x", maxSynthesizedFallbackCommentRunes+100)
	if got := []rune(CanonicalIssueCompletionComment(over)); len(got) > maxSynthesizedFallbackCommentRunes {
		t.Fatalf("canonical comment runes = %d, want <= %d", len(got), maxSynthesizedFallbackCommentRunes)
	}
}
