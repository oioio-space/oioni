package ui

import (
	"image"
	"testing"

	"github.com/oioio-space/oioni/drivers/touch"
	"github.com/oioio-space/oioni/ui/gui"
)

func TestModeSelector_TapSwitches(t *testing.T) {
	sel := newModeSelector(modeDHCP)
	if sel.Mode() != modeDHCP {
		t.Error("initial mode should be DHCP")
	}
	sel.SetBounds(image.Rect(0, 0, 150, 44))
	sel.HandleTouch(touch.TouchPoint{X: 100, Y: 22})
	if sel.Mode() != modeStatic {
		t.Error("expected Static after tapping right side")
	}
}

func TestIPConfigScene_Structure(t *testing.T) {
	nav := gui.NewNavigator(fakeDisplay{})
	s := newIPConfigScene(nav, nil, "wlan0")
	if s.Title != "Network" {
		t.Errorf("expected title Network, got %q", s.Title)
	}
	if len(s.Widgets) < 2 {
		t.Errorf("expected mode selector in Widgets, got %d", len(s.Widgets))
	}
}
