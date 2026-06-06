//go:build !windows

package platform

import "context"

// IsWindowsService always returns false on non-Windows platforms.
func IsWindowsService() bool {
	return false
}

// RunAsWindowsService is never called on non-Windows platforms.
func RunAsWindowsService(_ string, _ func(ctx context.Context) error) error {
	panic("RunAsWindowsService is only available on Windows")
}
