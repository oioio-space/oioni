package gui

import (
	"testing"

	"github.com/oioio-space/oioni/drivers/touch"
)

func TestShowAlert_PushesScene(t *testing.T) {
	d := &fakeDisplay{}
	nav := NewNavigator(d)
	nav.Push(&Scene{Widgets: []Widget{NewLabel("base")}}) //nolint

	ShowAlert(nav, "Title", "Message")
	if len(nav.stack) != 2 {
		t.Fatalf("expected 2 scenes after ShowAlert, got %d", len(nav.stack))
	}
}

func TestShowAlert_OKButtonPops(t *testing.T) {
	d := &fakeDisplay{}
	nav := NewNavigator(d)
	nav.Push(&Scene{Widgets: []Widget{NewLabel("base")}}) //nolint

	var pressed bool
	ShowAlert(nav, "T", "M", AlertButton{
		Label:   "OK",
		OnPress: func() { pressed = true },
	})
	// Find the button and tap it
	scene := nav.stack[len(nav.stack)-1]
	tapAll(scene.Widgets)

	if !pressed {
		t.Error("OnPress not called")
	}
}

func TestShowMenu_PushesScene(t *testing.T) {
	d := &fakeDisplay{}
	nav := NewNavigator(d)
	nav.Push(&Scene{Widgets: []Widget{NewLabel("base")}}) //nolint
	items := []MenuItem{{Label: "A"}, {Label: "B"}}
	ShowMenu(nav, "My Menu", items)
	if len(nav.stack) != 2 {
		t.Fatalf("expected 2 scenes after ShowMenu, got %d", len(nav.stack))
	}
}

func TestShowTextInput_PushesScene(t *testing.T) {
	d := &fakeDisplay{}
	nav := NewNavigator(d)
	nav.Push(&Scene{Widgets: []Widget{NewLabel("base")}}) //nolint
	ShowTextInput(nav, "enter text", 20, func(s string) {})
	if len(nav.stack) != 2 {
		t.Fatalf("expected 2 scenes after ShowTextInput, got %d", len(nav.stack))
	}
}

// tapAll recursively taps all Touchable widgets at their center.
func tapAll(widgets []Widget) {
	type hasChildren interface{ Children() []Widget }
	for _, w := range widgets {
		if t, ok := w.(Touchable); ok {
			r := w.Bounds()
			if !r.Empty() {
				pt := touch.TouchPoint{
					X: uint16(r.Min.X + r.Dx()/2),
					Y: uint16(r.Min.Y + r.Dy()/2),
				}
				t.HandleTouch(pt)
			}
		}
		if c, ok := w.(hasChildren); ok {
			tapAll(c.Children())
		}
	}
}
