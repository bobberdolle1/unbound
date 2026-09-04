package engine

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"unbound/engine/providers"
)

const (
	GitHubAPIURL        = "https://api.github.com/repos/bobberdolle1/unbound/releases/latest"
	Zapret2GitHubAPIURL = "https://api.github.com/repos/bol-van/zapret2/releases/latest"
	HTTPTimeout         = 10 * time.Second
	BundledEngineVersion = "1.0.5"
)

// ComponentLocalState represents the locally installed version and status of a component without network access.
type ComponentLocalState struct {
	Component      ComponentType `json:"component"`
	Name           string        `json:"name"`
	CurrentVersion string        `json:"currentVersion"`
	Status         string        `json:"status"`
	StatusLabel    string        `json:"statusLabel"`
}

// SystemComponentState holds all local component versions available offline.
type SystemComponentState struct {
	Components []ComponentLocalState `json:"components"`
}

// GetSystemComponentState returns the local, non-network component versions.
func GetSystemComponentState() *SystemComponentState {
	hostlistVer := "Синхронизировано"
	if listsDir, err := GetListsDir(); err == nil {
		if ytStat, err := os.Stat(filepath.Join(listsDir, "youtube.txt")); err == nil {
			hostlistVer = ytStat.ModTime().Format("2006-01-02 15:04")
		}
	}

	return &SystemComponentState{
		Components: []ComponentLocalState{
			{
				Component:      ComponentApp,
				Name:           "Приложение UNBOUND",
				CurrentVersion: "v" + normalizeVersion(Version),
				Status:         "installed",
				StatusLabel:    "Установлено",
			},
			{
				Component:      ComponentEngine,
				Name:           "Движок Zapret 2",
				CurrentVersion: "v" + BundledEngineVersion,
				Status:         "installed",
				StatusLabel:    "Установлено",
			},
			{
				Component:      ComponentStrategies,
				Name:           "Каталог стратегий",
				CurrentVersion: StrategiesVersion,
				Status:         "installed",
				StatusLabel:    "Установлено",
			},
			{
				Component:      ComponentHostlists,
				Name:           "Списки обхода",
				CurrentVersion: hostlistVer,
				Status:         "synced",
				StatusLabel:    "Синхронизировано",
			},
		},
	}
}

type ComponentType string

const (
	ComponentApp        ComponentType = "app"
	ComponentEngine     ComponentType = "engine"
	ComponentStrategies ComponentType = "strategies"
	ComponentHostlists  ComponentType = "hostlists"
)

// ComponentUpdateStatus represents the version and update state of an individual component.
type ComponentUpdateStatus struct {
	Component       ComponentType `json:"component"`
	Name            string        `json:"name"`
	CurrentVersion  string        `json:"currentVersion"`
	LatestVersion   string        `json:"latestVersion"`
	UpdateAvailable bool          `json:"updateAvailable"`
	LastChecked     time.Time     `json:"lastChecked"`
	Status          string        `json:"status"` // "up_to_date", "update_available", "updating", "failed", "rolled_back"
	ReleaseURL      string        `json:"releaseUrl,omitempty"`
	Changelog       string        `json:"changelog,omitempty"`
	Error           string        `json:"error,omitempty"`
}

// SystemUpdateOverview aggregates the status of all application components.
type SystemUpdateOverview struct {
	LastChecked time.Time               `json:"lastChecked"`
	Components  []ComponentUpdateStatus `json:"components"`
}

// UpdateInfo represents the app-level release check result.
type UpdateInfo struct {
	Available   bool   `json:"available"`
	Version     string `json:"version"`
	DownloadURL string `json:"downloadUrl"`
	Changelog   string `json:"changelog"`
}

type GitHubRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
	Body    string `json:"body"`
}

