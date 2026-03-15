# GUI Widgets Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add 10 new widgets, an Icon system, reusable keyboard widget, 3 scene helpers, swipe gestures, and `DrawImageScaled` to the OiOni GUI framework.

**Architecture:** Widgets embed `BaseWidget`, implement `Widget` + optionally `Touchable`/`Stoppable`. Scene helpers (`ShowAlert`, `ShowMenu`, `ShowTextInput`) push composed Scenes onto the Navigator. The internal `keyboardWidget` is reusable across any scene. Swipe detection uses a `time.Timer` channel in the `Run()` select loop — no goroutines, no races.

**Tech Stack:** Go 1.26, `ui/gui` module, `ui/canvas` module, `rsc.io/qr v0.2.0` (already added to go.mod).

**Spec:** `docs/superpowers/specs/2026-03-15-gui-widgets-design.md`

**Run tests:** `cd ui/gui && go test ./... -v`
**Run canvas tests:** `cd ui/canvas && go test ./... -v`

---

## Chunk 1: Foundation

### Task 1: `DrawImageScaled` on canvas

**Files:**
- Modify: `ui/canvas/draw.go`
- Modify: `ui/canvas/canvas_test.go` (or create `ui/canvas/draw_scaled_test.go`)

- [ ] **Step 1: Write failing test**

Add to `ui/canvas/canvas_test.go`:
```go
func TestDrawImageScaled_2x2To4x4(t *testing.T) {
	// 2×2 source: top-left black, rest white
	src := image.NewGray(image.Rect(0, 0, 2, 2))
	src.SetGray(0, 0, color.Gray{Y: 0})   // black
	src.SetGray(1, 0, color.Gray{Y: 255}) // white
	src.SetGray(0, 1, color.Gray{Y: 255}) // white
	src.SetGray(1, 1, color.Gray{Y: 255}) // white

	c := New(4, 4, Rot0)
	c.Fill(White)
	c.DrawImageScaled(image.Rect(0, 0, 4, 4), src)

	img := c.ToImage()
	// top-left 2×2 should be black (scaled ×2)
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			if img.GrayAt(x, y).Y != 0 {
				t.Errorf("pixel (%d,%d) should be black", x, y)
			}
		}
	}
	// top-right pixel should be white
	if img.GrayAt(2, 0).Y == 0 {
		t.Error("pixel (2,0) should be white")
	}
}

func TestDrawImageScaled_EmptySource(t *testing.T) {
	c := New(10, 10, Rot0)
	// nil image — should not panic
	c.DrawImageScaled(image.Rect(0, 0, 10, 10), image.NewGray(image.Rect(0, 0, 0, 0)))
}
```

- [ ] **Step 2: Run test — expect failure**
```bash
cd ui/canvas && go test ./... -run TestDrawImageScaled -v
```
Expected: compile error (`DrawImageScaled undefined`).

- [ ] **Step 3: Implement `DrawImageScaled` in `draw.go`**

Add after `DrawImage`:
```go
// DrawImageScaled renders img scaled to fit r using nearest-neighbour resampling,
// then thresholds to 1-bit (luma < 128 → black). Letterboxed and centered.
// No-op if img bounds or r are empty.
func (c *Canvas) DrawImageScaled(r image.Rectangle, img image.Image) {
	sb := img.Bounds()
	if sb.Empty() || r.Empty() {
		return
	}
	// float64 scale so downscaling works correctly (s < 1)
	s := min(float64(r.Dx())/float64(sb.Dx()), float64(r.Dy())/float64(sb.Dy()))
	dw := int(float64(sb.Dx()) * s)
	dh := int(float64(sb.Dy()) * s)
	// center in r
	offX := r.Min.X + (r.Dx()-dw)/2
	offY := r.Min.Y + (r.Dy()-dh)/2

	for dy := 0; dy < dh; dy++ {
		for dx := 0; dx < dw; dx++ {
			srcX := sb.Min.X + clamp(int(float64(dx)/s), 0, sb.Dx()-1)
			srcY := sb.Min.Y + clamp(int(float64(dy)/s), 0, sb.Dy()-1)
			r32, g32, b32, _ := img.At(srcX, srcY).RGBA()
			luma := (19595*r32 + 38470*g32 + 7471*b32) >> 24
			if luma < 128 {
				c.SetPixel(offX+dx, offY+dy, Black)
			}
		}
	}
}

func clamp(v, lo, hi int) int {
	if v < lo { return lo }
	if v > hi { return hi }
	return v
}
```

> Note: `clamp` may already exist in canvas — check first; if so, skip the definition.

- [ ] **Step 4: Run tests — expect pass**
```bash
cd ui/canvas && go test ./... -v
```
Expected: all pass including `TestDrawImageScaled_*`.

- [ ] **Step 5: Commit**
```bash
git add ui/canvas/draw.go ui/canvas/canvas_test.go
git commit -m "feat(canvas): add DrawImageScaled — nearest-neighbour letterbox scale to 1-bit"
```

---

### Task 2: `Stoppable` + `scrollable` interfaces in `gui.go`

**Files:**
- Modify: `ui/gui/gui.go`

- [ ] **Step 1: Add interfaces** — append to `gui.go` after `Touchable`:
```go
// Stoppable is implemented by widgets that own background goroutines.
// Navigator.Pop() calls Stop() recursively on all widgets in a popped scene.
type Stoppable interface {
	Stop()
}

// scrollable is package-internal. Navigator.Run() calls Scroll on widgets
// that implement it when a SwipeUp or SwipeDown gesture is detected.
type scrollable interface {
	Scroll(dy int)
}
```

- [ ] **Step 2: Verify compile**
```bash
cd ui/gui && go build ./...
```
Expected: success.

- [ ] **Step 3: Commit**
```bash
git add ui/gui/gui.go
git commit -m "feat(gui): add Stoppable and scrollable interfaces"
```

---

### Task 3: `Children()` on layout containers + `stopWidgets` helper

**Files:**
- Modify: `ui/gui/layout.go`
- Modify: `ui/gui/navigator.go` (add `stopWidgets`)

- [ ] **Step 1: Add `Children()` to `box` (VBox/HBox)**

In `layout.go`, add to `box`:
```go
// Children returns the widgets contained in this box (unwrapped from layoutHint).
func (b *box) Children() []Widget {
	out := make([]Widget, 0, len(b.children))
	for _, ch := range b.children {
		out = append(out, ch.widget)
	}
	return out
}
```

- [ ] **Step 2: Add `Children()` to `Fixed`**
```go
func (f *Fixed) Children() []Widget {
	out := make([]Widget, 0, len(f.children))
	for _, ch := range f.children {
		out = append(out, ch.widget)
	}
	return out
}
```

- [ ] **Step 3: Add `Children()` to `Overlay`**
```go
func (o *Overlay) Children() []Widget { return []Widget{o.content} }
```

- [ ] **Step 4: Add `Children()` to `paddingWidget`**

Find the `paddingWidget` struct in `layout.go` and add:
```go
func (p *paddingWidget) Children() []Widget { return []Widget{p.inner} }
```

- [ ] **Step 5: Add `stopWidgets` to `navigator.go`**

Add before `Pop()`:
```go
// stopWidgets recursively calls Stop() on any widget implementing Stoppable,
// walking into layout containers via Children() []Widget.
func stopWidgets(widgets []Widget) {
	type hasChildren interface{ Children() []Widget }
	for _, w := range widgets {
		if s, ok := w.(Stoppable); ok {
			s.Stop()
		}
		if c, ok := w.(hasChildren); ok {
			stopWidgets(c.Children())
		}
	}
}
```

- [ ] **Step 6: Call `stopWidgets` in `Pop()`**

In `navigator.go`, inside `Pop()`, after the `OnLeave` call, add:
```go
stopWidgets(top.Widgets)
```

- [ ] **Step 7: Verify compile and existing tests pass**
```bash
cd ui/gui && go test ./... -v
```

- [ ] **Step 8: Commit**
```bash
git add ui/gui/layout.go ui/gui/navigator.go
git commit -m "feat(gui): Children() on layout containers + recursive stopWidgets in Pop()"
```

---

### Task 4: Swipe detection in `navigator.go`

**Files:**
- Modify: `ui/gui/navigator.go`
- Modify: `ui/gui/gui_test.go`

- [ ] **Step 1: Write swipe tests**

