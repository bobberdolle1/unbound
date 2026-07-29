//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unbound/engine"
	"unbound/engine/providers"
	"unsafe"
)

func attachConsole() {
	fi, err := os.Stdout.Stat()
	if err == nil && (fi.Mode()&os.ModeCharDevice) == 0 {
		// stdout is piped or redirected; keep the pipe so CLI tools and tests receive output
		return
	}

	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	attachConsoleProc := kernel32.NewProc("AttachConsole")
	r1, _, _ := attachConsoleProc.Call(uintptr(0xFFFFFFFF)) // ATTACH_PARENT_PROCESS = -1
	if r1 == 0 {
		return
	}

	// Reopen stdout and stderr to ensure output works when launched from a GUI app console
	stdout, _ := os.OpenFile("CONOUT$", os.O_WRONLY, 0)
	if stdout != nil {
		os.Stdout = stdout
		os.Stderr = stdout
	}
}

func relaunchElevatedIfNeeded(required bool) (bool, error) {
	if !required {
		return false, nil
	}
	elevated, err := checkAdminPrivileges()
	if err != nil {
		return false, err
	}
	if elevated {
		return false, nil
	}

	executable, err := os.Executable()
	if err != nil {
		return false, err
	}
	verb, _ := syscall.UTF16PtrFromString("runas")
	file, _ := syscall.UTF16PtrFromString(executable)
	parameters, _ := syscall.UTF16PtrFromString(commandLine(os.Args[1:]))
	directory, _ := syscall.UTF16PtrFromString(filepath.Dir(executable))
	shellExecute := syscall.NewLazyDLL("shell32.dll").NewProc("ShellExecuteW")
	result, _, _ := shellExecute.Call(
		0,
		uintptr(unsafe.Pointer(verb)),
		uintptr(unsafe.Pointer(file)),
		uintptr(unsafe.Pointer(parameters)),
		uintptr(unsafe.Pointer(directory)),
		1,
	)
	if result <= 32 {
		return false, fmt.Errorf("ShellExecuteW failed with code %d", result)
	}
	return true, nil
}

func commandLine(args []string) string {
	var command strings.Builder
	for i, arg := range args {
		if i > 0 {
			command.WriteByte(' ')
		}
		command.WriteString(syscall.EscapeArg(arg))
	}
	return command.String()
}

func registerHeadlessProvider(manager *providers.ProviderManager, assets *engine.AssetPaths, listsDir string, debugMode bool) {
	provider := providers.NewZapret2WindowsProvider(assets.BinDir, assets.LuaDir, listsDir, assets.EngineSHA256, debugMode, true)

	engine.RegisterWindowsProfileCatalog(provider, assets.LuaDir)

	manager.Register(provider)
}
