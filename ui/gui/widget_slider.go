package gui

import (
	"fmt"
	"image"
	"math"

	"github.com/oioio-space/oioni/drivers/touch"
	"github.com/oioio-space/oioni/ui/canvas"
)

// Slider is a horizontal value picker. Tap anywhere on the bar to set the value.
type Slider struct {
	BaseWidget
	Min, Max, Step float64
	OnChange       func(float64)
	value          float64
}

func NewSlider(min, max, step float64) *Slider {
	s := &Slider{Min: min, Max: max, Step: step, value: min}
	s.SetDirty()
	return s
}

func (s *Slider) Value() float64 { return s.value }

func (s *Slider) SetValue(v float64) {
	// snap to step
	if s.Step > 0 {
		v = math.Round(v/s.Step) * s.Step
	}
	if v < s.Min {
		v = s.Min
	}
	if v > s.Max {
		v = s.Max
	}
	s.value = v
	s.SetDirty()
}

func (s *Slider) PreferredSize() image.Point { return image.Pt(120, 24) }
func (s *Slider) MinSize() image.Point       { return image.Pt(80, 24) }

func (s *Slider) Draw(c *canvas.Canvas) {
	r := s.Bounds()
	if r.Empty() {
		return
	}
	c.DrawRect(r, canvas.White, true)

	// Track line at vertical center
	midY := r.Min.Y + r.Dy()/2
	c.DrawLine(r.Min.X+4, midY, r.Max.X-4, midY, canvas.Black)

	// Thumb
	ratio := 0.0
	if s.Max > s.Min {
		ratio = (s.value - s.Min) / (s.Max - s.Min)
	}
	thumbX := r.Min.X + 4 + int(ratio*float64(r.Dx()-8))
	thumbW := 6
	thumb := image.Rect(thumbX-thumbW/2, r.Min.Y+2, thumbX+thumbW/2, r.Max.Y-2)
	c.DrawRect(thumb, canvas.Black, true)

	// Value label above thumb
	f := canvas.EmbeddedFont(8)
	if f != nil {
		label := fmt.Sprintf("%.0f", s.value)
		tw := 0
		for _, ch := range label {
			_, gw, _ := f.Glyph(ch)
			tw += gw
		}
		lx := thumbX - tw/2
		if lx < r.Min.X {
			lx = r.Min.X
		}
		if lx+tw > r.Max.X {
			lx = r.Max.X - tw
		}
		c.DrawText(lx, r.Min.Y, label, f, canvas.Black)
	}
	s.MarkClean()
}

func (s *Slider) HandleTouch(pt touch.TouchPoint) bool {
	r := s.Bounds()
	if r.Empty() || r.Dx() == 0 {
		return false
	}
	ratio := float64(int(pt.X)-r.Min.X) / float64(r.Dx())
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}
	v := s.Min + ratio*(s.Max-s.Min)
	s.SetValue(v)
	if s.OnChange != nil {
		s.OnChange(s.value)
	}
	return true
}
