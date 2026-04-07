//go:build !windows
// +build !windows

package engine

// RunHealthCheck is a no-op on non-Windows platforms.
func RunHealthCheck() error {
	return nil
}
