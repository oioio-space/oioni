package gui

import (
	"image"
	"testing"

	"awesomeProject/epaper/canvas"
	"awesomeProject/epaper/touch"
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

// ── layout tests ──────────────────────────────────────────────────────────────

// fixedWidget is a test widget with fixed preferred and min sizes.
type fixedWidget struct {
	BaseWidget
	pref image.Point
	min  image.Point
	drew bool
}

func newFixedWidget(pw, ph, mw, mh int) *fixedWidget {
	w := &fixedWidget{pref: image.Pt(pw, ph), min: image.Pt(mw, mh)}
	w.SetDirty()
	return w
}
func (w *fixedWidget) PreferredSize() image.Point { return w.pref }
func (w *fixedWidget) MinSize() image.Point       { return w.min }
func (w *fixedWidget) Draw(c *canvas.Canvas)      { w.drew = true }

// touchWidget is a fixedWidget that also implements Touchable.
type touchWidget struct {
	fixedWidget
	touched bool
}

func newTouchWidget(pw, ph int) *touchWidget {
	tw := &touchWidget{}
	tw.pref = image.Pt(pw, ph)
	tw.min = image.Pt(pw, ph)
	tw.SetDirty()
	return tw
}
func (tw *touchWidget) HandleTouch(pt touch.TouchPoint) bool { tw.touched = true; return true }

func TestVBoxAllocatesChildren(t *testing.T) {
	a := newFixedWidget(100, 20, 0, 10)
	b := newFixedWidget(100, 30, 0, 10)
	box := NewVBox(a, b)
	box.SetBounds(image.Rect(0, 0, 100, 100))

	if a.Bounds().Dy() != 20 {
		t.Errorf("child a height = %d, want 20", a.Bounds().Dy())
	}
	if b.Bounds().Dy() != 30 {
		t.Errorf("child b height = %d, want 30", b.Bounds().Dy())
	}
	if a.Bounds().Min.Y != 0 {
		t.Errorf("child a y = %d, want 0", a.Bounds().Min.Y)
	}
	if b.Bounds().Min.Y != 20 {
		t.Errorf("child b y = %d, want 20", b.Bounds().Min.Y)
	}
}

func TestVBoxExpandTakesRemainingHeight(t *testing.T) {
	fixed := newFixedWidget(100, 20, 0, 10)
	expanded := newFixedWidget(100, 10, 0, 5)
	box := NewVBox(fixed, Expand(expanded))
	box.SetBounds(image.Rect(0, 0, 100, 100))

	if fixed.Bounds().Dy() != 20 {
		t.Errorf("fixed child height = %d, want 20", fixed.Bounds().Dy())
	}
	if expanded.Bounds().Dy() != 80 {
		t.Errorf("expand child height = %d, want 80", expanded.Bounds().Dy())
	}
}

func TestVBoxEnforces20pxForTouchable(t *testing.T) {
	small := newTouchWidget(100, 5)
	box := NewVBox(small)
	box.SetBounds(image.Rect(0, 0, 100, 100))
	if small.Bounds().Dy() < 20 {
		t.Errorf("Touchable child height = %d, want >= 20", small.Bounds().Dy())
	}
}

func TestVBoxIsDirtyIfChildDirty(t *testing.T) {
	a := newFixedWidget(100, 20, 0, 10)
	box := NewVBox(a)
	box.SetBounds(image.Rect(0, 0, 100, 50))
	box.MarkClean()
	a.SetDirty()
	if !box.IsDirty() {
		t.Error("VBox should be dirty when child is dirty")
	}
}

func TestVBoxMarkCleanClearsChildren(t *testing.T) {
	a := newFixedWidget(100, 20, 0, 10)
	box := NewVBox(a)
	box.SetBounds(image.Rect(0, 0, 100, 50))
	box.MarkClean()
	if a.IsDirty() {
		t.Error("MarkClean should clear children")
	}
}

func TestHBoxAllocatesChildren(t *testing.T) {
	a := newFixedWidget(40, 20, 0, 0)
	b := newFixedWidget(60, 20, 0, 0)
	box := NewHBox(a, b)
	box.SetBounds(image.Rect(0, 0, 200, 20))

	if a.Bounds().Dx() != 40 {
		t.Errorf("child a width = %d, want 40", a.Bounds().Dx())
	}
	if b.Bounds().Dx() != 60 {
		t.Errorf("child b width = %d, want 60", b.Bounds().Dx())
	}
	if a.Bounds().Min.X != 0 {
		t.Errorf("child a x = %d, want 0", a.Bounds().Min.X)
	}
	if b.Bounds().Min.X != 40 {
		t.Errorf("child b x = %d, want 40", b.Bounds().Min.X)
	}
}

func TestHBoxExpandTakesRemainingWidth(t *testing.T) {
	fixed := newFixedWidget(40, 20, 0, 0)
	expanded := newFixedWidget(10, 20, 0, 0)
	box := NewHBox(fixed, Expand(expanded))
	box.SetBounds(image.Rect(0, 0, 200, 20))

	if fixed.Bounds().Dx() != 40 {
		t.Errorf("fixed width = %d, want 40", fixed.Bounds().Dx())
	}
	if expanded.Bounds().Dx() != 160 {
		t.Errorf("expand width = %d, want 160", expanded.Bounds().Dx())
	}
}

func TestFixedPutsWidgetAtAbsolutePosition(t *testing.T) {
	w := newFixedWidget(30, 15, 0, 0)
	f := NewFixed(200, 100)
	f.Put(w, 10, 5)
	f.SetBounds(image.Rect(0, 0, 200, 100))

	if w.Bounds().Min.X != 10 {
		t.Errorf("widget x = %d, want 10", w.Bounds().Min.X)
	}
	if w.Bounds().Min.Y != 5 {
		t.Errorf("widget y = %d, want 5", w.Bounds().Min.Y)
	}
}

func TestOverlayCentersContent(t *testing.T) {
	content := newFixedWidget(60, 30, 60, 30)
	o := NewOverlay(content, AlignCenter)
	o.setScreen(250, 122)

	wantX := (250 - 60) / 2
	wantY := (122 - 30) / 2
	if content.Bounds().Min.X != wantX {
		t.Errorf("overlay x = %d, want %d", content.Bounds().Min.X, wantX)
	}
	if content.Bounds().Min.Y != wantY {
		t.Errorf("overlay y = %d, want %d", content.Bounds().Min.Y, wantY)
	}
}

func TestWithPaddingAddsPadding(t *testing.T) {
	inner := newFixedWidget(40, 20, 40, 20)
	padded := WithPadding(4, inner)
	padded.SetBounds(image.Rect(0, 0, 100, 50))

	if inner.Bounds().Min.X != 4 {
		t.Errorf("inner x = %d, want 4", inner.Bounds().Min.X)
	}
	if inner.Bounds().Min.Y != 4 {
		t.Errorf("inner y = %d, want 4", inner.Bounds().Min.Y)
	}
	if inner.Bounds().Max.X != 96 {
		t.Errorf("inner max x = %d, want 96", inner.Bounds().Max.X)
	}
}
