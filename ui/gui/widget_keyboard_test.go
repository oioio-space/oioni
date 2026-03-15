package gui

import (
	"image"
	"testing"

	"github.com/oioio-space/oioni/drivers/touch"
)

func TestKeyboard_TapFiresOnKey(t *testing.T) {
	var got rune
	kb := newKeyboard(keyboardConfig{
		Rows:   []string{"ABC", "DEF"},
		MaxLen: 10,
		OnKey:  func(r rune) { got = r },
	})
	// 3 cols, 2 rows, give it 90×40 px
	kb.SetBounds(image.Rect(0, 0, 90, 40))
	// Tap at x=15, y=10 → col 0, row 0 → 'A'
	kb.HandleTouch(touch.TouchPoint{X: 15, Y: 10})
	if got != 'A' {
		t.Errorf("expected 'A', got %q", got)
	}
}

func TestKeyboard_OnBackCalled(t *testing.T) {
	called := false
	kb := newKeyboard(keyboardConfig{
		Rows:   []string{"AB"},
		MaxLen: 10,
		OnBack: func() { called = true },
	})
	kb.SetBounds(image.Rect(0, 0, 250, 20))
	kb.Back()
	if !called {
		t.Error("OnBack not called")
	}
}

func TestKeyboard_DrawDoesNotPanic(t *testing.T) {
	kb := newKeyboard(defaultKeyboardConfig(10, nil, nil, nil, nil))
	kb.SetBounds(image.Rect(0, 0, 250, 100))
	c := newTestCanvas()
	kb.Draw(c)
}