Add to `gui_test.go`:
```go
func TestNavigator_SwipeLeft_Pops(t *testing.T) {
	d := &fakeDisplay{}
	nav := NewNavigator(d)

	s1 := &Scene{Widgets: []Widget{NewLabel("one")}}
	s2 := &Scene{Widgets: []Widget{NewLabel("two")}}
	nav.Push(s1) //nolint
	nav.Push(s2) //nolint

	if len(nav.stack) != 2 {
		t.Fatalf("expected 2 scenes, got %d", len(nav.stack))
	}

	// Simulate swipe left: two touches 40px apart in X, within 300ms
	ctx, cancel := context.WithCancel(context.Background())
	events := make(chan touch.TouchEvent, 2)
	events <- touch.TouchEvent{Points: []touch.TouchPoint{{X: 100, Y: 60}}}
	events <- touch.TouchEvent{Points: []touch.TouchPoint{{X: 60, Y: 60}}} // ΔX=-40
	cancel() // stop Run after processing
	nav.Run(ctx, events)

	if len(nav.stack) != 1 {
		t.Errorf("expected 1 scene after swipe left, got %d", len(nav.stack))
	}
}

func TestNavigator_SlowTap_NotLost(t *testing.T) {
	d := &fakeDisplay{}
	nav := NewNavigator(d)
	tapped := false
	btn := NewButton("ok")
	btn.OnClick(func() { tapped = true })
	s := &Scene{Widgets: []Widget{btn}}
	nav.Push(s) //nolint
	btn.SetBounds(image.Rect(0, 0, 250, 122))

	ctx, cancel := context.WithCancel(context.Background())
	events := make(chan touch.TouchEvent, 1)
	// Single tap with no second event — timer should flush it
	events <- touch.TouchEvent{Points: []touch.TouchPoint{{X: 60, Y: 10}}}
	// Give timer (300ms) time to fire, then cancel
	go func() {
		time.Sleep(400 * time.Millisecond)
		cancel()
	}()
	nav.Run(ctx, events)

	if !tapped {
		t.Error("slow single tap should not be lost")
	}
}
```

- [ ] **Step 2: Run — expect failure**
```bash
cd ui/gui && go test ./... -run TestNavigator_Swipe -v
```

- [ ] **Step 3: Add swipe state to `Navigator` struct**

In `Navigator` struct definition, add:
```go
// swipe detection state (used only within Run goroutine — no lock needed)
swipePt    *touch.TouchPoint
swipeTimer *time.Timer
```

- [ ] **Step 4: Replace `Run()` with swipe-aware version**

```go
func (nav *Navigator) Run(ctx context.Context, events <-chan touch.TouchEvent) {
	var swipePt *touch.TouchPoint
	var swipeTimer *time.Timer
	timerCh := func() <-chan time.Time {
		if swipeTimer != nil {
			return swipeTimer.C
		}
		return nil
	}

	flush := func() {
		if swipePt != nil {
			nav.handleTouch(*swipePt)
			swipePt = nil
		}
		if swipeTimer != nil {
			swipeTimer.Stop()
			swipeTimer = nil
		}
	}

	for {
		select {
		case <-ctx.Done():
			flush()
			return

		case <-timerCh():
			// No second touch arrived within 300ms — treat buffered event as tap.
			if swipePt != nil {
				nav.handleTouch(*swipePt)
				swipePt = nil
			}
			swipeTimer = nil

		case ev, ok := <-events:
			if !ok {
				flush()
				return
			}
			for _, pt := range ev.Points {
				if swipePt == nil {
					// First touch: buffer, start timer.
					cp := pt
					swipePt = &cp
					swipeTimer = time.NewTimer(300 * time.Millisecond)
					continue
				}
				// Second touch within 300ms: classify.
				swipeTimer.Stop()
				swipeTimer = nil
				firstPt := *swipePt // save before clearing
				swipePt = nil
				// Cast to int — touch coords may be uint16, negative delta would wrap
				dx := int(pt.X) - int(firstPt.X)
				dy := int(pt.Y) - int(firstPt.Y)

				adx := dx
				if adx < 0 { adx = -adx }
				ady := dy
				if ady < 0 { ady = -ady }

				const threshold = 30
				if adx >= ady && adx > threshold {
					// Horizontal swipe
					if dx < 0 {
						nav.Pop() //nolint
					}
					// SwipeRight: reserved, no-op
				} else if ady > adx && ady > threshold {
					// Vertical swipe — route to scrollable top widget
					if len(nav.stack) > 0 {
						for _, w := range nav.stack[len(nav.stack)-1].Widgets {
							if s, ok := w.(scrollable); ok {
								if dy < 0 {
									s.Scroll(-1)
								} else {
									s.Scroll(1)
								}
								break
							}
						}
					}
				} else {
					// Not a swipe — deliver both touches as taps
					nav.handleTouch(firstPt)
					nav.handleTouch(pt)
				}
			}
			nav.Render() //nolint:errcheck
		}
	}
}
```

- [ ] **Step 5: Run tests**
```bash
cd ui/gui && go test ./... -v
```

- [ ] **Step 6: Commit**
```bash
git add ui/gui/navigator.go ui/gui/gui_test.go
git commit -m "feat(gui): swipe detection in Run() — SwipeLeft=Pop, SwipeUp/Down=scroll, timer-based tap flush"
```

---

## Chunk 2: Icon + Display Widgets

### Task 5: Icon system (`icon.go`)

**Files:**
- Create: `ui/gui/icon.go`
- Create: `ui/gui/icon_test.go`

- [ ] **Step 1: Write test**

Create `ui/gui/icon_test.go`:
```go
package gui

import (
	"image"
	"image/color"
	"testing"

	"github.com/oioio-space/oioni/ui/canvas"
)

func TestNewBitmapIcon_DrawsBlack(t *testing.T) {
	// 8×8 bitmap: all black (0x00 = all bits 0 = all black in e-ink convention)
	data := make([]byte, 8) // 8 rows × 1 byte/row for 8px wide
	ic := NewBitmapIcon(data, 8, 8)
	w, h := ic.Size()
	if w != 8 || h != 8 {
		t.Fatalf("Size() = (%d,%d), want (8,8)", w, h)
	}
	c := canvas.New(8, 8, canvas.Rot0)
	c.Fill(canvas.White)
	ic.Draw(c, image.Rect(0, 0, 8, 8))
	img := c.ToImage()
	if img.GrayAt(0, 0).Y != 0 {
		t.Error("expected black pixel at (0,0)")
	}
}

func TestNewImageIcon_DrawsThresholded(t *testing.T) {
	src := image.NewGray(image.Rect(0, 0, 4, 4))
	src.SetGray(0, 0, color.Gray{Y: 0}) // black
	ic := NewImageIcon(src)
	c := canvas.New(4, 4, canvas.Rot0)
	c.Fill(canvas.White)
	ic.Draw(c, image.Rect(0, 0, 4, 4))
	img := c.ToImage()
	if img.GrayAt(0, 0).Y != 0 {
		t.Error("expected black pixel from image icon")
	}
}
```

- [ ] **Step 2: Run — expect compile error**
```bash
cd ui/gui && go test ./... -run TestNewBitmapIcon -v
```

- [ ] **Step 3: Implement `icon.go`**

```go
package gui

import (
	"image"

	"github.com/oioio-space/oioni/ui/canvas"
)

// Icon is a renderable 1-bit image. Value type — copy-safe.
// Use NewBitmapIcon for compile-time assets, NewImageIcon for runtime images.
type Icon struct {
	w, h   int
	render func(c *canvas.Canvas, r image.Rectangle)
}

// NewBitmapIcon creates an icon from a packed 1-bit bitmap.
// Layout: MSB-first, ((w+7)/8) bytes per row × h rows.
// Bit=0 → black, bit=1 → white (e-ink convention, same as canvas buffer).
func NewBitmapIcon(data []byte, w, h int) Icon {
	// copy data to avoid aliasing
	d := make([]byte, len(data))
	copy(d, data)
	stride := (w + 7) / 8
	return Icon{w: w, h: h, render: func(c *canvas.Canvas, r image.Rectangle) {
		s := min(float64(r.Dx())/float64(w), float64(r.Dy())/float64(h))
		dw := int(float64(w) * s)
		dh := int(float64(h) * s)
		ox := r.Min.X + (r.Dx()-dw)/2
		oy := r.Min.Y + (r.Dy()-dh)/2
		for dy := 0; dy < dh; dy++ {
			srcY := int(float64(dy) / s)
			for dx := 0; dx < dw; dx++ {
				srcX := int(float64(dx) / s)
				byteIdx := srcY*stride + srcX/8
				bit := uint(7 - srcX%8)
				if byteIdx < len(d) && (d[byteIdx]>>bit)&1 == 0 {
					c.SetPixel(ox+dx, oy+dy, canvas.Black)
				} else {
					c.SetPixel(ox+dx, oy+dy, canvas.White)
				}
			}
		}
	}}
}

// NewImageIcon creates an icon from any image.Image.
// Thresholded to 1-bit via DrawImageScaled at draw time.
func NewImageIcon(img image.Image) Icon {
	b := img.Bounds()
	return Icon{w: b.Dx(), h: b.Dy(), render: func(c *canvas.Canvas, r image.Rectangle) {
		c.DrawImageScaled(r, img)
	}}
}

// Draw renders the icon scaled to fit r (letterboxed, centered).
func (ic Icon) Draw(c *canvas.Canvas, r image.Rectangle) {
	if ic.render == nil {
		return
	}
	ic.render(c, r)
}

// Size returns the icon's natural size in pixels.
func (ic Icon) Size() (w, h int) { return ic.w, ic.h }
```

