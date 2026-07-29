package providers

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Engine binary names per platform.
//
// Only Windows embeds its engine (winws2.exe plus the WinDivert driver live in
// engine/core_bin/windows). Linux uses nfqws against NFQUEUE; macOS uses tpws
// as a transparent TCP proxy with pf route-to/rdr rules. Both come from the
// user's package manager, so they are looked up at runtime.
//
// These are declared here, in an untagged file, so code that has to reason
// about every platform at once - such as the startup validator - can name them.
const (
	WindowsEngineBinary = "winws2.exe"
	LinuxEngineBinary   = "nfqws2"
	MacOSEngineBinary   = "tpws"
)

// EngineBinaryName returns the engine executable expected on the host OS.
func EngineBinaryName() string {
	switch runtime.GOOS {
	case "windows":
		return WindowsEngineBinary
	case "darwin":
		return MacOSEngineBinary
	default:
		return LinuxEngineBinary
	}
}

// searchPathsByOS lists the well-known locations a Zapret build installs into,
// beyond whatever is in $PATH.
var searchPathsByOS = map[string][]string{
	"linux": {
		"/usr/local/bin",
		"/usr/bin",
		"/opt/zapret",
		"/opt/zapret/binaries/x86_64",
		"/usr/lib/zapret",
	},
	"darwin": {
		"/opt/homebrew/bin",
		"/opt/homebrew/sbin",
		"/opt/homebrew/opt/zapret/bin",
		"/opt/homebrew/opt/zapret/sbin",
		"/usr/local/bin",
		"/usr/local/sbin",
		"/usr/local/opt/zapret/bin",
		"/usr/local/opt/zapret/sbin",
		"/opt/zapret",
		"/opt/zapret/bin",
		"/opt/zapret/binaries/x86_64",
		"/opt/zapret/binaries/aarch64",
		"/opt/zapret/binaries/mac64",
	},
}

// ResolveEngineBinary locates the bypass engine executable.
//
// Only the Windows build embeds its engine (winws2.exe and the WinDivert
// driver live in engine/core_bin/windows). On Linux the nfqws build and on
// macOS the tpws build are architecture-specific and GPL-licensed, so they are
// installed by the user's package manager instead of being vendored. Previously
// nothing looked outside the extracted asset directory, so those platforms
// could never find an engine no matter how it was installed.
//
// Search order:
//  1. the extracted asset directory, so a bundled build always wins
//  2. next to the running executable, for portable/AppImage-style layouts
//  3. $PATH
//  4. the distribution's usual install prefixes
func ResolveEngineBinary(name string, assetBinDir string) (string, error) {
	if runtime.GOOS == "windows" && !strings.HasSuffix(name, ".exe") {
		name += ".exe"
	}

	var checked []string

	consider := func(dir string) (string, bool) {
		if dir == "" {
			return "", false
		}
		candidate := filepath.Join(dir, name)
		checked = append(checked, candidate)
		info, err := os.Stat(candidate)
		if err != nil || info.IsDir() {
			return "", false
		}
		if runtime.GOOS != "windows" && info.Mode().Perm()&0111 == 0 {
			// Present but not executable - extraction dropped the bit, or the
			// user copied it off a FAT volume. Try to fix it rather than
			// reporting the binary as missing.
			if err := os.Chmod(candidate, info.Mode().Perm()|0755); err != nil {
				return "", false
			}
		}
		return candidate, true
	}

	if path, ok := consider(assetBinDir); ok {
		return path, nil
	}

	if exe, err := os.Executable(); err == nil {
		if path, ok := consider(filepath.Dir(exe)); ok {
			return path, nil
		}
	}

	if path, err := exec.LookPath(name); err == nil {
		return path, nil
	}
	checked = append(checked, name+" (в $PATH)")

	for _, dir := range searchPathsByOS[runtime.GOOS] {
		if path, ok := consider(dir); ok {
			return path, nil
		}
	}

	return "", fmt.Errorf("не найден бинарник движка %q; проверены пути: %s",
		name, strings.Join(checked, ", "))
}
