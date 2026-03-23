package main

import (
	"context"
	"fmt"
	"os/exec"
	stdruntime "runtime"
	"strings"
	"time"

	"unbound/engine"
	"unbound/engine/providers"
	"unbound/engine/tester"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx            context.Context
	manager        *providers.ProviderManager
	startMinimized bool
	debugMode      bool
	autoTuneCancel context.CancelFunc
}

func NewApp() *App {
	return &App{
		manager: providers.NewProviderManager(),
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	assets, err := engine.ExtractAssets()
	if err != nil {
		wailsruntime.LogErrorf(ctx, "Failed to extract assets: %v", err)
		return
	}

	if err := providers.ValidateBinaries(assets.BinDir); err != nil {
		wailsruntime.LogWarningf(ctx, "Binary validation warning: %v", err)
	}

	// Ensure dynamic lists exist
	if err := engine.EnsureListsExist(); err != nil {
		wailsruntime.LogWarningf(ctx, "Failed to ensure lists exist: %v", err)
	}

	// Apply system settings
	settings, _ := engine.GetSettings()
	if settings != nil {
		if settings.EnableTCPTimestamps {
			engine.EnableTCPTimestamps()
		}
		if settings.DiscordCacheAutoClean {
			engine.ClearDiscordCache()
		}
	}

	registerOSProviders(a, assets)

	a.setupTray()

	if a.startMinimized {
		wailsruntime.WindowMinimise(ctx)
	}

	if a.debugMode {
		wailsruntime.LogInfo(ctx, "Debug mode enabled - verbose logging active")
	}

	wailsruntime.LogInfo(ctx, "UNBOUND initialized successfully")
}

func (a *App) shutdown(ctx context.Context) {
	a.manager.Stop()
}

func (a *App) GetEngineNames() []string {
	return a.manager.GetEngineNames()
}

func (a *App) GetProfiles(engineName string) []string {
	return a.manager.GetProfiles(engineName)
}

func (a *App) StartEngine(engineName string, profileName string) error {
	hasPriv, err := a.manager.CheckPrivileges()
	if err != nil {
		return err
	}
	if !hasPriv {
		wailsruntime.LogError(a.ctx, "Administrator privileges required")
		wailsruntime.EventsEmit(a.ctx, "privilege_error", "Administrator privileges required. Please restart the application as administrator.")
		return fmt.Errorf("administrator privileges required")
	}

	currentStatus := a.manager.GetStatus()
	if currentStatus == providers.StatusRunning {
		wailsruntime.LogInfo(a.ctx, "Stopping current engine before starting new profile")
		if err := a.manager.Stop(); err != nil {
			wailsruntime.LogErrorf(a.ctx, "Failed to stop current engine: %v", err)
		}
		time.Sleep(1 * time.Second)
	}

	err = a.manager.Start(a.ctx, engineName, profileName)
	if err == nil {
		wailsruntime.EventsEmit(a.ctx, "status_changed", a.manager.GetStatus())
		wailsruntime.LogInfof(a.ctx, "Engine started with profile: %s", profileName)
	} else {
		wailsruntime.LogErrorf(a.ctx, "Failed to start engine: %v", err)
	}
	return err
}

func (a *App) StopEngine() error {
	err := a.manager.Stop()
	if err == nil {
		wailsruntime.EventsEmit(a.ctx, "status_changed", a.manager.GetStatus())
	}
	return err
}

func (a *App) GetStatus() string {
	return string(a.manager.GetStatus())
}

func (a *App) GetLogs() []string {
	return a.manager.GetLogs()
}

func (a *App) GetSystemInfo() map[string]string {
	return map[string]string{
		"os":   stdruntime.GOOS,
		"arch": stdruntime.GOARCH,
	}
}

func (a *App) RunDiagnostics() []engine.DiagnosticResult {
	return engine.RunDiagnostics()
}

func (a *App) ClearDiscordCache() error {
	return engine.ClearDiscordCache()
}

func (a *App) EnableTCPTimestamps() error {
	return engine.EnableTCPTimestamps()
}

func (a *App) KillWinws2() error {
	wailsruntime.LogInfo(a.ctx, "Force killing winws2.exe...")
	cmd := exec.Command("taskkill", "/F", "/IM", "winws2.exe")
	cmd.Run()
	time.Sleep(500 * time.Millisecond)
	wailsruntime.LogInfo(a.ctx, "winws2.exe terminated")
	return nil
}

func (a *App) GetSettings() (*engine.Settings, error) {
	return engine.GetSettings()
}

func (a *App) SaveSettings(settings *engine.Settings) error {
	err := engine.SaveSettings(settings)
	if err == nil {
		if settings.EnableTCPTimestamps {
			engine.EnableTCPTimestamps()
		}
	}
	return err
}

func (a *App) TestProfile(engineName string, profileName string) (string, error) {
	hasPriv, err := a.manager.CheckPrivileges()
	if err != nil {
		return "", err
	}
	if !hasPriv {
		return "", fmt.Errorf("administrator/root privileges required")
	}

	err = a.manager.Start(a.ctx, engineName, profileName)
	if err != nil {
		return "", err
	}

	time.Sleep(2 * time.Second)

	testCtx, cancel := context.WithTimeout(a.ctx, 15*time.Second)
	defer cancel()

	results := tester.TestProfile(testCtx, tester.TestURLs, 10*time.Second)
	score := tester.CalculateScore(results)
	output := tester.FormatResults(results)

	a.manager.Stop()

	return fmt.Sprintf("Score: %d/600\n\n%s", score, output), nil
}

func (a *App) AutoTune() string {
	wailsruntime.LogInfo(a.ctx, "Starting Auto-Tune V2...")
	
	updateLog := func(msg string) {
		wailsruntime.EventsEmit(a.ctx, "autotune_log", msg)
	}

	tuneCtx, cancel := context.WithCancel(a.ctx)
	a.autoTuneCancel = cancel
	defer func() {
		a.autoTuneCancel = nil
		cancel()
	}()

	updateLog("🔍 Initializing Auto-Tune V2...")
	
	assets, err := engine.ExtractAssets()
	if err != nil {
		updateLog("❌ Failed to extract assets: " + err.Error())
		return "Failed"
	}

	provider := providers.NewZapret2WindowsProvider(
		assets.BinDir,
		assets.LuaDir,
		assets.ListDir,
		a.debugMode,
		false,
	)

	profiles := engine.GetProfiles(assets.LuaDir)
	for _, prof := range profiles {
		provider.RegisterProfile(prof.Name, prof.Args)
	}

	updateLog(fmt.Sprintf("📋 Loaded %d profiles for testing", len(profiles)))
	
	go func() {
		for {
			select {
			case <-tuneCtx.Done():
				return
			case <-time.After(500 * time.Millisecond):
				logs := provider.GetLogs()
				if len(logs) > 0 {
					lastLog := logs[len(logs)-1]
					wailsruntime.EventsEmit(a.ctx, "engine_log", lastLog)
				}
			}
		}
	}()

	result, err := engine.RunAutoTuneV2(provider, profiles)
	if err != nil {
		if tuneCtx.Err() == context.Canceled {
			updateLog("⏹️ Auto-Tune cancelled by user")
		} else {
			updateLog("❌ Auto-Tune failed: " + err.Error())
		}
		wailsruntime.EventsEmit(a.ctx, "autotune_complete", map[string]interface{}{
			"success": false,
			"profile": "",
		})
		return "Failed"
	}

	updateLog(fmt.Sprintf("✅ Found working profile: %s (Score: %d)", result.ProfileName, result.Score))
	
	for name, status := range result.Results {
		icon := "❌"
		if status.OK {
			icon = "✅"
		}
		tlsInfo := ""
		if status.TLS13 {
			tlsInfo = " [TLS 1.3]"
		}
		updateLog(fmt.Sprintf("   %s %s (%v)%s", icon, name, status.Latency.Truncate(time.Millisecond), tlsInfo))
	}
	
	time.Sleep(500 * time.Millisecond)
	
	if err := a.StartEngine("Zapret 2 (winws)", result.ProfileName); err != nil {
		updateLog("❌ Failed to start engine: " + err.Error())
		wailsruntime.EventsEmit(a.ctx, "autotune_complete", map[string]interface{}{
			"success": false,
			"profile": result.ProfileName,
		})
		return "Failed"
	}

	wailsruntime.EventsEmit(a.ctx, "autotune_complete", map[string]interface{}{
		"success": true,
		"profile": result.ProfileName,
	})
	
	return result.ProfileName
}

func (a *App) CancelAutoTune() {
	if a.autoTuneCancel != nil {
		a.autoTuneCancel()
		wailsruntime.LogInfo(a.ctx, "Auto-Tune cancellation requested")
	}
}

func (a *App) UpdateLists() error {
	if err := engine.UpdateLists(); err != nil {
		wailsruntime.LogErrorf(a.ctx, "Failed to update lists: %v", err)
		return err
	}
	wailsruntime.LogInfo(a.ctx, "Lists updated successfully")
	return nil
}

func (a *App) GetLivePing() map[string]interface{} {
	status := a.manager.GetStatus()
	
	if status != providers.StatusRunning {
		return map[string]interface{}{
			"active":  false,
			"latency": 0,
			"status":  "stopped",
		}
	}

	testCtx, cancel := context.WithTimeout(a.ctx, 3*time.Second)
	defer cancel()

	// Use Cloudflare for faster ping checks
	latency, err := engine.SimplePing(testCtx, "https://1.1.1.1")
	
	if err != nil {
		return map[string]interface{}{
			"active":  true,
			"latency": 0,
			"status":  "error",
		}
	}

	return map[string]interface{}{
		"active":  true,
		"latency": latency.Milliseconds(),
		"status":  "ok",
	}
}

func (a *App) CheckForUpdates(currentVersion string) (engine.UpdateInfo, error) {
	updateInfo, err := engine.CheckForUpdates(currentVersion)
	if err != nil {
		wailsruntime.LogWarningf(a.ctx, "Update check failed: %v", err)
		return updateInfo, err
	}
	
	if updateInfo.Available {
		wailsruntime.LogInfof(a.ctx, "Update available: %s", updateInfo.Version)
	} else {
		wailsruntime.LogInfo(a.ctx, "No updates available")
	}
	
	return updateInfo, nil
}

func (a *App) GetProfileCategories() []string {
	return []string{"All Profiles"}
}

func (a *App) GetProfilesByCategory(category string) []engine.Profile {
	return append(engine.GetProfiles(""), engine.GetAdvancedProfiles("")...)
}

func (a *App) GetBlobList() []engine.BlobPayload {
	return engine.ListBlobs()
}

func (a *App) GenerateCustomBlob(blobType string, sni string) (string, error) {
	switch blobType {
	case "tls_random":
		data := engine.GenerateRandomTLSClientHello(sni)
		return string(data), nil
	case "quic_random":
		data := engine.GenerateRandomQUICInitial()
		return string(data), nil
	default:
		return "", fmt.Errorf("unknown blob type: %s", blobType)
	}
}

func (a *App) EnableAutoStart() error {
	wailsruntime.LogInfo(a.ctx, "Enabling auto-start via Task Scheduler...")
	if err := engine.EnableAutoStart(); err != nil {
		wailsruntime.LogErrorf(a.ctx, "Failed to enable auto-start: %v", err)
		return err
	}
	wailsruntime.LogInfo(a.ctx, "Auto-start enabled successfully")
	return nil
}

func (a *App) DisableAutoStart() error {
	wailsruntime.LogInfo(a.ctx, "Disabling auto-start...")
	if err := engine.DisableAutoStart(); err != nil {
		wailsruntime.LogErrorf(a.ctx, "Failed to disable auto-start: %v", err)
		return err
	}
	wailsruntime.LogInfo(a.ctx, "Auto-start disabled successfully")
	return nil
}

func (a *App) IsAutoStartEnabled() bool {
	enabled, err := engine.IsAutoStartEnabled()
	if err != nil {
		wailsruntime.LogErrorf(a.ctx, "Failed to check auto-start status: %v", err)
		return false
	}
	return enabled
}

func (a *App) CheckPrivileges() (bool, error) {
	return a.manager.CheckPrivileges()
}

func (a *App) CheckConflicts() []string {
	conflicts := []string{}
	
	// Check for running winws2.exe processes
	cmd := exec.Command("tasklist", "/FI", "IMAGENAME eq winws2.exe", "/NH")
	output, err := cmd.Output()
	if err == nil && len(output) > 0 && !strings.Contains(string(output), "INFO: No tasks") {
		conflicts = append(conflicts, "⚠️ winws2.exe is already running")
	}
	
	// Check for common VPN software
	vpnProcesses := []string{
		"openvpn.exe",
		"wireguard.exe",
		"nordvpn.exe",
		"expressvpn.exe",
		"protonvpn.exe",
		"tunnelbear.exe",
		"cyberghost.exe",
		"privatevpn.exe",
		"windscribe.exe",
		"mullvad.exe",
	}
	
	for _, vpn := range vpnProcesses {
		cmd := exec.Command("tasklist", "/FI", fmt.Sprintf("IMAGENAME eq %s", vpn), "/NH")
		output, err := cmd.Output()
		if err == nil && len(output) > 0 && !strings.Contains(string(output), "INFO: No tasks") {
			conflicts = append(conflicts, fmt.Sprintf("⚠️ VPN detected: %s", strings.TrimSuffix(vpn, ".exe")))
		}
	}
	
	// Check for other DPI bypass tools
	dpiTools := []string{
		"goodbyedpi.exe",
		"zapret.exe",
		"green-tunnel.exe",
		"powertunnel.exe",
		"dpidetector.exe",
	}
	
	for _, tool := range dpiTools {
		cmd := exec.Command("tasklist", "/FI", fmt.Sprintf("IMAGENAME eq %s", tool), "/NH")
		output, err := cmd.Output()
		if err == nil && len(output) > 0 && !strings.Contains(string(output), "INFO: No tasks") {
			conflicts = append(conflicts, fmt.Sprintf("⚠️ DPI bypass tool detected: %s", strings.TrimSuffix(tool, ".exe")))
		}
	}
	
	return conflicts
}

func (a *App) KillConflicts() error {
	wailsruntime.LogInfo(a.ctx, "Killing conflicting processes...")
	
	processesToKill := []string{
		"winws2.exe",
		"openvpn.exe",
		"wireguard.exe",
		"nordvpn.exe",
		"expressvpn.exe",
		"protonvpn.exe",
		"tunnelbear.exe",
		"cyberghost.exe",
		"privatevpn.exe",
		"windscribe.exe",
		"mullvad.exe",
		"goodbyedpi.exe",
		"zapret.exe",
		"green-tunnel.exe",
		"powertunnel.exe",
		"dpidetector.exe",
	}
	
	for _, proc := range processesToKill {
		cmd := exec.Command("taskkill", "/F", "/IM", proc)
		cmd.Run() // Ignore errors - process might not be running
	}
	
	time.Sleep(500 * time.Millisecond)
	wailsruntime.LogInfo(a.ctx, "Conflicting processes terminated")
	return nil
}

