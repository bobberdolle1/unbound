package providers

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

func ValidateBinaries(binPath string) error {
	var requiredFiles []string

	switch runtime.GOOS {
	case "windows":
		requiredFiles = []string{
			"nfqws.exe",
			"WinDivert.dll",
			"WinDivert64.sys",
		}
	case "linux":
		requiredFiles = []string{"nfqws"}
	case "darwin":
		requiredFiles = []string{"nfqws"}
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}

	missing := []string{}
	for _, file := range requiredFiles {
		fullPath := filepath.Join(binPath, file)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			missing = append(missing, file)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing binaries: %v", missing)
	}

	return nil
}
