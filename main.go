package main

import (
	"embed"
	"os"

	"github.com/opendownload/opendownload/cmd"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:client/dist
var assets embed.FS

func main() {
	if len(os.Args) > 1 {
		if err := cmd.Execute(); err != nil {
			os.Exit(1)
		}
		return
	}

	app := NewApp()

	err := wails.Run(&options.App{
		Title:  "OpenDownload",
		Width:  1200,
		Height: 800,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup: app.startup,
		OnShutdown: app.shutdown,
		Bind: []interface{}{
			app,
		},
	})
	if err != nil {
		println("Error:", err.Error())
	}
}
