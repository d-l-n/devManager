package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"strconv"
	"strings"
	goruntime "runtime"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/energye/systray"
)

// Tray (Fase 3, spec §5.1): icono en bandeja conviviendo con Wails.
// RunWithExternalLoop devuelve (start, end): start es un pump de mensajes
// (GetMessageW) que DEBE correr en su propio hilo con LockOSThread; end se
// llama al cerrar la app. OnBeforeClose oculta la ventana salvo salida forzada
// (paridad closeEvent Python). El menú se reconstruye dinámicamente con
// submenús por proyecto + notificaciones nativas cuando la ventana está oculta
// (paridad DevManagerTray de Python).

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

	a.rebuildTrayMenu()

	// Click-to-focus (paridad _on_activated Trigger/DoubleClick).
	systray.SetOnClick(func(menu systray.IMenu) { a.showMainWindow() })
	systray.SetOnDClick(func(menu systray.IMenu) { a.showMainWindow() })

	// Refresco periódico del menú (solo si cambió la firma de estados).
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			a.mu.Lock()
			ok := a.trayOK
			a.mu.Unlock()
			if !ok {
				return
			}
			a.rebuildTrayMenu()
		}
	}()
}

// itoa convierte int a string para etiquetas del menú.
func itoa(n int) string { return strconv.Itoa(n) }

// rebuildTrayMenu reconstruye el menú de bandeja con un submenú por proyecto
// (Start/Stop Server + Open URL), acciones globales y salida. Read-only si la
// firma de estados no cambió. Las llamadas a systray.* se pueden invocar desde
// cualquier goroutine (el lib las encola al pump).
func (a *App) rebuildTrayMenu() {
	a.mu.Lock()
	if !a.trayOK {
		a.mu.Unlock()
		return
	}
	a.mu.Unlock()

	// Conduce: el tray puede disparar onTrayReady antes de que startup
	// inicialice cfg (p.ej. al regenerar bindings Wails). Sin cfg no hay menú.
	if a.cfg == nil {
		return
	}

	projects := a.cfg.Projects()

	var sig strings.Builder
	for i, p := range projects {
		st := a.GetServerStatus(i)
		sig.WriteString(fmt.Sprintf("%d:%s:%s;", i, p.Name, st.State))
	}
	newSig := sig.String()

	a.mu.Lock()
	if newSig == a.traySig {
		a.mu.Unlock()
		return
	}
	a.traySig = newSig
	a.mu.Unlock()

	systray.ResetMenu()

	title := systray.AddMenuItem("Local Dev Manager", "Mostrar ventana")
	title.Click(func() { a.showMainWindow() })
	systray.AddSeparator()

	if len(projects) == 0 {
		noP := systray.AddMenuItem("No projects", "")
		noP.Disable()
	} else {
		for i, p := range projects {
			st := a.GetServerStatus(i)
			sub := systray.AddMenuItem(p.Name, "Acciones del proyecto")
			if st.Running {
				stop := sub.AddSubMenuItem("Stop Server (:"+itoa(p.Server.Port)+")", "")
				stop.Click(func() { a.StopServer(i) })
			} else {
				start := sub.AddSubMenuItem("Start Server (:"+itoa(p.Server.Port)+")", "")
				start.Click(func() { a.StartServer(i) })
			}
			if p.Server.Enabled && p.Server.URL != "" {
				url := sub.AddSubMenuItem("Open "+p.Server.URL, "")
				url.Click(func() { a.OpenURL(p.Server.URL) })
			}
		}
	}

	systray.AddSeparator()
	startAll := systray.AddMenuItem("Start All Servers", "")
	startAll.Click(func() { a.startAllEnabledServers() })
	stopAll := systray.AddMenuItem("Stop All Servers", "")
	stopAll.Click(func() { a.stopAllRunners() })
	systray.AddSeparator()
	show := systray.AddMenuItem("Show / Focus Window", "")
	show.Click(func() { a.showMainWindow() })
	quit := systray.AddMenuItem("Quit (real exit)", "")
	quit.Click(func() {
		a.mu.Lock()
		a.forceExit = true
		a.mu.Unlock()
		wailsruntime.Quit(a.ctx)
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