- [ ] **Step 4: Run tests — expect pass**
```bash
cd ui/gui && go test ./... -v
```

- [ ] **Step 5: Commit**
```bash
git add ui/gui/icon.go ui/gui/icon_test.go
git commit -m "feat(gui): Icon type — NewBitmapIcon (1-bit packed) + NewImageIcon (image.Image)"
```

---

### Task 6: Toggle widget

**Files:**
- Create: `ui/gui/widget_toggle.go`
- Create: `ui/gui/widget_toggle_test.go`

- [ ] **Step 1: Write test**

```go
package gui

import (
	"image"
	"testing"
	"github.com/oioio-space/oioni/drivers/touch"
)

func TestToggle_InitialState(t *testing.T) {
	tog := NewToggle(true)
	if !tog.On {
		t.Error("expected initial On=true")
	}
	tog2 := NewToggle(false)
	if tog2.On {
		t.Error("expected initial On=false")
	}
}

func TestToggle_TapFlips(t *testing.T) {
	tog := NewToggle(false)
	tog.SetBounds(image.Rect(0, 0, 40, 20))
	tog.HandleTouch(touch.TouchPoint{X: 20, Y: 10})
	if !tog.On {
		t.Error("expected On=true after tap")
	}
	tog.HandleTouch(touch.TouchPoint{X: 20, Y: 10})
	if tog.On {
		t.Error("expected On=false after second tap")
	}
}

func TestToggle_OnChangeCalled(t *testing.T) {
	called := false
	var got bool
	tog := NewToggle(false)
	tog.OnChange = func(on bool) { called = true; got = on }
	tog.SetBounds(image.Rect(0, 0, 40, 20))
	tog.HandleTouch(touch.TouchPoint{X: 20, Y: 10})
	if !called {
		t.Error("OnChange not called")
	}
	if !got {
		t.Error("OnChange got false, want true")
	}
}

func TestToggle_DrawDoesNotPanic(t *testing.T) {
	c := newTestCanvas()
	tog := NewToggle(false)
	tog.SetBounds(image.Rect(0, 0, 40, 20))
	tog.Draw(c)
	tog.On = true
	tog.Draw(c)
}
```

- [ ] **Step 2: Run — compile error expected**

- [ ] **Step 3: Implement `widget_toggle.go`**

```go
package gui

import (
	"image"

	"github.com/oioio-space/oioni/ui/canvas"
	"github.com/oioio-space/oioni/drivers/touch"
)

// Toggle is an on/off switch widget.
type Toggle struct {
	BaseWidget
	On       bool
	OnChange func(bool)
}

func NewToggle(initial bool) *Toggle {
	t := &Toggle{On: initial}
	t.SetDirty()
	return t
}

func (t *Toggle) PreferredSize() image.Point { return image.Pt(40, 20) }
func (t *Toggle) MinSize() image.Point       { return image.Pt(40, 20) }

func (t *Toggle) Draw(c *canvas.Canvas) {
	r := t.Bounds()
	if r.Empty() {
		return
	}
	// Pill outline
	c.DrawRect(r, canvas.Black, false)
	// Knob: left half = OFF position, right half = ON position
	mid := r.Min.X + r.Dx()/2
	knob := image.Rect(r.Min.X+1, r.Min.Y+1, mid-1, r.Max.Y-1)
	if t.On {
		knob = image.Rect(mid+1, r.Min.Y+1, r.Max.X-1, r.Max.Y-1)
	}
	c.DrawRect(knob, canvas.Black, true)
	t.MarkClean()
}

func (t *Toggle) HandleTouch(_ touch.TouchPoint) bool {
	t.On = !t.On
	t.SetDirty()
	if t.OnChange != nil {
		t.OnChange(t.On)
	}
	return true
}
```

- [ ] **Step 4: Add `newTestCanvas` helper to `gui_test.go`** (if not already present):
```go
func newTestCanvas() *canvas.Canvas {
	return canvas.New(epd.Width, epd.Height, canvas.Rot90)
}
```

- [ ] **Step 5: Run — expect pass**
```bash
cd ui/gui && go test ./... -v
```

- [ ] **Step 6: Commit**
```bash
git add ui/gui/widget_toggle.go ui/gui/widget_toggle_test.go ui/gui/gui_test.go
git commit -m "feat(gui): Toggle widget — on/off switch with OnChange callback"
```

---

### Task 7: ImageWidget

**Files:**
- Create: `ui/gui/widget_image.go`
- Create: `ui/gui/widget_image_test.go`

- [ ] **Step 1: Write test**

```go
package gui

import (
	"image"
	"image/color"
	"testing"

	"github.com/oioio-space/oioni/ui/canvas"
)

func TestImageWidget_DrawDoesNotPanic(t *testing.T) {
	src := image.NewGray(image.Rect(0, 0, 10, 10))
	w := NewImageWidget(src)
	w.SetBounds(image.Rect(0, 0, 20, 20))
	c := newTestCanvas()
	w.Draw(c) // must not panic
}

func TestImageWidget_NilSafe(t *testing.T) {
	w := NewImageWidget(nil)
	w.SetBounds(image.Rect(0, 0, 20, 20))
	c := newTestCanvas()
	w.Draw(c) // must not panic
}

func TestImageWidget_SetImageRendersBlack(t *testing.T) {
	src := image.NewGray(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			src.SetGray(x, y, color.Gray{Y: 0}) // all black
		}
	}
	w := NewImageWidget(src)
	w.SetBounds(image.Rect(0, 0, 4, 4))
	// Use Rot0 canvas to avoid physical↔logical coordinate mapping confusion in test
	c := canvas.New(10, 10, canvas.Rot0)
	c.Fill(canvas.White)
	w.Draw(c)
	// At least one pixel in the 4×4 region should be black
	img := c.ToImage()
	found := false
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			if img.GrayAt(x, y).Y == 0 {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected at least one black pixel from all-black source image")
	}
}
```

- [ ] **Step 2: Implement `widget_image.go`**

```go
package gui

import (
	"image"

	"github.com/oioio-space/oioni/ui/canvas"
)

// ImageWidget renders an image.Image scaled to its bounds.
// No intrinsic size — must be given bounds via Expand or FixedSize in a layout.
type ImageWidget struct {
	BaseWidget
	img image.Image
}

func NewImageWidget(img image.Image) *ImageWidget {
	w := &ImageWidget{img: img}
	w.SetDirty()
	return w
}

func (w *ImageWidget) SetImage(img image.Image) {
	w.img = img
	w.SetDirty()
}

func (w *ImageWidget) PreferredSize() image.Point { return image.Point{} }
func (w *ImageWidget) MinSize() image.Point       { return image.Point{} }

func (w *ImageWidget) Draw(c *canvas.Canvas) {
	if w.img == nil || w.Bounds().Empty() {
		return
	}
	c.DrawImageScaled(w.Bounds(), w.img)
	w.MarkClean()
}
```

- [ ] **Step 3: Run + commit**
```bash
cd ui/gui && go test ./... -v
git add ui/gui/widget_image.go ui/gui/widget_image_test.go
git commit -m "feat(gui): ImageWidget — image.Image scaled to bounds via DrawImageScaled"
```

---

### Task 8: ClockWidget

**Files:**
- Create: `ui/gui/widget_clock.go`
- Create: `ui/gui/widget_clock_test.go`

- [ ] **Step 1: Write test**

```go
package gui

import (
	"image"
	"testing"
	"time"
)

func TestClock_DrawDoesNotPanic(t *testing.T) {
	clk := NewClock()
	clk.SetBounds(image.Rect(0, 0, 60, 24))
	c := newTestCanvas()
	clk.Draw(c)
	clk.Stop()
}

func TestClock_StopPreventsSetDirty(t *testing.T) {
	clk := NewClockFull()
	clk.SetBounds(image.Rect(0, 0, 80, 24))
	clk.MarkClean()
	clk.Stop()
	// After Stop, no more ticks should fire.
	time.Sleep(1200 * time.Millisecond)
	// IsDirty should remain false (no tick fired after Stop)
	if clk.IsDirty() {
		t.Error("ClockWidget set dirty after Stop() — goroutine not cancelled")
	}
}

func TestClock_ImplementsStoppable(t *testing.T) {
	clk := NewClock()
	var _ Stoppable = clk // compile-time check
	clk.Stop()
}
```

