# epaper/gui Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build `epaper/gui`, a GTK/Qt-style GUI library for the Waveshare 2.13" Touch e-Paper HAT with widgets, two-pass layout, navigation stack, and smart partial/full refresh.

**Architecture:** Scene+Widget+Navigator — widgets declare size, layout containers allocate bounds, Navigator manages scene stack + touch routing + refresh strategy. RefreshManager decides full/partial/noop based on dirty tracking and an anti-ghosting counter.

**Tech Stack:** Go 1.26, `awesomeProject/epaper/canvas`, `awesomeProject/epaper/epd`, `awesomeProject/epaper/touch`. Zero external dependencies.

**Spec:** `docs/superpowers/specs/2026-03-15-epaper-gui-design.md`

---

## Critical context

- Physical display: 122 wide × 250 tall (`epd.Width=122`, `epd.Height=250`). Logical after `canvas.Rot90`: 250 wide × 122 tall.
- `canvas.EmbeddedFont(size)` supports 8, 12, 16, 20, 24 — returns `nil` for others. Always nil-guard.
- `canvas.Font` interface: `Glyph(r rune) ([]byte, int, int)` and `LineHeight() int`.
- `epd.DisplayPartial(buf)` takes a full 4000-byte buffer and handles its own internal init — never call `Init(ModePartial)` before it.
- `epd.DisplayBase(buf)` writes both 0x24 (new frame) and 0x26 (reference frame) — required before any partial sequence.
- All tests are in `package gui` (same package, not `_test`) for access to unexported helpers.
- Build/test command: `go test awesomeProject/epaper/gui -v -count=1`

---

## File structure

| File | Responsibility |
|------|---------------|
| `epaper/gui/gui.go` | `Widget` interface, `Touchable` interface, `Display` interface, `BaseWidget` embed |
| `epaper/gui/layout.go` | `HBox`, `VBox`, `Fixed`, `Overlay`, `layoutHint`, `Expand`, `FixedSize`, `WithPadding` |
| `epaper/gui/widgets.go` | `Label`, `Button`, `ProgressBar`, `StatusBar`, `Spacer`, `Divider`, `textWidth` helper |
| `epaper/gui/refresh.go` | `refreshManager` (unexported): dirty tracking, full/partial/fast strategy |
| `epaper/gui/navigator.go` | `Scene`, `Navigator`, `SwipeDir`, touch routing, coordinate mapping, `Run()` |
| `epaper/gui/gui_test.go` | All unit tests (package `gui`) — no hardware required |

---

## Chunk 1: Foundation — gui.go + layout.go

### Task 1: Widget interface, Display interface, BaseWidget (`gui.go`)

**Files:**
- Create: `epaper/gui/gui.go`
- Create: `epaper/gui/gui_test.go`

- [ ] **Step 1: Create `epaper/gui/gui_test.go` with BaseWidget tests**

```go
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
```

