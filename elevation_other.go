//go:build !windows

package main

func relaunchElevatedIfNeeded(bool) (bool, error) {
	return false, nil
}
