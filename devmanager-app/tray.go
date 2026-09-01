package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
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

// trayIconPNG dibuja el logo de la app a 16x16, con el mismo diseño que
// frontend/src/icons/logo.svg: "D" indigo sobre cuadrado oscuro redondeado
// (paridad de marca con el header y el icono de la app).
//
// Las formas se definen en las coordenadas 0-64 del SVG y un píxel se colorea
// muestreando su centro (con (x+0.5)*4). Las curvas de la "D" se aproximan
// muestreando los Béziers cúbicos del path original.
func trayIconPNG() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	bg := color.RGBA{R: 23, G: 27, B: 46, A: 255}      // #171B2E
	accent := color.RGBA{R: 99, G: 102, B: 241, A: 255} // #6366F1
	dot := color.RGBA{R: 165, G: 180, B: 252, A: 255}  // #A5B4FC

	dPoly := trayDPoly()
	counterPoly := trayCounterPoly()

	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			cx, cy := (float64(x)+0.5)*4, (float64(y)+0.5)*4
			c := color.RGBA{}
			switch {
			case trayInDot(cx, cy):
				c = dot
			case pointInPoly(cx, cy, counterPoly):
				c = bg
			case pointInPoly(cx, cy, dPoly):
				c = accent
			case trayInRoundedRect(cx, cy):
				c = bg
			}
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

// trayInRoundedRect reporta si (x,y) cae dentro del rect bg redondeado del logo.
func trayInRoundedRect(x, y float64) bool {
	minX, minY, maxX, maxY := 6.0, 6.0, 58.0, 58.0
	r := 14.0
	if x < minX || x > maxX || y < minY || y > maxY {
		return false
	}
	// Esquinas: carrot x solo cuenta dentro de los radios.
	cx := math.Max(minX+r-x, x-(maxX-r))
	cy := math.Max(minY+r-y, y-(maxY-r))
	return cx*cx+cy*cy <= r*r
}

// trayInDot reporta si (x,y) cae dentro del punto #A5B4FC del logo.
func trayInDot(x, y float64) bool {
	dx, dy := x-22, y-32
	return dx*dx+dy*dy <= 3*3
}

// trayDPoly aproxima el path exterior de la "D":
// M19 18 H34 C43.389 18 49 23.467 49 32 C49 40.533 43.389 46 34 46 H19 V18 Z
func trayDPoly() [][2]float64 {
	p := [][2]float64{{19, 18}, {34, 18}}
	p = append(p, cubicBezier(p[len(p)-1], [2]float64{43.389, 18}, [2]float64{49, 23.467}, [2]float64{49, 32})...)
	p = append(p, cubicBezier(p[len(p)-1], [2]float64{49, 40.533}, [2]float64{43.389, 46}, [2]float64{34, 46})...)
	p = append(p, [2]float64{19, 46})
	return p
}

// trayCounterPoly aproxima la contraforma (agujero) de la "D":
// M27 25 H33.5 C38.194 25 41 27.567 41 32 C41 36.433 38.194 39 33.5 39 H27 V25 Z
func trayCounterPoly() [][2]float64 {
	p := [][2]float64{{27, 25}, {33.5, 25}}
	p = append(p, cubicBezier(p[len(p)-1], [2]float64{38.194, 25}, [2]float64{41, 27.567}, [2]float64{41, 32})...)
	p = append(p, cubicBezier(p[len(p)-1], [2]float64{41, 36.433}, [2]float64{38.194, 39}, [2]float64{33.5, 39})...)
	p = append(p, [2]float64{27, 39})
	return p
}

// cubicBezier muestra un Bézier cúbico desde start por c1/c2 hasta end.
func cubicBezier(start, c1, c2, end [2]float64) [][2]float64 {
	const n = 16
	out := make([][2]float64, 0, n)
	for i := 1; i <= n; i++ {
		t := float64(i) / n
		mt := 1 - t
		x := mt*mt*mt*start[0] + 3*mt*mt*t*c1[0] + 3*mt*t*t*c2[0] + t*t*t*end[0]
		y := mt*mt*mt*start[1] + 3*mt*mt*t*c1[1] + 3*mt*t*t*c2[1] + t*t*t*end[1]
		out = append(out, [2]float64{x, y})
	}
	return out
}

// pointInPoly aplica el ray-casting estándar sobre un polígono cerrado.
func pointInPoly(x, y float64, poly [][2]float64) bool {
	inside := false
	for i, j := 0, len(poly)-1; i < len(poly); j, i = i, i+1 {
		xi, yi := poly[i][0], poly[i][1]
		xj, yj := poly[j][0], poly[j][1]
		if (yi > y) != (yj > y) && x < (xj-xi)*(y-yi)/(yj-yi)+xi {
			inside = !inside
		}
	}
	return inside
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
