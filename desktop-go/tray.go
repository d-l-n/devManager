package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	goruntime "runtime"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/energye/systray"
)

// Tray spike (Fase 3, spec §5.1): icono en bandeja conviviendo con Wails.
// RunWithExternalLoop devuelve (start, end): start es un pump de mensajes
// (GetMessageW) que DEBE correr en su propio hilo con LockOSThread; end se
// llama al cerrar la app. OnBeforeClose oculta la ventana salvo salida forzada
// desde el menú Quit (paridad closeEvent Python).

// trayIcon genera un .ICO válido en memoria (ICONDIR + ICONDIRENTRY + PNG
// embebido, formato soportado desde Vista) porque LoadImage de Windows
// rechaza PNG crudo.
func trayIcon() []byte {
	png := trayIconPNG()
	ico := make([]byte, 6+16+len(png))
	le := binary.LittleEndian
	le.PutUint16(ico[0:], 0) // reserved
	le.PutUint16(ico[2:], 1) // type: icon
	le.PutUint16(ico[4:], 1) // count
	e := ico[6:]
	e[0] = 16              // width
	e[1] = 16              // height
	e[2] = 0               // palette
	e[3] = 0               // reserved
	le.PutUint16(e[4:], 1) // planes
	le.PutUint16(e[6:], 32)
	le.PutUint32(e[8:], uint32(len(png)))
	le.PutUint32(e[12:], 22) // offset
	copy(ico[22:], png)
	return ico
}

func trayIconPNG() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	c := color.RGBA{R: 99, G: 102, B: 241, A: 255} // accent dark #6366f1
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			dx, dy := float64(x)-7.5, float64(y)-7.5
			if dx*dx+dy*dy <= 36 {
				img.Set(x, y, c)
			}
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

// runTray lanza el pump de la bandeja en un hilo dedicado. Llamar una sola vez.
func runTray(onReady func()) (stop func()) {
	start, end := systray.RunWithExternalLoop(onReady, func() {})
	go func() {
		goruntime.LockOSThread()
		start() // bloquea hasta end()
	}()
	return func() { end() }
}

func (a *App) onTrayReady() {
	a.mu.Lock()
	a.trayOK = true
	a.mu.Unlock()

	systray.SetIcon(trayIcon())
	systray.SetTooltip("Local Dev Manager")

	show := systray.AddMenuItem("Show Window", "Mostrar la ventana principal")
	startAll := systray.AddMenuItem("Start All Enabled", "")
	stopAll := systray.AddMenuItem("Stop All", "")
	systray.AddSeparator()
	quit := systray.AddMenuItem("Quit (real exit)", "")

	show.Click(func() {
		wailsruntime.WindowUnminimise(a.ctx)
		wailsruntime.WindowShow(a.ctx)
	})
	startAll.Click(func() { a.startAllEnabledServers() })
	stopAll.Click(func() { a.stopAllRunners() })
	quit.Click(func() {
		a.mu.Lock()
		a.forceExit = true
		a.mu.Unlock()
		systray.Quit()
	})
}

// startAllEnabledServers replica _on_start_all del Python.
func (a *App) startAllEnabledServers() {
	projects := a.cfg.Projects()
	for idx, p := range projects {
		if !p.Server.Enabled {
			continue
		}
		if sm, _, _ := a.ensureManagers(idx); sm != nil {
			sm.Start()
		}
	}
}
