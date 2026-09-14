//go:build !windows

package daemon

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestOpenIssueOutcomeArtifactDoesNotBlockOnFIFO(t *testing.T) {
	path := filepath.Join(t.TempDir(), "issue-outcome.json")
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Skipf("FIFO unsupported: %v", err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := readIssueOutcomeArtifact(path, Task{}, "final")
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("issue outcome FIFO was accepted")
		}
	case <-time.After(time.Second):
		t.Fatal("opening issue outcome FIFO blocked")
	}
}

func TestOpenIssueOutcomeArtifactRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.json")
	if err := os.WriteFile(target, []byte(`{"version":1,"outcome":"blocked"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "issue-outcome.json")
	if err := os.Symlink(target, path); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	_, err := readIssueOutcomeArtifact(path, Task{}, "final")
	if err == nil {
		t.Fatal("read issue outcome through symlink")
	}
}
