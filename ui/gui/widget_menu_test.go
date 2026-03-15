package gui

import (
	"image"
	"testing"

	"github.com/oioio-space/oioni/drivers/touch"
)

func TestMenu_TapSelectsItem(t *testing.T) {
	selected := ""
	items := []MenuItem{
		{Label: "Alpha", OnSelect: func() { selected = "alpha" }},
		{Label: "Beta", OnSelect: func() { selected = "beta" }},
	}
	m := NewMenu(items)
	// Each item is 20px tall
	m.SetBounds(image.Rect(0, 0, 100, 40))
	// Tap on second item (y=25 → row index 1)
	m.HandleTouch(touch.TouchPoint{X: 50, Y: 25})
	if selected != "beta" {
		t.Errorf("expected 'beta', got %q", selected)
	}
}

func TestMenu_ScrollClampsOffset(t *testing.T) {
	items := make([]MenuItem, 10)
	for i := range items {
		items[i] = MenuItem{Label: "item"}
	}
	m := NewMenu(items)
	m.SetBounds(image.Rect(0, 0, 100, 60)) // shows 3 items
	m.Scroll(100)                           // clamp to max
	// max offset = 10 - 3 = 7
	if m.offset > 7 {
		t.Errorf("offset %d exceeds max 7", m.offset)
	}
	m.Scroll(-100)
	if m.offset != 0 {
		t.Errorf("offset %d below 0", m.offset)
	}
}

func TestMenu_DrawDoesNotPanic(t *testing.T) {
	items := []MenuItem{
		{Label: "A"},
		{Label: "B"},
		{Label: "C"},
	}
	m := NewMenu(items)
	m.SetBounds(image.Rect(0, 0, 100, 60))
	c := newTestCanvas()
	m.Draw(c)
}
