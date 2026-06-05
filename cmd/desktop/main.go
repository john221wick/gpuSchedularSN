package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"

	"github.com/john221wick/gpuSchedularSN/internal/desktop"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := desktop.NewApp()

	err := wails.Run(&options.App{
		Title:         "GPU Scheduler",
		Width:         1280,
		Height:        800,
		MinWidth:      900,
		MinHeight:     600,
		DisableResize: false,
		Mac: &mac.Options{
			DisableZoom: false,
			Preferences: &mac.Preferences{
				FullscreenEnabled: mac.Enabled,
			},
		},
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup:  app.Startup,
		OnShutdown: app.Shutdown,
		Bind: []interface{}{
			app,
		},
	})
	if err != nil {
		panic(err)
	}
}
