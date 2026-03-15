package gui

import (
	"image"
	"testing"
)

func TestBaseWidgetInitiallyClean(t *testing.T) {
	var b BaseWidget
	if b.IsDirty() {
		t.Error("new BaseWidget should not be dirty")
	}
}

func TestBaseWidgetSetDirty(t *testing.T) {
	var b BaseWidget
	b.SetDirty()
	if !b.IsDirty() {
		t.Error("expected dirty after SetDirty()")
	}
}

func TestBaseWidgetMarkClean(t *testing.T) {
	var b BaseWidget
	b.SetDirty()
	b.MarkClean()
	if b.IsDirty() {
		t.Error("expected clean after MarkClean()")
	}
}

func TestBaseWidgetSetBoundsMarksDirty(t *testing.T) {
	var b BaseWidget
	r := image.Rect(10, 20, 50, 40)
	b.SetBounds(r)
	if b.Bounds() != r {
		t.Errorf("Bounds = %v, want %v", b.Bounds(), r)
	}
	if !b.IsDirty() {
		t.Error("SetBounds should mark dirty")
	}
}

func TestBaseWidgetPreferredAndMinSizeZero(t *testing.T) {
	var b BaseWidget
	if b.PreferredSize() != (image.Point{}) {
		t.Errorf("PreferredSize should be zero, got %v", b.PreferredSize())
	}
	if b.MinSize() != (image.Point{}) {
		t.Errorf("MinSize should be zero, got %v", b.MinSize())
	}
}
