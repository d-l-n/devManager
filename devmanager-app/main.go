package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

// El ejecutable debe servir el bundle de Vite: los módulos fuente contienen
// imports bare (por ejemplo, "reicon") que el WebView no puede resolver.
//go:embed all:frontend/dist
var assets embed.FS

const singleInstanceID = "0bb0205d-84ed-4e78-b74c-91df6449898c"

func singleInstanceLock(app *App) *options.SingleInstanceLock {
	return &options.SingleInstanceLock{
		UniqueId: singleInstanceID,
		OnSecondInstanceLaunch: func(options.SecondInstanceData) {
			app.showMainWindow()
		},
	}
}

func main() {
	app := NewApp()

	// Tray spike: pump dedicado; stop al salir de main.
	stopTray := runTray(app.onTrayReady)
	defer stopTray()

	err := wails.Run(&options.App{
		Title:     "Local Dev Manager",
		Width:     1280,
		Height:    800,
		MinWidth:  960,
		MinHeight: 640,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup:     app.startup,
		OnShutdown:    app.shutdown,
		OnBeforeClose: app.beforeClose,
		SingleInstanceLock: singleInstanceLock(app),
		Bind: []interface{}{
			app,
		},
		Windows: &windows.Options{
			Theme: windows.Dark,
		},
	})
	if err != nil {
		println("Error:", err.Error())
	}
}
