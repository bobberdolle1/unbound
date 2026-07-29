//go:build !windows

package engine

import "syscall"

func GetHiddenSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{}
}

// AddDefenderExclusion is a no-op on non-Windows platforms
func AddDefenderExclusion() error {
	return nil
}
