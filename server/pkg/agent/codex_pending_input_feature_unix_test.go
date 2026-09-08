//go:build !windows

package agent

import (
	"bufio"
	"bytes"
	"context"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCodexRequestUserInputFeatureProbeIsCached(t *testing.T) {
	t.Parallel()

	countPath := filepath.Join(t.TempDir(), "count")
	scriptPath := filepath.Join(t.TempDir(), "codex")
	script := "#!/bin/sh\n" +
		"printf 'probe\\n' >> \"$COUNT_PATH\"\n" +
		"printf 'default_mode_request_user_input under development false\\n'\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	b := &codexBackend{cfg: Config{Env: map[string]string{"COUNT_PATH": countPath}}}
	for range 2 {
		if supported, diagnostic := b.requestUserInputFeatureSupport(t.Context(), NewCommand(scriptPath, nil)); !supported {
			t.Fatalf("feature unsupported: %s", diagnostic)
		}
	}
	data, err := os.ReadFile(countPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(data), "probe\n"); got != 1 {
		t.Fatalf("feature probes = %d, want 1", got)
	}
}

func TestCodexRequestUserInputFeatureProbeRejectsUnsupportedCLI(t *testing.T) {
	t.Parallel()

	scriptPath := filepath.Join(t.TempDir(), "codex")
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\nprintf 'other_feature stable true\\n'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	b := &codexBackend{cfg: Config{}}
	supported, diagnostic := b.requestUserInputFeatureSupport(t.Context(), NewCommand(scriptPath, nil))
	if supported || !strings.Contains(diagnostic, codexRequestUserInputFeature) || !strings.Contains(diagnostic, "does not advertise") {
		t.Fatalf("unsupported CLI result = %t, %q", supported, diagnostic)
	}
}

func TestCodexRequestUserInputFeatureProbeFailureIsActionable(t *testing.T) {
	t.Parallel()

	scriptPath := filepath.Join(t.TempDir(), "codex")
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\nexit 2\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	b := &codexBackend{cfg: Config{}}
	supported, diagnostic := b.requestUserInputFeatureSupport(t.Context(), NewCommand(scriptPath, nil))
	if supported || !strings.Contains(diagnostic, "could not verify") || !strings.Contains(diagnostic, codexRequestUserInputFeature) {
		t.Fatalf("probe failure result = %t, %q", supported, diagnostic)
	}
}

func TestCodexUnsupportedRequestUserInputFeatureContinuesOrdinaryTask(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name         string
		probeCommand string
		diagnostic   string
	}{
		{name: "unsupported", probeCommand: `printf 'other_feature stable true\n'; exit 0`, diagnostic: "does not advertise feature"},
		{name: "unknown", probeCommand: `exit 2`, diagnostic: "could not verify"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			testCodexUnavailableRequestUserInputFeatureContinues(t, tc.probeCommand, tc.diagnostic)
		})
	}
}

