package daemon

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"
)

func TestRepositorySetupLockRegistryReleasesHistoricalKeys(t *testing.T) {
	root := t.TempDir()
	for i := range 256 {
		path := filepath.Join(root, fmt.Sprint(i))
		release, err := lockRepositorySetupArtifact(context.Background(), path)
		if err != nil {
			t.Fatal(err)
		}
		release()
		if _, exists := repositorySetupLocks.Load(path); exists {
			t.Fatal("released historical artifact remains in the lock registry")
		}
	}
}

func TestRepositorySetupLockRegistryKeepsLiveWaiters(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shared")
	var active atomic.Int32
	results := make(chan bool, 64)
	for range cap(results) {
		go func() {
			release, err := lockRepositorySetupArtifact(context.Background(), path)
			if err != nil {
				results <- false
				return
			}
			exclusive := active.Add(1) == 1
			runtime.Gosched()
			active.Add(-1)
			release()
			results <- exclusive
		}()
	}
	for range cap(results) {
		if !<-results {
			t.Error("artifact lock was not exclusive")
		}
	}
	if _, exists := repositorySetupLocks.Load(path); exists {
		t.Fatal("completed waiters remain in the registry")
	}
}

func TestRepositorySetupLockRegistryCancelledWaiterReleasesReference(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cancelled")
	release, err := lockRepositorySetupArtifact(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := lockRepositorySetupArtifact(ctx, path); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled waiter error: %v", err)
	}
	if _, exists := repositorySetupLocks.Load(path); !exists {
		t.Fatal("cancelled waiter removed the active owner's lock")
	}
	release()
	if _, exists := repositorySetupLocks.Load(path); exists {
		t.Fatal("cancelled waiter leaked a lock reference")
	}
}