- [ ] **Step 2: Implement `widget_clock.go`**

```go
package gui

import (
	"context"
	"image"
	"time"

	"github.com/oioio-space/oioni/ui/canvas"
)

// ClockWidget displays the current time and auto-refreshes each minute (NewClock)
// or each second (NewClockFull). Implements Stoppable — Navigator.Pop() calls Stop().
type ClockWidget struct {
	BaseWidget
	format string
	cancel context.CancelFunc
}

func NewClock() *ClockWidget     { return newClock("15:04", time.Minute) }
func NewClockFull() *ClockWidget { return newClock("15:04:05", time.Second) }

func newClock(format string, interval time.Duration) *ClockWidget {
	ctx, cancel := context.WithCancel(context.Background())
	w := &ClockWidget{format: format, cancel: cancel}
	w.SetDirty()
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				w.SetDirty()
			}
		}
	}()
	return w
}

func (w *ClockWidget) Stop() { w.cancel() }

func (w *ClockWidget) PreferredSize() image.Point { return image.Pt(60, 24) }
func (w *ClockWidget) MinSize() image.Point       { return image.Pt(40, 16) }

func (w *ClockWidget) Draw(c *canvas.Canvas) {
	r := w.Bounds()
	if r.Empty() {
		return
	}
	text := time.Now().Format(w.format)
	f := canvas.EmbeddedFont(20)
	tw := 0
	for _, ch := range text {
		_, gw, _ := f.Glyph(ch)
		tw += gw
	}
	x := r.Min.X + (r.Dx()-tw)/2
	y := r.Min.Y + (r.Dy()-f.LineHeight())/2
	c.DrawRect(r, canvas.White, true) // clear background
	c.DrawText(x, y, text, f, canvas.Black)
	w.MarkClean()
}
```

- [ ] **Step 3: Run + commit**
```bash
cd ui/gui && go test ./... -v
git add ui/gui/widget_clock.go ui/gui/widget_clock_test.go
git commit -m "feat(gui): ClockWidget — auto-refresh HH:MM or HH:MM:SS, implements Stoppable"
```

---

### Task 9: QRCode widget

**Files:**
- Create: `ui/gui/widget_qrcode.go`
- Create: `ui/gui/widget_qrcode_test.go`

- [ ] **Step 1: Write test**

```go
package gui

import (
	"image"
	"testing"
)

func TestQRCode_DrawDoesNotPanic(t *testing.T) {
	q := NewQRCode("https://oioni.local")
	q.SetBounds(image.Rect(0, 0, 80, 80))
	c := newTestCanvas()
	q.Draw(c)
}

func TestQRCode_EmptyData(t *testing.T) {
	q := NewQRCode("")
	q.SetBounds(image.Rect(0, 0, 40, 40))
	c := newTestCanvas()
	q.Draw(c) // must not panic
}

func TestQRCode_SetDataRegenerates(t *testing.T) {
	q := NewQRCode("hello")
	q.SetBounds(image.Rect(0, 0, 60, 60))
	c1 := newTestCanvas()
	q.Draw(c1)
	b1 := make([]byte, len(c1.Bytes()))
	copy(b1, c1.Bytes())

	q.SetData("different content that produces a different QR code")
	c2 := newTestCanvas()
	q.Draw(c2)

	same := true
	for i := range b1 {
		if b1[i] != c2.Bytes()[i] {
			same = false
			break
		}
	}
	if same {
		t.Error("expected different QR code after SetData")
	}
}
```

- [ ] **Step 2: Implement `widget_qrcode.go`**

```go
package gui

import (
	"image"

	"github.com/oioio-space/oioni/ui/canvas"
	"rsc.io/qr"
)

// QRCode renders a QR code from a string. Matrix is cached and regenerated
// only when SetData is called.
type QRCode struct {
	BaseWidget
	data   string
	matrix [][]bool // true = black module
}

func NewQRCode(data string) *QRCode {
	q := &QRCode{}
	q.SetData(data)
	return q
}

func (q *QRCode) SetData(data string) {
	q.data = data
	q.matrix = nil
	if data != "" {
		if code, err := qr.Encode(data, qr.M); err == nil {
			n := code.Size
			q.matrix = make([][]bool, n)
			for y := 0; y < n; y++ {
				q.matrix[y] = make([]bool, n)
				for x := 0; x < n; x++ {
					q.matrix[y][x] = code.Black(x, y)
				}
			}
		}
	}
	q.SetDirty()
}

func (q *QRCode) PreferredSize() image.Point { return image.Pt(80, 80) }
func (q *QRCode) MinSize() image.Point       { return image.Pt(40, 40) }

func (q *QRCode) Draw(c *canvas.Canvas) {
	r := q.Bounds()
	if r.Empty() || len(q.matrix) == 0 {
		return
	}
	n := len(q.matrix)
	// scale: largest integer that fits (or float for small displays)
	scale := min(r.Dx(), r.Dy()) / n
	if scale < 1 {
		scale = 1
	}
	dw := n * scale
	dh := n * scale
	ox := r.Min.X + (r.Dx()-dw)/2
	oy := r.Min.Y + (r.Dy()-dh)/2

	// White background
	c.DrawRect(r, canvas.White, true)
	for y, row := range q.matrix {
		for x, black := range row {
			if black {
				px := image.Rect(ox+x*scale, oy+y*scale, ox+x*scale+scale, oy+y*scale+scale)
				c.DrawRect(px, canvas.Black, true)
			}
		}
	}
	q.MarkClean()
}
```

- [ ] **Step 3: Run + commit**
```bash
cd ui/gui && go test ./... -v
git add ui/gui/widget_qrcode.go ui/gui/widget_qrcode_test.go
git commit -m "feat(gui): QRCode widget — rsc.io/qr, cached matrix, scales to bounds"
```

---

### Task 10: ProgressArc widget

**Files:**
- Create: `ui/gui/widget_arc.go`
- Create: `ui/gui/widget_arc_test.go`

- [ ] **Step 1: Write test**

```go
package gui

import (
	"image"
	"testing"
)

func TestProgressArc_Clamps(t *testing.T) {
	a := NewProgressArc(0.5)
	a.SetProgress(1.5)
	if a.progress != 1.0 {
		t.Errorf("expected 1.0, got %f", a.progress)
	}
	a.SetProgress(-0.1)
	if a.progress != 0.0 {
		t.Errorf("expected 0.0, got %f", a.progress)
	}
}

func TestProgressArc_DrawDoesNotPanic(t *testing.T) {
	a := NewProgressArc(0.75)
	a.SetBounds(image.Rect(0, 0, 60, 60))
	c := newTestCanvas()
	a.Draw(c)
}

func TestProgressArc_ZeroAndOne(t *testing.T) {
	c := newTestCanvas()
	for _, p := range []float64{0, 1} {
		a := NewProgressArc(p)
		a.SetBounds(image.Rect(0, 0, 60, 60))
		a.Draw(c) // must not panic
	}
}
```

- [ ] **Step 2: Implement `widget_arc.go`**

```go
package gui

import (
	"fmt"
	"image"
	"math"

	"github.com/oioio-space/oioni/ui/canvas"
)

// ProgressArc displays a circular arc showing a progress value (0.0–1.0).
type ProgressArc struct {
	BaseWidget
	progress float64
}

func NewProgressArc(progress float64) *ProgressArc {
	a := &ProgressArc{}
	a.SetProgress(progress)
	return a
}

func (a *ProgressArc) SetProgress(v float64) {
	if v < 0 { v = 0 }
	if v > 1 { v = 1 }
	a.progress = v
	a.SetDirty()
}

func (a *ProgressArc) PreferredSize() image.Point { return image.Pt(60, 60) }
func (a *ProgressArc) MinSize() image.Point       { return image.Pt(40, 40) }

func (a *ProgressArc) Draw(c *canvas.Canvas) {
	r := a.Bounds()
	if r.Empty() {
		return
	}
	cx := r.Min.X + r.Dx()/2
	cy := r.Min.Y + r.Dy()/2
	radius := min(r.Dx(), r.Dy())/2 - 2

	// White background
	c.DrawRect(r, canvas.White, true)
	// Background circle outline
	c.DrawCircle(cx, cy, radius, canvas.Black, false)

	// Filled sector: iterate pixels within bounding square, test angle
	// Angle 0 = top (−π/2), increases clockwise
	end := a.progress * 2 * math.Pi
	for py := r.Min.Y; py < r.Max.Y; py++ {
		for px := r.Min.X; px < r.Max.X; px++ {
			dx := float64(px - cx)
			dy := float64(py - cy)
			dist := math.Sqrt(dx*dx + dy*dy)
			if dist > float64(radius) {
				continue
			}
			// atan2: 0=right, CCW positive. Shift so 0=top, CW positive.
			angle := math.Atan2(dy, dx) + math.Pi/2
			if angle < 0 {
				angle += 2 * math.Pi
			}
			if angle <= end {
				c.SetPixel(px, py, canvas.Black)
			}
		}
	}

	// Center label
	label := fmt.Sprintf("%d%%", int(a.progress*100))
	f := canvas.EmbeddedFont(12)
	tw := 0
	for _, ch := range label {
		_, gw, _ := f.Glyph(ch)
		tw += gw
	}
	lx := cx - tw/2
	ly := cy - f.LineHeight()/2
	c.DrawText(lx, ly, label, f, canvas.Black)

	a.MarkClean()
}
```

