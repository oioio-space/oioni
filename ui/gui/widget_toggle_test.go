package gui

import (
	"image"
	"testing"

	"github.com/oioio-space/oioni/drivers/touch"
)

func TestToggle_InitialState(t *testing.T) {
	tog := NewToggle(true)
	if !tog.On {
		t.Error("expected initial On=true")
	}
	tog2 := NewToggle(false)
	if tog2.On {
		t.Error("expected initial On=false")
	}
}

func TestToggle_TapFlips(t *testing.T) {
	tog := NewToggle(false)
	tog.SetBounds(image.Rect(0, 0, 40, 20))
	tog.HandleTouch(touch.TouchPoint{X: 20, Y: 10})
	if !tog.On {
		t.Error("expected On=true after tap")
	}
	tog.HandleTouch(touch.TouchPoint{X: 20, Y: 10})
	if tog.On {
		t.Error("expected On=false after second tap")
	}
}

func TestToggle_OnChangeCalled(t *testing.T) {
	called := false
	var got bool
	tog := NewToggle(false)
	tog.OnChange = func(on bool) { called = true; got = on }
	tog.SetBounds(image.Rect(0, 0, 40, 20))
	tog.HandleTouch(touch.TouchPoint{X: 20, Y: 10})
	if !called {
		t.Error("OnChange not called")
	}
	if !got {
		t.Error("OnChange got false, want true")
	}
}

func TestToggle_DrawDoesNotPanic(t *testing.T) {
	c := newTestCanvas()
	tog := NewToggle(false)
	tog.SetBounds(image.Rect(0, 0, 40, 20))
	tog.Draw(c)
	tog.On = true
	tog.Draw(c)
}