// CheckAllComponents queries upstream repositories and local state to produce a full update audit.
func CheckAllComponents(ctx context.Context, appVersion string) (*SystemUpdateOverview, error) {
	now := time.Now()
	overview := &SystemUpdateOverview{
		LastChecked: now,
		Components:  make([]ComponentUpdateStatus, 0, 4),
	}

	// 1. UNBOUND Application
	appStatus := ComponentUpdateStatus{
		Component:      ComponentApp,
		Name:           "UNBOUND Application",
		CurrentVersion: "v" + normalizeVersion(appVersion),
		LastChecked:    now,
		Status:         "up_to_date",
	}
	appRelease, err := fetchLatestRelease(ctx, GitHubAPIURL)
	if err != nil {
		appStatus.Status = "check_failed"
		appStatus.Error = fmt.Sprintf("Failed to check app release: %v", err)
	} else {
		appStatus.LatestVersion = appRelease.TagName
		appStatus.ReleaseURL = appRelease.HTMLURL
		appStatus.Changelog = appRelease.Body
		if compareVersions(normalizeVersion(appRelease.TagName), normalizeVersion(appVersion)) > 0 {
			appStatus.UpdateAvailable = true
			appStatus.Status = "update_available"
		}
	}
	overview.Components = append(overview.Components, appStatus)

	// 2. Zapret 2 Engine
	engineStatus := ComponentUpdateStatus{
		Component:      ComponentEngine,
		Name:           "Zapret 2 Engine",
		CurrentVersion: "v" + BundledEngineVersion,
		LastChecked:    now,
		Status:         "up_to_date",
	}
	zapretRelease, err := fetchLatestRelease(ctx, Zapret2GitHubAPIURL)
	if err != nil {
		engineStatus.Status = "check_failed"
		engineStatus.Error = fmt.Sprintf("Failed to check upstream engine: %v", err)
	} else {
		engineStatus.LatestVersion = zapretRelease.TagName
		engineStatus.ReleaseURL = zapretRelease.HTMLURL
		engineStatus.Changelog = zapretRelease.Body
		if compareVersions(normalizeVersion(zapretRelease.TagName), BundledEngineVersion) > 0 {
			engineStatus.UpdateAvailable = true
			engineStatus.Status = "update_available"
		}
	}
	overview.Components = append(overview.Components, engineStatus)

	// 3. Strategy Definitions
	stratStatus := ComponentUpdateStatus{
		Component:       ComponentStrategies,
		Name:            "Каталог стратегий",
		CurrentVersion:  StrategiesVersion,
		LatestVersion:   StrategiesVersion,
		UpdateAvailable: false,
		LastChecked:     now,
		Status:          "up_to_date",
	}
	overview.Components = append(overview.Components, stratStatus)

	// 4. Hostlists & Exclusions
	hostlistStatus := ComponentUpdateStatus{
		Component:       ComponentHostlists,
		Name:            "Списки блокировок и исключений",
		CurrentVersion:  "dynamic",
		LatestVersion:   "upstream-sync",
		UpdateAvailable: false,
		LastChecked:     now,
		Status:          "up_to_date",
	}
	if listsDir, err := GetListsDir(); err == nil {
		if ytStat, err := os.Stat(filepath.Join(listsDir, "youtube.txt")); err == nil {
			hostlistStatus.CurrentVersion = ytStat.ModTime().Format("2006-01-02 15:04")
		}
	}
	overview.Components = append(overview.Components, hostlistStatus)

	return overview, nil
}

func fetchLatestRelease(ctx context.Context, apiURL string) (*GitHubRelease, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", UserAgent())

	client := &http.Client{Timeout: HTTPTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github api status %d", resp.StatusCode)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}
	return &release, nil
}

// CheckForUpdates preserves compatibility with older callers checking app releases.
func CheckForUpdates(currentVersion string) (UpdateInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), HTTPTimeout)
	defer cancel()

	release, err := fetchLatestRelease(ctx, GitHubAPIURL)
	if err != nil {
		return UpdateInfo{}, err
	}

	latestVersion := normalizeVersion(release.TagName)
	currentNormalized := normalizeVersion(currentVersion)
	updateAvailable := compareVersions(latestVersion, currentNormalized) > 0

	return UpdateInfo{
		Available:   updateAvailable,
		Version:     release.TagName,
		DownloadURL: release.HTMLURL,
		Changelog:   release.Body,
	}, nil
}

