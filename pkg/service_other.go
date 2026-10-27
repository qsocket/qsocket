//go:build !windows
// +build !windows

package qsock

import (
	"errors"

	"github.com/qsocket/qsocket/config"
)

// ErrNoServiceSupport is returned when the service control manager routines
// are used on a platform other than Windows.
var ErrNoServiceSupport = errors.New("Windows service mode is not supported on this platform")

// IsWindowsService always reports false on non-Windows platforms.
func IsWindowsService() bool {
	return false
}

// RunService is not supported on non-Windows platforms.
func RunService(opts *config.Options) error {
	return ErrNoServiceSupport
}
