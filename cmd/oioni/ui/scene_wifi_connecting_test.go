package ui

import (
	"testing"

	"github.com/oioio-space/oioni/ui/gui"
)

func TestConnectingScene_Structure(t *testing.T) {
	nav := gui.NewNavigator(fakeDisplay{})
	s := newConnectingScene(nav, nil, "TestNet")
	if s.Title != "WiFi" {
		t.Errorf("expected title WiFi, got %q", s.Title)
	}
	if s.OnLeave == nil {
		t.Error("OnLeave must be set to cancel the polling goroutine")
	}
}
