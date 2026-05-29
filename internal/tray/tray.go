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
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/math/f64"
)

//go:embed fan.svg
var fanSVG string

const (
	numFrames = 600
	frameStep = 360.0 / numFrames // 0.6° per frame
	frameSize = 44                // px, @2x retina
	animFPS   = 10
)

// palette[theme 0=light 1=dark][cpu step 0-3]
var palette = [2][4]string{
	{"#111111", "#882200", "#BB1100", "#DD0000"},
	{"#EEEEEE", "#FF9999", "#FF4444", "#FF2222"},
}

// pngFrames holds encoded PNG bytes used only during startup loading.
// Cleared after NSImages are pre-loaded to free the memory.
var pngFrames [2][4][numFrames][]byte

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

func rotateImage(src *image.RGBA, degrees float64) *image.RGBA {
	b := src.Bounds()
	cx, cy := float64(b.Max.X)/2, float64(b.Max.Y)/2
	rad := degrees * math.Pi / 180
	cos, sin := math.Cos(rad), math.Sin(rad)
	// Inverse transform (dst→src) for clockwise rotation.
	// Negative degrees produce counter-clockwise rotation.
	m := f64.Aff3{
		cos, -sin, cx*(1-cos) + cy*sin,
		sin, cos, cy*(1-cos) - cx*sin,
	}
	dst := image.NewRGBA(b)
	xdraw.BiLinear.Transform(dst, m, src, b, xdraw.Src, nil)
	return dst
}

func toPNG(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	err := png.Encode(&buf, img)
	return buf.Bytes(), err
}

func preRenderFrames() error {
	log.Printf("tray: pre-rendering %d frames...", 2*4*numFrames)

	var bases [2][4]*image.RGBA
	for theme := range bases {
		for step := range bases[theme] {
			base, err := renderBase(palette[theme][step])
			if err != nil {
				return err
			}
			bases[theme][step] = base
		}
	}

	var wg sync.WaitGroup
	errCh := make(chan error, 8)

	for theme := range pngFrames {
		for step := range pngFrames[theme] {
			wg.Add(1)
			go func(theme, step int) {
				defer wg.Done()
				base := bases[theme][step]
				for f := range pngFrames[theme][step] {
					rot := rotateImage(base, -float64(f)*frameStep)
					data, err := toPNG(rot)
					if err != nil {
						errCh <- err
						return
					}
					pngFrames[theme][step][f] = data
				}
			}(theme, step)
		}
	}

	wg.Wait()
	close(errCh)
	if err := <-errCh; err != nil {
		return err
	}
	log.Printf("tray: frame pre-render complete")
	return nil
}

// frameIndex maps (theme, step, f) to a flat index in the C image table.
func frameIndex(theme, step, f int) int {
	return theme*4*numFrames + step*numFrames + f
}

func loadFramesIntoCocoa() {
	total := 2 * 4 * numFrames
	C.preloadImagesInit(C.int(total))

	var wg sync.WaitGroup
	for theme := range pngFrames {
		for step := range pngFrames[theme] {
			wg.Add(1)
			go func(theme, step int) {
				defer wg.Done()
				for f, data := range pngFrames[theme][step] {
					idx := frameIndex(theme, step, f)
					C.loadImageAtIndex(
						C.int(idx),
						(*C.uchar)(unsafe.Pointer(&data[0])),
						C.int(len(data)),
					)
				}
			}(theme, step)
		}
	}
	wg.Wait()

	// Free PNG bytes — NSImages are now loaded, these are no longer needed.
	pngFrames = [2][4][numFrames][]byte{}
	log.Printf("tray: NSImages loaded")
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

// Run pre-renders frames, loads them into Cocoa as NSImages, then starts the
// NSApp run loop. Blocks until quit. Must be called from the main goroutine.
func (t *Tray) Run(ctx context.Context) {
	if err := preRenderFrames(); err != nil {
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

	// One-time cost: pre-load all frames as NSImages so the animate loop
	// does a pointer swap instead of a PNG decode on each frame.
	loadFramesIntoCocoa()
	C.setIconIndex(0)

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
	lastTheme, lastStep, lastFrame := -1, -1, -1
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
		frameIdx := int(angle/frameStep) % numFrames

		if theme == lastTheme && step == lastStep && frameIdx == lastFrame {
			continue
		}
		C.setIconIndex(C.int(frameIndex(theme, step, frameIdx)))
		lastTheme, lastStep, lastFrame = theme, step, frameIdx
	}
}
