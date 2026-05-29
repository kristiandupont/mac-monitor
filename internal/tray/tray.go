package tray

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa -framework QuartzCore
#include "statusbar.h"
#include <stdlib.h>
*/
import "C"
import (
	"bytes"
	"context"
	_ "embed"
	"image"
	"image/png"
	"log"
	"math"
	"os/exec"
	"strings"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
)

//go:embed fan.svg
var fanSVG string

const (
	frameSize = 44 // px, @2x retina
	animFPS   = 30
)

// palette[theme 0=light 1=dark][cpu step 0-3]
var palette = [2][4]string{
	{"#111111", "#882200", "#BB1100", "#DD0000"},
	{"#EEEEEE", "#FF9999", "#FF4444", "#FF2222"},
}

// pngBases holds 8 encoded PNGs (2 themes × 4 color steps). Cleared after loading.
var pngBases [2][4][]byte

var (
	darkModeCached bool
	darkModeAt     time.Time
)

func isDarkMode() bool {
	if time.Since(darkModeAt) > 5*time.Second {
		out, _ := exec.Command("defaults", "read", "-g", "AppleInterfaceStyle").Output()
		darkModeCached = strings.TrimSpace(string(out)) == "Dark"
		darkModeAt = time.Now()
	}
	return darkModeCached
}

// colorStep returns 0 for CPU < 80%; transitions through 3 steps in the top 20%.
func colorStep(cpu float64) int {
	switch {
	case cpu < 80:
		return 0
	case cpu < 87:
		return 1
	case cpu < 93:
		return 2
	default:
		return 3
	}
}

// angularVelocity returns degrees/second.
// ~15% CPU ≈ 1 RPM; ~100% CPU ≈ 1 RPS.
func angularVelocity(cpu float64) float64 {
	if cpu < 0.5 {
		return 0
	}
	return 0.015 * math.Pow(cpu, 2.2)
}

func renderBase(hexColor string) (*image.RGBA, error) {
	svg := strings.ReplaceAll(fanSVG, "currentColor", hexColor)
	icon, err := oksvg.ReadIconStream(strings.NewReader(svg))
	if err != nil {
		return nil, err
	}
	icon.SetTarget(0, 0, frameSize, frameSize)
	img := image.NewRGBA(image.Rect(0, 0, frameSize, frameSize))
	scanner := rasterx.NewScannerGV(frameSize, frameSize, img, img.Bounds())
	raster := rasterx.NewDasher(frameSize, frameSize, scanner)
	icon.Draw(raster, 1.0)
	return img, nil
}

func toPNG(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	err := png.Encode(&buf, img)
	return buf.Bytes(), err
}

func preRenderBases() error {
	log.Printf("tray: pre-rendering %d base images...", 2*4)
	for theme := range pngBases {
		for step := range pngBases[theme] {
			base, err := renderBase(palette[theme][step])
			if err != nil {
				return err
			}
			data, err := toPNG(base)
			if err != nil {
				return err
			}
			pngBases[theme][step] = data
		}
	}
	log.Printf("tray: base image render complete")
	return nil
}

// colorIndex maps (theme, step) to a flat index in the color image table.
func colorIndex(theme, step int) int {
	return theme*4 + step
}

func loadColorImagesIntoCocoa() {
	total := 2 * 4
	C.preloadColorImagesInit(C.int(total))
	for theme := range pngBases {
		for step, data := range pngBases[theme] {
			idx := colorIndex(theme, step)
			C.loadColorImage(
				C.int(idx),
				(*C.uchar)(unsafe.Pointer(&data[0])),
				C.int(len(data)),
			)
		}
	}
	pngBases = [2][4][]byte{}
	log.Printf("tray: color images loaded")
}

// Tray manages the macOS menu bar icon and menu.
type Tray struct {
	cpu    atomic.Int64 // 0–100
	cancel func()
	addr   string
}

func New(cancel func(), addr string) *Tray {
	return &Tray{cancel: cancel, addr: addr}
}

func (t *Tray) SetCPU(pct float64) {
	t.cpu.Store(int64(pct))
}

// Run pre-renders base images, loads them into Cocoa, then starts the NSApp run loop.
// Blocks until quit. Must be called from the main goroutine.
func (t *Tray) Run(ctx context.Context) {
	if err := preRenderBases(); err != nil {
		log.Printf("tray: prerender: %v", err)
		return
	}

	C.initCocoaApp()

	tooltip := C.CString("Mac Monitor")
	C.setupStatusItem(tooltip)
	C.free(unsafe.Pointer(tooltip))

	openLabel := C.CString("Open Dashboard")
	C.addMenuItemCStr(openLabel, C.int(menuItemOpen))
	C.free(unsafe.Pointer(openLabel))
	C.addMenuSeparatorItem()
	quitLabel := C.CString("Quit")
	C.addMenuItemCStr(quitLabel, C.int(menuItemQuit))
	C.free(unsafe.Pointer(quitLabel))

	loadColorImagesIntoCocoa()
	C.setIconFrame(0, 0)

	go t.animate()
	go t.handleEvents(ctx)

	C.runCocoaApp() // blocks until quitCocoaApp is called
}

func (t *Tray) handleEvents(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			C.quitCocoaApp()
			return
		case itemID := <-menuClickCh:
			switch itemID {
			case menuItemOpen:
				exec.Command("open", "http://localhost"+t.addr).Run() //nolint:errcheck
			case menuItemQuit:
				t.cancel()
				C.quitCocoaApp()
				return
			}
		}
	}
}

func (t *Tray) animate() {
	ticker := time.NewTicker(time.Second / animFPS)
	defer ticker.Stop()
	var angle, smoothedVel float64
	last := time.Now()
	lastTheme, lastStep := -1, -1
	const velTau = 3.0

	for range ticker.C {
		now := time.Now()
		dt := now.Sub(last).Seconds()
		last = now

		cpu := float64(t.cpu.Load())
		alpha := 1 - math.Exp(-dt/velTau)
		smoothedVel += alpha * (angularVelocity(cpu) - smoothedVel)
		angle = math.Mod(angle+smoothedVel*dt, 360)

		theme := 0
		if isDarkMode() {
			theme = 1
		}
		step := colorStep(cpu)
		colorIdx := colorIndex(theme, step)

		// Only skip if neither color nor motion changed.
		if smoothedVel == 0 && theme == lastTheme && step == lastStep {
			continue
		}
		C.setIconFrame(C.int(colorIdx), C.float(angle))
		lastTheme, lastStep = theme, step
	}
}
