package main

import (
	"context"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unbound/engine"
	"unbound/engine/providers"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	attachConsole()

	cliMode := flag.Bool("cli", false, "Run in headless CLI mode")
	profileName := flag.String("profile", "", "Profile to use in CLI mode (default: interactive selection)")
	autoTuneMode := flag.Bool("autotune", false, "Run AutoTune benchmark in CLI mode and start the best profile")
	testMode := flag.Bool("test", false, "Run quick connectivity diagnostic probe for targets and exit")
	installService := flag.Bool("install-service", false, "Register autostart service for Unbound")
	uninstallService := flag.Bool("uninstall-service", false, "Remove autostart service for Unbound")
	jsonOutput := flag.Bool("json", false, "Output profile list or status in JSON format")
	trayMode := flag.Bool("tray", false, "Start minimized to system tray")
	debugMode := flag.Bool("debug", false, "Enable verbose debug logging")
	showVersion := flag.Bool("version", false, "Print the version and exit")
	listProfiles := flag.Bool("list-profiles", false, "List the profiles available on this platform and exit")
	controlMode := flag.Bool("control", false, "Run interactive Control Center menu in CLI")
	runDuration := flag.Duration("run-duration", 0, "Stop CLI automatically after this duration (0 = wait for signal)")

	flag.Usage = func() {
		fmt.Printf("UNBOUND v%s (%s/%s)\n", engine.Version, runtime.GOOS, runtime.GOARCH)
		fmt.Println("Usage: unbound [options]")
		fmt.Println("Options:")
		flag.PrintDefaults()
		fmt.Println("\nExamples:")
		fmt.Println("  unbound --cli                                Run headless CLI mode")
		fmt.Println("  unbound --cli --autotune                     Run AutoTune in CLI and start best profile")
		fmt.Println("  unbound --cli --profile=\"Alternative 2\"       Start CLI with specific profile")
		fmt.Println("  unbound --test                               Run quick connectivity diagnostic probe")
		fmt.Println("  unbound --list-profiles --json               List profiles in JSON format")
		fmt.Println("  unbound --install-service                    Enable OS autostart service")
		fmt.Println("  unbound --uninstall-service                  Disable OS autostart service")
		fmt.Println("  unbound --control                            Open the interactive Control Center")
	}

	flag.Parse()
	if !isBindingsBuild() {
		if relaunched, err := relaunchElevatedIfNeeded(requiresElevationForMode(*showVersion, *testMode, *listProfiles, *cliMode, *autoTuneMode, *installService, *uninstallService, *controlMode)); err != nil {
			log.Fatalf("Failed to request administrator privileges: %v", err)
		} else if relaunched {
			return
		}
	}

	if *showVersion {
		if *jsonOutput {
			fmt.Printf("{\"version\":\"%s\",\"os\":\"%s\",\"arch\":\"%s\"}\n", engine.Version, runtime.GOOS, runtime.GOARCH)
		} else {
			fmt.Printf("unbound %s (%s/%s)\n", engine.Version, runtime.GOOS, runtime.GOARCH)
		}
		return
	}

	if *testMode {
		runTestProbe()
		return
	}

	if *installService {
		if err := engine.EnableAutoStart(); err != nil {
			log.Fatalf("Failed to enable auto-start: %v", err)
		}
		fmt.Println("✓ Auto-start enabled successfully")
		return
	}

	if *uninstallService {
		if err := engine.DisableAutoStart(); err != nil {
			log.Fatalf("Failed to disable auto-start: %v", err)
		}
		fmt.Println("✓ Auto-start disabled successfully")
		return
	}

	if *listProfiles {
		if *jsonOutput {
			runListProfilesJSON(*debugMode)
		} else {
			runListProfiles(*debugMode)
		}
		return
	}
	if *controlMode {
		runControlCenterMenu(*debugMode)
		return
	}

	if *cliMode || *autoTuneMode {
		runHeadlessMode(*profileName, *autoTuneMode, *debugMode, *runDuration)
		return
	}
	app := NewApp()
	app.startMinimized = *trayMode
	app.debugMode = *debugMode

	err := wails.Run(&options.App{
		Title:             "UNBOUND",
		Width:             940,
		Height:            700,
		MinWidth:          360,
		MinHeight:         560,
		Frameless:         true,
		DisableResize:     false,
		HideWindowOnClose: true,
		OnBeforeClose:     app.onBeforeClose,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 10, G: 10, B: 10, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
		},
		Menu: getAppMenu(app),
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			DisableWindowIcon:    false,
		},
		Mac: &mac.Options{
			TitleBar: mac.TitleBarHiddenInset(),
			About: &mac.AboutInfo{
				Title:   "UNBOUND",
				Message: "Ultimate DPI bypass engine",
			},
		},
	})

	if err != nil {
		log.Fatal(err)
	}
}

