//go:build !darwin || !cgo

package applecontainer

import "fmt"

func newNativeClient() (nativeClient, error) {
	return nil, fmt.Errorf("applecontainer runtime requires macOS with CGO_ENABLED=1")
}
