package agent

import (
	"context"
	"errors"
	"log/slog"
	"reflect"
	"testing"
)

func TestBuildCodexArgsEnablesRequestUserInputOnlyWithCallback(t *testing.T) {
	t.Parallel()

	withoutCallback := buildCodexArgs(ExecOptions{}, slog.Default())
	if containsCodexFeatureOverride(withoutCallback, codexRequestUserInputFeature) {
		t.Fatalf("callback-free task changed request-user-input feature: %v", withoutCallback)
	}

	withCallback := buildCodexArgs(ExecOptions{
		RequestUserInput: func(context.Context, PendingInputRequest) (PendingInputAnswer, error) {
			return PendingInputAnswer{}, errors.New("not called")
		},
		ExtraArgs: []string{"--disable", codexRequestUserInputFeature, "--disable", "memory_tool"},
		CustomArgs: []string{
			"-c", "features.default_mode_request_user_input=false",
			"--enable", "multi_agent",
		},
	}, slog.Default())
	want := []string{
		"app-server", "--listen", "stdio://",
		"--disable", "memory_tool",
		"--enable", "multi_agent",
		"--enable", codexRequestUserInputFeature,
	}
	if !reflect.DeepEqual(withCallback, want) {
		t.Fatalf("callback-enabled args = %v, want %v", withCallback, want)
	}
}

func TestCodexFeaturesListAdvertisesRequestUserInput(t *testing.T) {
	t.Parallel()

	if !codexFeaturesListAdvertises([]byte("default_mode_request_user_input  under development  false\n"), codexRequestUserInputFeature) {
		t.Fatal("supported feature was not detected")
	}
	if codexFeaturesListAdvertises([]byte("default_mode_request_user_input_v2 under development false\n"), codexRequestUserInputFeature) {
		t.Fatal("prefix match incorrectly treated as supported")
	}
}

func TestCodexEffectiveConfigRequiresRequestUserInputFeature(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		config map[string]any
		want   bool
	}{
		{name: "enabled", config: map[string]any{"features": map[string]any{codexRequestUserInputFeature: true}}, want: true},
		{name: "disabled", config: map[string]any{"features": map[string]any{codexRequestUserInputFeature: false}}},
		{name: "missing", config: map[string]any{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := codexEffectiveConfigHasFeature(tc.config, codexRequestUserInputFeature); got != tc.want {
				t.Fatalf("effective feature = %t, want %t", got, tc.want)
			}
		})
	}
}

func containsCodexFeatureOverride(args []string, feature string) bool {
	for i := 0; i+1 < len(args); i++ {
		if (args[i] == "--enable" || args[i] == "--disable") && args[i+1] == feature {
			return true
		}
	}
	return false
}
