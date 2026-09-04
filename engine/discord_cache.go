package engine

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// DiscordCacheCleanupResult provides a complete, honest audit of the cache clearing operation.
type DiscordCacheCleanupResult struct {
	InstallationsFound []string `json:"installationsFound"`
	PathsScanned       []string `json:"pathsScanned"`
	PathsRemoved       []string `json:"pathsRemoved"`
	FilesRemoved       int      `json:"filesRemoved"`
	BytesBefore        int64    `json:"bytesBefore"`
	BytesAfter         int64    `json:"bytesAfter"`
	BytesFreed         int64    `json:"bytesFreed"`
	Failures           []string `json:"failures"`
	DiscordRunning     bool     `json:"discordRunning"`
	RunningProcesses   []string `json:"runningProcesses,omitempty"`
	Status             string   `json:"status"` // "SUCCESS", "PARTIAL", "NO_CACHE_FOUND", "FAILED"
	Message            string   `json:"message"`
}

var safeCacheSubdirs = []string{
	"Cache",
	"Code Cache",
	"GPUCache",
	"DawnCache",
	"DnsCache",
}

// ClearDiscordCache preserves backwards compatibility with older callers returning error.
func ClearDiscordCache() error {
	res := ClearDiscordCacheStructured()
	if res.Status == "FAILED" && len(res.Failures) > 0 {
		return fmt.Errorf("%s", res.Failures[0])
	}
	return nil
}

// ClearDiscordCacheStructured inspects all installed Discord flavors (Stable, PTB, Canary),
// calculates cache sizes before and after, detects running processes, and cleans only safe cache folders.
func ClearDiscordCacheStructured() *DiscordCacheCleanupResult {
	result := &DiscordCacheCleanupResult{
		InstallationsFound: make([]string, 0, 3),
		PathsScanned:       make([]string, 0, 15),
		PathsRemoved:       make([]string, 0, 15),
		Failures:           make([]string, 0),
	}

	// 1. Detect running Discord processes
	running, procs := checkRunningDiscordProcesses()
	result.DiscordRunning = running
	result.RunningProcesses = procs

	// 2. Discover Discord installation roots
	roots := getDiscordInstallationRoots()

	var existingCacheDirs []string
	for flavor, rootDir := range roots {
		if stat, err := os.Stat(rootDir); err == nil && stat.IsDir() {
			result.InstallationsFound = append(result.InstallationsFound, flavor)
			for _, sub := range safeCacheSubdirs {
				cachePath := filepath.Join(rootDir, sub)
				result.PathsScanned = append(result.PathsScanned, cachePath)
				if cStat, err := os.Stat(cachePath); err == nil && cStat.IsDir() {
					existingCacheDirs = append(existingCacheDirs, cachePath)
				}
			}
		}
	}

	if len(existingCacheDirs) == 0 {
		result.Status = "NO_CACHE_FOUND"
		result.Message = "Файлы кэша Discord не обнаружены (кэш уже чист)."
		return result
	}

	// 3. Measure bytes before deletion
	var totalBefore int64
	var fileCount int
	for _, dir := range existingCacheDirs {
		bytes, files := computeDirSize(dir)
		totalBefore += bytes
		fileCount += files
	}
	result.BytesBefore = totalBefore

	// 4. Perform safe removal
	for _, dir := range existingCacheDirs {
		err := os.RemoveAll(dir)
		if err != nil {
			result.Failures = append(result.Failures, fmt.Sprintf("%s: %v", filepath.Base(filepath.Dir(dir))+"/"+filepath.Base(dir), err))
		} else {
			result.PathsRemoved = append(result.PathsRemoved, dir)
		}
	}

	// 5. Measure bytes after deletion to confirm real freed space
	var totalAfter int64
	for _, dir := range existingCacheDirs {
		if stat, err := os.Stat(dir); err == nil && stat.IsDir() {
			bytes, _ := computeDirSize(dir)
			totalAfter += bytes
		}
	}
	result.BytesAfter = totalAfter
	result.BytesFreed = totalBefore - totalAfter
	if result.BytesFreed < 0 {
		result.BytesFreed = 0
	}
	result.FilesRemoved = fileCount

	// 6. Compute status and human-readable message
	freedStr := formatBytes(result.BytesFreed)

	if result.DiscordRunning {
		if len(result.Failures) > 0 {
			result.Status = "PARTIAL"
			result.Message = fmt.Sprintf("Discord запущен. Часть файлов кэша занята процессом (%s освобождено). Закройте Discord и повторите очистку.", freedStr)
		} else {
			result.Status = "SUCCESS"
			result.Message = fmt.Sprintf("Кэш Discord очищен. Освобождено %s (Discord работает в фоне).", freedStr)
		}
	} else {
		if len(result.Failures) == 0 {
			result.Status = "SUCCESS"
			result.Message = fmt.Sprintf("Кэш Discord успешно очищен. Освобождено %s.", freedStr)
		} else if result.BytesFreed > 0 {
			result.Status = "PARTIAL"
			result.Message = fmt.Sprintf("Кэш очищен частично (%s освобождено; %d папок заблокировано).", freedStr, len(result.Failures))
		} else {
			result.Status = "FAILED"
			result.Message = fmt.Sprintf("Не удалось очистить кэш Discord (%d ошибок доступа).", len(result.Failures))
		}
	}

	return result
}