- [ ] **Step 3: Run + commit**
```bash
cd ui/gui && go test ./... -v
git add ui/gui/widget_arc.go ui/gui/widget_arc_test.go
git commit -m "feat(gui): ProgressArc — pixel-test sector fill, percentage label"
```

---

## Chunk 3: Input Widgets

### Task 11: Checkbox

**Files:**
- Create: `ui/gui/widget_checkbox.go`
- Create: `ui/gui/widget_checkbox_test.go`

- [ ] **Step 1: Write test**

```go
package gui

import (
	"image"
	"testing"
	"github.com/oioio-space/oioni/drivers/touch"
)

func TestCheckbox_TapToggles(t *testing.T) {
	cb := NewCheckbox("enable", false)
	cb.SetBounds(image.Rect(0, 0, 100, 20))
	cb.HandleTouch(touch.TouchPoint{X: 10, Y: 10})
	if !cb.Checked {
		t.Error("expected Checked=true after tap")
	}
	cb.HandleTouch(touch.TouchPoint{X: 10, Y: 10})
	if cb.Checked {
		t.Error("expected Checked=false after second tap")
	}
}

func TestCheckbox_OnChangeCalled(t *testing.T) {
	var got bool
	cb := NewCheckbox("x", false)
	cb.OnChange = func(v bool) { got = v }
	cb.SetBounds(image.Rect(0, 0, 60, 20))
	cb.HandleTouch(touch.TouchPoint{X: 10, Y: 10})
	if !got {
		t.Error("OnChange not called with true")
	}
}

func TestCheckbox_DrawDoesNotPanic(t *testing.T) {
	c := newTestCanvas()
	for _, checked := range []bool{false, true} {
		cb := NewCheckbox("label", checked)
		cb.SetBounds(image.Rect(0, 0, 100, 20))
		cb.Draw(c)
	}
}
```

- [ ] **Step 2: Implement `widget_checkbox.go`**

```go
package gui

import (
	"image"

	"github.com/oioio-space/oioni/ui/canvas"
	"github.com/oioio-space/oioni/drivers/touch"
)

// Checkbox is a labeled toggle with a visible check box.
type Checkbox struct {
	BaseWidget
	Label    string
	Checked  bool
	OnChange func(bool)
}

func NewCheckbox(label string, initial bool) *Checkbox {
	c := &Checkbox{Label: label, Checked: initial}
	c.SetDirty()
	return c
}

func (cb *Checkbox) PreferredSize() image.Point {
	f := canvas.EmbeddedFont(12)
	tw := 0
	for _, r := range cb.Label {
		_, gw, _ := f.Glyph(r)
		tw += gw
	}
	return image.Pt(tw+20+4, 20)
}
func (cb *Checkbox) MinSize() image.Point { return image.Pt(40, 20) }

func (cb *Checkbox) Draw(c *canvas.Canvas) {
	r := cb.Bounds()
	if r.Empty() {
		return
	}
	// Box: 14×14 at left edge, vertically centered
	boxSize := 14
	by := r.Min.Y + (r.Dy()-boxSize)/2
	box := image.Rect(r.Min.X+2, by, r.Min.X+2+boxSize, by+boxSize)
	c.DrawRect(box, canvas.Black, false)
	if cb.Checked {
		// X mark: two diagonals
		c.DrawLine(box.Min.X+2, box.Min.Y+2, box.Max.X-2, box.Max.Y-2, canvas.Black)
		c.DrawLine(box.Max.X-2, box.Min.Y+2, box.Min.X+2, box.Max.Y-2, canvas.Black)
	}
	// Label
	f := canvas.EmbeddedFont(12)
	ty := r.Min.Y + (r.Dy()-f.LineHeight())/2
	c.DrawText(r.Min.X+2+boxSize+4, ty, cb.Label, f, canvas.Black)
	cb.MarkClean()
}

func (cb *Checkbox) HandleTouch(_ touch.TouchPoint) bool {
	cb.Checked = !cb.Checked
	cb.SetDirty()
	if cb.OnChange != nil {
		cb.OnChange(cb.Checked)
	}
	return true
}
```

- [ ] **Step 3: Run + commit**
```bash
cd ui/gui && go test ./... -v
git add ui/gui/widget_checkbox.go ui/gui/widget_checkbox_test.go
git commit -m "feat(gui): Checkbox widget — tap to toggle, X mark when checked"
```

---

### Task 12: Slider

**Files:**
- Create: `ui/gui/widget_slider.go`
- Create: `ui/gui/widget_slider_test.go`

- [ ] **Step 1: Write test**

```go
package gui

import (
	"image"
	"testing"
	"github.com/oioio-space/oioni/drivers/touch"
)

func TestSlider_SetValueClamps(t *testing.T) {
	s := NewSlider(0, 100, 1)
	s.SetValue(150)
	if s.Value() != 100 {
		t.Errorf("expected 100, got %f", s.Value())
	}
	s.SetValue(-10)
	if s.Value() != 0 {
		t.Errorf("expected 0, got %f", s.Value())
	}
}

func TestSlider_SetValueSnapsToStep(t *testing.T) {
	s := NewSlider(0, 10, 2.5)
	s.SetValue(3.0)
	// nearest step: 2.5
	if s.Value() != 2.5 {
		t.Errorf("expected 2.5, got %f", s.Value())
	}
}

func TestSlider_TapSetsValue(t *testing.T) {
	s := NewSlider(0, 100, 1)
	s.SetBounds(image.Rect(0, 0, 100, 24))
	// Tap at x=50 out of width 100 → value = 50
	s.HandleTouch(touch.TouchPoint{X: 50, Y: 12})
	if s.Value() != 50 {
		t.Errorf("expected 50, got %f", s.Value())
	}
}

func TestSlider_OnChangeCalled(t *testing.T) {
	var got float64
	s := NewSlider(0, 100, 1)
	s.SetBounds(image.Rect(0, 0, 100, 24))
	s.OnChange = func(v float64) { got = v }
	s.HandleTouch(touch.TouchPoint{X: 75, Y: 12})
	if got != 75 {
		t.Errorf("OnChange got %f, want 75", got)
	}
}

func TestSlider_DrawDoesNotPanic(t *testing.T) {
	s := NewSlider(0, 100, 1)
	s.SetValue(42)
	s.SetBounds(image.Rect(0, 0, 120, 24))
	c := newTestCanvas()
	s.Draw(c)
}
```

- [ ] **Step 2: Implement `widget_slider.go`**

