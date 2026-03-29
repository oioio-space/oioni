package gui

import (
	"image"
	"testing"

)

func TestSlider_SetValueClamps(t *testing.T) {
	s := NewSlider(0, 100, 1)
	s.SetValue(150)
	if s.Value() != 100 {
		t.Errorf("expected 100, got %f", s.Value())
	}
	s.SetValue(-10)
	if s.Value() != 0 {
		t.Errorf("expected 0, got %f", s.Value())
	}
}

func TestSlider_SetValueSnapsToStep(t *testing.T) {
	s := NewSlider(0, 10, 2.5)
	s.SetValue(3.0)
	// nearest step: 2.5
	if s.Value() != 2.5 {
		t.Errorf("expected 2.5, got %f", s.Value())
	}
}

func TestSlider_TapSetsValue(t *testing.T) {
	s := NewSlider(0, 100, 1)
	s.SetBounds(image.Rect(0, 0, 100, 24))
	// Tap at x=50 out of width 100 → value = 50
	s.HandleTouch(TouchPoint{X: 50, Y: 12})
	if s.Value() != 50 {
		t.Errorf("expected 50, got %f", s.Value())
	}
}

func TestSlider_OnChangeCalled(t *testing.T) {
	var got float64
	s := NewSlider(0, 100, 1)
	s.SetBounds(image.Rect(0, 0, 100, 24))
	s.OnChange = func(v float64) { got = v }
	s.HandleTouch(TouchPoint{X: 75, Y: 12})
	if got != 75 {
		t.Errorf("OnChange got %f, want 75", got)
	}
}

func TestSlider_DrawDoesNotPanic(t *testing.T) {
	s := NewSlider(0, 100, 1)
	s.SetValue(42)
	s.SetBounds(image.Rect(0, 0, 120, 24))
	c := newTestCanvas()
	s.Draw(c)
}
