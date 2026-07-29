package engine

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

//go:embed core_bin/* lua_scripts/* lists/* windivert.filter/* ENGINE_ASSETS.sha256
var EmbeddedAssets embed.FS

type AssetPaths struct {
	RootDir      string
	BinDir       string
	LuaDir       string
	ListDir      string
	EngineSHA256 string

	extractedFiles map[string]string
}

var (
	extractAssetsOnce sync.Once
	extractedAssets   *AssetPaths
	extractAssetsErr  error
)

func ExtractAssets() (*AssetPaths, error) {
	extractAssetsOnce.Do(func() {
		extractedAssets, extractAssetsErr = extractAssets()
	})
	return extractedAssets, extractAssetsErr
}

// CleanupExtractedAssets removes the per-process runtime after every provider
// has stopped. Retries cover the short interval in which WinDivert releases
// mapped DLL/driver files after the child process exits.
func CleanupExtractedAssets() error {
	if extractedAssets == nil || extractedAssets.RootDir == "" {
		return nil
	}
	var lastErr error
	for range 10 {
		if err := os.RemoveAll(extractedAssets.RootDir); err == nil {
			return nil
		} else {
			lastErr = err
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("remove extracted runtime %s: %w", extractedAssets.RootDir, lastErr)
}

func extractAssets() (*AssetPaths, error) {
	workspace, err := prepareAssetRuntime()
	if err != nil {
		return nil, err
	}
	committed := false
	defer func() {
		if !committed {
			workspace.cleanup()
		}
	}()

	binDir := filepath.Join(workspace.stagingDir, "core_bin")
	luaDir := filepath.Join(workspace.stagingDir, "lua_scripts")
	listDir := filepath.Join(workspace.stagingDir, "lists")
	for _, dir := range []string{binDir, luaDir, listDir} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return nil, fmt.Errorf("create runtime asset directory: %w", err)
		}
	}

	extracted := make(map[string]string)
	extractPrefix := func(sourcePrefix, targetDir string) error {
		entries, err := EmbeddedAssets.ReadDir(sourcePrefix)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			sourcePath := sourcePrefix + "/" + entry.Name()
			data, err := EmbeddedAssets.ReadFile(sourcePath)
			if err != nil {
				return fmt.Errorf("read embedded asset %s: %w", sourcePath, err)
			}
			targetPath := filepath.Join(targetDir, entry.Name())
			expectedHash := sha256Hex(data)
			if err := writeFileAtomicVerified(targetPath, data, 0700, expectedHash); err != nil {
				return fmt.Errorf("extract %s: %w", sourcePath, err)
			}
			extracted[targetPath] = expectedHash
		}
		return nil
	}

	if err := extractPrefix("core_bin", binDir); err != nil {
		return nil, err
	}
	if err := extractPrefix(platformAssetDirectory(), binDir); err != nil {
		return nil, err
	}
	if err := extractPrefix("lua_scripts", luaDir); err != nil {
		return nil, err
	}
	if err := extractPrefix("lists", listDir); err != nil {
		return nil, err
	}

	if err := extractWinDivertFilters(); err != nil {
		return nil, err
	}
	if err := copyDefaultUserLists(); err != nil {
		return nil, err
	}
	if err := verifyExtractedFiles(extracted); err != nil {
		return nil, err
	}
	if err := workspace.commit(); err != nil {
		return nil, err
	}
	committed = true

	finalFiles := make(map[string]string, len(extracted))
	for stagingPath, expectedHash := range extracted {
		relPath, err := filepath.Rel(workspace.stagingDir, stagingPath)
		if err != nil {
			return nil, fmt.Errorf("resolve activated asset path: %w", err)
		}
		finalFiles[filepath.Join(workspace.finalDir, relPath)] = expectedHash
	}

	paths := &AssetPaths{
		RootDir:        workspace.finalDir,
		BinDir:         filepath.Join(workspace.finalDir, "core_bin"),
		LuaDir:         filepath.Join(workspace.finalDir, "lua_scripts"),
		ListDir:        filepath.Join(workspace.finalDir, "lists"),
		extractedFiles: finalFiles,
	}
	enginePath := filepath.Join(paths.BinDir, platformEngineBinary())
	paths.EngineSHA256 = finalFiles[enginePath]
	if err := verifyExtractedFiles(paths.extractedFiles); err != nil {
		return nil, err
	}
	return paths, nil
}