```go
package gui

import (
	"fmt"
	"image"
	"math"

	"github.com/oioio-space/oioni/ui/canvas"
	"github.com/oioio-space/oioni/drivers/touch"
)

// Slider is a horizontal value picker. Tap anywhere on the bar to set the value.
type Slider struct {
	BaseWidget
	Min, Max, Step float64
	OnChange       func(float64)
	value          float64
}

func NewSlider(min, max, step float64) *Slider {
	s := &Slider{Min: min, Max: max, Step: step, value: min}
	s.SetDirty()
	return s
}

func (s *Slider) Value() float64 { return s.value }

func (s *Slider) SetValue(v float64) {
	// snap to step
	if s.Step > 0 {
		v = math.Round(v/s.Step) * s.Step
	}
	if v < s.Min { v = s.Min }
	if v > s.Max { v = s.Max }
	s.value = v
	s.SetDirty()
}

func (s *Slider) PreferredSize() image.Point { return image.Pt(120, 24) }
func (s *Slider) MinSize() image.Point       { return image.Pt(80, 24) }

func (s *Slider) Draw(c *canvas.Canvas) {
	r := s.Bounds()
	if r.Empty() {
		return
	}
	c.DrawRect(r, canvas.White, true)

	// Track line at vertical center
	midY := r.Min.Y + r.Dy()/2
	c.DrawLine(r.Min.X+4, midY, r.Max.X-4, midY, canvas.Black)

	// Thumb
	ratio := 0.0
	if s.Max > s.Min {
		ratio = (s.value - s.Min) / (s.Max - s.Min)
	}
	thumbX := r.Min.X + 4 + int(ratio*float64(r.Dx()-8))
	thumbW := 6
	thumb := image.Rect(thumbX-thumbW/2, r.Min.Y+2, thumbX+thumbW/2, r.Max.Y-2)
	c.DrawRect(thumb, canvas.Black, true)

	// Value label above thumb
	f := canvas.EmbeddedFont(8)
	label := fmt.Sprintf("%.0f", s.value)
	tw := 0
	for _, ch := range label {
		_, gw, _ := f.Glyph(ch)
		tw += gw
	}
	lx := thumbX - tw/2
	if lx < r.Min.X { lx = r.Min.X }
	if lx+tw > r.Max.X { lx = r.Max.X - tw }
	c.DrawText(lx, r.Min.Y, label, f, canvas.Black)
	s.MarkClean()
}

func (s *Slider) HandleTouch(pt touch.TouchPoint) bool {
	r := s.Bounds()
	if r.Empty() || r.Dx() <= 8 {
		return false
	}
	ratio := float64(int(pt.X)-r.Min.X-4) / float64(r.Dx()-8)
	if ratio < 0 { ratio = 0 }
	if ratio > 1 { ratio = 1 }
	v := s.Min + ratio*(s.Max-s.Min)
	s.SetValue(v)
	if s.OnChange != nil {
		s.OnChange(s.value)
	}
	return true
}
```

- [ ] **Step 3: Run + commit**
```bash
cd ui/gui && go test ./... -v
git add ui/gui/widget_slider.go ui/gui/widget_slider_test.go
git commit -m "feat(gui): Slider widget — tap-to-set, step snapping, OnChange callback"
```

---

### Task 13: Menu widget

**Files:**
- Create: `ui/gui/widget_menu.go`
- Create: `ui/gui/widget_menu_test.go`

- [ ] **Step 1: Write test**

```go
package gui

import (
	"image"
	"testing"
	"github.com/oioio-space/oioni/drivers/touch"
)

func TestMenu_TapSelectsItem(t *testing.T) {
	selected := ""
	items := []MenuItem{
		{Label: "Alpha", OnSelect: func() { selected = "alpha" }},
		{Label: "Beta",  OnSelect: func() { selected = "beta" }},
	}
	m := NewMenu(items)
	// Each item is 20px tall
	m.SetBounds(image.Rect(0, 0, 100, 40))
	// Tap on second item (y=25 → row index 1)
	m.HandleTouch(touch.TouchPoint{X: 50, Y: 25})
	if selected != "beta" {
		t.Errorf("expected 'beta', got %q", selected)
	}
}

func TestMenu_ScrollClampsOffset(t *testing.T) {
	items := make([]MenuItem, 10)
	for i := range items {
		items[i] = MenuItem{Label: "item"}
	}
	m := NewMenu(items)
	m.SetBounds(image.Rect(0, 0, 100, 60)) // shows 3 items
	m.Scroll(100) // clamp to max
	// max offset = 10 - 3 = 7
	if m.offset > 7 {
		t.Errorf("offset %d exceeds max 7", m.offset)
	}
	m.Scroll(-100)
	if m.offset != 0 {
		t.Errorf("offset %d below 0", m.offset)
	}
}

func TestMenu_DrawDoesNotPanic(t *testing.T) {
	items := []MenuItem{
		{Label: "A"},
		{Label: "B"},
		{Label: "C"},
	}
	m := NewMenu(items)
	m.SetBounds(image.Rect(0, 0, 100, 60))
	c := newTestCanvas()
	m.Draw(c)
}
```

- [ ] **Step 2: Implement `widget_menu.go`**

```go
package gui

import (
	"image"

	"image/color"

	"github.com/oioio-space/oioni/ui/canvas"
	"github.com/oioio-space/oioni/drivers/touch"
)

const menuRowHeight = 20
const menuIconSize  = 20

// MenuItem is one entry in a Menu. Icon is optional.
type MenuItem struct {
	Label    string
	Icon     *Icon
	OnSelect func()
}

// Menu is a scrollable list of items. Implements scrollable (for swipe gestures).
type Menu struct {
	BaseWidget
	Items    []MenuItem
	offset   int // index of first visible item
	selected int // index of last tapped item; -1 = none
}

func NewMenu(items []MenuItem) *Menu {
	m := &Menu{Items: items, selected: -1}
	m.SetDirty()
	return m
}

func (m *Menu) PreferredSize() image.Point {
	return image.Pt(80, len(m.Items)*menuRowHeight)
}
func (m *Menu) MinSize() image.Point { return image.Pt(40, menuRowHeight) }

func (m *Menu) visibleRows() int {
	r := m.Bounds()
	if r.Empty() {
		return 0
	}
	return r.Dy() / menuRowHeight
}

func (m *Menu) Scroll(dy int) {
	maxOff := len(m.Items) - m.visibleRows()
	if maxOff < 0 {
		maxOff = 0
	}
	m.offset += dy
	if m.offset < 0 {
		m.offset = 0
	}
	if m.offset > maxOff {
		m.offset = maxOff
	}
	m.SetDirty()
}

func (m *Menu) Draw(c *canvas.Canvas) {
	r := m.Bounds()
	if r.Empty() {
		return
	}
	c.DrawRect(r, canvas.White, true)
	f := canvas.EmbeddedFont(12)
	rows := m.visibleRows()

	for i := 0; i < rows; i++ {
		idx := m.offset + i
		if idx >= len(m.Items) {
			break
		}
		item := m.Items[idx]
		rowRect := image.Rect(r.Min.X, r.Min.Y+i*menuRowHeight,
			r.Max.X, r.Min.Y+(i+1)*menuRowHeight)

		if idx == m.selected {
			// Inverted background for selected row
			c.DrawRect(rowRect, canvas.Black, true)
			var fg color.Color = canvas.White
			m.drawItemContent(c, item, rowRect, f, fg)
		} else {
			c.DrawRect(rowRect, canvas.Black, false) // row border
			m.drawItemContent(c, item, rowRect, f, canvas.Black)
		}
	}

	// Scroll indicator (2px right bar) if content overflows
	if len(m.Items) > rows && rows > 0 {
		barH := r.Dy() * rows / len(m.Items)
		barY := r.Min.Y + r.Dy()*m.offset/len(m.Items)
		bar := image.Rect(r.Max.X-2, barY, r.Max.X, barY+barH)
		c.DrawRect(bar, canvas.Black, true)
	}
	m.MarkClean()
}

func (m *Menu) drawItemContent(c *canvas.Canvas, item MenuItem, row image.Rectangle, f canvas.Font, col color.Color) {
	x := row.Min.X + 4
	if item.Icon != nil {
		iconR := image.Rect(x, row.Min.Y+2, x+menuIconSize-2, row.Max.Y-2)
		item.Icon.Draw(c, iconR)
		x += menuIconSize + 2
	}
	ty := row.Min.Y + (row.Dy()-f.LineHeight())/2
	c.DrawText(x, ty, item.Label, f, col)
}

func (m *Menu) HandleTouch(pt touch.TouchPoint) bool {
	r := m.Bounds()
	if r.Empty() {
		return false
	}
	row := (int(pt.Y) - r.Min.Y) / menuRowHeight
	idx := m.offset + row
	if idx < 0 || idx >= len(m.Items) {
		return false
	}
	m.selected = idx
	m.SetDirty()
	if m.Items[idx].OnSelect != nil {
		m.Items[idx].OnSelect()
	}
	return true
}
```

- [ ] **Step 3: Run + commit**
```bash
cd ui/gui && go test ./... -v
git add ui/gui/widget_menu.go ui/gui/widget_menu_test.go
git commit -m "feat(gui): Menu widget — scrollable list, Icon support, scroll indicator"
```

---

## Chunk 4: Scene Helpers

### Task 14: `keyboardWidget` (internal)

**Files:**
- Create: `ui/gui/widget_keyboard.go`
- Create: `ui/gui/widget_keyboard_test.go`

- [ ] **Step 1: Write test**