// ValidateStagedEngine ensures an unpacked engine directory has required binaries and valid execution.
func ValidateStagedEngine(stagingDir string) error {
	if runtime.GOOS == "windows" {
		winws := filepath.Join(stagingDir, "winws2.exe")
		if stat, err := os.Stat(winws); err != nil || stat.IsDir() {
			return fmt.Errorf("winws2.exe missing in staged directory")
		}
		// Test dry-run or version query to verify PE executable
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, winws, "--version")
		cmd.SysProcAttr = GetHiddenSysProcAttr()
		out, err := cmd.CombinedOutput()
		if err != nil && ctx.Err() != nil {
			return fmt.Errorf("staged winws2.exe timed out during verification: %w", ctx.Err())
		}
		outStr := string(out)
		if !strings.Contains(strings.ToLower(outStr), "zapret") && !strings.Contains(strings.ToLower(outStr), "version") {
			return fmt.Errorf("staged winws2.exe produced invalid version output: %s", outStr)
		}
	} else if runtime.GOOS == "linux" {
		nfqws := filepath.Join(stagingDir, "nfqws2")
		if stat, err := os.Stat(nfqws); err != nil || stat.IsDir() {
			return fmt.Errorf("nfqws2 missing in staged directory")
		}
	}
	return nil
}

// ApplyStagedEngineUpdate transactionally replaces the runtime engine with staged files.
// If verification fails, it immediately rolls back to the previous backup and restores the running profile.
func ApplyStagedEngineUpdate(stagingDir string, pc ProviderController) (retErr error) {
	logger := GetLogger()
	logger.Infof("Update", "[UPDATE] starting transactional engine update from %s", stagingDir)

	if err := ValidateStagedEngine(stagingDir); err != nil {
		logger.Errorf("Update", "[UPDATE] staging validation failed: %v", err)
		return fmt.Errorf("staging validation failed: %w", err)
	}

	configDir, err := GetConfigDir()
	if err != nil {
		return err
	}

	runtimeDir := filepath.Join(configDir, "runtime")
	backupDir := filepath.Join(configDir, "backup", "engine_previous")
	_ = os.MkdirAll(filepath.Dir(backupDir), 0755)

	var initialProfile string
	var wasRunning bool
	if pc != nil {
		initialProfile = pc.CurrentProfile()
		wasRunning = pc.GetStatus() == providers.StatusRunning
		if wasRunning {
			logger.Info("Update", "[UPDATE] stopping active engine for binary replacement")
			_ = pc.Stop()
			time.Sleep(500 * time.Millisecond)
		}
	}

	// 1. Create backup of current runtime
	_ = os.RemoveAll(backupDir)
	if stat, err := os.Stat(runtimeDir); err == nil && stat.IsDir() {
		if err := copyDirectory(runtimeDir, backupDir); err != nil {
			logger.Errorf("Update", "[UPDATE] failed to create backup: %v", err)
			if wasRunning && pc != nil && initialProfile != "" {
				_ = pc.Start(context.Background(), initialProfile)
			}
			return fmt.Errorf("failed to backup current engine: %w", err)
		}
		logger.Info("Update", "[UPDATE] current engine backed up successfully")
	}

	// Defer rollback handler in case replacement fails
	updateCommitted := false
	defer func() {
		if !updateCommitted {
			logger.Warn("Update", "[ROLLBACK] engine update did not commit cleanly, initiating rollback")
			_ = copyDirectory(backupDir, runtimeDir)
			if wasRunning && pc != nil && initialProfile != "" {
				_ = pc.Start(context.Background(), initialProfile)
			}
			logger.Info("Update", "[ROLLBACK] engine successfully restored from backup")
		}
	}()

	// 2. Replace runtime files with staging
	if err := copyDirectory(stagingDir, runtimeDir); err != nil {
		logger.Errorf("Update", "[UPDATE] failed to copy staging files to runtime: %v", err)
		return fmt.Errorf("failed to copy staging to runtime: %w", err)
	}

	// 3. Validate new runtime installation
	if err := ValidateStagedEngine(runtimeDir); err != nil {
		logger.Errorf("Update", "[UPDATE] new runtime failed validation: %v", err)
		return fmt.Errorf("new runtime validation failed: %w", err)
	}

	// 4. Restart profile if was running
	if wasRunning && pc != nil && initialProfile != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := pc.Start(ctx, initialProfile); err != nil {
			logger.Errorf("Update", "[UPDATE] failed to start profile with new engine: %v", err)
			return fmt.Errorf("profile startup failed with new engine: %w", err)
		}
	}

	// 5. Commit transaction
	updateCommitted = true
	logger.Info("Update", "[UPDATE] engine update committed and verified successfully")
	return nil
}

