// cmd/oioni/ui/menu_test.go
package ui

import (
	"image"
	"testing"

	"github.com/oioio-space/oioni/drivers/epd"
	"github.com/oioio-space/oioni/ui/canvas"
)

func TestHomeListItem_OnTap(t *testing.T) {
	called := false
	item := &homeListItem{name: "Config", onTap: func() { called = true }}
	item.OnTap()
	if !called {
		t.Error("onTap not called")
	}
}

func TestHomeListItem_OnTapNilIsNoOp(t *testing.T) {
	item := &homeListItem{name: "Config"} // onTap nil
	item.OnTap()                          // must not panic
}

func TestHomeListItem_DrawDoesNotPanic(t *testing.T) {
	item := &homeListItem{name: "Config"}
	r := image.Rect(0, 22, 200, 47) // one homeRowH=25px slot
	c := canvas.New(epd.Width, epd.Height, canvas.Rot90)
	item.Draw(c, r)
}

func TestHomeListItem_DrawWithIcon(t *testing.T) {
	item := &homeListItem{name: "Attack", icon: Icons.Attack}
	r := image.Rect(0, 22, 200, 47)
	c := canvas.New(epd.Width, epd.Height, canvas.Rot90)
	item.Draw(c, r) // must not panic even with real icon
}