func extractWinDivertFilters() error {
	configDir, err := GetConfigDir()
	if err != nil {
		return fmt.Errorf("get WinDivert filter directory: %w", err)
	}
	windivertDir := filepath.Join(configDir, "windivert.filter")
	if err := os.MkdirAll(windivertDir, 0700); err != nil {
		return fmt.Errorf("create WinDivert filter directory: %w", err)
	}
	entries, err := EmbeddedAssets.ReadDir("windivert.filter")
	if err != nil {
		return fmt.Errorf("read embedded WinDivert filters: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		data, err := EmbeddedAssets.ReadFile("windivert.filter/" + entry.Name())
		if err != nil {
			return err
		}
		if err := writeFileAtomicVerified(filepath.Join(windivertDir, entry.Name()), data, 0600, sha256Hex(data)); err != nil {
			return fmt.Errorf("extract WinDivert filter %s: %w", entry.Name(), err)
		}
	}
	return nil
}

func copyDefaultUserLists() error {
	userListsDir, err := GetListsDir()
	if err != nil {
		return fmt.Errorf("get persistent lists directory: %w", err)
	}
	entries, err := EmbeddedAssets.ReadDir("lists")
	if err != nil {
		return fmt.Errorf("read embedded lists: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		destPath := filepath.Join(userListsDir, entry.Name())
		if _, err := os.Stat(destPath); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("inspect persistent list %s: %w", entry.Name(), err)
		}
		data, err := EmbeddedAssets.ReadFile("lists/" + entry.Name())
		if err != nil {
			return err
		}
		if err := writeFileAtomicVerified(destPath, data, 0600, sha256Hex(data)); err != nil {
			return fmt.Errorf("create persistent list %s: %w", entry.Name(), err)
		}
	}
	return nil
}

func writeFileAtomicVerified(targetPath string, data []byte, mode fs.FileMode, expectedHash string) error {
	if err := os.MkdirAll(filepath.Dir(targetPath), 0700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(targetPath), ".asset-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	actualHash, err := hashFileSHA256(tmpPath)
	if err != nil {
		return err
	}
	if actualHash != expectedHash {
		return fmt.Errorf("staged hash mismatch: got %s, want %s", actualHash, expectedHash)
	}
	if err := os.Remove(targetPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove previous asset: %w", err)
	}
	if err := os.Rename(tmpPath, targetPath); err != nil {
		return fmt.Errorf("activate staged asset: %w", err)
	}
	actualHash, err = hashFileSHA256(targetPath)
	if err != nil {
		return err
	}
	if actualHash != expectedHash {
		_ = os.Remove(targetPath)
		return fmt.Errorf("activated hash mismatch: got %s, want %s", actualHash, expectedHash)
	}
	return nil
}

func verifyExtractedFiles(files map[string]string) error {
	for path, expectedHash := range files {
		actualHash, err := hashFileSHA256(path)
		if err != nil {
			return fmt.Errorf("verify extracted asset %s: %w", path, err)
		}
		if actualHash != expectedHash {
			return fmt.Errorf("extracted asset hash mismatch for %s", path)
		}
	}
	return nil
}

func VerifyExtractedAssets(paths *AssetPaths) error {
	if paths == nil {
		return fmt.Errorf("asset paths are not initialized")
	}
	return verifyExtractedFiles(paths.extractedFiles)
}

func platformAssetDirectory() string {
	if runtime.GOOS == "linux" {
		return "core_bin/linux/" + runtime.GOARCH
	}
	return "core_bin/" + runtime.GOOS
}

func platformEngineBinary() string {
	switch runtime.GOOS {
	case "windows":
		return "winws2.exe"
	case "darwin":
		return "tpws"
	default:
		return "nfqws2"
	}
}

type AssetVerificationResult struct {
	TotalFiles int    `json:"totalFiles"`
	Verified   bool   `json:"verified"`
	Error      string `json:"error"`
}

func VerifyAssets() AssetVerificationResult {
	total, err := verifyEmbeddedAssets()
	if err != nil {
		return AssetVerificationResult{Verified: false, Error: err.Error()}
	}
	paths, err := ExtractAssets()
	if err != nil {
		return AssetVerificationResult{TotalFiles: total, Verified: false, Error: fmt.Sprintf("Не удалось безопасно извлечь ассеты: %v", err)}
	}
	if err := VerifyExtractedAssets(paths); err != nil {
		return AssetVerificationResult{TotalFiles: total, Verified: false, Error: err.Error()}
	}
	return AssetVerificationResult{TotalFiles: total, Verified: true}
}

func verifyEmbeddedAssets() (int, error) {
	manifestBytes, err := EmbeddedAssets.ReadFile("ENGINE_ASSETS.sha256")
	if err != nil {
		return 0, fmt.Errorf("не удалось прочитать манифест ENGINE_ASSETS.sha256")
	}

	expectedFiles := make(map[string]bool)
	total := 0
	for _, line := range strings.Split(string(manifestBytes), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) != 2 {
			return total, fmt.Errorf("некорректная строка манифеста: %q", line)
		}
		expectedHash := strings.ToLower(parts[0])
		relPath := filepath.ToSlash(parts[1])
		if strings.Contains(relPath, "..") || !strings.HasPrefix(relPath, "engine/") {
			return total, fmt.Errorf("небезопасный путь в манифесте: %s", relPath)
		}
		embedPath := strings.TrimPrefix(relPath, "engine/")
		fileData, err := EmbeddedAssets.ReadFile(embedPath)
		if err != nil {
			return total, fmt.Errorf("отсутствует файл %s", relPath)
		}
		if actualHash := sha256Hex(fileData); actualHash != expectedHash {
			return total, fmt.Errorf("хеш %s не совпадает", relPath)
		}
		expectedFiles[embedPath] = true
		total++
	}

	for _, root := range []string{"core_bin", "lua_scripts", "windivert.filter"} {
		err := fs.WalkDir(EmbeddedAssets, root, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if !entry.IsDir() && !expectedFiles[path] {
				return fmt.Errorf("ассет отсутствует в манифесте: engine/%s", path)
			}
			return nil
		})
		if err != nil {
			return total, err
		}
	}
	return total, nil
}

func hashFileSHA256(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return sha256Hex(data), nil
}

func sha256Hex(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}
