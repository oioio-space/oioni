package gui

import (
	"image"

	"github.com/oioio-space/oioni/ui/canvas"
)

// Toggle is an on/off switch widget.
type Toggle struct {
	BaseWidget
	On       bool
	OnChange func(bool)
}

func NewToggle(initial bool) *Toggle {
	t := &Toggle{On: initial}
	t.SetDirty()
	return t
}

func (t *Toggle) PreferredSize() image.Point { return image.Pt(40, 20) }
func (t *Toggle) MinSize() image.Point       { return image.Pt(40, 20) }

func (t *Toggle) Draw(c *canvas.Canvas) {
	r := t.Bounds()
	if r.Empty() {
		return
	}
	// Pill outline
	c.DrawRect(r, canvas.Black, false)
	// Knob: left half = OFF position, right half = ON position
	mid := r.Min.X + r.Dx()/2
	knob := image.Rect(r.Min.X+1, r.Min.Y+1, mid-1, r.Max.Y-1)
	if t.On {
		knob = image.Rect(mid+1, r.Min.Y+1, r.Max.X-1, r.Max.Y-1)
	}
	c.DrawRect(knob, canvas.Black, true)
}

func (t *Toggle) HandleTouch(_ TouchPoint) bool {
	t.On = !t.On
	t.SetDirty()
	if t.OnChange != nil {
		t.OnChange(t.On)
	}
	return true
}