func getDiscordInstallationRoots() map[string]string {
	roots := make(map[string]string)

	switch runtime.GOOS {
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData != "" {
			roots["Discord Stable"] = filepath.Join(appData, "discord")
			roots["Discord PTB"] = filepath.Join(appData, "discordptb")
			roots["Discord Canary"] = filepath.Join(appData, "discordcanary")
		}
	case "darwin":
		home, err := os.UserHomeDir()
		if err == nil {
			appSupport := filepath.Join(home, "Library", "Application Support")
			roots["Discord Stable"] = filepath.Join(appSupport, "discord")
			roots["Discord PTB"] = filepath.Join(appSupport, "discordptb")
			roots["Discord Canary"] = filepath.Join(appSupport, "discordcanary")
		}
	default:
		home, err := os.UserHomeDir()
		if err == nil {
			configDir := filepath.Join(home, ".config")
			roots["Discord Stable"] = filepath.Join(configDir, "discord")
			roots["Discord PTB"] = filepath.Join(configDir, "discordptb")
			roots["Discord Canary"] = filepath.Join(configDir, "discordcanary")
		}
	}
	return roots
}

func checkRunningDiscordProcesses() (bool, []string) {
	var procs []string

	if runtime.GOOS == "windows" {
		cmd := exec.Command("tasklist.exe")
		cmd.SysProcAttr = GetHiddenSysProcAttr()
		out, err := cmd.Output()
		if err == nil {
			outStr := strings.ToLower(string(out))
			candidates := []string{"discord.exe", "discordptb.exe", "discordcanary.exe"}
			for _, c := range candidates {
				if strings.Contains(outStr, c) {
					procs = append(procs, c)
				}
			}
		}
	} else {
		cmd := exec.Command("pgrep", "-l", "discord")
		out, err := cmd.Output()
		if err == nil && len(out) > 0 {
			lines := strings.Split(strings.TrimSpace(string(out)), "\n")
			for _, l := range lines {
				if l != "" {
					procs = append(procs, strings.TrimSpace(l))
				}
			}
		}
	}

	return len(procs) > 0, procs
}

func computeDirSize(dir string) (int64, int) {
	var totalBytes int64
	var fileCount int

	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			fileCount++
			if info, err := d.Info(); err == nil {
				totalBytes += info.Size()
			}
		}
		return nil
	})

	return totalBytes, fileCount
}

func formatBytes(bytes int64) string {
	const (
		kb = 1024
		mb = 1024 * kb
		gb = 1024 * mb
	)

	switch {
	case bytes >= gb:
		return fmt.Sprintf("%.2f ГБ", float64(bytes)/float64(gb))
	case bytes >= mb:
		return fmt.Sprintf("%.1f МБ", float64(bytes)/float64(mb))
	case bytes >= kb:
		return fmt.Sprintf("%d КБ", bytes/kb)
	default:
		return fmt.Sprintf("%d байт", bytes)
	}
}
