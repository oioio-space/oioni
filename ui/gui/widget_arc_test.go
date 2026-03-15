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
	c := newTestCanvas()
	for _, p := range []float64{0, 1} {
		a := NewProgressArc(p)
		a.SetBounds(image.Rect(0, 0, 60, 60))
		a.Draw(c) // must not panic
	}
}
