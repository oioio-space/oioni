// cmd/oioni/ui/menu_test.go
package ui

import (
	"image"
	"testing"

	"github.com/oioio-space/oioni/drivers/epd"
	"github.com/oioio-space/oioni/drivers/touch"
	"github.com/oioio-space/oioni/ui/canvas"
)

func newTestMenu() *HomeMenuWidget {
	items := []homeMenuItem{
		{name: "Config", desc: "reseau"},
		{name: "System", desc: "services"},
		{name: "Attack", desc: "MITM"},
		{name: "DFIR", desc: "capture"},
		{name: "Info", desc: "aide"},
	}
	return newHomeMenuWidget(items)
}

func TestHomeMenuWidget_PreferredSize(t *testing.T) {
	m := newTestMenu()
	sz := m.PreferredSize()
	if sz.Y != 100 {
		t.Errorf("PreferredSize().Y = %d, want 100", sz.Y)
	}
}

func TestHomeMenuWidget_TapCallsOnTap(t *testing.T) {
	called := ""
	items := []homeMenuItem{
		{name: "A", desc: "a", onTap: func() { called = "A" }},
		{name: "B", desc: "b", onTap: func() { called = "B" }},
		{name: "C", desc: "c"},
		{name: "D", desc: "d"},
		{name: "E", desc: "e"},
	}
	m := newHomeMenuWidget(items)
	m.SetBounds(image.Rect(0, 0, 250, 100))
	// Row 1 = y=20..39 → center y=29
	m.HandleTouch(touch.TouchPoint{X: 100, Y: 29})
	if called != "B" {
		t.Errorf("expected B, got %q", called)
	}
}

func TestHomeMenuWidget_TapNilOnTapIsNoOp(t *testing.T) {
	items := []homeMenuItem{
		{name: "A", desc: "a"}, // onTap is nil
	}
	m := newHomeMenuWidget(items)
	m.SetBounds(image.Rect(0, 0, 250, 100))
	// Should not panic
	m.HandleTouch(touch.TouchPoint{X: 100, Y: 5})
}

func TestHomeMenuWidget_DrawDoesNotPanic(t *testing.T) {
	m := newTestMenu()
	m.SetBounds(image.Rect(0, 0, 250, 100))
	c := canvas.New(epd.Width, epd.Height, canvas.Rot90)
	m.Draw(c)
}
