package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()

	err := wails.Run(&options.App{
		Title:  "UNBOUND",
		Width:  1280,
		Height: 800,
		HideWindowOnClose: true,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 10, G: 10, B: 10, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
		},
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
