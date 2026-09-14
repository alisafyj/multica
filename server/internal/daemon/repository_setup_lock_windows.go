//go:build windows

package daemon

import (
	"context"
	"errors"
	"os"
	"time"

	"golang.org/x/sys/windows"
)

func lockRepositorySetupFile(ctx context.Context, path string, wait bool) (func(), bool, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, false, err
	}
	handle, err := windows.CreateFile(
		p,
		windows.GENERIC_READ|windows.GENERIC_WRITE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_ALWAYS,
		windows.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		return nil, false, err
	}
	file := os.NewFile(uintptr(handle), path)
	for {
		overlapped := new(windows.Overlapped)
		err = windows.LockFileEx(handle, windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, overlapped)
		switch {
		case err == nil:
			return func() {
				_ = windows.UnlockFileEx(handle, 0, 1, 0, new(windows.Overlapped))
				_ = file.Close()
			}, true, nil
		case errors.Is(err, windows.ERROR_LOCK_VIOLATION), errors.Is(err, windows.ERROR_IO_PENDING):
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
