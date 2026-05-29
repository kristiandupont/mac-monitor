package tray

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

	"github.com/getlantern/systray"
	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/math/f64"
)

//go:embed fan.svg
var fanSVG string

const (
	numFrames = 600              // 0.6° per frame
	frameStep = 360.0 / numFrames
	frameSize = 44
	animFPS   = 30
)

// palette[theme 0=light 1=dark][cpu step 0-3]
// Color only starts shifting at 80% CPU; both themes converge toward red.
var palette = [2][4]string{
	{"#111111", "#882200", "#BB1100", "#DD0000"}, // light: near-black → dark red
	{"#EEEEEE", "#FF9999", "#FF4444", "#FF2222"}, // dark: near-white → bright red
}

// frames[theme][step][angleIndex] = PNG bytes
var frames [2][4][numFrames][]byte

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

// colorStep returns 0 below 80% CPU; transitions through 3 steps in the top 20%.
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

// angularVelocity returns degrees/second for a given CPU %.
// Calibrated so ~15% CPU ≈ 1 RPM and ~100% CPU ≈ 1 RPS.
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
	// Inverse transform (dst→src) for clockwise rotation of the image.
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

	for theme := range frames {
		for step := range frames[theme] {
			wg.Add(1)
			go func(theme, step int) {
				defer wg.Done()
				base := bases[theme][step]
				for f := range frames[theme][step] {
					// Negative angle = counter-clockwise rotation.
					rot := rotateImage(base, -float64(f)*frameStep)
					data, err := toPNG(rot)
					if err != nil {
						errCh <- err
						return
					}
					frames[theme][step][f] = data
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

// Run pre-renders frames then starts the systray loop. Blocks until quit.
// Must be called from the main goroutine.
func (t *Tray) Run(ctx context.Context) {
	if err := preRenderFrames(); err != nil {
		log.Printf("tray: prerender: %v", err)
		return
	}
	go func() {
		<-ctx.Done()
		systray.Quit()
	}()
	systray.Run(t.onReady, func() {})
}

func (t *Tray) onReady() {
	systray.SetIcon(frames[0][0][0])
	systray.SetTooltip("Mac Monitor")
	mOpen := systray.AddMenuItem("Open Dashboard", "Open in browser")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Quit", "Quit Mac Monitor")
	go t.animate()
	go func() {
		for {
			select {
			case <-mOpen.ClickedCh:
				exec.Command("open", "http://localhost"+t.addr).Run() //nolint:errcheck
			case <-mQuit.ClickedCh:
				t.cancel()
				systray.Quit()
				return
			}
		}
	}()
}

func (t *Tray) animate() {
	ticker := time.NewTicker(time.Second / animFPS)
	defer ticker.Stop()
	var angle, smoothedVel float64
	last := time.Now()
	lastTheme, lastStep, lastFrame := -1, -1, -1
	const velTau = 3.0 // velocity smoothing time constant in seconds

	for range ticker.C {
		now := time.Now()
		dt := now.Sub(last).Seconds()
		last = now

		cpu := float64(t.cpu.Load())

		// Exponential moving average toward the target velocity so speed
		// changes blend in over ~velTau seconds rather than snapping.
		alpha := 1 - math.Exp(-dt/velTau)
		smoothedVel += alpha * (angularVelocity(cpu) - smoothedVel)

		angle = math.Mod(angle+smoothedVel*dt, 360)

		theme := 0
		if isDarkMode() {
			theme = 1
		}
		step := colorStep(cpu)
		frameIdx := int(angle/frameStep) % numFrames

		// Skip SetIcon when nothing has visually changed to reduce flicker.
		if theme == lastTheme && step == lastStep && frameIdx == lastFrame {
			continue
		}
		systray.SetIcon(frames[theme][step][frameIdx])
		lastTheme, lastStep, lastFrame = theme, step, frameIdx
	}
}
