package ui

import (
	"testing"

	"github.com/oioio-space/oioni/ui/gui"
)

func TestWifiScene_Structure(t *testing.T) {
	nav := gui.NewNavigator(fakeDisplay{})
	s := NewWifiScene(nav, nil)
	if len(s.Widgets) < 1 {
		t.Fatal("expected at least 1 widget")
	}
	if s.Title != "WiFi" {
		t.Errorf("expected title WiFi, got %q", s.Title)
	}
}
