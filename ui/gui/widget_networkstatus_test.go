// ui/gui/widget_networkstatus_test.go
package gui

import (
	"image"
	"testing"

)

func TestNetworkStatusBar_SetInterfacesMarksDirty(t *testing.T) {
	nsb := NewNetworkStatusBar(nil)
	nsb.MarkClean()
	nsb.SetInterfaces([]IfaceInfo{{Name: "eth0", IP: "1.2.3.4", Up: true}})
	if !nsb.IsDirty() {
		t.Error("SetInterfaces should mark dirty")
	}
}

func TestNetworkStatusBar_SetToolsMarksDirty(t *testing.T) {
	nsb := NewNetworkStatusBar(nil)
	nsb.MarkClean()
	nsb.SetTools([]ToolStatus{{Label: "MITM", Progress: 0.5}})
	if !nsb.IsDirty() {
		t.Error("SetTools should mark dirty")
	}
}

func TestNetworkStatusBar_PreferredSize(t *testing.T) {
	nsb := NewNetworkStatusBar(nil)
	sz := nsb.PreferredSize()
	if sz.Y != 22 {
		t.Errorf("PreferredSize().Y = %d, want 22", sz.Y)
	}
}

func TestNetworkStatusBar_DrawDoesNotPanic(t *testing.T) {
	nsb := NewNetworkStatusBar(nil)
	nsb.SetBounds(image.Rect(0, 0, 250, 22))
	nsb.SetInterfaces([]IfaceInfo{
		{Name: "eth0", IP: "192.168.0.33", Up: true},
		{Name: "usb0", IP: "192.168.42.1", Up: true},
	})
	nsb.SetTools([]ToolStatus{
		{Label: "MITM", Progress: 1.0},
		{Label: "SCAN", Progress: 0.6},
	})
	c := newTestCanvas()
	nsb.Draw(c) // must not panic
}

func TestNetworkStatusBar_DrawOfflineDoesNotPanic(t *testing.T) {
	nsb := NewNetworkStatusBar(nil)
	nsb.SetBounds(image.Rect(0, 0, 250, 22))
	// No interfaces set → OFFLINE state
	c := newTestCanvas()
	nsb.Draw(c)
}

func TestNetworkStatusBar_DrawTrayOverflowDoesNotPanic(t *testing.T) {
	nsb := NewNetworkStatusBar(nil)
	nsb.SetBounds(image.Rect(0, 0, 250, 22))
	nsb.SetTools([]ToolStatus{
		{Label: "MITM", Progress: 1.0},
		{Label: "SCAN", Progress: 0.5},
		{Label: "HID", Progress: 0.3}, // 3rd tool → triggers +N badge
	})
	c := newTestCanvas()
	nsb.Draw(c)
}

func TestNetworkStatusBar_BadgeTouchDispatchesWhenNavNil(t *testing.T) {
	// nav=nil means badge tap is a no-op (no panic)
	nsb := NewNetworkStatusBar(nil)
	nsb.SetBounds(image.Rect(0, 0, 250, 22))
	nsb.SetInterfaces([]IfaceInfo{
		{Name: "eth0", IP: "1.2.3.4", Up: true},
		{Name: "usb0", IP: "192.168.42.1", Up: true}, // both Up → badge is drawn
	})
	// Draw first to populate badgeBounds
	c := newTestCanvas()
	nsb.Draw(c)
	if nsb.badgeBounds.Empty() {
		t.Fatal("badgeBounds should be non-empty after Draw with 2 up interfaces")
	}
	// Tap somewhere in the header — nav=nil so dispatch is skipped, must not panic
	nsb.HandleTouch(TouchPoint{X: 80, Y: 5})
}