func requiresElevationForMode(showVersion, testMode, listProfiles, cliMode, autoTuneMode, installService, uninstallService, controlMode bool) bool {
	return !(showVersion || testMode || listProfiles || cliMode || autoTuneMode || installService || uninstallService || controlMode)
}

// newHeadlessManager performs the setup shared by --cli and --list-profiles.
func newHeadlessManager(debugMode bool) (*providers.ProviderManager, *engine.AssetPaths) {
	assets, err := engine.ExtractAssets()
	if err != nil {
		log.Fatalf("Failed to extract assets: %v", err)
	}

	listsDir, err := engine.GetListsDir()
	if err != nil {
		listsDir = assets.ListDir
	}

	manager := providers.NewProviderManager()
	registerHeadlessProvider(manager, assets, listsDir, debugMode)
	return manager, assets
}

func runListProfilesJSON(debugMode bool) {
	manager, _ := newHeadlessManager(debugMode)
	defer func() { _ = engine.CleanupExtractedAssets() }()
	res := make(map[string][]string)
	for _, name := range manager.GetEngineNames() {
		res[name] = manager.GetProfiles(name)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(res)
}

func runListProfiles(debugMode bool) {
	attachConsole()

	manager, _ := newHeadlessManager(debugMode)
	defer func() { _ = engine.CleanupExtractedAssets() }()
	fmt.Printf("UNBOUND v%s (%s/%s) — Available Profiles:\n", engine.Version, runtime.GOOS, runtime.GOARCH)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	for _, name := range manager.GetEngineNames() {
		fmt.Printf("Engine: %s\n", name)
		for i, profile := range manager.GetProfiles(name) {
			fmt.Printf("  [%d] %s\n", i+1, profile)
		}
	}
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
}

func runTestProbe() {
	fmt.Println("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("🔍 UNBOUND - Connectivity Diagnostic Probe")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	targets := []struct{ Name, URL string }{
		{"YouTube", "https://www.youtube.com"},
		{"Discord", "https://discord.com"},
		{"Instagram", "https://www.instagram.com"},
		{"Cloudflare", "https://1.1.1.1"},
		{"Ozon", "https://www.ozon.ru"},
	}
	for _, t := range targets {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		lat, err := engine.SimplePing(ctx, t.URL)
		cancel()
		if err == nil {
			fmt.Printf("  ✓ %-12s OK   (%d ms)\n", t.Name, lat.Milliseconds())
		} else {
			fmt.Printf("  ✗ %-12s FAIL (%v)\n", t.Name, err)
		}
	}
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
}

func runHeadlessMode(profileName string, runAutoTune bool, debugMode bool, runDuration time.Duration) {
	attachConsole()

	fmt.Println("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("🚀 UNBOUND - Headless CLI Mode")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Ensure dynamic lists exist
	fmt.Println("Checking for updated bypass lists...")
	if err := engine.EnsureListsExist(); err != nil {
		fmt.Printf("Warning: Failed to update lists: %v\n", err)
	}

	manager, assets := newHeadlessManager(debugMode)
	defer func() { _ = engine.CleanupExtractedAssets() }()

	engineNames := manager.GetEngineNames()
	if len(engineNames) == 0 {
		log.Fatalf("No bypass engine is available on %s", runtime.GOOS)
	}
	engineName := engineNames[0]

	if runAutoTune {
		fmt.Println("⚡ Running AutoTune benchmark in CLI mode...")
		provider := providers.NewAutoTuneProvider(assets.BinDir, assets.LuaDir, assets.ListDir, assets.EngineSHA256)
		if provider == nil {
			log.Fatalf("No engine provider available for AutoTune on %s", runtime.GOOS)
		}
		allProfiles := engine.PrepareAutoTuneProfiles(provider, assets.LuaDir)
		progressFn := func(step, total int, profile string, okCount, totalTargets int, msg string) {
			pct := (step * 100) / total
			barLen := 20
			filled := (pct * barLen) / 100
			bar := strings.Repeat("█", filled) + strings.Repeat("░", barLen-filled)
			fmt.Printf("\r[%s] %3d%% | [%d/%d] %s", bar, pct, step, total, msg)
		}
		result, err := engine.RunAutoTuneV2WithProgress(context.Background(), provider, allProfiles, progressFn)
		fmt.Println()
		if err != nil {
			log.Fatalf("AutoTune failed: %v", err)
		}
		fmt.Printf("✅ AutoTune completed! Best profile: %s (score: %d)\n", result.ProfileName, result.Score)
		profileName = result.ProfileName
	} else if profileName != "" {
		profiles := manager.GetProfiles(engineName)
		resolved := resolveProfileAlias(profiles, profileName)
		if resolved != "" {
			profileName = resolved
		}
	} else if profileName == "" {
		profiles := manager.GetProfiles(engineName)
		if len(profiles) == 0 {
			log.Fatalf("Engine %q exposes no profiles", engineName)
		}

		fmt.Println("\nSelect a bypass profile to activate:")
		fmt.Println("  [A] AutoTune (Benchmark all profiles automatically)")
		for i, p := range profiles {
			fmt.Printf("  [%d] %s\n", i+1, p)
		}
		fmt.Printf("\nEnter choice [1-%d, A] (default: 1): ", len(profiles))

		var choice string
		fmt.Scanln(&choice)
		choice = strings.TrimSpace(strings.ToUpper(choice))

		if choice == "A" {
			fmt.Println("⚡ Starting AutoTune...")
			provider := providers.NewAutoTuneProvider(assets.BinDir, assets.LuaDir, assets.ListDir, assets.EngineSHA256)
			if provider == nil {
				log.Fatalf("No engine provider available for AutoTune on %s", runtime.GOOS)
			}
			allProfiles := engine.PrepareAutoTuneProfiles(provider, assets.LuaDir)
			result, err := engine.RunAutoTuneV2WithContext(context.Background(), provider, allProfiles)
			if err != nil {
				log.Fatalf("AutoTune failed: %v", err)
			}
			profileName = result.ProfileName
		} else {
			idx := 0
			if choice != "" {
				fmt.Sscanf(choice, "%d", &idx)
				idx--
			}
			if idx >= 0 && idx < len(profiles) {
				profileName = profiles[idx]
			} else {
				profileName = profiles[0]
			}
		}
	}
	fmt.Printf("Engine:  %s\n", engineName)
	fmt.Printf("Profile: %s\n", profileName)
	if debugMode {
		fmt.Println("Debug: ENABLED")
	}
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	hasPriv, err := manager.CheckPrivileges()
	if err != nil {
		log.Fatalf("Failed to check privileges: %v", err)
	}
	if !hasPriv {
		if runtime.GOOS == "windows" {
			log.Fatal("Administrator privileges required. Run as administrator.")
		}
		log.Fatal("Root privileges required. Re-run with sudo.")
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	ctx := context.Background()
	if err := manager.Start(ctx, engineName, profileName); err != nil {
		log.Fatalf("Failed to start engine: %v", err)
	}

	fmt.Println("✓ Engine started successfully")
	fmt.Println("Press Ctrl+C to stop...")

	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		var lastPrintedCount int
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				logs := manager.GetLogs()
				if len(logs) > lastPrintedCount {
					for _, line := range logs[lastPrintedCount:] {
						fmt.Printf("[LOG] %s\n", line)
					}
					lastPrintedCount = len(logs)
				}
			}
		}
	}()

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				targets := []struct{ Name, URL string }{
					{"YouTube", "https://www.youtube.com"},
					{"Discord", "https://discord.com"},
				}
				var parts []string
				for _, t := range targets {
					ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
					lat, err := engine.SimplePing(ctx, t.URL)
					cancel()
					if err == nil {
						parts = append(parts, fmt.Sprintf("%s: %dms", t.Name, lat.Milliseconds()))
					} else {
						parts = append(parts, fmt.Sprintf("%s: BLOCKED", t.Name))
					}
				}
				fmt.Printf("[PING] %s\n", strings.Join(parts, " | "))
			}
		}
	}()
	if runDuration > 0 {
		timer := time.NewTimer(runDuration)
		defer timer.Stop()
		select {
		case <-sigChan:
		case <-timer.C:
			fmt.Printf("Run duration %s elapsed; stopping...\n", runDuration)
		}
	} else {
		<-sigChan
	}

	fmt.Println("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("Shutting down gracefully...")
	close(done)

	stopDone := make(chan error, 1)
	go func() {
		stopDone <- manager.Stop()
	}()

	select {
	case err := <-stopDone:
		if err != nil {
			log.Printf("Error stopping engine: %v", err)
		}
	case <-time.After(5 * time.Second):
		log.Printf("Warning: engine stop timed out after 5 seconds, forcing exit")
		os.Exit(1)
	}
	fmt.Println("✓ Engine stopped")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
}
func resolveProfileAlias(available []string, input string) string {
	inputLower := strings.ToLower(strings.TrimSpace(input))
	if inputLower == "" {
		return ""
	}
	for _, p := range available {
		if strings.ToLower(p) == inputLower {
			return p
		}
	}
	for _, p := range available {
		if strings.Contains(strings.ToLower(p), inputLower) {
			return p
		}
	}
	keywords := map[string]string{
		"rec": "recommended", "univ": "universal", "alt1": "alternative 1",
		"alt2": "alternative 2", "alt3": "alternative 3", "adv1": "advanced 1",
		"adv2": "advanced 2", "adv3": "advanced 3", "adv4": "advanced 4", "adv5": "advanced 5",
		"ult": "ultimate", "disc": "discord", "yt": "youtube", "tg": "telegram",
	}
	if kw, ok := keywords[inputLower]; ok {
		for _, p := range available {
			if strings.Contains(strings.ToLower(p), kw) {
				return p
			}
		}
	}
	return input
}

