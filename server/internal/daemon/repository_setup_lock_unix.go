//go:build !windows

package daemon

import (
	"context"
	"errors"
	"os"
	"syscall"
	"time"
)

func lockRepositorySetupFile(ctx context.Context, path string, wait bool) (func(), bool, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, false, err
	}
	for {
		err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		switch {
		case err == nil:
			return func() {
				_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
				_ = file.Close()
			}, true, nil
		case errors.Is(err, syscall.EWOULDBLOCK), errors.Is(err, syscall.EAGAIN):
			if !wait {
				_ = file.Close()
				return nil, false, nil
			}
		case err != nil:
			_ = file.Close()
			return nil, false, err
		}
		select {
		case <-ctx.Done():
			_ = file.Close()
			return nil, false, context.Cause(ctx)
		case <-time.After(25 * time.Millisecond):
		}
	}
}
