package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
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
	// Empty means "the engine's first profile", which is the only default that
	// is correct on every platform: the Windows and Linux engines ship
	// completely different profile names.
	profileName := flag.String("profile", "", "Profile to use in CLI mode (default: the engine's first profile)")
	trayMode := flag.Bool("tray", false, "Start minimized to system tray")
	debugMode := flag.Bool("debug", false, "Enable verbose debug logging")
	showVersion := flag.Bool("version", false, "Print the version and exit")
	listProfiles := flag.Bool("list-profiles", false, "List the profiles available on this platform and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("unbound %s (%s/%s)\n", engine.Version, runtime.GOOS, runtime.GOARCH)
		return
	}

	if *listProfiles {
		runListProfiles(*debugMode)
		return
	}

	if *cliMode {
		runHeadlessMode(*profileName, *debugMode)
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

func runListProfiles(debugMode bool) {
	attachConsole()

	manager, _ := newHeadlessManager(debugMode)
	for _, name := range manager.GetEngineNames() {
		fmt.Printf("%s:\n", name)
		for _, profile := range manager.GetProfiles(name) {
			fmt.Printf("  %s\n", profile)
		}
	}
}

func runHeadlessMode(profileName string, debugMode bool) {
	attachConsole()

	fmt.Println("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("🚀 UNBOUND - Headless CLI Mode")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Ensure dynamic lists exist
	fmt.Println("Checking for updated bypass lists...")
	if err := engine.EnsureListsExist(); err != nil {
		fmt.Printf("Warning: Failed to update lists: %v\n", err)
	}

	manager, _ := newHeadlessManager(debugMode)

	// The engine name was hardcoded to "Zapret 2 (winws)", so --cli could only
	// ever work on Windows; on any other platform it aborted with
	// "engine not found" naming an engine that does not exist there.
	engineNames := manager.GetEngineNames()
	if len(engineNames) == 0 {
		log.Fatalf("No bypass engine is available on %s", runtime.GOOS)
	}
	engineName := engineNames[0]

	// Likewise the default profile was a Windows profile name. Fall back to
	// whatever the active engine actually offers.
	if profileName == "" {
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
