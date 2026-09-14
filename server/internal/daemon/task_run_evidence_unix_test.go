//go:build !windows

package daemon

import (
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestVerifiedRuntimeIdentityRejectsFIFOWithoutOpeningIt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runtime-fifo")
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Fatalf("create fifo: %v", err)
	}

	returned := make(chan struct{})
	go func() {
		_, _, _ = verifiedRuntimeIdentity("0.153.4", path)
		close(returned)
	}()
	select {
	case <-returned:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("runtime identity opened a known non-regular FIFO")
	}
}
