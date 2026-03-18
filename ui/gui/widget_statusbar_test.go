package gui

import "testing"

func TestStatusBar_SetLine(t *testing.T) {
	s := NewStatusBar("left", "right")
	s.MarkClean()
	s.SetLine(0, "newleft")
	if !s.IsDirty() {
		t.Error("SetLine(0) should mark dirty")
	}
	s.MarkClean()
	s.SetLine(1, "newright")
	if !s.IsDirty() {
		t.Error("SetLine(1) should mark dirty")
	}
	// Out-of-range: no-op
	s.MarkClean()
	s.SetLine(2, "ignored")
	if s.IsDirty() {
		t.Error("SetLine(2) out-of-range should not mark dirty")
	}
}