```go
package gui

import (
	"image"
	"testing"
	"github.com/oioio-space/oioni/drivers/touch"
)

func TestKeyboard_TapFiresOnKey(t *testing.T) {
	var got rune
	kb := newKeyboard(keyboardConfig{
		Rows:   []string{"ABC", "DEF"},
		MaxLen: 10,
		OnKey:  func(r rune) { got = r },
	})
	// 3 cols, 2 rows, give it 90×40 px
	kb.SetBounds(image.Rect(0, 0, 90, 40))
	// Tap at x=15, y=10 → col 0, row 0 → 'A'
	kb.HandleTouch(touch.TouchPoint{X: 15, Y: 10})
	if got != 'A' {
		t.Errorf("expected 'A', got %q", got)
	}
}

func TestKeyboard_OnBackCalled(t *testing.T) {
	called := false
	kb := newKeyboard(keyboardConfig{
		Rows:   []string{"AB"},
		MaxLen: 10,
		OnBack: func() { called = true },
	})
	kb.SetBounds(image.Rect(0, 0, 250, 20))
	// [⌫] is in the header row — keyboard widget itself doesn't draw header.
	// So OnBack is called when the caller invokes kb.Back() directly.
	kb.Back()
	if !called {
		t.Error("OnBack not called")
	}
}

func TestKeyboard_DrawDoesNotPanic(t *testing.T) {
	kb := newKeyboard(defaultKeyboardConfig(10, nil, nil, nil, nil))
	kb.SetBounds(image.Rect(0, 0, 250, 100))
	c := newTestCanvas()
	kb.Draw(c)
}
```

- [ ] **Step 2: Implement `widget_keyboard.go`**

```go
package gui

import (
	"image"
	"unicode/utf8"

	"github.com/oioio-space/oioni/ui/canvas"
	"github.com/oioio-space/oioni/drivers/touch"
)

// keyboardConfig configures the internal keyboard widget.
type keyboardConfig struct {
	Rows      []string  // each string = characters for that row
	MaxLen    int       // max text length; 0 = unlimited
	Current   func() int // returns current text length (for MaxLen check)
	OnKey     func(rune)
	OnBack    func()
	OnConfirm func()
}

// defaultKeyboardConfig returns a standard alphanumeric layout.
func defaultKeyboardConfig(maxLen int, current func() int, onKey func(rune), onBack, onConfirm func()) keyboardConfig {
	return keyboardConfig{
		Rows: []string{
			"ABCDEFGHIJ",
			"KLMNOPQRST",
			"UVWXYZ!@#$",
			"0123456789",
			"_-./:=?+( ",
		},
		MaxLen:    maxLen,
		Current:   current,
		OnKey:     onKey,
		OnBack:    onBack,
		OnConfirm: onConfirm,
	}
}

// keyboardWidget is a package-internal reusable keyboard grid widget.
type keyboardWidget struct {
	BaseWidget
	cfg keyboardConfig
}

func newKeyboard(cfg keyboardConfig) *keyboardWidget {
	kb := &keyboardWidget{cfg: cfg}
	kb.SetDirty()
	return kb
}

// Back calls OnBack directly (used by header row [⌫] button).
func (kb *keyboardWidget) Back() {
	if kb.cfg.OnBack != nil {
		kb.cfg.OnBack()
	}
}

// Confirm calls OnConfirm directly (used by header row [OK] button).
func (kb *keyboardWidget) Confirm() {
	if kb.cfg.OnConfirm != nil {
		kb.cfg.OnConfirm()
	}
}

func (kb *keyboardWidget) PreferredSize() image.Point { return image.Pt(250, 100) }
func (kb *keyboardWidget) MinSize() image.Point       { return image.Pt(100, 40) }

func (kb *keyboardWidget) keySize() (keyW, keyH int) {
	r := kb.Bounds()
	if r.Empty() || len(kb.cfg.Rows) == 0 {
		return 1, 1
	}
	maxCols := 0
	for _, row := range kb.cfg.Rows {
		n := utf8.RuneCountInString(row)
		if n > maxCols {
			maxCols = n
		}
	}
	if maxCols == 0 {
		maxCols = 1
	}
	return r.Dx() / maxCols, r.Dy() / len(kb.cfg.Rows)
}

func (kb *keyboardWidget) Draw(c *canvas.Canvas) {
	r := kb.Bounds()
	if r.Empty() {
		return
	}
	c.DrawRect(r, canvas.White, true)
	keyW, keyH := kb.keySize()
	f := canvas.EmbeddedFont(12)

	atMax := kb.cfg.MaxLen > 0 && kb.cfg.Current != nil && kb.cfg.Current() >= kb.cfg.MaxLen

	for row, chars := range kb.cfg.Rows {
		for col, ch := range chars {
			x := r.Min.X + col*keyW
			y := r.Min.Y + row*keyH
			keyR := image.Rect(x, y, x+keyW, y+keyH)
			// Border
			c.DrawRect(keyR, canvas.Black, false)
			// Label — gray-ish (outline only) when at max
			label := string(ch)
			if ch == ' ' {
				label = "SP"
			}
			_, gw, _ := f.Glyph(ch)
			tx := x + (keyW-gw)/2
			ty := y + (keyH-f.LineHeight())/2
			if atMax && ch != ' ' {
				// Disabled: draw text in white on white background (faded)
				c.DrawText(tx, ty, label, f, canvas.White)
			} else {
				c.DrawText(tx, ty, label, f, canvas.Black)
			}
		}
	}
	kb.MarkClean()
}

func (kb *keyboardWidget) HandleTouch(pt touch.TouchPoint) bool {
	r := kb.Bounds()
	if r.Empty() {
		return false
	}
	keyW, keyH := kb.keySize()
	if keyW == 0 || keyH == 0 {
		return false
	}
	col := (int(pt.X) - r.Min.X) / keyW
	row := (int(pt.Y) - r.Min.Y) / keyH
	if row < 0 || row >= len(kb.cfg.Rows) {
		return false
	}
	runes := []rune(kb.cfg.Rows[row])
	if col < 0 || col >= len(runes) {
		return false
	}
	ch := runes[col]
	// Check max
	if kb.cfg.MaxLen > 0 && kb.cfg.Current != nil && kb.cfg.Current() >= kb.cfg.MaxLen {
		return true // consumed but no-op
	}
	if kb.cfg.OnKey != nil {
		kb.cfg.OnKey(ch)
	}
	return true
}
```

- [ ] **Step 3: Fix `defaultKeyboardConfig` signature** — the `onBack, onConfirm` parameters are separate; adjust call sites accordingly.

- [ ] **Step 4: Run + commit**
```bash
cd ui/gui && go test ./... -v
git add ui/gui/widget_keyboard.go ui/gui/widget_keyboard_test.go
git commit -m "feat(gui): keyboardWidget — internal reusable keyboard grid, dynamic key sizing"
```

---

### Task 15: `widget_alert.go` + `widget_textinput.go`

**Files:**
- Create: `ui/gui/widget_alert.go`
- Create: `ui/gui/widget_textinput.go`

These files define types used by `helpers.go`. No logic here.

- [ ] **Step 1: Create `widget_alert.go`**

```go
package gui

// AlertButton is a button shown in a ShowAlert dialog.
type AlertButton struct {
	Label   string
	OnPress func()
}
```

- [ ] **Step 2: Create `widget_textinput.go`**

```go
package gui

// textInputState holds the mutable state of a ShowTextInput scene.
type textInputState struct {
	text     []rune
	maxLen   int
}

func (s *textInputState) append(r rune) {
	if s.maxLen > 0 && len(s.text) >= s.maxLen {
		return
	}
	if r == ' ' {
		s.text = append(s.text, ' ')
	} else {
		s.text = append(s.text, r)
	}
}

func (s *textInputState) backspace() {
	if len(s.text) > 0 {
		s.text = s.text[:len(s.text)-1]
	}
}

func (s *textInputState) String() string { return string(s.text) }
func (s *textInputState) Len() int       { return len(s.text) }
```

- [ ] **Step 3: Compile check**
```bash
cd ui/gui && go build ./...
```

- [ ] **Step 4: Commit**
```bash
git add ui/gui/widget_alert.go ui/gui/widget_textinput.go
git commit -m "feat(gui): AlertButton type + textInputState helper"
```

---

### Task 16: `helpers.go` — ShowAlert, ShowMenu, ShowTextInput

**Files:**
- Create: `ui/gui/helpers.go`
- Create: `ui/gui/helpers_test.go`

- [ ] **Step 1: Write test**

