package ui

import (
	"testing"

	"github.com/oioio-space/oioni/ui/gui"
)

func TestNetworkScene_Structure(t *testing.T) {
	nav := gui.NewNavigator(fakeDisplay{})
	s := NewNetworkScene(nav, nil)
	if s.Title != "Network" {
		t.Errorf("expected title Network, got %q", s.Title)
	}
}
