package main

import (
	"embed"
	"log"

	"github.com/AbdeljalilB92/lldl/app"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	service := app.WireForGUI(app.WireGUIConfig{})

	err := wails.Run(&options.App{
		Title:  "lldl - LinkedIn Learning Downloader",
		Width:  900,
		Height: 700,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup:  service.OnStartup,
		OnShutdown: service.OnShutdown,
		Bind: []interface{}{
			service,
		},
	})
	if err != nil {
		log.Fatalf("failed to start GUI: %v", err)
	}
}
