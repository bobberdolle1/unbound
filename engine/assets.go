package engine

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

//go:embed core_bin/* lua_scripts/* lists/*
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

	// Extract OS-specific binaries
	osBinDir := "core_bin/" + runtime.GOOS
	if err := extract(osBinDir, binDir); err != nil {
		return nil, fmt.Errorf("failed to extract %s binaries: %w", runtime.GOOS, err)
	}

	if runtime.GOOS == "windows" {
		winws2Path := filepath.Join(binDir, "winws2.exe")
		winwsPath := filepath.Join(binDir, "winws.exe")
		if _, err := os.Stat(winws2Path); err == nil {
			os.Rename(winws2Path, winwsPath)
		}
	}

	// Extract Lua scripts (platform independent)
	if err := extract("lua_scripts", luaDir); err != nil {
		return nil, fmt.Errorf("failed to extract lua scripts: %w", err)
	}

	// Extract hostlists and ipsets
	if err := extract("lists", listDir); err != nil {
		return nil, fmt.Errorf("failed to extract lists: %w", err)
	}

	return &AssetPaths{BinDir: binDir, LuaDir: luaDir, ListDir: listDir}, nil
}
