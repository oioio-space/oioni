// ui/gui/widget_navbutton_test.go
package gui

import (
	"image"
	"testing"

	"github.com/oioio-space/oioni/ui/canvas"
)

func setNavBtnBounds(b *NavButton) {
	b.SetBounds(image.Rect(200, 22, 250, 72)) // 50×50px
}

func TestNavButton_TapCallsOnTap(t *testing.T) {
	called := false
	b := NewNavButton("^", func() { called = true }, func() bool { return true })
	setNavBtnBounds(b)
	b.HandleTouch(TouchPoint{X: 225, Y: 47})
	if !called {
		t.Error("onTap not called")
	}
}

func TestNavButton_TapWhenDisabledStillCallsOnTap(t *testing.T) {
	// NavButton fires onTap regardless of active state — onTap decides the no-op logic.
	called := false
	b := NewNavButton("^", func() { called = true }, func() bool { return false })
	setNavBtnBounds(b)
	b.HandleTouch(TouchPoint{X: 225, Y: 47})
	if !called {
		t.Error("onTap not called when disabled")
	}
}

func TestNavButton_NilOnTap_Noop(t *testing.T) {
	b := NewNavButton("^", nil, func() bool { return true })
	setNavBtnBounds(b)
	b.HandleTouch(TouchPoint{X: 225, Y: 47}) // must not panic
}

func TestNavButton_NilIsActive_DefaultFalse(t *testing.T) {
	b := NewNavButton("^", func() {}, nil) // nil isActive → default to false
	b.SetBounds(image.Rect(0, 0, 50, 50))
	c := canvas.New(ScreenWidth, ScreenHeight, canvas.Rot90)
	b.Draw(c) // must not panic (disabled path)
}

func TestNavButton_DrawActiveDoesNotPanic(t *testing.T) {
	b := NewNavButton("^", func() {}, func() bool { return true })
	setNavBtnBounds(b)
	c := canvas.New(ScreenWidth, ScreenHeight, canvas.Rot90)
	b.Draw(c)
}

func TestNavButton_DrawDisabledDoesNotPanic(t *testing.T) {
	b := NewNavButton("v", func() {}, func() bool { return false })
	setNavBtnBounds(b)
	c := canvas.New(ScreenWidth, ScreenHeight, canvas.Rot90)
	b.Draw(c)
}

func TestNavButton_DrawEmptyBounds(t *testing.T) {
	b := NewNavButton("^", func() {}, func() bool { return true })
	// bounds zero → early return, no panic
	c := canvas.New(ScreenWidth, ScreenHeight, canvas.Rot90)
	b.Draw(c)
}
