package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		input int64
		want  string
	}{
		{500, "500 байт"},
		{1024, "1 КБ"},
		{2048, "2 КБ"},
		{10 * 1024 * 1024, "10.0 МБ"},
		{1536 * 1024 * 1024, "1.50 ГБ"},
	}

	for _, tc := range tests {
		got := formatBytes(tc.input)
		if got != tc.want {
			t.Errorf("formatBytes(%d) = %q; want %q", tc.input, got, tc.want)
		}
	}
}

func TestComputeDirSize(t *testing.T) {
	tempDir := t.TempDir()
	sub := filepath.Join(tempDir, "test_sub")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}

	f1 := filepath.Join(tempDir, "file1.txt")
	f2 := filepath.Join(sub, "file2.txt")

	_ = os.WriteFile(f1, []byte("12345"), 0644)       // 5 bytes
	_ = os.WriteFile(f2, []byte("1234567890"), 0644)  // 10 bytes

	bytes, files := computeDirSize(tempDir)
	if bytes != 15 {
		t.Errorf("computeDirSize bytes = %d; want 15", bytes)
	}
	if files != 2 {
		t.Errorf("computeDirSize files = %d; want 2", files)
	}
}

func TestClearDiscordCacheMockDirectories(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("APPDATA", tempHome)
	t.Setenv("HOME", tempHome)
	t.Setenv("XDG_CONFIG_HOME", tempHome)

	// Create mock Discord Stable and Canary installations
	stableRoot := filepath.Join(tempHome, "discord")
	canaryRoot := filepath.Join(tempHome, "discordcanary")

	cacheDir := filepath.Join(stableRoot, "Cache")
	codeCacheDir := filepath.Join(stableRoot, "Code Cache")
	localStorageDir := filepath.Join(stableRoot, "Local Storage") // Sensitive: must NOT be touched!

	canaryGpuCache := filepath.Join(canaryRoot, "GPUCache")

	for _, d := range []string{cacheDir, codeCacheDir, localStorageDir, canaryGpuCache} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}

	// Write mock data
	_ = os.WriteFile(filepath.Join(cacheDir, "data_0"), []byte(strings.Repeat("A", 1024)), 0644)
	_ = os.WriteFile(filepath.Join(codeCacheDir, "code_0"), []byte(strings.Repeat("B", 2048)), 0644)
	_ = os.WriteFile(filepath.Join(canaryGpuCache, "gpu_0"), []byte(strings.Repeat("C", 4096)), 0644)

	sensitiveFile := filepath.Join(localStorageDir, "tokens.ldb")
	_ = os.WriteFile(sensitiveFile, []byte("sensitive_account_data"), 0644)

	// Run cleaner
	res := ClearDiscordCacheStructured()

	// Verify status
	if res.Status != "SUCCESS" {
		t.Errorf("Expected status SUCCESS, got %s (msg: %s)", res.Status, res.Message)
	}
	if res.BytesFreed < 7168 {
		t.Errorf("Expected at least 7168 bytes freed, got %d", res.BytesFreed)
	}

	// Verify sensitive directory was preserved
	if _, err := os.Stat(sensitiveFile); os.IsNotExist(err) {
		t.Fatal("CRITICAL: Local Storage sensitive file was deleted by cache cleaner!")
	}

	// Verify cache files were deleted
	if _, err := os.Stat(filepath.Join(cacheDir, "data_0")); !os.IsNotExist(err) {
		t.Errorf("Cache data_0 was not deleted")
	}
}

func TestClearDiscordCacheNoCacheFound(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("APPDATA", tempHome)
	t.Setenv("HOME", tempHome)
	t.Setenv("XDG_CONFIG_HOME", tempHome)

	res := ClearDiscordCacheStructured()
	if res.Status != "NO_CACHE_FOUND" {
		t.Errorf("Expected NO_CACHE_FOUND on empty system, got %s", res.Status)
	}
	if !strings.Contains(res.Message, "не обнаружены") {
		t.Errorf("Unexpected message: %s", res.Message)
	}
}

func TestIsPathWithinSafeDiscordBoundary(t *testing.T) {
	tempAppdata := filepath.Clean(t.TempDir())
	roots := map[string]string{
		"Discord Stable": filepath.Join(tempAppdata, "discord"),
		"Discord PTB":    filepath.Join(tempAppdata, "discordptb"),
		"Discord Canary": filepath.Join(tempAppdata, "discordcanary"),
	}

	// Allowed paths
	for _, sub := range safeCacheSubdirs {
		validPath := filepath.Join(roots["Discord Stable"], sub)
		if !isPathWithinSafeDiscordBoundary(validPath, roots) {
			t.Errorf("Expected valid path to pass: %s", validPath)
		}
	}

	// Forbidden paths
	forbidden := []string{
		filepath.Join(roots["Discord Stable"], "Local Storage"),
		filepath.Join(roots["Discord Stable"], "Session Storage"),
		filepath.Join(roots["Discord Stable"], "IndexedDB"),
		filepath.Join(roots["Discord Stable"], "settings.json"),
		filepath.Join(tempAppdata, "Microsoft", "Internet Explorer", "Quick Launch", "User Pinned", "TaskBar"),
		roots["Discord Stable"], // Root itself
		tempAppdata,             // AppData root
		"",
		"C:\\Windows\\System32",
	}

	for _, p := range forbidden {
		if isPathWithinSafeDiscordBoundary(p, roots) {
			t.Errorf("CRITICAL: Safety boundary permitted forbidden path: %s", p)
		}
	}
}
