//go:build !darwin || !cgo

package applecontainer

import "context"

func acquireOperationLock(ctx context.Context) (func(), error) {
	return func() {}, nil
}
