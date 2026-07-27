package engine

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

//go:embed core_bin/* lua_scripts/* lists/* windivert.filter/* ENGINE_ASSETS.sha256
var EmbeddedAssets embed.FS

type AssetPaths struct {
	BinDir  string
	LuaDir  string
	ListDir string
}

func ExtractAssets() (*AssetPaths, error) {
	tempDir := filepath.Join(os.TempDir(), "clearflow")
	binDir := filepath.Join(tempDir, "core_bin")
	luaDir := filepath.Join(tempDir, "lua_scripts")
	listDir := filepath.Join(tempDir, "lists")

	dirs := []string{binDir, luaDir, listDir}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create temp dir: %w", err)
		}
	}

	// Extract windivert.filter to config directory
	configDir, err := GetConfigDir()
	if err == nil {
		windivertDir := filepath.Join(configDir, "windivert.filter")
		if err := os.MkdirAll(windivertDir, 0755); err == nil {
			extractWinDivertFilters := func() error {
				entries, err := EmbeddedAssets.ReadDir("windivert.filter")
				if err != nil {
					return nil
				}
				for _, entry := range entries {
					if entry.IsDir() {
						continue
					}
					data, err := EmbeddedAssets.ReadFile("windivert.filter/" + entry.Name())
					if err != nil {
						continue
					}
					targetPath := filepath.Join(windivertDir, entry.Name())
					os.WriteFile(targetPath, data, 0644)
				}
				return nil
			}
			extractWinDivertFilters()
		}
	}

	// Helper to extract embedded files from a specific folder
	extract := func(sourcePrefix string, targetDir string) error {
		entries, err := EmbeddedAssets.ReadDir(sourcePrefix)
		if err != nil {
			// If directory doesn't exist in embed (e.g. no linux binaries yet), skip gracefully
			return nil
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			data, err := EmbeddedAssets.ReadFile(sourcePrefix + "/" + entry.Name())
			if err != nil {
				return err
			}
			targetPath := filepath.Join(targetDir, entry.Name())
			if err := os.WriteFile(targetPath, data, 0755); err != nil {
				// If the file is locked (e.g. driver in use), reuse it
				if _, statErr := os.Stat(targetPath); statErr == nil {
					continue
				}
				return err
			}
		}
		return nil
	}

	// Extract blob files from core_bin root (platform-independent)
	if err := extract("core_bin", binDir); err != nil {
		return nil, fmt.Errorf("failed to extract blob files: %w", err)
	}

	// Extract OS-specific binaries
	osBinDir := "core_bin/" + runtime.GOOS
	if err := extract(osBinDir, binDir); err != nil {
		return nil, fmt.Errorf("failed to extract %s binaries: %w", runtime.GOOS, err)
	}

	// Extract Lua scripts (platform independent)
	if err := extract("lua_scripts", luaDir); err != nil {
		return nil, fmt.Errorf("failed to extract lua scripts: %w", err)
	}

	// Extract hostlists and ipsets
	if err := extract("lists", listDir); err != nil {
		return nil, fmt.Errorf("failed to extract lists: %w", err)
	}

	// Copy missing default lists to user's persistent lists directory
	userListsDir, err := GetListsDir()
	if err == nil {
		entries, err := EmbeddedAssets.ReadDir("lists")
		if err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}
				destPath := filepath.Join(userListsDir, entry.Name())
				if _, err := os.Stat(destPath); os.IsNotExist(err) {
					data, err := EmbeddedAssets.ReadFile("lists/" + entry.Name())
					if err == nil {
						os.WriteFile(destPath, data, 0644)
					}
				}
			}
		}
	}

	return &AssetPaths{BinDir: binDir, LuaDir: luaDir, ListDir: listDir}, nil
}

type AssetVerificationResult struct {
	TotalFiles int    `json:"totalFiles"`
	Verified   bool   `json:"verified"`
	Error      string `json:"error"`
}

func VerifyAssets() AssetVerificationResult {
	manifestBytes, err := EmbeddedAssets.ReadFile("ENGINE_ASSETS.sha256")
	if err != nil {
		return AssetVerificationResult{Verified: false, Error: "Не удалось прочитать манифест ENGINE_ASSETS.sha256"}
	}

	lines := strings.Split(string(manifestBytes), "\n")
	total := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		expectedHash := parts[0]
		relPath := parts[1]

		embedPath := strings.TrimPrefix(relPath, "engine/")
		fileData, err := EmbeddedAssets.ReadFile(embedPath)
		if err != nil {
			return AssetVerificationResult{Verified: false, Error: fmt.Sprintf("Отсутствует файл %s", relPath)}
		}

		hash := sha256.Sum256(fileData)
		actualHash := hex.EncodeToString(hash[:])
		if actualHash != expectedHash {
			return AssetVerificationResult{Verified: false, Error: fmt.Sprintf("Хеш %s не совпадает", relPath)}
		}
		total++
	}

	return AssetVerificationResult{TotalFiles: total, Verified: true}
}
