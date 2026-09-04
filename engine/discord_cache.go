package engine

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
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

// IsDiscordRunning returns whether any Discord client processes are currently active.
func IsDiscordRunning() (bool, []string) {
	return checkRunningDiscordProcesses()
}

// CloseDiscordProcesses safely terminates running Discord instances (Stable, PTB, Canary).
// Uses exact process image matching to avoid collateral process termination.
func CloseDiscordProcesses(timeout time.Duration) error {
	logger := GetLogger()
	running, procs := checkRunningDiscordProcesses()
	if !running || len(procs) == 0 {
		return nil
	}

	logger.Infof("Cache", "[CACHE] Discord processes detected: %v", procs)
	if timeout <= 0 {
		timeout = 3 * time.Second
	}

	if runtime.GOOS == "windows" {
		candidates := []string{"Discord.exe", "DiscordPTB.exe", "DiscordCanary.exe"}
		// 1. Attempt graceful close first
		for _, c := range candidates {
			cmd := exec.Command("taskkill.exe", "/IM", c)
			cmd.SysProcAttr = GetHiddenSysProcAttr()
			_ = cmd.Run()
		}

		// Wait up to 1.5 seconds for graceful shutdown
		deadline := time.Now().Add(1500 * time.Millisecond)
		for time.Now().Before(deadline) {
			stillRunning, _ := checkRunningDiscordProcesses()
			if !stillRunning {
				logger.Info("Cache", "[CACHE] Discord closed gracefully")
				time.Sleep(500 * time.Millisecond) // settle handle release
				return nil
			}
			time.Sleep(200 * time.Millisecond)
		}

		// 2. Force termination if still alive
		logger.Warn("Cache", "[CACHE] graceful shutdown timed out, force terminating Discord process tree")
		for _, c := range candidates {
			cmd := exec.Command("taskkill.exe", "/F", "/T", "/IM", c)
			cmd.SysProcAttr = GetHiddenSysProcAttr()
			_ = cmd.Run()
		}
	} else {
		// Unix: kill -TERM then kill -KILL
		pkillCmd := exec.Command("pkill", "-x", "discord")
		_ = pkillCmd.Run()
		time.Sleep(1 * time.Second)
		if stillRunning, _ := checkRunningDiscordProcesses(); stillRunning {
			_ = exec.Command("pkill", "-9", "-x", "discord").Run()
		}
	}

	time.Sleep(800 * time.Millisecond) // allow OS file handles to release
	stillRunning, remaining := checkRunningDiscordProcesses()
	if stillRunning {
		return fmt.Errorf("some Discord processes could not be terminated: %v", remaining)
	}
	logger.Info("Cache", "[CACHE] all Discord processes stopped successfully")
	return nil
}

// isPathWithinSafeDiscordBoundary validates that the candidate deletion path strictly
// resides inside an authorized Discord user data root and matches a permitted cache directory.
func isPathWithinSafeDiscordBoundary(candidatePath string, knownRoots map[string]string) bool {
	if candidatePath == "" {
		return false
	}

	cleanPath := filepath.Clean(candidatePath)
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return false
	}

	base := filepath.Base(absPath)
	parent := filepath.Clean(filepath.Dir(absPath))

	// Verify base is strictly in safe cache allowlist
	isAllowedBase := false
	for _, safe := range safeCacheSubdirs {
		if strings.EqualFold(base, safe) {
			isAllowedBase = true
			break
		}
	}
	if !isAllowedBase {
		return false
	}

	// Verify parent is strictly one of the known Discord root directories
	isAllowedParent := false
	for _, root := range knownRoots {
		cleanRoot := filepath.Clean(root)
		absRoot, err := filepath.Abs(cleanRoot)
		if err == nil && strings.EqualFold(parent, absRoot) {
			isAllowedParent = true
			break
		}
	}
	if !isAllowedParent {
		return false
	}

	// Never delete root, root parent, or drive root
	if absPath == parent || filepath.Dir(parent) == parent {
		return false
	}

	// Verify directory is not an external reparse point / junction leading outside Discord root
	if lstat, err := os.Lstat(absPath); err == nil {
		if lstat.Mode()&os.ModeSymlink != 0 {
			// Target is a symlink: evaluate destination
			evalTarget, err := filepath.EvalSymlinks(absPath)
			if err != nil || !strings.HasPrefix(strings.ToLower(evalTarget), strings.ToLower(parent)) {
				return false
			}
		}
	}

	return true
}

// ClearDiscordCacheStructured cleans Discord cache without closing processes.
func ClearDiscordCacheStructured() *DiscordCacheCleanupResult {
	return ClearDiscordCacheWithOptions(false)
}

// ClearDiscordCacheWithOptions executes Discord cache cleanup, optionally terminating
// Discord processes upon user consent to ensure complete, unlocked deletion.
func ClearDiscordCacheWithOptions(closeIfRunning bool) *DiscordCacheCleanupResult {
	logger := GetLogger()
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

	if running && closeIfRunning {
		logger.Info("Cache", "[CACHE] user requested Discord termination for full cache cleanup")
		if err := CloseDiscordProcesses(3 * time.Second); err != nil {
			logger.Warnf("Cache", "[CACHE] close Discord warning: %v", err)
		}
		// Re-check running state
		runningAfter, procsAfter := checkRunningDiscordProcesses()
		result.DiscordRunning = runningAfter
		result.RunningProcesses = procsAfter
	}

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

	// 4. Perform safe removal with strict filesystem boundary checks
	for _, dir := range existingCacheDirs {
		if !isPathWithinSafeDiscordBoundary(dir, roots) {
			logger.Errorf("Cache", "[CACHE][SECURITY] rejected unsafe deletion target: %s", dir)
			result.Failures = append(result.Failures, fmt.Sprintf("%s: rejected by safety boundary", filepath.Base(dir)))
			continue
		}

		logger.Infof("Cache", "[CACHE] removing safe cache directory: %s", dir)
		err := os.RemoveAll(dir)
		if err != nil {
			logger.Warnf("Cache", "[CACHE] remove failed for %s: %v", dir, err)
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