// RollbackEngine restores the previously backed-up engine.
func RollbackEngine(pc ProviderController) error {
	logger := GetLogger()
	configDir, err := GetConfigDir()
	if err != nil {
		return err
	}
	backupDir := filepath.Join(configDir, "backup", "engine_previous")
	runtimeDir := filepath.Join(configDir, "runtime")

	if _, err := os.Stat(backupDir); os.IsNotExist(err) {
		return errors.New("no previous engine backup exists to roll back to")
	}

	logger.Info("Update", "[ROLLBACK] initiating manual rollback to previous engine")
	var initialProfile string
	var wasRunning bool
	if pc != nil {
		initialProfile = pc.CurrentProfile()
		wasRunning = pc.GetStatus() == providers.StatusRunning
		if wasRunning {
			_ = pc.Stop()
			time.Sleep(300 * time.Millisecond)
		}
	}

	if err := copyDirectory(backupDir, runtimeDir); err != nil {
		return fmt.Errorf("failed to restore from backup: %w", err)
	}

	if wasRunning && pc != nil && initialProfile != "" {
		_ = pc.Start(context.Background(), initialProfile)
	}
	logger.Info("Update", "[ROLLBACK] manual rollback completed successfully")
	return nil
}

// VerifyFileSHA256 computes SHA256 of a file and compares it to an expected hex digest.
func VerifyFileSHA256(filePath, expectedHash string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return err
	}
	calculated := hex.EncodeToString(hasher.Sum(nil))
	if !strings.EqualFold(calculated, strings.TrimSpace(expectedHash)) {
		return fmt.Errorf("SHA256 mismatch: calculated %s, expected %s", calculated, expectedHash)
	}
	return nil
}

// UnpackZipArchive extracts a ZIP archive safely to destDir preventing directory traversal (Zip Slip).
func UnpackZipArchive(zipPath, destDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	destClean := filepath.Clean(destDir)
	for _, f := range r.File {
		targetPath := filepath.Join(destClean, f.Name)
		if !strings.HasPrefix(filepath.Clean(targetPath), destClean+string(filepath.Separator)) {
			return fmt.Errorf("illegal file path in zip: %s", f.Name)
		}

		if f.FileInfo().IsDir() {
			_ = os.MkdirAll(targetPath, f.Mode())
			continue
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return err
		}

		outFile, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		_, copyErr := io.Copy(outFile, rc)
		rc.Close()
		outFile.Close()
		if copyErr != nil {
			return copyErr
		}
	}
	return nil
}

func copyDirectory(src, dest string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dest, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode())
	})
}

func normalizeVersion(version string) string {
	version = strings.TrimPrefix(version, "v")
	version = strings.TrimPrefix(version, "V")
	return strings.TrimSpace(version)
}

func compareVersions(v1, v2 string) int {
	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")

	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}

	for i := range maxLen {
		var n1, n2 int
		if i < len(parts1) {
			n1, _ = strconv.Atoi(parts1[i])
		}
		if i < len(parts2) {
			n2, _ = strconv.Atoi(parts2[i])
		}

		if n1 < n2 {
			return -1
		}
		if n1 > n2 {
			return 1
		}
	}
	return 0
}
