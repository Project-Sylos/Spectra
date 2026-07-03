//go:build !linux && !darwin

package mount

import (
	"fmt"
	"runtime"
)

// MountServer is not supported on this platform.
func MountServer(mountPoint string, backend Backend) (*FUSEMount, error) {
	return nil, fmt.Errorf("FUSE mount is not supported on %s/%s (requires Linux or macOS with FUSE)", runtime.GOOS, runtime.GOARCH)
}

// IsNotSupported returns true on unsupported platforms.
func IsNotSupported() bool {
	return true
}
