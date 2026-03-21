// ui/gui/widget_ifacepopup_test.go
package gui

import (
	"image"
	"testing"
)

func TestInterfaceDetailPopup_DrawDoesNotPanic(t *testing.T) {
	popup := newInterfaceDetailPopup(nil, []IfaceInfo{
		{Name: "eth0", IP: "192.168.0.33", Up: true},
		{Name: "usb0", IP: "192.168.42.1", Up: true},
		{Name: "wlan0", IP: "", Up: false},
	})
	popup.SetBounds(image.Rect(0, 0, 250, 122))
	c := newTestCanvas()
	popup.Draw(c)
}

func TestInterfaceDetailPopup_DrawEmptyDoesNotPanic(t *testing.T) {
	popup := newInterfaceDetailPopup(nil, nil)
	popup.SetBounds(image.Rect(0, 0, 250, 122))
	c := newTestCanvas()
	popup.Draw(c)
}

func TestInterfaceDetailPopup_ScrollDownDismisses(t *testing.T) {
	d := &fakeDisplay{}
	nav := NewNavigator(d)
	nav.Push(&Scene{Widgets: []Widget{NewLabel("base")}}) //nolint

	popup := newInterfaceDetailPopup(nav, nil)
	popup.SetBounds(image.Rect(0, 0, 250, 122))

	// Push popup scene so nav has something to pop
	nav.Push(&Scene{Widgets: []Widget{popup}}) //nolint

	depthBefore := nav.Depth()
	popup.Scroll(1) // swipe down — dispatches Pop
	// Drain the dispatch channel so it executes
	nav.drainDispatch()

	if nav.Depth() != depthBefore-1 {
		t.Errorf("depth = %d, want %d", nav.Depth(), depthBefore-1)
	}
}

func TestInterfaceDetailPopup_ScrollUpNoOp(t *testing.T) {
	d := &fakeDisplay{}
	nav := NewNavigator(d)
	nav.Push(&Scene{Widgets: []Widget{NewLabel("base")}}) //nolint
	popup := newInterfaceDetailPopup(nav, nil)
	popup.SetBounds(image.Rect(0, 0, 250, 122))
	nav.Push(&Scene{Widgets: []Widget{popup}}) //nolint

	depthBefore := nav.Depth()
	popup.Scroll(-1) // swipe up — no-op
	nav.drainDispatch()

	if nav.Depth() != depthBefore {
		t.Errorf("scroll up should not pop; depth = %d, want %d", nav.Depth(), depthBefore)
	}
}
