package gui

import (
	"image"
	"strings"
	"testing"
)

func TestNavBar_PreferredSize(t *testing.T) {
	nb := NewNavBar("Home")
	ps := nb.PreferredSize()
	if ps.Y != 16 {
		t.Errorf("NavBar height = %d, want 16", ps.Y)
	}
}

func TestNavBar_SetPath_MarksDirty(t *testing.T) {
	nb := NewNavBar("Home")
	nb.MarkClean()
	nb.SetPath("Home", "Config")
	if !nb.IsDirty() {
		t.Error("SetPath should mark dirty")
	}
}

func TestNavBar_Path_Updated(t *testing.T) {
	nb := NewNavBar("Home")
	nb.SetPath("Home", "System")
	if len(nb.path) != 2 || nb.path[1] != "System" {
		t.Errorf("path not updated: %v", nb.path)
	}
}

func TestNavBar_Draw_NoPanic(t *testing.T) {
	c := newTestCanvas()
	nb := NewNavBar("Home", "Config")
	nb.SetBounds(image.Rect(0, 0, 206, 16))
	nb.Draw(c)
}

func TestNavBar_Breadcrumb_LongPath_NoPanic(t *testing.T) {
	nb := NewNavBar("Home", strings.Repeat("X", 200))
	nb.SetBounds(image.Rect(0, 0, 206, 16))
	c := newTestCanvas()
	nb.Draw(c) // must not panic on very long path
}
