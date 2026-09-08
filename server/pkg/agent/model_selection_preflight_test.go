package agent

import (
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestCodexModelSelectionPreflightUsesConfigReadModelBeforeThreadStart(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake app-server fixture uses a POSIX shell")
	}
	marker := filepath.Join(t.TempDir(), "thread-started")
	fakePath := writeFakeCodexAppServer(t, ""+
		`read line`+"\n"+
		`echo '{"jsonrpc":"2.0","id":1,"result":{}}'`+"\n"+
		`read line`+"\n"+
		`read line`+"\n"+
		codexConfigReadEcho(t, 2, map[string]any{"model": "configured-model"}, nil)+"\n"+
		`if read line; then printf reached > `+shellQuote(marker)+`; fi`+"\n")

	called := 0
	result := executeFakeCodexWithConfig(t, fakePath, Config{
		Logger: slog.Default(),
		Env:    map[string]string{"CODEX_HOME": t.TempDir(), "HOME": t.TempDir()},
	}, ExecOptions{
		Cwd:              t.TempDir(),
		Timeout:          3 * time.Second,
		HandshakeTimeout: time.Second,
		ValidateResolvedModelSelection: func(model string) error {
			called++
			if model != "configured-model" {
				t.Fatalf("preflight model = %q, want config/read model", model)
			}
			return errors.New("selected model does not support requested thinking_level")
		},
	})
	if result.Status != "failed" || !strings.Contains(result.Error, "requested thinking_level") {
		t.Fatalf("model selection preflight result = %+v", result)
	}
	if called != 1 {
		t.Fatalf("preflight calls = %d, want 1", called)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("thread/start was sent after failed model preflight: %v", err)
	}
}