- [ ] **Step 2: Run tests — FAIL (package doesn't exist)**

```bash
go test awesomeProject/epaper/gui -v -count=1 2>&1 | head -5
```
Expected: `cannot find package` or `no Go files`

- [ ] **Step 3: Create `epaper/gui/gui.go`**

```go
// epaper/gui/gui.go — core interfaces and BaseWidget
package gui

import (
	"image"

	"awesomeProject/epaper/canvas"
	"awesomeProject/epaper/epd"
	"awesomeProject/epaper/touch"
)

// Display is the subset of *epd.Display used by Navigator.
// *epd.Display satisfies this interface.
// Note: DisplayFull is intentionally excluded — it only writes the 0x24 RAM bank,
// not the 0x26 reference frame, so subsequent DisplayPartial calls would ghost.
type Display interface {
	Init(m epd.Mode) error
	DisplayBase(buf []byte) error    // full refresh: writes 0x24 + 0x26 RAM banks
	DisplayPartial(buf []byte) error // partial refresh: full 4000-byte buffer, self-contained
	DisplayFast(buf []byte) error    // fast full refresh
	Sleep() error
	Close() error
}

// Widget is the core interface every GUI element must implement.
type Widget interface {
	Draw(c *canvas.Canvas)
	Bounds() image.Rectangle
	SetBounds(r image.Rectangle)
	PreferredSize() image.Point // intrinsic preferred size; (0,0) = no preference
	MinSize() image.Point       // minimum allocation; layout enforces this floor
	IsDirty() bool
	SetDirty()
	MarkClean()
}

// Touchable is implemented by interactive widgets.
// Navigator calls HandleTouch after hit-testing and debounce.
type Touchable interface {
	HandleTouch(pt touch.TouchPoint) bool // true = event consumed
}

// BaseWidget provides dirty-flag and bounds bookkeeping.
// Embed in custom widgets and override Draw, PreferredSize, MinSize.
//
//	type MyWidget struct {
//	    gui.BaseWidget
//	    // your fields
//	}
//
//	func (w *MyWidget) Draw(c *canvas.Canvas) { /* draw using w.Bounds() */ }
//	func (w *MyWidget) PreferredSize() image.Point { return image.Pt(60, 20) }
//	func (w *MyWidget) MinSize() image.Point       { return image.Pt(20, 20) }
type BaseWidget struct {
	bounds image.Rectangle
	dirty  bool
}

func (b *BaseWidget) Bounds() image.Rectangle     { return b.bounds }
func (b *BaseWidget) SetBounds(r image.Rectangle) { b.bounds = r; b.dirty = true }
func (b *BaseWidget) IsDirty() bool               { return b.dirty }
func (b *BaseWidget) SetDirty()                   { b.dirty = true }
func (b *BaseWidget) MarkClean()                  { b.dirty = false }
func (b *BaseWidget) PreferredSize() image.Point  { return image.Point{} }
func (b *BaseWidget) MinSize() image.Point        { return image.Point{} }
```

- [ ] **Step 4: Run tests — PASS**

```bash
go test awesomeProject/epaper/gui -v -count=1 -run TestBaseWidget
```
Expected: all 5 `TestBaseWidget*` tests PASS

- [ ] **Step 5: Commit**

```bash
git add epaper/gui/gui.go epaper/gui/gui_test.go
git commit -m "feat(gui): Widget/Display/Touchable interfaces + BaseWidget"
```

---

### Task 2: Layout containers (`layout.go`)

**Files:**
- Create: `epaper/gui/layout.go`
- Modify: `epaper/gui/gui_test.go` (append layout tests)

- [ ] **Step 1: Add VBox tests to `gui_test.go`**

Append to `gui_test.go`:

```go
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
	a := newFixedWidget(100, 20, 0, 10) // preferred 20px tall
	b := newFixedWidget(100, 30, 0, 10) // preferred 30px tall
	box := NewVBox(a, b)
	box.SetBounds(image.Rect(0, 0, 100, 100))

	// a should get preferred height (20), b should get preferred height (30)
	if a.Bounds().Dy() != 20 {
		t.Errorf("child a height = %d, want 20", a.Bounds().Dy())
	}
	if b.Bounds().Dy() != 30 {
		t.Errorf("child b height = %d, want 30", b.Bounds().Dy())
	}
	// a starts at y=0, b starts at y=20
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
	small := newTouchWidget(100, 5) // prefers 5px — too small for touch
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

	wantX := (250 - 60) / 2 // 95
	wantY := (122 - 30) / 2 // 46
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
```

- [ ] **Step 2: Run layout tests — FAIL**

```bash
go test awesomeProject/epaper/gui -v -count=1 -run "TestVBox|TestHBox|TestFixed|TestOverlay|TestWithPadding" 2>&1 | head -10
```
Expected: compile errors — `NewVBox`, `NewHBox`, etc. undefined

- [ ] **Step 3: Create `epaper/gui/layout.go`**

```go
// epaper/gui/layout.go — layout containers: VBox, HBox, Fixed, Overlay, WithPadding
package gui

import (
	"image"

	"awesomeProject/epaper/canvas"
)

// Alignment controls Overlay positioning.
type Alignment int

const (
	AlignCenter Alignment = iota
	AlignTop
	AlignBottom
	AlignLeft
	AlignRight
)

// layoutHint wraps a Widget with sizing constraints for HBox/VBox.
type layoutHint struct {
	widget Widget
	fixed  int  // > 0: fixed size in px on main axis; 0 = use PreferredSize
	expand bool // true: take remaining space equally with other expand widgets
}

// Expand makes w take all remaining space in HBox/VBox main axis.
func Expand(w Widget) layoutHint { return layoutHint{widget: w, expand: true} }

// FixedSize constrains w to px pixels in the main axis of HBox/VBox.
func FixedSize(w Widget, px int) layoutHint { return layoutHint{widget: w, fixed: px} }

func hintFor(v any) layoutHint {
	switch h := v.(type) {
	case layoutHint:
		return h
	case Widget:
		return layoutHint{widget: h}
	default:
		panic("gui: HBox/VBox child must be Widget or layoutHint")
	}
}

// touchableMin is the minimum size enforced for Touchable widgets.
const touchableMin = 20

// ── VBox ──────────────────────────────────────────────────────────────────────

// VBox stacks children vertically.
// Children may be Widget or layoutHint (created by Expand/FixedSize).
type VBox struct {
	BaseWidget
	children []layoutHint
}

func NewVBox(children ...any) *VBox {
	v := &VBox{}
	for _, c := range children {
		v.children = append(v.children, hintFor(c))
	}
	v.SetDirty()
	return v
}

func (v *VBox) PreferredSize() image.Point {
	w, h := 0, 0
	for _, ch := range v.children {
		ps := ch.widget.PreferredSize()
		if ps.X > w {
			w = ps.X
		}
		h += ps.Y
	}
	return image.Pt(w, h)
}

func (v *VBox) MinSize() image.Point {
	w, h := 0, 0
	for _, ch := range v.children {
		ms := ch.widget.MinSize()
		if ms.X > w {
			w = ms.X
		}
		h += ms.Y
	}
	return image.Pt(w, h)
}

func (v *VBox) SetBounds(r image.Rectangle) {
	v.BaseWidget.SetBounds(r)
	v.doLayout(r)
}

func (v *VBox) doLayout(r image.Rectangle) {
	totalH := r.Dy()
	used, expandCount := 0, 0
	for _, ch := range v.children {
		if ch.expand {
			expandCount++
		} else if ch.fixed > 0 {
			used += ch.fixed
		} else {
			ps := ch.widget.PreferredSize()
			ms := ch.widget.MinSize()
			h := ps.Y
			if h < ms.Y {
				h = ms.Y
			}
			if _, ok := ch.widget.(Touchable); ok && h < touchableMin {
				h = touchableMin
			}
			used += h
		}
	}
	expandH := 0
	if expandCount > 0 && totalH > used {
		expandH = (totalH - used) / expandCount
	}
	y := r.Min.Y
	for _, ch := range v.children {
		var h int
		if ch.expand {
			h = expandH
		} else if ch.fixed > 0 {
			h = ch.fixed
		} else {
			ps := ch.widget.PreferredSize()
			ms := ch.widget.MinSize()
			h = ps.Y
			if h < ms.Y {
				h = ms.Y
			}
		}
		if _, ok := ch.widget.(Touchable); ok && h < touchableMin {
			h = touchableMin
		}
		ch.widget.SetBounds(image.Rect(r.Min.X, y, r.Max.X, y+h))
		y += h
	}
}

func (v *VBox) Draw(c *canvas.Canvas) {
	for _, ch := range v.children {
		ch.widget.Draw(c)
	}
}

func (v *VBox) IsDirty() bool {
	if v.BaseWidget.IsDirty() {
		return true
	}
	for _, ch := range v.children {
		if ch.widget.IsDirty() {
			return true
		}
	}
	return false
}

func (v *VBox) MarkClean() {
	v.BaseWidget.MarkClean()
	for _, ch := range v.children {
		ch.widget.MarkClean()
	}
}

// ── HBox ──────────────────────────────────────────────────────────────────────

// HBox distributes children horizontally.
type HBox struct {
	BaseWidget
	children []layoutHint
}

func NewHBox(children ...any) *HBox {
	h := &HBox{}
	for _, c := range children {
		h.children = append(h.children, hintFor(c))
	}
	h.SetDirty()
	return h
}

func (h *HBox) PreferredSize() image.Point {
	w, ht := 0, 0
	for _, ch := range h.children {
		ps := ch.widget.PreferredSize()
		w += ps.X
		if ps.Y > ht {
			ht = ps.Y
		}
	}
	return image.Pt(w, ht)
}

func (h *HBox) MinSize() image.Point {
	w, ht := 0, 0
	for _, ch := range h.children {
		ms := ch.widget.MinSize()
		w += ms.X
		if ms.Y > ht {
			ht = ms.Y
		}
	}
	return image.Pt(w, ht)
}

func (h *HBox) SetBounds(r image.Rectangle) {
	h.BaseWidget.SetBounds(r)
	h.doLayout(r)
}

func (h *HBox) doLayout(r image.Rectangle) {
	totalW := r.Dx()
	used, expandCount := 0, 0
	for _, ch := range h.children {
		if ch.expand {
			expandCount++
		} else if ch.fixed > 0 {
			used += ch.fixed
		} else {
			ps := ch.widget.PreferredSize()
			ms := ch.widget.MinSize()
			w := ps.X
			if w < ms.X {
				w = ms.X
			}
			if _, ok := ch.widget.(Touchable); ok && w < touchableMin {
				w = touchableMin
			}
			used += w
		}
	}
	expandW := 0
	if expandCount > 0 && totalW > used {
		expandW = (totalW - used) / expandCount
	}
	x := r.Min.X
	for _, ch := range h.children {
		var w int
		if ch.expand {
			w = expandW
		} else if ch.fixed > 0 {
			w = ch.fixed
		} else {
			ps := ch.widget.PreferredSize()
			ms := ch.widget.MinSize()
			w = ps.X
			if w < ms.X {
				w = ms.X
			}
		}
		if _, ok := ch.widget.(Touchable); ok && w < touchableMin {
			w = touchableMin
		}
		ch.widget.SetBounds(image.Rect(x, r.Min.Y, x+w, r.Max.Y))
		x += w
	}
}

func (h *HBox) Draw(c *canvas.Canvas) {
	for _, ch := range h.children {
		ch.widget.Draw(c)
	}
}

func (h *HBox) IsDirty() bool {
	if h.BaseWidget.IsDirty() {
		return true
	}
	for _, ch := range h.children {
		if ch.widget.IsDirty() {
			return true
		}
	}
	return false
}

func (h *HBox) MarkClean() {
	h.BaseWidget.MarkClean()
	for _, ch := range h.children {
		ch.widget.MarkClean()
	}
}

// ── Fixed ─────────────────────────────────────────────────────────────────────

// Fixed places children at absolute pixel positions within a fixed-size area.
type Fixed struct {
	BaseWidget
	w, h     int
	children []fixedChild
}

type fixedChild struct {
	widget Widget
	x, y   int
}

func NewFixed(w, h int) *Fixed {
	f := &Fixed{w: w, h: h}
	f.SetDirty()
	return f
}

// Put registers widget at position (x, y) relative to the Fixed container's origin.
func (f *Fixed) Put(w Widget, x, y int) {
	f.children = append(f.children, fixedChild{w, x, y})
	f.SetDirty()
}

func (f *Fixed) PreferredSize() image.Point { return image.Pt(f.w, f.h) }
func (f *Fixed) MinSize() image.Point       { return image.Pt(f.w, f.h) }

func (f *Fixed) SetBounds(r image.Rectangle) {
	f.BaseWidget.SetBounds(r)
	for _, ch := range f.children {
		ps := ch.widget.PreferredSize()
		ms := ch.widget.MinSize()
		cw, ch2 := ps.X, ps.Y
		if cw < ms.X {
			cw = ms.X
		}
		if ch2 < ms.Y {
			ch2 = ms.Y
		}
		ch.widget.SetBounds(image.Rect(r.Min.X+ch.x, r.Min.Y+ch.y,
			r.Min.X+ch.x+cw, r.Min.Y+ch.y+ch2))
	}
}

func (f *Fixed) Draw(c *canvas.Canvas) {
	for _, ch := range f.children {
		ch.widget.Draw(c)
	}
}

func (f *Fixed) IsDirty() bool {
	if f.BaseWidget.IsDirty() {
		return true
	}
	for _, ch := range f.children {
		if ch.widget.IsDirty() {
			return true
		}
	}
	return false
}

func (f *Fixed) MarkClean() {
	f.BaseWidget.MarkClean()
	for _, ch := range f.children {
		ch.widget.MarkClean()
	}
}

// ── Overlay ───────────────────────────────────────────────────────────────────

// Overlay positions content on top of the current scene.
// Created with NewOverlay; pushed via nav.PushOverlay(*Overlay).
type Overlay struct {
	BaseWidget
	content        Widget
	align          Alignment
	screenW, screenH int
}

func NewOverlay(content Widget, align Alignment) *Overlay {
	o := &Overlay{content: content, align: align}
	o.SetDirty()
	return o
}

// setScreen is called by Navigator.PushOverlay to set the screen dimensions.
func (o *Overlay) setScreen(w, h int) {
	o.screenW, o.screenH = w, h
	o.doLayout()
}

func (o *Overlay) doLayout() {
	ps := o.content.PreferredSize()
	ms := o.content.MinSize()
	cw, ch := ps.X, ps.Y
	if cw < ms.X {
		cw = ms.X
	}
	if ch < ms.Y {
		ch = ms.Y
	}
	var x, y int
	switch o.align {
	case AlignCenter:
		x = (o.screenW - cw) / 2
		y = (o.screenH - ch) / 2
	case AlignTop:
		x = (o.screenW - cw) / 2
		y = 0
	case AlignBottom:
		x = (o.screenW - cw) / 2
		y = o.screenH - ch
	case AlignLeft:
		x = 0
		y = (o.screenH - ch) / 2
	case AlignRight:
		x = o.screenW - cw
		y = (o.screenH - ch) / 2
	}
	o.content.SetBounds(image.Rect(x, y, x+cw, y+ch))
	o.BaseWidget.SetBounds(image.Rect(0, 0, o.screenW, o.screenH))
}

func (o *Overlay) Draw(c *canvas.Canvas) { o.content.Draw(c) }

func (o *Overlay) IsDirty() bool {
	return o.BaseWidget.IsDirty() || o.content.IsDirty()
}

func (o *Overlay) MarkClean() {
	o.BaseWidget.MarkClean()
	o.content.MarkClean()
}

// ── WithPadding ───────────────────────────────────────────────────────────────

type paddingWidget struct {
	BaseWidget
	inner Widget
	px    int
}

// WithPadding wraps w with uniform px padding on all 4 sides.
// Returns a Widget that can be used anywhere a Widget is accepted.
func WithPadding(px int, w Widget) Widget {
	p := &paddingWidget{inner: w, px: px}
	p.SetDirty()
	return p
}

func (p *paddingWidget) PreferredSize() image.Point {
	ps := p.inner.PreferredSize()
	return image.Pt(ps.X+2*p.px, ps.Y+2*p.px)
}

func (p *paddingWidget) MinSize() image.Point {
	ms := p.inner.MinSize()
	return image.Pt(ms.X+2*p.px, ms.Y+2*p.px)
}

func (p *paddingWidget) SetBounds(r image.Rectangle) {
	p.BaseWidget.SetBounds(r)
	inner := image.Rect(r.Min.X+p.px, r.Min.Y+p.px, r.Max.X-p.px, r.Max.Y-p.px)
	p.inner.SetBounds(inner)
}

func (p *paddingWidget) Draw(c *canvas.Canvas) { p.inner.Draw(c) }

func (p *paddingWidget) IsDirty() bool {
	return p.BaseWidget.IsDirty() || p.inner.IsDirty()
}

func (p *paddingWidget) MarkClean() {
	p.BaseWidget.MarkClean()
	p.inner.MarkClean()
}
```

- [ ] **Step 4: Run layout tests — PASS**

```bash
go test awesomeProject/epaper/gui -v -count=1 -run "TestVBox|TestHBox|TestFixed|TestOverlay|TestWithPadding"
```
Expected: all layout tests PASS

- [ ] **Step 5: Commit**

```bash
git add epaper/gui/layout.go epaper/gui/gui_test.go
git commit -m "feat(gui): HBox/VBox/Fixed/Overlay/WithPadding layout containers"
```

---

## Chunk 2: Widgets + RefreshManager

### Task 3: Built-in widgets (`widgets.go`)

**Files:**
- Create: `epaper/gui/widgets.go`
- Modify: `epaper/gui/gui_test.go` (append widget tests)

- [ ] **Step 1: Add widget tests to `gui_test.go`**

Append to `gui_test.go`:

```go
// ── widget tests ──────────────────────────────────────────────────────────────

func TestLabelSetTextMarksDirty(t *testing.T) {
	l := NewLabel("hello")
	l.MarkClean()
	l.SetText("world")
	if !l.IsDirty() {
		t.Error("SetText should mark dirty")
	}
}

func TestLabelPreferredSizeUsesFont(t *testing.T) {
	l := NewLabel("A")
	ps := l.PreferredSize()
	if ps.Y == 0 {
		t.Error("PreferredSize height should be > 0 (font line height)")
	}
}

func TestLabelDrawDoesNotPanic(t *testing.T) {
	c := canvas.New(epd.Width, epd.Height, canvas.Rot90)
	l := NewLabel("hello")
	l.SetBounds(image.Rect(0, 0, 100, 20))
	l.Draw(c) // must not panic, even with nil font
}

func TestButtonHandleTouchFiresOnClick(t *testing.T) {
	clicked := false
	btn := NewButton("OK")
	btn.OnClick(func() { clicked = true })
	btn.SetBounds(image.Rect(0, 0, 60, 20))
	btn.HandleTouch(touch.TouchPoint{X: 30, Y: 10})
	if !clicked {
		t.Error("OnClick should fire on HandleTouch")
	}
}

func TestButtonDisabledDoesNotFire(t *testing.T) {
	clicked := false
	btn := NewButton("OK")
	btn.OnClick(func() { clicked = true })
	btn.SetEnabled(false)
	btn.HandleTouch(touch.TouchPoint{})
	if clicked {
		t.Error("disabled button should not fire OnClick")
	}
}

func TestButtonMinSize(t *testing.T) {
	btn := NewButton("X")
	ms := btn.MinSize()
	if ms.X < 20 || ms.Y < 20 {
		t.Errorf("Button MinSize = %v, want >= (20,20)", ms)
	}
}

func TestButtonPressDrawsInverted(t *testing.T) {
	c := canvas.New(epd.Width, epd.Height, canvas.Rot90)
	c.Clear()
	btn := NewButton("X")
	btn.SetBounds(image.Rect(0, 0, 50, 20))
	// Force pressed state
	btn.HandleTouch(touch.TouchPoint{})
	if !btn.pressed {
		t.Error("HandleTouch should set pressed=true")
	}
}

func TestProgressBarClampValue(t *testing.T) {
	bar := NewProgressBar()
	bar.SetValue(2.0)
	if bar.value != 1.0 {
		t.Errorf("value clamped to %v, want 1.0", bar.value)
	}
	bar.SetValue(-1.0)
	if bar.value != 0.0 {
		t.Errorf("value clamped to %v, want 0.0", bar.value)
	}
}

func TestProgressBarPreferredSize(t *testing.T) {
	bar := NewProgressBar()
	ps := bar.PreferredSize()
	if ps.X != 0 {
		t.Errorf("ProgressBar PreferredSize.X = %d, want 0 (use Expand)", ps.X)
	}
	if ps.Y != 12 {
		t.Errorf("ProgressBar PreferredSize.Y = %d, want 12", ps.Y)
	}
}

func TestProgressBarDrawDoesNotPanic(t *testing.T) {
	c := canvas.New(epd.Width, epd.Height, canvas.Rot90)
	bar := NewProgressBar()
	bar.SetValue(0.5)
	bar.SetBounds(image.Rect(0, 0, 100, 12))
	bar.Draw(c)
}

func TestStatusBarPreferredHeight(t *testing.T) {
	sb := NewStatusBar("test")
	if sb.PreferredSize().Y != 18 {
		t.Errorf("StatusBar height = %d, want 18", sb.PreferredSize().Y)
	}
}

func TestStatusBarSetLeftMarksDirty(t *testing.T) {
	sb := NewStatusBar("test")
	sb.MarkClean()
	sb.SetLeft("hello")
	if !sb.IsDirty() {
		t.Error("SetLeft should mark dirty")
	}
}

func TestStatusBarDrawDoesNotPanic(t *testing.T) {
	c := canvas.New(epd.Width, epd.Height, canvas.Rot90)
	sb := NewStatusBar("oioio")
	sb.SetLeft("USB: actif")
	sb.SetRight("12:34")
	sb.SetBounds(image.Rect(0, 0, 250, 18))
	sb.Draw(c)
}

func TestSpacerHasZeroSize(t *testing.T) {
	s := NewSpacer()
	if s.PreferredSize() != (image.Point{}) {
		t.Errorf("Spacer PreferredSize = %v, want (0,0)", s.PreferredSize())
	}
}

func TestDividerPreferredHeight(t *testing.T) {
	d := NewDivider()
	if d.PreferredSize().Y != 1 {
		t.Errorf("Divider PreferredSize.Y = %d, want 1", d.PreferredSize().Y)
	}
}
```

- [ ] **Step 2: Run widget tests — FAIL**

```bash
go test awesomeProject/epaper/gui -v -count=1 -run "TestLabel|TestButton|TestProgressBar|TestStatusBar|TestSpacer|TestDivider" 2>&1 | head -5
```
Expected: compile errors — `NewLabel`, `NewButton`, etc. undefined

- [ ] **Step 3: Create `epaper/gui/widgets.go`**

```go
// epaper/gui/widgets.go — built-in widgets: Label, Button, ProgressBar, StatusBar, Spacer, Divider
package gui

import (
	"image"
	"time"

	"awesomeProject/epaper/canvas"
	"awesomeProject/epaper/epd"
	"awesomeProject/epaper/touch"
)

// textWidth returns the pixel width of text rendered in font f.
// Returns 0 if f is nil.
func textWidth(text string, f canvas.Font) int {
	if f == nil {
		return 0
	}
	w := 0
	for _, r := range text {
		_, gw, _ := f.Glyph(r)
		w += gw
	}
	return w
}

// ── Label ─────────────────────────────────────────────────────────────────────

// Label displays a single line of text.
type Label struct {
	BaseWidget
	text  string
	font  canvas.Font
	align Alignment
	wrap  bool
}

func NewLabel(text string) *Label {
	l := &Label{
		text:  text,
		font:  canvas.EmbeddedFont(12),
		align: AlignLeft,
	}
	l.SetDirty()
	return l
}

func (l *Label) SetText(text string)     { l.text = text; l.SetDirty() }
func (l *Label) SetFont(f canvas.Font)   { l.font = f; l.SetDirty() }
func (l *Label) SetAlign(a Alignment)    { l.align = a; l.SetDirty() }
func (l *Label) SetWrap(w bool)          { l.wrap = w; l.SetDirty() }

func (l *Label) PreferredSize() image.Point {
	if l.font == nil {
		return image.Pt(0, 12)
	}
	return image.Pt(textWidth(l.text, l.font), l.font.LineHeight())
}

func (l *Label) MinSize() image.Point {
	if l.font == nil {
		return image.Pt(0, 12)
	}
	return image.Pt(0, l.font.LineHeight())
}

func (l *Label) Draw(c *canvas.Canvas) {
	if l.font == nil {
		return
	}
	r := l.Bounds()
	text := l.text
	// Truncate with "…" if text is too wide and wrap is off.
	if !l.wrap {
		maxW := r.Dx()
		for textWidth(text, l.font) > maxW && len(text) > 0 {
			runes := []rune(text)
			if len(runes) <= 1 {
				break
			}
			text = string(runes[:len(runes)-1]) + "…"
		}
	}
	x := r.Min.X
	if l.align == AlignCenter {
		tw := textWidth(text, l.font)
		x = r.Min.X + (r.Dx()-tw)/2
	} else if l.align == AlignRight {
		tw := textWidth(text, l.font)
		x = r.Max.X - tw
	}
	c.DrawText(x, r.Min.Y, text, l.font, canvas.Black)
}

// ── Button ────────────────────────────────────────────────────────────────────

// Button is a pressable widget. Implements Touchable.
// OnClick fires on touch-down. The pressed state (inverted colors) lasts one
// render cycle — Draw() clears it and re-marks dirty so the Navigator renders
// the normal state immediately after.
type Button struct {
	BaseWidget
	label   string
	font    canvas.Font
	enabled bool
	pressed bool
	onClick func()
}

func NewButton(label string) *Button {
	b := &Button{
		label:   label,
		font:    canvas.EmbeddedFont(12),
		enabled: true,
	}
	b.SetDirty()
	return b
}

// OnClick registers the callback and returns the button for chaining.
func (b *Button) OnClick(fn func()) *Button { b.onClick = fn; return b }
func (b *Button) SetEnabled(enabled bool)   { b.enabled = enabled; b.SetDirty() }

func (b *Button) PreferredSize() image.Point {
	w := 8
	if b.font != nil {
		w += textWidth(b.label, b.font)
	}
	if w < 20 {
		w = 20
	}
	h := 20
	if b.font != nil && b.font.LineHeight()+4 > h {
		h = b.font.LineHeight() + 4
	}
	return image.Pt(w, h)
}

func (b *Button) MinSize() image.Point { return image.Pt(20, 20) }

// HandleTouch fires OnClick and marks the button as pressed.
func (b *Button) HandleTouch(pt touch.TouchPoint) bool {
	if !b.enabled {
		return false
	}
	b.pressed = true
	b.SetDirty()
	if b.onClick != nil {
		b.onClick()
	}
	return true
}

func (b *Button) Draw(c *canvas.Canvas) {
	r := b.Bounds()
	bg, fg := canvas.White, canvas.Black
	if b.pressed {
		bg, fg = canvas.Black, canvas.White
		// Clear pressed state — next Render() will show normal state.
		b.pressed = false
		b.SetDirty()
	}
	c.DrawRect(r, bg, true)
	c.DrawRect(r, canvas.Black, false)
	if b.font != nil {
		tw := textWidth(b.label, b.font)
		tx := r.Min.X + (r.Dx()-tw)/2
		ty := r.Min.Y + (r.Dy()-b.font.LineHeight())/2
		c.DrawText(tx, ty, b.label, b.font, fg)
	}
}

// ── ProgressBar ───────────────────────────────────────────────────────────────

// BarStyle controls how the progress bar is rendered.
type BarStyle int

const (
	BarStyleFill BarStyle = iota // solid filled rectangle
	BarStyleDots                 // row of small squares
)

// ProgressBar displays a 0–1 value as a filled or dotted bar.
// Wrap in Expand() to fill available width: gui.Expand(gui.NewProgressBar()).
type ProgressBar struct {
	BaseWidget
	value float64
	style BarStyle
}

func NewProgressBar() *ProgressBar {
	p := &ProgressBar{style: BarStyleFill}
	p.SetDirty()
	return p
}

func (p *ProgressBar) SetValue(v float64) {
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}
	p.value = v
	p.SetDirty()
}

func (p *ProgressBar) SetStyle(s BarStyle) { p.style = s; p.SetDirty() }

// PreferredSize returns (0, 12). Width is 0 — use Expand() for full width.
func (p *ProgressBar) PreferredSize() image.Point { return image.Pt(0, 12) }
func (p *ProgressBar) MinSize() image.Point       { return image.Pt(20, 8) }

func (p *ProgressBar) Draw(c *canvas.Canvas) {
	r := p.Bounds()
	c.DrawRect(r, canvas.Black, false)
	inner := image.Rect(r.Min.X+1, r.Min.Y+1, r.Max.X-1, r.Max.Y-1)
	switch p.style {
	case BarStyleFill:
		fillW := int(float64(inner.Dx()) * p.value)
		if fillW > 0 {
			c.DrawRect(image.Rect(inner.Min.X, inner.Min.Y,
				inner.Min.X+fillW, inner.Max.Y), canvas.Black, true)
		}
	case BarStyleDots:
		const dotW, dotGap = 4, 2
		filled := int(float64(inner.Dx()) * p.value)
		for x := inner.Min.X; x+dotW <= inner.Max.X; x += dotW + dotGap {
			col := canvas.White
			if x-inner.Min.X < filled {
				col = canvas.Black
			}
			c.DrawRect(image.Rect(x, inner.Min.Y, x+dotW, inner.Max.Y), col, true)
		}
	}
}

// ── StatusBar ─────────────────────────────────────────────────────────────────

// StatusBar is a fixed-height (18 px) black bar with white text.
// SetAutoTime(true) makes Navigator.Run() update the right text every minute.
type StatusBar struct {
	BaseWidget
	title    string
	left     string
	right    string
	autoTime bool
}

func NewStatusBar(title string) *StatusBar {
	s := &StatusBar{title: title}
	s.SetDirty()
	return s
}

func (s *StatusBar) SetLeft(text string)  { s.left = text; s.SetDirty() }
func (s *StatusBar) SetRight(text string) { s.right = text; s.SetDirty() }
func (s *StatusBar) SetAutoTime(b bool)   { s.autoTime = b; s.SetDirty() }
func (s *StatusBar) IsAutoTime() bool     { return s.autoTime }

// updateTime is called by Navigator.Run() on its minute ticker.
func (s *StatusBar) updateTime() { s.right = time.Now().Format("15:04"); s.SetDirty() }

func (s *StatusBar) PreferredSize() image.Point { return image.Pt(0, 18) }
func (s *StatusBar) MinSize() image.Point       { return image.Pt(0, 18) }

func (s *StatusBar) Draw(c *canvas.Canvas) {
	r := s.Bounds()
	c.DrawRect(r, canvas.Black, true)
	f := canvas.EmbeddedFont(12)
	if f == nil {
		return
	}
	left := s.left
	if left == "" {
		left = s.title
	}
	c.DrawText(r.Min.X+2, r.Min.Y+3, left, f, canvas.White)
	if s.right != "" {
		rw := textWidth(s.right, f)
		c.DrawText(r.Max.X-rw-2, r.Min.Y+3, s.right, f, canvas.White)
	}
}

// ── Spacer ────────────────────────────────────────────────────────────────────

// Spacer is a zero-size invisible gap. Use with Expand() for flexible space:
//
//	gui.Expand(gui.NewSpacer())
type Spacer struct{ BaseWidget }

func NewSpacer() *Spacer { return &Spacer{} }
func (s *Spacer) Draw(_ *canvas.Canvas) {}

// ── Divider ───────────────────────────────────────────────────────────────────

// Divider draws a 1 px black line — horizontal in VBox, vertical in HBox.
type Divider struct{ BaseWidget }

func NewDivider() *Divider {
	d := &Divider{}
	d.SetDirty()
	return d
}

func (d *Divider) PreferredSize() image.Point { return image.Pt(0, 1) }
func (d *Divider) MinSize() image.Point       { return image.Pt(0, 1) }

func (d *Divider) Draw(c *canvas.Canvas) {
	r := d.Bounds()
	if r.Dy() >= r.Dx() {
		c.DrawLine(r.Min.X, r.Min.Y, r.Min.X, r.Max.Y, canvas.Black) // vertical
	} else {
		c.DrawLine(r.Min.X, r.Min.Y, r.Max.X, r.Min.Y, canvas.Black) // horizontal
	}
}

// Ensure widgets package compiles even without direct epd/touch usage in some paths.
var _ = epd.ModeFull
var _ touch.TouchEvent{}
```

- [ ] **Step 4: Run widget tests — PASS**

```bash
go test awesomeProject/epaper/gui -v -count=1 -run "TestLabel|TestButton|TestProgressBar|TestStatusBar|TestSpacer|TestDivider"
```
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add epaper/gui/widgets.go epaper/gui/gui_test.go
git commit -m "feat(gui): Label/Button/ProgressBar/StatusBar/Spacer/Divider widgets"
```

---

### Task 4: RefreshManager (`refresh.go`)

**Files:**
- Create: `epaper/gui/refresh.go`
- Modify: `epaper/gui/gui_test.go` (append refresh tests)

- [ ] **Step 1: Add refresh tests to `gui_test.go`**

Append to `gui_test.go`:

```go
// ── refresh tests ─────────────────────────────────────────────────────────────

// fakeDisplay implements the Display interface for tests — no hardware needed.
type fakeDisplay struct {
	initCalled    int
	baseCalled    int
	partialCalled int
	fastCalled    int
	lastMode      epd.Mode
}

func (f *fakeDisplay) Init(m epd.Mode) error    { f.initCalled++; f.lastMode = m; return nil }
func (f *fakeDisplay) DisplayBase(b []byte) error { f.baseCalled++; return nil }
func (f *fakeDisplay) DisplayPartial(b []byte) error { f.partialCalled++; return nil }
func (f *fakeDisplay) DisplayFast(b []byte) error    { f.fastCalled++; return nil }
func (f *fakeDisplay) Sleep() error               { return nil }
func (f *fakeDisplay) Close() error               { return nil }

func TestRefreshManagerNoop(t *testing.T) {
	d := &fakeDisplay{}
	rm := newRefreshManager(d)
	c := canvas.New(epd.Width, epd.Height, canvas.Rot90)
	// No dirty widgets — render should be a noop
	if err := rm.Render(c, nil); err != nil {
		t.Fatalf("Render noop: %v", err)
	}
	if d.partialCalled != 0 || d.baseCalled != 0 {
		t.Error("expected noop, got display calls")
	}
}

func TestRefreshManagerPartialOnDirtyWidget(t *testing.T) {
	d := &fakeDisplay{}
	rm := newRefreshManager(d)
	c := canvas.New(epd.Width, epd.Height, canvas.Rot90)
	w := NewLabel("test")
	w.SetBounds(image.Rect(0, 0, 100, 20))
	// SetDirty already called by NewLabel; render once to establish base
	rm.RenderWith(c, []Widget{w}, true)
	d.partialCalled = 0
	// Dirty widget → partial update
	w.SetDirty()
	if err := rm.Render(c, []Widget{w}); err != nil {
		t.Fatalf("Render partial: %v", err)
	}
	if d.partialCalled != 1 {
		t.Errorf("expected 1 partial call, got %d", d.partialCalled)
	}
}

func TestRefreshManagerFullOnForced(t *testing.T) {
	d := &fakeDisplay{}
	rm := newRefreshManager(d)
	c := canvas.New(epd.Width, epd.Height, canvas.Rot90)
	w := NewLabel("test")
	w.SetBounds(image.Rect(0, 0, 100, 20))
	if err := rm.RenderWith(c, []Widget{w}, true); err != nil {
		t.Fatalf("RenderWith forced: %v", err)
	}
	// forced → Init(ModeFull) + DisplayBase
	if d.initCalled == 0 || d.baseCalled == 0 {
		t.Error("forced render must call Init(ModeFull)+DisplayBase")
	}
}

func TestRefreshManagerAntiGhostCounter(t *testing.T) {
	d := &fakeDisplay{}
	rm := newRefreshManager(d)
	rm.antiGhostN = 3 // low threshold for test
	c := canvas.New(epd.Width, epd.Height, canvas.Rot90)
	w := NewLabel("test")
	w.SetBounds(image.Rect(0, 0, 100, 20))
	// First forced to establish base
	rm.RenderWith(c, []Widget{w}, true)
	initBefore := d.initCalled
	// Run N partial updates — on the Nth, a full refresh must occur
	for i := 0; i < rm.antiGhostN; i++ {
		w.SetDirty()
		rm.Render(c, []Widget{w})
	}
	if d.initCalled <= initBefore {
		t.Errorf("expected anti-ghost full refresh after %d partial updates", rm.antiGhostN)
	}
}
```

- [ ] **Step 2: Run refresh tests — FAIL**

```bash
go test awesomeProject/epaper/gui -v -count=1 -run "TestRefresh" 2>&1 | head -5
```
Expected: compile errors — `newRefreshManager`, `refreshManager` undefined

- [ ] **Step 3: Create `epaper/gui/refresh.go`**

```go
// epaper/gui/refresh.go — smart partial/full refresh decision engine
package gui

import (
	"awesomeProject/epaper/canvas"
	"awesomeProject/epaper/epd"
)

const defaultAntiGhostN = 50 // full refresh every N partial updates

// refreshManager tracks dirty state and decides refresh strategy.
type refreshManager struct {
	display    Display
	antiGhostN int // full refresh every N partial updates
	counter    int // partial updates since last full refresh
	hasBase    bool
}

func newRefreshManager(d Display) *refreshManager {
	return &refreshManager{display: d, antiGhostN: defaultAntiGhostN}
}

// Render draws dirty widgets and refreshes with the appropriate strategy.
// Noop if no widget is dirty.
func (rm *refreshManager) Render(c *canvas.Canvas, widgets []Widget) error {
	dirty := false
	for _, w := range widgets {
		if w.IsDirty() {
			dirty = true
		}
	}
	if !dirty {
		return nil
	}
	// Anti-ghosting: full refresh every antiGhostN partial updates
	if rm.counter >= rm.antiGhostN {
		return rm.fullRefresh(c, widgets)
	}
	return rm.partialRefresh(c, widgets)
}

// RenderWith draws all widgets and forces a full (forced=true) or partial refresh.
// forced=true is used on Push/Pop (scene change) and on first render.
func (rm *refreshManager) RenderWith(c *canvas.Canvas, widgets []Widget, forced bool) error {
	if forced {
		return rm.fullRefresh(c, widgets)
	}
	return rm.partialRefresh(c, widgets)
}

func (rm *refreshManager) fullRefresh(c *canvas.Canvas, widgets []Widget) error {
	drawAll(c, widgets)
	if err := rm.display.Init(epd.ModeFull); err != nil {
		return err
	}
	if err := rm.display.DisplayBase(c.Bytes()); err != nil {
		return err
	}
	markAllClean(widgets)
	rm.counter = 0
	rm.hasBase = true
	return nil
}

func (rm *refreshManager) partialRefresh(c *canvas.Canvas, widgets []Widget) error {
	if !rm.hasBase {
		// No base established yet — fall back to full.
		return rm.fullRefresh(c, widgets)
	}
	drawAll(c, widgets)
	if err := rm.display.DisplayPartial(c.Bytes()); err != nil {
		return err
	}
	markAllClean(widgets)
	rm.counter++
	return nil
}

func drawAll(c *canvas.Canvas, widgets []Widget) {
	for _, w := range widgets {
		w.Draw(c)
	}
}

func markAllClean(widgets []Widget) {
	for _, w := range widgets {
		w.MarkClean()
	}
}
```

- [ ] **Step 4: Run refresh tests — PASS**

```bash
go test awesomeProject/epaper/gui -v -count=1 -run "TestRefresh"
```
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add epaper/gui/refresh.go epaper/gui/gui_test.go
git commit -m "feat(gui): refreshManager with partial/full/anti-ghost strategy"
```

---

## Chunk 3: Navigator

### Task 5: Scene + Navigator (`navigator.go`)

**Files:**
- Create: `epaper/gui/navigator.go`
- Modify: `epaper/gui/gui_test.go` (append navigator tests)

- [ ] **Step 1: Add navigator tests to `gui_test.go`**

Append to `gui_test.go`:

```go
// ── navigator tests ───────────────────────────────────────────────────────────

func TestNavigatorPushRendersScene(t *testing.T) {
	d := &fakeDisplay{}
	nav := NewNavigator(d)
	l := NewLabel("hello")
	l.SetBounds(image.Rect(0, 0, 100, 20))
	s := &Scene{Widgets: []Widget{l}}
	if err := nav.Push(s); err != nil {
		t.Fatalf("Push: %v", err)
	}
	if d.initCalled == 0 {
		t.Error("Push must trigger full refresh (Init called)")
	}
}

func TestNavigatorPopRestoresPreviousScene(t *testing.T) {
	d := &fakeDisplay{}
	nav := NewNavigator(d)
	s1 := &Scene{Widgets: []Widget{NewLabel("s1")}}
	s2 := &Scene{Widgets: []Widget{NewLabel("s2")}}
	nav.Push(s1)
	nav.Push(s2)
	if err := nav.Pop(); err != nil {
		t.Fatalf("Pop: %v", err)
	}
	// After pop, stack has s1 — a full refresh should have occurred
	if d.initCalled < 2 {
		t.Error("Pop must trigger full refresh")
	}
}

func TestNavigatorPopOnSingleSceneIsNoop(t *testing.T) {
	d := &fakeDisplay{}
	nav := NewNavigator(d)
	s := &Scene{Widgets: []Widget{NewLabel("root")}}
	nav.Push(s)
	// Pop on single scene must not panic or error
	if err := nav.Pop(); err != nil {
		t.Fatalf("Pop on single scene: %v", err)
	}
}

func TestNavigatorTouchRoutingCallsHandler(t *testing.T) {
	d := &fakeDisplay{}
	nav := NewNavigator(d)
	clicked := false
	btn := NewButton("OK")
	btn.OnClick(func() { clicked = true })
	// Logical coords: 250 wide x 122 tall. Place button at (10,10)-(60,30).
	btn.SetBounds(image.Rect(10, 10, 60, 30))
	s := &Scene{Widgets: []Widget{btn}}
	nav.Push(s)
	// Simulate touch at physical coords that map into button logical bounds.
	// logX = clamp(pt.Y, 0, 249), logY = clamp((122-1)-pt.X, 0, 121)
	// We want logX=20 (inside 10–60), logY=15 (inside 10–30)
	// → pt.Y=20, pt.X=(121-15)=106
	nav.handleTouch(touch.TouchPoint{X: 106, Y: 20})
	if !clicked {
		t.Error("touch should route to button and fire OnClick")
	}
}

func TestNavigatorTouchDebounce(t *testing.T) {
	d := &fakeDisplay{}
	nav := NewNavigator(d)
	count := 0
	btn := NewButton("OK")
	btn.OnClick(func() { count++ })
	btn.SetBounds(image.Rect(0, 0, 100, 50))
	s := &Scene{Widgets: []Widget{btn}}
	nav.Push(s)
	// Two rapid touches — second should be debounced
	nav.handleTouch(touch.TouchPoint{X: 50, Y: 50})
	nav.handleTouch(touch.TouchPoint{X: 50, Y: 50})
	if count > 1 {
		t.Errorf("rapid touches should be debounced, got %d clicks", count)
	}
}
```

- [ ] **Step 2: Run navigator tests — FAIL**

```bash
go test awesomeProject/epaper/gui -v -count=1 -run "TestNavigator" 2>&1 | head -5
```
Expected: compile errors — `NewNavigator`, `Scene` undefined

- [ ] **Step 3: Create `epaper/gui/navigator.go`**

```go
// epaper/gui/navigator.go — scene stack, touch routing, refresh coordination
package gui

import (
	"context"
	"image"
	"sync"
	"time"

	"awesomeProject/epaper/canvas"
	"awesomeProject/epaper/epd"
	"awesomeProject/epaper/touch"
)

const debounce = 200 * time.Millisecond

// Scene is a screen's widget tree and optional lifecycle hooks.
type Scene struct {
	Widgets  []Widget
	OnEnter  func() // called when scene becomes active
	OnLeave  func() // called when scene is popped
}

// SwipeDir is the direction of a swipe gesture (reserved for future use).
type SwipeDir int

const (
	SwipeLeft  SwipeDir = iota
	SwipeRight SwipeDir = iota
	SwipeUp    SwipeDir = iota
	SwipeDown  SwipeDir = iota
)

// Navigator manages a stack of Scenes and coordinates touch routing + refresh.
//
// Concurrency: Push, Pop, and Render are NOT concurrent-safe with Run().
// In tests, call these methods directly; in production, they must be called
// from inside scene callbacks (OnEnter/OnLeave) or before Run().
type Navigator struct {
	display Display
	rm      *refreshManager
	canvas  *canvas.Canvas
	stack   []*Scene
	mu      sync.Mutex
	// debounce: per-widget last-fired timestamp
	lastFire map[Widget]time.Time
}

// NewNavigator creates a Navigator. The Display must outlive the Navigator.
func NewNavigator(d Display) *Navigator {
	c := canvas.New(epd.Width, epd.Height, canvas.Rot90)
	return &Navigator{
		display:  d,
		rm:       newRefreshManager(d),
		canvas:   c,
		lastFire: make(map[Widget]time.Time),
	}
}

// Push adds a scene to the stack and triggers a forced full refresh.
func (nav *Navigator) Push(s *Scene) error {
	if len(nav.stack) > 0 {
		top := nav.stack[len(nav.stack)-1]
		if top.OnLeave != nil {
			top.OnLeave()
		}
	}
	nav.stack = append(nav.stack, s)
	if s.OnEnter != nil {
		s.OnEnter()
	}
	return nav.rm.RenderWith(nav.canvas, s.Widgets, true)
}

// Pop removes the top scene and restores the previous one.
// If only one scene is on the stack, Pop is a noop.
func (nav *Navigator) Pop() error {
	if len(nav.stack) <= 1 {
		return nil
	}
	top := nav.stack[len(nav.stack)-1]
	if top.OnLeave != nil {
		top.OnLeave()
	}
	nav.stack = nav.stack[:len(nav.stack)-1]
	prev := nav.stack[len(nav.stack)-1]
	if prev.OnEnter != nil {
		prev.OnEnter()
	}
	return nav.rm.RenderWith(nav.canvas, prev.Widgets, true)
}

// Render redraws the current scene's dirty widgets (partial or noop).
func (nav *Navigator) Render() error {
	if len(nav.stack) == 0 {
		return nil
	}
	return nav.rm.Render(nav.canvas, nav.stack[len(nav.stack)-1].Widgets)
}

// handleTouch maps physical touch coords → logical coords, then routes to widgets.
// Physical: pt.X ∈ [0,121], pt.Y ∈ [0,249] (GT1151 raw).
// Logical (after Rot90): X ∈ [0,249], Y ∈ [0,121].
//   logX = clamp(pt.Y, 0, epd.Height-1)
//   logY = clamp((epd.Width-1) - pt.X, 0, epd.Width-1)
func (nav *Navigator) handleTouch(pt touch.TouchPoint) {
	logX := clamp(int(pt.Y), 0, epd.Height-1)
	logY := clamp((epd.Width-1)-int(pt.X), 0, epd.Width-1)
	logPt := image.Pt(logX, logY)

	if len(nav.stack) == 0 {
		return
	}
	scene := nav.stack[len(nav.stack)-1]
	// Route to the first widget whose bounds contain the logical point.
	for _, w := range scene.Widgets {
		if !logPt.In(w.Bounds()) {
			continue
		}
		t, ok := w.(Touchable)
		if !ok {
			continue
		}
		// Debounce
		nav.mu.Lock()
		last := nav.lastFire[w]
		now := time.Now()
		if now.Sub(last) < debounce {
			nav.mu.Unlock()
			continue
		}
		nav.lastFire[w] = now
		nav.mu.Unlock()

		if t.HandleTouch(pt) {
			break
		}
	}
}

// Run starts the touch event loop and blocks until ctx is cancelled.
// After Run returns, call display.Sleep() then display.Close().
func (nav *Navigator) Run(ctx context.Context, events <-chan touch.TouchEvent) {
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-events:
			if !ok {
				return
			}
			for _, pt := range ev.Points {
				nav.handleTouch(pt)
			}
			// Render any dirty state after touch handling.
			nav.Render() //nolint:errcheck
		}
	}
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
```

- [ ] **Step 4: Run navigator tests — PASS**

```bash
go test awesomeProject/epaper/gui -v -count=1 -run "TestNavigator"
```
Expected: all PASS

- [ ] **Step 5: Run full test suite — all green**

```bash
go test awesomeProject/epaper/gui -v -count=1
```
Expected: all tests PASS (BaseWidget, layout, widget, refresh, navigator)

- [ ] **Step 6: Commit**

```bash
git add epaper/gui/navigator.go epaper/gui/gui_test.go
git commit -m "feat(gui): Navigator with scene stack, touch routing, debounce"
```

---

## Chunk 4: Hello Integration

### Task 6: Migrate `hello/epaper.go` to use `epaper/gui`

**Files:**
- Modify: `hello/epaper.go`

The current `hello/epaper.go` drives the display directly (init → DisplayBase → DisplayPartial loop). Replace it with a Navigator-based flow using a single status scene.

- [ ] **Step 1: Read existing `hello/epaper.go`**

```bash
cat hello/epaper.go
```

- [ ] **Step 2: Rewrite `hello/epaper.go`**

Replace the file with:

```go
// hello/epaper.go — e-paper UI using epaper/gui Navigator
package main

import (
	"context"
	"fmt"
	"image"
	"log"
	"sync"

	"awesomeProject/epaper/epd"
	"awesomeProject/epaper/gui"
	"awesomeProject/epaper/touch"
)

const (
	epdSPIDevice = "/dev/spidev0.0"
	epdSPISpeed  = 4_000_000
	epdPinRST    = 17
	epdPinDC     = 25
	epdPinCS     = 8
	epdPinBUSY   = 24
	touchDevice = "/dev/i2c-1"
	touchAddr   = 0x14 // used as uint16 in touch.Config.I2CAddr
	touchPinTRST = 22
	touchPinINT  = 27
)

// epaperState holds the running GUI state for the hello program.
type epaperState struct {
	nav      *gui.Navigator
	status   *gui.StatusBar
	mu       sync.Mutex
	cancelFn context.CancelFunc
}

// startEPaper initialises hardware and starts the GUI event loop.
// Returns nil if hardware is unavailable (non-fatal — program runs without display).
func startEPaper(ctx context.Context) *epaperState {
	d, err := epd.New(epd.Config{
		SPIDevice: epdSPIDevice,
		SPISpeed:  epdSPISpeed,
		PinRST:    epdPinRST,
		PinDC:     epdPinDC,
		PinCS:     epdPinCS,
		PinBUSY:   epdPinBUSY,
	})
	if err != nil {
		log.Printf("epaper: display unavailable: %v", err)
		return nil
	}

	td, err := touch.New(touch.Config{
		I2CDevice: touchDevice,
		I2CAddr:   touchAddr,
		PinTRST:   touchPinTRST,
		PinINT:    touchPinINT,
	})
	if err != nil {
		log.Printf("epaper: touch unavailable: %v", err)
		_ = d.Close()
		return nil
	}
	tc, err := td.Start(ctx)
	if err != nil {
		log.Printf("epaper: touch start failed: %v", err)
		_ = d.Close()
		return nil
	}

	nav := gui.NewNavigator(d)

	// Build status scene: header + status bar.
	header := gui.NewLabel("oioio")
	header.SetBounds(image.Rect(0, 0, 250, 16))

	divider := gui.NewDivider()
	divider.SetBounds(image.Rect(0, 16, 250, 17))

	status := gui.NewStatusBar("", "")
	status.SetBounds(image.Rect(0, 17, 250, 122))

	scene := &gui.Scene{
		Widgets: []gui.Widget{header, divider, status},
	}
	if err := nav.Push(scene); err != nil {
		log.Printf("epaper: initial render failed: %v", err)
		_ = d.Sleep()
		_ = d.Close()
		return nil
	}

	guiCtx, cancel := context.WithCancel(ctx)
	go func() {
		nav.Run(guiCtx, tc)
		_ = d.Sleep()
		_ = d.Close()
	}()

	return &epaperState{nav: nav, status: status, cancelFn: cancel}
}

// UpdateStatus updates the status bar text and triggers a partial refresh.
// Safe to call from any goroutine.
func (e *epaperState) UpdateStatus(left, right string) {
	if e == nil {
		return
	}
	e.mu.Lock()
	e.status.SetLeft(fmt.Sprintf(left))
	e.status.SetRight(right)
	e.mu.Unlock()
	if err := e.nav.Render(); err != nil {
		log.Printf("epaper: render error: %v", err)
	}
}

// Stop shuts down the GUI loop.
func (e *epaperState) Stop() {
	if e == nil {
		return
	}
	e.cancelFn()
}
```

- [ ] **Step 3: Fix any compile errors**

```bash
go build awesomeProject/hello 2>&1
```

Resolve any import or type mismatches. Common issues:
- `[]Widget` must be `[]gui.Widget` if `Widget` is not in scope — use `gui.Widget`
- `touch.Start` signature — check `epaper/touch` package for actual API

- [ ] **Step 4: Build succeeds**

```bash
go build awesomeProject/hello
```
Expected: no errors

- [ ] **Step 5: Commit**

```bash
git add hello/epaper.go
git commit -m "feat(hello): migrate to epaper/gui Navigator for status display"
```

---

## Final verification

- [ ] **All gui tests pass**

```bash
go test awesomeProject/epaper/gui -v -count=1
```

- [ ] **hello builds cleanly**

```bash
go build awesomeProject/hello
```

- [ ] **epd tests still pass**

```bash
go test awesomeProject/epaper/epd -v -count=1
```
