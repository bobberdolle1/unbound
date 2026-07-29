package engine

import (
	"fmt"
	"os"
)

type assetRuntimeWorkspace struct {
	stagingDir string
	finalDir   string
	commit     func() error
	cleanup    func()
}

func newPrivateTempWorkspace() (*assetRuntimeWorkspace, error) {
	dir, err := os.MkdirTemp("", "unbound-runtime-")
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
