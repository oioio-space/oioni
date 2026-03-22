package ui

import (
	"testing"

	"github.com/oioio-space/oioni/ui/gui"
)

func TestPasswordScene_Structure(t *testing.T) {
	nav := gui.NewNavigator(fakeDisplay{})
	s := newPasswordScene(nav, nil, "TestNet")
	if s.Title != "WiFi" {
		t.Errorf("expected title WiFi, got %q", s.Title)
	}
	if len(s.Widgets) < 1 {
		t.Fatal("expected widgets")
	}
}
