//go:build !windows

package daemon

import (
	"slices"
	"testing"
)

func TestCodexRequestEnvelopeNativeSessionArgs(t *testing.T) {
	for _, tc := range []struct {
		mode string
		want []string
		fail bool
	}{
		{"", []string{"--ephemeral"}, false},
		{"ephemeral", []string{"--ephemeral"}, false},
		{"persistent", nil, false},
		{"Persistent", nil, true},
		{" persistent ", nil, true},
		{"unknown", nil, true},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			got, err := codexRequestEnvelopeNativeSessionArgs(tc.mode)
			if (err != nil) != tc.fail || !slices.Equal(got, tc.want) {
				t.Fatalf("session args=%q error=%v, want %q failure=%t", got, err, tc.want, tc.fail)
			}
		})
	}
}
