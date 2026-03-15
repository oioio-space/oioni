package gui

import (
	"fmt"
	"image"
	"math"

	"github.com/oioio-space/oioni/ui/canvas"
)

// ProgressArc displays a circular arc showing a progress value (0.0–1.0).
type ProgressArc struct {
	BaseWidget
	progress float64
}

func NewProgressArc(progress float64) *ProgressArc {
	a := &ProgressArc{}
	a.SetProgress(progress)
	return a
}

func (a *ProgressArc) SetProgress(v float64) {
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}
	a.progress = v
	a.SetDirty()
}

func (a *ProgressArc) PreferredSize() image.Point { return image.Pt(60, 60) }
func (a *ProgressArc) MinSize() image.Point       { return image.Pt(40, 40) }

func (a *ProgressArc) Draw(c *canvas.Canvas) {
	r := a.Bounds()
	if r.Empty() {
		return
	}
	cx := r.Min.X + r.Dx()/2
	cy := r.Min.Y + r.Dy()/2
	radius := min(r.Dx(), r.Dy())/2 - 2

	// White background
	c.DrawRect(r, canvas.White, true)
	// Background circle outline
	c.DrawCircle(cx, cy, radius, canvas.Black, false)

	// Filled sector: iterate pixels within bounding square, test angle.
	// Angle 0 = top (−π/2), increases clockwise.
	end := a.progress * 2 * math.Pi
	for py := r.Min.Y; py < r.Max.Y; py++ {
		for px := r.Min.X; px < r.Max.X; px++ {
			dx := float64(px - cx)
			dy := float64(py - cy)
			dist := math.Sqrt(dx*dx + dy*dy)
			if dist > float64(radius) {
				continue
			}
			// atan2: 0=right, CCW positive. Shift so 0=top, CW positive.
			angle := math.Atan2(dy, dx) + math.Pi/2
			if angle < 0 {
				angle += 2 * math.Pi
			}
			if angle <= end {
				c.SetPixel(px, py, canvas.Black)
			}
		}
	}

	// Center label
	label := fmt.Sprintf("%d%%", int(a.progress*100))
	f := canvas.EmbeddedFont(12)
	if f == nil {
		a.MarkClean()
		return
	}
	tw := 0
	for _, ch := range label {
		_, gw, _ := f.Glyph(ch)
		tw += gw
	}
	lx := cx - tw/2
	ly := cy - f.LineHeight()/2
	c.DrawText(lx, ly, label, f, canvas.Black)

	a.MarkClean()
}
