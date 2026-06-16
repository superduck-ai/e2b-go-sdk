//go:build darwin && cgo

package applecontainer

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

const operationLockPoll = 100 * time.Millisecond

func acquireOperationLock(ctx context.Context) (func(), error) {
	lockPath := filepath.Join(os.TempDir(), "e2b-applecontainer-operation.lock")
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	for {
		err = syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			return func() {
				_ = syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)
				_ = lockFile.Close()
			}, nil
		}
		if err != syscall.EWOULDBLOCK && err != syscall.EAGAIN {
			_ = lockFile.Close()
			return nil, err
		}

		timer := time.NewTimer(operationLockPoll)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			_ = lockFile.Close()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}
