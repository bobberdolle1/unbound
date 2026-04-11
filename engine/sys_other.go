//go:build !windows

package engine

import "syscall"

func GetHiddenSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{}
}