```go
package gui

import (
	"image"
	"testing"

	"github.com/oioio-space/oioni/drivers/touch"
)

func TestShowAlert_PushesScene(t *testing.T) {
	d := &fakeDisplay{}
	nav := NewNavigator(d)
	nav.Push(&Scene{Widgets: []Widget{NewLabel("base")}}) //nolint

	ShowAlert(nav, "Title", "Message")
	if len(nav.stack) != 2 {
		t.Fatalf("expected 2 scenes after ShowAlert, got %d", len(nav.stack))
	}
}

func TestShowAlert_OKButtonPops(t *testing.T) {
	d := &fakeDisplay{}
	nav := NewNavigator(d)
	nav.Push(&Scene{Widgets: []Widget{NewLabel("base")}}) //nolint

	var pressed bool
	ShowAlert(nav, "T", "M", AlertButton{
		Label:   "OK",
		OnPress: func() { pressed = true },
	})
	// Find the button and tap it
	scene := nav.stack[len(nav.stack)-1]
	tapAll(scene.Widgets)

	if !pressed {
		t.Error("OnPress not called")
	}
}

func TestShowMenu_PushesScene(t *testing.T) {
	d := &fakeDisplay{}
	nav := NewNavigator(d)
	nav.Push(&Scene{Widgets: []Widget{NewLabel("base")}}) //nolint
	items := []MenuItem{{Label: "A"}, {Label: "B"}}
	ShowMenu(nav, "My Menu", items)
	if len(nav.stack) != 2 {
		t.Fatalf("expected 2 scenes after ShowMenu, got %d", len(nav.stack))
	}
}

func TestShowTextInput_PushesScene(t *testing.T) {
	d := &fakeDisplay{}
	nav := NewNavigator(d)
	nav.Push(&Scene{Widgets: []Widget{NewLabel("base")}}) //nolint
	ShowTextInput(nav, "enter text", 20, func(s string) {})
	if len(nav.stack) != 2 {
		t.Fatalf("expected 2 scenes after ShowTextInput, got %d", len(nav.stack))
	}
}

// tapAll recursively taps all Touchable widgets at their center.
func tapAll(widgets []Widget) {
	type hasChildren interface{ Children() []Widget }
	for _, w := range widgets {
		if t, ok := w.(Touchable); ok {
			r := w.Bounds()
			if !r.Empty() {
				pt := touch.TouchPoint{
					X: uint16(r.Min.X + r.Dx()/2),
					Y: uint16(r.Min.Y + r.Dy()/2),
				}
				t.HandleTouch(pt)
			}
		}
		if c, ok := w.(hasChildren); ok {
			tapAll(c.Children())
		}
	}
}
```

- [ ] **Step 2: Implement `helpers.go`**

```go
package gui

import (
	"image"

	"github.com/oioio-space/oioni/drivers/epd"
	"github.com/oioio-space/oioni/ui/canvas"
)

// ShowAlert pushes a modal alert scene with a title, message, and buttons.
// If no buttons are provided, a single "OK" button that pops the scene is added.
// Call nav.Pop() from button handlers to dismiss.
func ShowAlert(nav *Navigator, title, message string, buttons ...AlertButton) {
	if len(buttons) == 0 {
		buttons = []AlertButton{{Label: "OK"}}
	}

	// Ensure each button pops after its callback
	btns := make([]AlertButton, len(buttons))
	copy(btns, buttons)
	for i := range btns {
		orig := btns[i].OnPress
		btns[i].OnPress = func() {
			if orig != nil {
				orig()
			}
			nav.Pop() //nolint
		}
	}

	titleLbl := NewLabel(title)
	titleLbl.SetFont(canvas.EmbeddedFont(12))
	titleLbl.SetAlign(AlignCenter)

	msgLbl := NewLabel(message)
	msgLbl.SetFont(canvas.EmbeddedFont(8))
	msgLbl.SetAlign(AlignCenter)

	btnWidgets := make([]any, 0, len(btns))
	for _, ab := range btns {
		ab := ab
		btn := NewButton(ab.Label)
		btn.OnClick(ab.OnPress)
		btnWidgets = append(btnWidgets, btn)
	}

	content := NewVBox(
		FixedSize(titleLbl, 20),
		Expand(msgLbl),
		FixedSize(NewHBox(btnWidgets...), 24),
	)

	ov := NewOverlay(content, AlignCenter)
	ov.setScreen(epd.Height, epd.Width) // logical screen after Rot90: 250×122

	_ = nav.Push(&Scene{Widgets: []Widget{ov}})
}

// ShowMenu pushes a scrollable menu scene with an optional title StatusBar.
func ShowMenu(nav *Navigator, title string, items []MenuItem) {
	menu := NewMenu(items)

	var top Widget
	if title != "" {
		top = NewVBox(FixedSize(NewStatusBar(title, ""), 16), Expand(menu))
	} else {
		top = NewVBox(Expand(menu))
	}
	// SetBounds required: Navigator does not set bounds automatically.
	top.SetBounds(image.Rect(0, 0, epd.Height, epd.Width))

	scene := &Scene{
		Widgets: []Widget{top},
		OnEnter: func() { menu.SetDirty() },
	}
	_ = nav.Push(scene)
}

// ShowTextInput pushes a keyboard scene. onConfirm is called with the entered
// string when the user taps [OK]. Swipe left cancels without calling onConfirm.
func ShowTextInput(nav *Navigator, placeholder string, maxLen int, onConfirm func(string)) {
	state := &textInputState{maxLen: maxLen}

	// Header label showing current text
	header := newTextInputHeader(state, placeholder)

	kb := newKeyboard(defaultKeyboardConfig(
		maxLen,
		state.Len,
		func(r rune) {
			state.append(r)
			header.refresh(state, placeholder)
		},
		func() {
			state.backspace()
			header.refresh(state, placeholder)
		},
		func() {
			onConfirm(state.String())
			nav.Pop() //nolint
		},
	))

	// Header row: text display (Expand) + [⌫] + [OK]
	backBtn := NewButton("⌫")
	backBtn.OnClick(func() { kb.Back() })
	okBtn := NewButton("OK")
	okBtn.OnClick(func() { kb.Confirm() })

	headerRow := NewHBox(
		Expand(header),
		FixedSize(backBtn, 30),
		FixedSize(okBtn, 30),
	)

	vbox := NewVBox(
		FixedSize(headerRow, 22),
		Expand(kb),
	)
	// SetBounds required: Navigator does not set bounds automatically.
	vbox.SetBounds(image.Rect(0, 0, epd.Height, epd.Width))

	scene := &Scene{Widgets: []Widget{vbox}}
	_ = nav.Push(scene)
}

// textInputHeader is a Label that shows the current text + cursor.
type textInputHeader struct {
	*Label
}

func newTextInputHeader(state *textInputState, placeholder string) *textInputHeader {
	text := placeholder
	if state.Len() > 0 {
		text = state.String() + "│"
	}
	h := &textInputHeader{Label: NewLabel(text)}
	h.SetFont(canvas.EmbeddedFont(12))
	return h
}

func (h *textInputHeader) refresh(state *textInputState, placeholder string) {
	if state.Len() == 0 {
		h.SetText(placeholder)
		return
	}
	text := state.String() + "│"
	// Truncate from left if overflowing (keep last N chars)
	maxRunes := 18
	runes := []rune(text)
	if len(runes) > maxRunes {
		runes = runes[len(runes)-maxRunes:]
	}
	h.SetText(string(runes))
}
```

> Note: remove the `strings` import if unused after implementation; it's there as a placeholder. Check imports before running.

- [ ] **Step 3: Add missing `touch` import to `helpers_test.go`**

The `tapAll` helper needs `touch.TouchPoint`. Make sure the import is present:
```go
import (
    "testing"
    "github.com/oioio-space/oioni/drivers/touch"
)
```

- [ ] **Step 4: Run**
```bash
cd ui/gui && go test ./... -v
```
Fix any compile errors (unused imports, type mismatches). Common issues:
- `canvas.Color` → use `color.Color` from `image/color`
- `defaultKeyboardConfig` signature mismatch

- [ ] **Step 5: Commit**
```bash
git add ui/gui/helpers.go ui/gui/helpers_test.go ui/gui/widget_alert.go ui/gui/widget_textinput.go
git commit -m "feat(gui): ShowAlert / ShowMenu / ShowTextInput helpers + textInputHeader"
```

---

### Task 17: Fix gokrazy config + final push

**Files:**
- Check/Modify: `oioio/config.json`

- [ ] **Step 1: Check if config still references `awesomeProject/hello`**
```bash
grep -r "awesomeProject" oioio/ 2>/dev/null
```

- [ ] **Step 2: If found, update the package path** in `oioio/config.json`:
Replace `awesomeProject/hello` → `github.com/oioio-space/oioni/cmd/oioni`

- [ ] **Step 3: Run all tests across the workspace**
```bash
go test ./drivers/epd/... ./drivers/touch/... ./drivers/usbgadget/... ./system/storage/... ./ui/canvas/... ./ui/gui/... 2>&1
```
Expected: all pass.

- [ ] **Step 4: Final commit**
```bash
git add oioio/config.json  # if changed
git add ui/gui/go.mod ui/gui/go.sum
git commit -m "chore: fix gokrazy config package path + go.sum for rsc.io/qr"
```

- [ ] **Step 5: Push**
```bash
# Push using stored credentials (gh auth or SSH key)
git push origin master
```
