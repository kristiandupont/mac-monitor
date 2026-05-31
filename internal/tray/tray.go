package tray

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa -framework QuartzCore -framework ServiceManagement
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

// baseColor[theme] is the icon color at low CPU (near-black / near-white).
// hotColor[theme] is the fully-heated red at 100% CPU.
// Between heatThreshold and 100% CPU the two are linearly interpolated.
const heatThreshold = 50.0

var baseColor = [2][3]float64{
	{0x11 / 255.0, 0x11 / 255.0, 0x11 / 255.0}, // light: near-black
	{0xEE / 255.0, 0xEE / 255.0, 0xEE / 255.0}, // dark: near-white
}

var hotColor = [2][3]float64{
	{0xDD / 255.0, 0x00 / 255.0, 0x00 / 255.0}, // light: deep red
	{0xFF / 255.0, 0x22 / 255.0, 0x22 / 255.0}, // dark: bright red
}

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

// interpolateColor returns an sRGB tint for the current CPU% and theme.
// Below heatThreshold the icon stays at its base color; above it glides to red.
func interpolateColor(cpu float64, theme int) (float32, float32, float32) {
	t := math.Max(0, (cpu-heatThreshold)/(100-heatThreshold))
	base, hot := baseColor[theme], hotColor[theme]
	return float32(base[0] + t*(hot[0]-base[0])),
		float32(base[1] + t*(hot[1]-base[1])),
		float32(base[2] + t*(hot[2]-base[2]))
}

// angularVelocity returns degrees/second.
// ~15% CPU ≈ 1 RPM; ~100% CPU ≈ 1 RPS.
func angularVelocity(cpu float64) float64 {
	if cpu < 0.5 {
		return 0
	}
	return 0.020 * math.Pow(cpu, 2.2)
}

// renderBasePNG renders the fan SVG in white and returns the PNG bytes.
// The alpha channel of the rendered image is used as a CALayer mask; the
// actual pixel color is irrelevant — only the shape matters.
func renderBasePNG() ([]byte, error) {
	svg := strings.ReplaceAll(fanSVG, "currentColor", "#FFFFFF")
	icon, err := oksvg.ReadIconStream(strings.NewReader(svg))
	if err != nil {
		return nil, err
	}
	icon.SetTarget(0, 0, frameSize, frameSize)
	img := image.NewRGBA(image.Rect(0, 0, frameSize, frameSize))
	scanner := rasterx.NewScannerGV(frameSize, frameSize, img, img.Bounds())
	raster := rasterx.NewDasher(frameSize, frameSize, scanner)
	icon.Draw(raster, 1.0)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Tray manages the macOS menu bar icon and menu.
type Tray struct {
	cpu         atomic.Int64 // 0–100
	animateIcon atomic.Bool
	cancel      func()
	addr        string
}

func New(cancel func(), addr string) *Tray {
	return &Tray{cancel: cancel, addr: addr}
}

func (t *Tray) SetCPU(pct float64) {
	t.cpu.Store(int64(pct))
}

// Run renders the base icon, loads it into Cocoa, then starts the NSApp run loop.
// Blocks until quit. Must be called from the main goroutine.
func (t *Tray) Run(ctx context.Context) {
	baseData, err := renderBasePNG()
	if err != nil {
		log.Printf("tray: render base: %v", err)
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

	animKey := C.CString("animateIcon")
	animOn := int(C.loadUserDefaultBool(animKey, 1)) != 0
	C.free(unsafe.Pointer(animKey))
	t.animateIcon.Store(animOn)
	animLabel := C.CString("Animate Icon")
	C.addCheckboxMenuItemCStr(animLabel, C.int(menuItemAnimateIcon), boolToInt(animOn))
	C.free(unsafe.Pointer(animLabel))

	launchOn := int(C.getLaunchAtLogin()) != 0
	launchLabel := C.CString("Launch at Login")
	C.addCheckboxMenuItemCStr(launchLabel, C.int(menuItemLaunchAtLogin), boolToInt(launchOn))
	C.free(unsafe.Pointer(launchLabel))

	C.addMenuSeparatorItem()
	quitLabel := C.CString("Quit")
	C.addMenuItemCStr(quitLabel, C.int(menuItemQuit))
	C.free(unsafe.Pointer(quitLabel))

	C.loadBaseImage((*C.uchar)(unsafe.Pointer(&baseData[0])), C.int(len(baseData)))

	theme := 0
	if isDarkMode() {
		theme = 1
	}
	r, g, b := interpolateColor(0, theme)
	C.setIconFrame(C.float(0), C.float(r), C.float(g), C.float(b))

	go t.animate()
	go t.handleEvents(ctx)

	C.runCocoaApp() // blocks until quitCocoaApp is called
}

func boolToInt(b bool) C.int {
	if b {
		return 1
	}
	return 0
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
			case menuItemAnimateIcon:
				next := !t.animateIcon.Load()
				t.animateIcon.Store(next)
				C.setMenuItemChecked(C.int(menuItemAnimateIcon), boolToInt(next))
				key := C.CString("animateIcon")
				C.saveUserDefaultBool(key, boolToInt(next))
				C.free(unsafe.Pointer(key))
			case menuItemLaunchAtLogin:
				next := int(C.getLaunchAtLogin()) == 0 // toggle
				C.setLaunchAtLogin(boolToInt(next))
				// Re-read the actual state in case registration failed.
				actual := int(C.getLaunchAtLogin()) != 0
				C.setMenuItemChecked(C.int(menuItemLaunchAtLogin), boolToInt(actual))
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
	var angle, smoothedVel, smoothedCPU float64
	last := time.Now()
	lastTheme := -1
	lastSmoothedCPU := -1.0
	lastAnimating := true
	const velTau = 3.0
	const colorTau = 0.4

	for range ticker.C {
		now := time.Now()
		dt := now.Sub(last).Seconds()
		last = now

		cpu := float64(t.cpu.Load())
		animating := t.animateIcon.Load()

		if animating {
			velAlpha := 1 - math.Exp(-dt/velTau)
			smoothedVel += velAlpha * (angularVelocity(cpu) - smoothedVel)
			angle = math.Mod(angle+smoothedVel*dt, 360)
		} else {
			smoothedVel = 0
			angle = 0
		}

		colorAlpha := 1 - math.Exp(-dt/colorTau)
		smoothedCPU += colorAlpha * (cpu - smoothedCPU)

		theme := 0
		if isDarkMode() {
			theme = 1
		}

		animChanged := animating != lastAnimating
		lastAnimating = animating

		if !animChanged && smoothedVel == 0 && theme == lastTheme && math.Abs(smoothedCPU-lastSmoothedCPU) < 0.1 {
			continue
		}
		r, g, b := interpolateColor(smoothedCPU, theme)
		C.setIconFrame(C.float(angle), C.float(r), C.float(g), C.float(b))
		lastTheme, lastSmoothedCPU = theme, smoothedCPU
	}
}
