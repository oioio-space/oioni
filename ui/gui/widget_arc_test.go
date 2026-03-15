package gui

import (
	"image"
	"testing"
)

func TestProgressArc_Clamps(t *testing.T) {
	a := NewProgressArc(0.5)
	a.SetProgress(1.5)
	if a.progress != 1.0 {
		t.Errorf("expected 1.0, got %f", a.progress)
	}
	a.SetProgress(-0.1)
	if a.progress != 0.0 {
		t.Errorf("expected 0.0, got %f", a.progress)
	}
}

func TestProgressArc_DrawDoesNotPanic(t *testing.T) {
	a := NewProgressArc(0.75)
	a.SetBounds(image.Rect(0, 0, 60, 60))
	c := newTestCanvas()
	a.Draw(c)
}

func TestProgressArc_ZeroAndOne(t *testing.T) {
	for _, p := range []float64{0, 1} {
		c := newTestCanvas()
		a := NewProgressArc(p)
		a.SetBounds(image.Rect(0, 0, 60, 60))
		a.Draw(c)
		// Circle outline is always drawn — at least one black pixel must exist
		found := false
		for _, b := range c.Bytes() {
			if b != 0xFF {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("progress=%v: expected at least one black pixel", p)
		}
	}
}
