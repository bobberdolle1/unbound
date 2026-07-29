//go:build !windows

package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

func prepareAssetRuntime() (*assetRuntimeWorkspace, error) {
	baseDir := os.TempDir()
	cleanupStaleUnixRuntimes(baseDir)
	dir, err := os.MkdirTemp(baseDir, fmt.Sprintf("unbound-runtime-%d-", os.Getpid()))
	if err != nil {
		return nil, fmt.Errorf("create private runtime directory: %w", err)
	}
	if err := os.Chmod(dir, 0700); err != nil {
		_ = os.RemoveAll(dir)
		return nil, fmt.Errorf("protect private runtime directory: %w", err)
	}
	return &assetRuntimeWorkspace{
		stagingDir: dir,
		finalDir:   dir,
		commit:     func() error { return nil },
		cleanup:    func() { _ = os.RemoveAll(dir) },
	}, nil
}

func cleanupStaleUnixRuntimes(baseDir string) {
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "unbound-runtime-") {
			continue
		}
		pidText := strings.SplitN(strings.TrimPrefix(entry.Name(), "unbound-runtime-"), "-", 2)[0]
		pid, err := strconv.Atoi(pidText)
		if err != nil || unixProcessAlive(pid) {
			continue
		}
		_ = os.RemoveAll(filepath.Join(baseDir, entry.Name()))
	}
}

func unixProcessAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || err == syscall.EPERM
}