func editBypassListCLI(listName string) {
	listsDir, err := engine.GetListsDir()
	if err != nil {
		fmt.Printf("❌ Ошибка получения директории списков: %v\n", err)
		return
	}
	targetPath := filepath.Join(listsDir, listName)
	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		_ = os.WriteFile(targetPath, []byte("# Добавьте домены/IP по одному на строку\n"), 0644)
	}

	fmt.Printf("Открываем %s...\n", targetPath)
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("notepad", targetPath)
	case "darwin":
		cmd = exec.Command("open", "-e", targetPath)
	default:
		editor := os.Getenv("EDITOR")
		if editor == "" {
			editor = "nano"
		}
		cmd = exec.Command(editor, targetPath)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	if err := cmd.Run(); err != nil {
		fmt.Printf("Warning: не удалось открыть редактор: %v\nПуть к файлу: %s\n", err, targetPath)
	}
}

func toggleSecureDNSCLI() {
	enabled := isSecureDNSEnabledImpl()
	newState := !enabled
	if err := setSecureDNSImpl(newState); err != nil {
		fmt.Printf("❌ Ошибка переключения Secure DNS: %v\n", err)
		return
	}
	if newState {
		fmt.Println("✓ Secure DNS (Cloudflare DoH) успешно включён!")
	} else {
		fmt.Println("✓ Secure DNS выключен (возвращены системные настройки DNS).")
	}
}

