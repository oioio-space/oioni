package gui

import (
	"image"
	"testing"
)

func TestCheckbox_TapToggles(t *testing.T) {
	cb := NewCheckbox("enable", false)
	cb.SetBounds(image.Rect(0, 0, 100, 20))
	cb.HandleTouch(TouchPoint{X: 10, Y: 10})
	if !cb.Checked {
		t.Error("expected Checked=true after tap")
	}
	cb.HandleTouch(TouchPoint{X: 10, Y: 10})
	if cb.Checked {
		t.Error("expected Checked=false after second tap")
	}
}

func TestCheckbox_OnChangeCalled(t *testing.T) {
	var got bool
	cb := NewCheckbox("x", false)
	cb.OnChange = func(v bool) { got = v }
	cb.SetBounds(image.Rect(0, 0, 60, 20))
	cb.HandleTouch(TouchPoint{X: 10, Y: 10})
	if !got {
		t.Error("OnChange not called with true")
	}
}

func TestCheckbox_DrawDoesNotPanic(t *testing.T) {
	c := newTestCanvas()
	for _, checked := range []bool{false, true} {
		cb := NewCheckbox("label", checked)
		cb.SetBounds(image.Rect(0, 0, 100, 20))
		cb.Draw(c)
	}
}