func testCodexUnavailableRequestUserInputFeatureContinues(t *testing.T, probeCommand, diagnostic string) {
	t.Helper()

	argsPath := filepath.Join(t.TempDir(), "args")
	fakePath := filepath.Join(t.TempDir(), "codex")
	script := "#!/bin/sh\n" +
		`if [ "$1" = "features" ]; then ` + probeCommand + `; fi` + "\n" +
		`printf '%s\n' "$@" > "$ARGS_PATH"` + "\n" +
		`read line` + "\n" +
		`echo '{"jsonrpc":"2.0","id":1,"result":{}}'` + "\n" +
		`read line` + "\n" +
		`read line` + "\n" +
		`echo '{"jsonrpc":"2.0","id":2,"result":{"thread":{"id":"thread-compatible"}}}'` + "\n" +
		`read line` + "\n" +
		`echo '{"jsonrpc":"2.0","id":3,"result":{}}'` + "\n" +
		`echo '{"jsonrpc":"2.0","method":"turn/started","params":{"threadId":"thread-compatible","turn":{"id":"turn-compatible"}}}'` + "\n" +
		`echo '{"jsonrpc":"2.0","method":"item/completed","params":{"threadId":"thread-compatible","turnId":"turn-compatible","item":{"type":"agentMessage","id":"answer","text":"done","phase":"final_answer"}}}'` + "\n" +
		`echo '{"jsonrpc":"2.0","method":"turn/completed","params":{"threadId":"thread-compatible","turn":{"id":"turn-compatible","status":"completed"}}}'` + "\n"
	if err := os.WriteFile(fakePath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	var logs bytes.Buffer
	result, _ := executeFakeCodexCollectingMessagesWithConfig(t, fakePath, Config{
		Env:    map[string]string{"ARGS_PATH": argsPath},
		Logger: slog.New(slog.NewTextHandler(&logs, nil)),
	}, ExecOptions{
		Timeout: 5 * time.Second, SemanticInactivityTimeout: 5 * time.Second,
		RequestUserInput: func(context.Context, PendingInputRequest) (PendingInputAnswer, error) {
			t.Fatal("unsupported CLI must not retain the pending-input callback")
			return PendingInputAnswer{}, nil
		},
	}, 10*time.Second)
	if result.Status != "completed" || result.Output != "done" {
		t.Fatalf("ordinary task did not continue: %+v", result)
	}
	args, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(args), codexRequestUserInputFeature) {
		t.Fatalf("unsupported task enabled request-user-input feature: %s", args)
	}
	if !strings.Contains(logs.String(), "continuing without question bridge") ||
		!strings.Contains(logs.String(), diagnostic) {
		t.Fatalf("missing compatibility diagnostic: %s", logs.String())
	}
}

func TestInstalledCodexRequestUserInputFeatureOptInWithoutModelCall(t *testing.T) {
	if os.Getenv("MULTICA_TEST_REAL_CODEX_CONFIG_READ") != "1" {
		t.Skip("set MULTICA_TEST_REAL_CODEX_CONFIG_READ=1 to run the local no-model Codex config/read probe")
	}
	if testing.Short() {
		t.Skip("local CLI integration")
	}
	path, err := exec.LookPath("codex")
	if err != nil {
		t.Skip("codex CLI is not installed")
	}
	for _, tc := range []struct {
		name string
		args []string
		want bool
	}{
		{name: "callback disabled", want: false},
		{name: "callback enabled", args: []string{"--enable", codexRequestUserInputFeature}, want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			effective := readInstalledCodexConfigWithoutModelCall(t, path, tc.args...)
			if got := codexEffectiveConfigHasFeature(effective.Config, codexRequestUserInputFeature); got != tc.want {
				t.Fatalf("effective %s = %t, want %t; features=%#v", codexRequestUserInputFeature, got, tc.want, effective.Config["features"])
			}
		})
	}
}

func readInstalledCodexConfigWithoutModelCall(t *testing.T, path string, launchArgs ...string) codexConfigReadResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	args := append([]string{"app-server", "--listen", "stdio://"}, launchArgs...)
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Env = append(os.Environ(), "CODEX_HOME="+t.TempDir())
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	processDone := make(chan struct{})
	c := &codexClient{
		stdin:            &lockedWriter{writer: stdin},
		pending:          make(map[int]*pendingRPC),
		processDone:      processDone,
		handshakeTimeout: 5 * time.Second,
	}
	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			c.handleLine(scanner.Text())
		}
	}()
	waitDone := make(chan error, 1)
	go func() {
		err := cmd.Wait()
		close(processDone)
		waitDone <- err
	}()

	_, err = c.request(ctx, "initialize", map[string]any{
		"clientInfo":   map[string]any{"name": "multica-feature-test", "version": "1"},
		"capabilities": map[string]any{"experimentalApi": true},
	})
	if err != nil {
		t.Fatalf("initialize: %v; stderr=%s", err, stderr.String())
	}
	c.notify("initialized")
	effective, err := readCodexConfiguration(ctx, c, "", codexConfigurationReadPurposeNeutral)
	if err != nil {
		t.Fatalf("config/read: %v; stderr=%s", err, stderr.String())
	}

	_ = stdin.Close()
	select {
	case <-readerDone:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	select {
	case <-waitDone:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	return effective
}