func runControlCenterMenu(debugMode bool) {
	attachConsole()
	for {
		dnsStatus := "отключен"
		if isSecureDNSEnabledImpl() {
			dnsStatus = "ВКЛЮЧЁН (Cloudflare DoH)"
		}

		fmt.Println("\n===================================================")
		fmt.Printf("  🚀 UNBOUND CONTROL CENTER v%s (%s/%s)\n", engine.Version, runtime.GOOS, runtime.GOARCH)
		fmt.Println("===================================================")
		fmt.Println(" [1] AutoTune — Автоматический поиск и запуск")
		fmt.Println(" [2] Запустить профиль по выбору")
		fmt.Println(" [3] Диагностика подключения (--test)")
		fmt.Println(" [4] Установить в автозапуск / службу")
		fmt.Println(" [5] Удалить из автозапуска / службы")
		fmt.Println(" [6] Редактировать список YouTube (youtube.txt)")
		fmt.Println(" [7] Редактировать список Discord (discord.txt)")
		fmt.Println(" [8] Редактировать список исключений (ipset-exclude.txt)")
		fmt.Printf(" [9] Переключить Secure DNS [Текущий статус: %s]\n", dnsStatus)
		fmt.Println(" [10] Проверить конфликты и статус")
		fmt.Println(" [11] Очистить кэш Discord")
		fmt.Println(" [12] Остановить все процессы обхода и сбросить драйверы")
		fmt.Println(" [13] Выход")
		fmt.Println("===================================================")
		fmt.Print("Выберите пункт меню (1-13): ")

		var choice string
		fmt.Scanln(&choice)
		choice = strings.TrimSpace(choice)

		switch choice {
		case "1":
			runHeadlessMode("", true, debugMode, 0)
			return
		case "2":
			runHeadlessMode("", false, debugMode, 0)
			return
		case "3":
			runTestProbe()
		case "4":
			if err := engine.EnableAutoStart(); err != nil {
				fmt.Printf("❌ Ошибка установки автозапуска: %v\n", err)
			} else {
				fmt.Println("✓ Автозапуск успешно установлен!")
			}
		case "5":
			if err := engine.DisableAutoStart(); err != nil {
				fmt.Printf("❌ Ошибка удаления автозапуска: %v\n", err)
			} else {
				fmt.Println("✓ Автозапуск успешно удалён!")
			}
		case "6":
			editBypassListCLI("youtube.txt")
		case "7":
			editBypassListCLI("discord.txt")
		case "8":
			editBypassListCLI("ipset-exclude.txt")
		case "9":
			toggleSecureDNSCLI()
		case "10":
			conflicts := checkConflictsImpl()
			if len(conflicts) == 0 {
				fmt.Println("✓ Конфликтующие процессы не обнаружены.")
			} else {
				fmt.Println("⚠️ Обнаружены конфликты:")
				for _, c := range conflicts {
					fmt.Println("  ", c)
				}
			}
		case "11":
			app := NewApp()
			if err := app.ClearDiscordCache(); err != nil {
				fmt.Printf("❌ Ошибка очистки кэша Discord: %v\n", err)
			} else {
				fmt.Println("✓ Кэш Discord очищен!")
			}
		case "12":
			app := NewApp()
			if err := app.KillWinws2(); err != nil {
				fmt.Printf("❌ Ошибка остановки процессов: %v\n", err)
			} else {
				fmt.Println("✓ Все процессы остановлены и правила сброшены!")
			}
		case "13":
			return
		default:
			fmt.Println("Неверный выбор, попробуйте снова.")
		}
	}
}
