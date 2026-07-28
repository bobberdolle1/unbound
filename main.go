package main

import (
	"context"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
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
	cliMode := flag.Bool("cli", false, "Run in headless CLI mode")
	profileName := flag.String("profile", "", "Profile to use in CLI mode (default: the engine's first profile)")
	autoTuneMode := flag.Bool("autotune", false, "Run AutoTune benchmark in CLI mode and start the best profile")
	jsonOutput := flag.Bool("json", false, "Output profile list or status in JSON format")
	trayMode := flag.Bool("tray", false, "Start minimized to system tray")
	debugMode := flag.Bool("debug", false, "Enable verbose debug logging")
	showVersion := flag.Bool("version", false, "Print the version and exit")
	listProfiles := flag.Bool("list-profiles", false, "List the profiles available on this platform and exit")

	flag.Usage = func() {
		fmt.Printf("UNBOUND ClearFlow Engine v%s (%s/%s)\n", engine.Version, runtime.GOOS, runtime.GOARCH)
		fmt.Println("Usage: unbound [options]")
		fmt.Println("Options:")
		flag.PrintDefaults()
		fmt.Println("\nExamples:")
		fmt.Println("  unbound --cli                                Run headless CLI mode with default profile")
		fmt.Println("  unbound --cli --autotune                     Run AutoTune in CLI and start the best profile")
		fmt.Println("  unbound --cli --profile=\"Alternative 2\"       Start CLI mode with specific profile")
		fmt.Println("  unbound --list-profiles                      List all profiles for this OS")
		fmt.Println("  unbound --list-profiles --json               List profiles in JSON format")
	}

	flag.Parse()

	if *showVersion {
		if *jsonOutput {
			fmt.Printf("{\"version\":\"%s\",\"os\":\"%s\",\"arch\":\"%s\"}\n", engine.Version, runtime.GOOS, runtime.GOARCH)
		} else {
			fmt.Printf("unbound %s (%s/%s)\n", engine.Version, runtime.GOOS, runtime.GOARCH)
		}
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

	if *cliMode || *autoTuneMode {
		runHeadlessMode(*profileName, *autoTuneMode, *debugMode)
		return
	}
	app := NewApp()
	app.startMinimized = *trayMode
	app.debugMode = *debugMode

	err := wails.Run(&options.App{
		Title:             "UNBOUND",
		Width:             400,
		Height:            650,
		Frameless:         true,
		DisableResize:     true,
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
func runHeadlessMode(profileName string, runAutoTune bool, debugMode bool) {
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

	engineNames := manager.GetEngineNames()
	if len(engineNames) == 0 {
		log.Fatalf("No bypass engine is available on %s", runtime.GOOS)
	}
	engineName := engineNames[0]

	if runAutoTune {
		fmt.Println("⚡ Running AutoTune benchmark in CLI mode...")
		provider := providers.NewAutoTuneProvider(assets.BinDir, assets.LuaDir, assets.ListDir)
		if provider == nil {
			log.Fatalf("No engine provider available for AutoTune on %s", runtime.GOOS)
		}
		allProfiles := append(engine.GetProfiles(assets.LuaDir), engine.GetAdvancedProfiles(assets.LuaDir)...)
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
	} else if profileName == "" {
		profiles := manager.GetProfiles(engineName)
		if len(profiles) == 0 {
			log.Fatalf("Engine %q exposes no profiles", engineName)
		}
		profileName = profiles[0]
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
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

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
