package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
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
	profileName := flag.String("profile", "Unbound Ultimate (God Mode)", "Profile to use in CLI mode")
	trayMode := flag.Bool("tray", false, "Start minimized to system tray")
	debugMode := flag.Bool("debug", false, "Enable verbose debug logging")
	flag.Parse()

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
		Menu:             getAppMenu(app),
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

func runHeadlessMode(profileName string, debugMode bool) {
	attachConsole()

	fmt.Println("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("🚀 UNBOUND - Headless CLI Mode")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("Profile: %s\n", profileName)
	if debugMode {
		fmt.Println("Debug: ENABLED")
	}
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	assets, err := engine.ExtractAssets()
	if err != nil {
		log.Fatalf("Failed to extract assets: %v", err)
	}

	// Ensure dynamic lists exist
	fmt.Println("Checking for updated bypass lists...")
	if err := engine.EnsureListsExist(); err != nil {
		fmt.Printf("Warning: Failed to update lists: %v\n", err)
	}

	listsDir, err := engine.GetListsDir()
	if err != nil {
		listsDir = assets.ListDir
	}

	manager := providers.NewProviderManager()
	registerHeadlessProvider(manager, assets, listsDir, debugMode)

	hasPriv, err := manager.CheckPrivileges()
	if err != nil {
		log.Fatalf("Failed to check privileges: %v", err)
	}
	if !hasPriv {
		log.Fatal("Administrator privileges required. Run as administrator.")
	}

	ctx := context.Background()
	if err := manager.Start(ctx, "Zapret 2 (winws)", profileName); err != nil {
		log.Fatalf("Failed to start engine: %v", err)
	}

	fmt.Println("✓ Engine started successfully")
	fmt.Println("Press Ctrl+C to stop...")

	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			logs := manager.GetLogs()
			if len(logs) > 0 {
				lastLog := logs[len(logs)-1]
				fmt.Printf("[LOG] %s\n", lastLog)
			}
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("Shutting down gracefully...")
	if err := manager.Stop(); err != nil {
		log.Printf("Error stopping engine: %v", err)
	}
	fmt.Println("✓ Engine stopped")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
}
