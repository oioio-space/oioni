package gui

import (
	"image"

	"github.com/oioio-space/oioni/ui/canvas"
)

// Checkbox is a labeled toggle with a visible check box.
type Checkbox struct {
	BaseWidget
	Label    string
	Checked  bool
	OnChange func(bool)
}

func NewCheckbox(label string, initial bool) *Checkbox {
	c := &Checkbox{Label: label, Checked: initial}
	c.SetDirty()
	return c
}

func (cb *Checkbox) PreferredSize() image.Point {
	f := canvas.EmbeddedFont(12)
	tw := 0
	if f != nil {
		for _, r := range cb.Label {
			_, gw, _ := f.Glyph(r)
			tw += gw
		}
	}
	return image.Pt(tw+20+4, 20)
}
func (cb *Checkbox) MinSize() image.Point { return image.Pt(40, 20) }

func (cb *Checkbox) Draw(c *canvas.Canvas) {
	r := cb.Bounds()
	if r.Empty() {
		return
	}
	// Box: 14×14 at left edge, vertically centered
	boxSize := 14
	by := r.Min.Y + (r.Dy()-boxSize)/2
	box := image.Rect(r.Min.X+2, by, r.Min.X+2+boxSize, by+boxSize)
	c.DrawRect(box, canvas.Black, false)
	if cb.Checked {
		// X mark: two diagonals
		c.DrawLine(box.Min.X+2, box.Min.Y+2, box.Max.X-2, box.Max.Y-2, canvas.Black)
		c.DrawLine(box.Max.X-2, box.Min.Y+2, box.Min.X+2, box.Max.Y-2, canvas.Black)
	}
	// Label
	f := canvas.EmbeddedFont(12)
	if f != nil {
		ty := r.Min.Y + (r.Dy()-f.LineHeight())/2
		c.DrawText(r.Min.X+2+boxSize+4, ty, cb.Label, f, canvas.Black)
	}
	cb.MarkClean()
}

func (cb *Checkbox) HandleTouch(_ TouchPoint) bool {
	cb.Checked = !cb.Checked
	cb.SetDirty()
	if cb.OnChange != nil {
		cb.OnChange(cb.Checked)
	}
	return true
}
