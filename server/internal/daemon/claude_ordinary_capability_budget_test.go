//go:build !windows

package daemon

import "testing"

func TestClaudeOrdinaryCapabilityBudget(t *testing.T) {
	three := 3.0
	for _, test := range []struct {
		name  string
		value *float64
		want  float64
	}{
		{name: "diagnostic default", want: 1},
		{name: "registered experimental ceiling", value: &three, want: 3},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := claudeOrdinaryCapabilityBudgetUSD(test.value)
			if err != nil || got != test.want {
				t.Fatalf("budget=%v error=%v, want budget=%v", got, err, test.want)
			}
		})
	}

	for _, value := range []float64{0, 1, 2, 4} {
		if _, err := claudeOrdinaryCapabilityBudgetUSD(&value); err == nil {
			t.Fatalf("explicit budget %v was accepted", value)
		}
	}
}
