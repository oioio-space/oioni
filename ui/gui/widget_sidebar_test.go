package gui

import (
	"image"
	"testing"

)

func TestActionSidebar_PreferredSize(t *testing.T) {
	s := NewActionSidebar()
	ps := s.PreferredSize()
	if ps.X != 44 {
		t.Errorf("sidebar width = %d, want 44", ps.X)
	}
}

func TestActionSidebar_SetButtons_MarksDirty(t *testing.T) {
	s := NewActionSidebar()
	s.MarkClean()
	s.SetButtons(SidebarButton{OnTap: func() {}})
	if !s.IsDirty() {
		t.Error("SetButtons should mark dirty")
	}
	if len(s.buttons) != 1 {
		t.Errorf("expected 1 button, got %d", len(s.buttons))
	}
}

func TestActionSidebar_HandleTouch_TapsCorrectButton(t *testing.T) {
	tapped := -1
	s := NewActionSidebar(
		SidebarButton{OnTap: func() { tapped = 0 }},
		SidebarButton{OnTap: func() { tapped = 1 }},
	)
	s.SetBounds(image.Rect(0, 0, 44, 122))

	// With 2 buttons, each is 61px tall. First button: y in [0, 61), second: [61, 122).
	// Touch at y=30 → first button
	s.HandleTouch(TouchPoint{X: 22, Y: 30})
	if tapped != 0 {
		t.Errorf("expected button 0 tapped, got %d", tapped)
	}

	// Touch at y=90 → second button
	tapped = -1
	s.HandleTouch(TouchPoint{X: 22, Y: 90})
	if tapped != 1 {
		t.Errorf("expected button 1 tapped, got %d", tapped)
	}
}

func TestActionSidebar_Draw_NoPanic(t *testing.T) {
	s := NewActionSidebar(
		SidebarButton{},
		SidebarButton{},
	)
	s.SetBounds(image.Rect(0, 0, 44, 122))
	cv := newTestCanvas()
	s.Draw(cv) // must not panic
}
