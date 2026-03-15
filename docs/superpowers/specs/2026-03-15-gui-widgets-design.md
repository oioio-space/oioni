# GUI Widgets — Design Spec

**Date:** 2026-03-15
**Module:** `ui/gui` + `ui/canvas`
**Status:** Approved (post-review)

---

## Context

The existing `ui/gui` package provides a retained-mode GUI framework for the Waveshare 2.13" Touch e-Paper HAT (250×122 px, 1-bit, touch via GT1151). Current widgets: Label, Button, ProgressBar, StatusBar, Spacer, Divider. Layout: VBox, HBox, Fixed, Overlay.

This spec adds 10 new widgets + 3 scene helpers + swipe gesture support + one canvas primitive.

---

## Approach

**B — Widgets + Scene helpers**: each widget is standalone; complex interactions (Alert, Menu, TextInput) are exposed via `Show*` helper functions that manage push/pop automatically.

---

## Canvas Change (prerequisite)

**`ui/canvas/draw.go`** gains one new method:

```go
// DrawImageScaled renders img scaled to fill r using nearest-neighbour, then
// thresholds to 1-bit. The image is letterboxed (aspect ratio preserved, centered).
func (c *Canvas) DrawImageScaled(r image.Rectangle, img image.Image)
```

Implementation: compute scale factor `s = min(float64(r.Dx())/float64(img.Bounds().Dx()), float64(r.Dy())/float64(img.Bounds().Dy()))` (float64, handles both upscale and downscale). Iterate destination pixels within `r`; map to source via `src_x = int((dst_x - offset_x) / s)`, clamp to source bounds. Threshold luma at 50%. If source image is empty or r is empty, no-op.
`ImageWidget` calls this method. The existing `DrawImage(pt, img)` (native size, no scale) is unchanged.

---

## New Files

```
ui/gui/
├── widget_toggle.go
├── widget_image.go
├── widget_clock.go
├── widget_qrcode.go
├── widget_arc.go
├── widget_checkbox.go
├── widget_slider.go
├── widget_menu.go
├── widget_alert.go      ← defines AlertButton type only
├── widget_textinput.go
└── helpers.go           ← ShowAlert / ShowMenu / ShowTextInput (all Show* functions here)
```

`ui/gui/gui.go` gains the `Stoppable` and `scrollable` interfaces.
`ui/gui/navigator.go` modified: `Pop()` calls `Stop()` on stoppable widgets; `Run()` gains swipe detection.
`ui/gui/go.mod` gains `rsc.io/qr v0.2.0`.

---

## Lifecycle Interfaces (in `gui.go`)

```go
// Stoppable is implemented by widgets that own background goroutines.
// Navigator.Pop() calls Stop() on every widget in the popped scene
// that implements this interface.
type Stoppable interface {
    Stop()
}

// scrollable is package-internal. Navigator.Run() calls Scroll on the
// top-level widget of the current scene if it implements this interface,
// on SwipeUp / SwipeDown events. Not exposed to callers.
type scrollable interface {
    Scroll(dy int)
}
```

---

## Widget Designs

### Toggle (`widget_toggle.go`)

```go
type Toggle struct {
    BaseWidget
    On       bool
    OnChange func(bool)
}
func NewToggle(initial bool) *Toggle
```

- Draw: pill shape (full bounds). Left half darker = OFF knob, right half = ON knob. Active side filled.
- Touch: tap anywhere flips `On`, calls `OnChange`, marks dirty.
- MinSize: (40, 20).

---

### ImageWidget (`widget_image.go`)

```go
type ImageWidget struct {
    BaseWidget
    // unexported: img image.Image
}
func NewImageWidget(img image.Image) *ImageWidget
func (w *ImageWidget) SetImage(img image.Image)
```

- Draw: calls `canvas.DrawImageScaled(w.Bounds(), img)` — letterboxed, centered, nearest-neighbour.
- No touch handling.
- MinSize: (1, 1).

---

### ClockWidget (`widget_clock.go`)

```go
type ClockWidget struct {
    BaseWidget
    // unexported: format string, cancel context.CancelFunc
}
func NewClock() *ClockWidget       // "15:04"
func NewClockFull() *ClockWidget   // "15:04:05"
func (w *ClockWidget) Stop()       // implements Stoppable
```

- Internal goroutine launched in constructor, cancelled by `Stop()` via `context.CancelFunc`.
- Tick interval: 1 minute for `NewClock()`, 1 second for `NewClockFull()`.
- On tick: `SetDirty()`. Navigator.Run() handles repaint.
- `Navigator.Pop()` automatically calls `Stop()` (Stoppable interface).
- Draw: centered text using `EmbeddedFont(20)`.
- MinSize: (60, 24).

---

### QRCode (`widget_qrcode.go`)

```go
type QRCode struct {
    BaseWidget
    // unexported: data string, matrix [][]bool (cached, regenerated on SetData)
}
func NewQRCode(data string) *QRCode
func (w *QRCode) SetData(data string)
```

- Uses `rsc.io/qr v0.2.0`, Level M error correction.
- Draw: compute `scale = min(bounds.Dx(), bounds.Dy()) / len(matrix)`. Iterate matrix cells, call `canvas.SetPixel` for each pixel in the scaled cell. Quiet zone = 2 modules (included in matrix by rsc.io/qr). Centered in bounds.
- Matrix regenerated only on `SetData` — cached until then.
- MinSize: (40, 40).

---

### ProgressArc (`widget_arc.go`)

```go
type ProgressArc struct {
    BaseWidget
    // unexported: progress float64 (0.0–1.0)
}
func NewProgressArc(progress float64) *ProgressArc
func (w *ProgressArc) SetProgress(v float64) // clamps to [0,1]
```

- Draw: center = bounds center, radius = `min(bounds.Dx(), bounds.Dy())/2 - 2`.
  1. Draw full-circle outline via `canvas.DrawCircle(cx, cy, r, Black, false)`.
  2. Fill sector: iterate every pixel `(px, py)` in the bounding box. Compute `angle = atan2(py-cy, px-cx)`. Normalise to `[0, 2π)` starting from −π/2 (top). If `angle < progress*2π` and `sqrt((px-cx)²+(py-cy)²) <= r`, set pixel Black.
  3. Center label: `fmt.Sprintf("%d%%", int(progress*100))` via `DrawText`, `EmbeddedFont(12)`, centered.
- MinSize: (40, 40).

---

### Checkbox (`widget_checkbox.go`)

```go
type Checkbox struct {
    BaseWidget
    Label    string
    Checked  bool
    OnChange func(bool)
}
func NewCheckbox(label string, initial bool) *Checkbox
```

- Draw: 14×14 px outlined box at left edge + label text. When checked: draw X inside (two diagonal lines).
- Touch: tap anywhere in bounds toggles `Checked`, calls `OnChange`, marks dirty.
- MinSize: (60, 20).

---

### Slider (`widget_slider.go`)

```go
type Slider struct {
    BaseWidget
    Min, Max, Step float64
    OnChange       func(float64)
    // unexported: value float64
}
func NewSlider(min, max, step float64) *Slider
func (s *Slider) Value() float64
func (s *Slider) SetValue(v float64) // clamps to [Min,Max], snaps to Step
```

- Draw: horizontal line at vertical center of bounds. Thumb = 6×bounds.Dy() filled rect at `x = bounds.Min.X + (value-Min)/(Max-Min) * bounds.Dx()`. Value text right-aligned, `EmbeddedFont(8)`, above thumb.
- Touch: `tap.X` → `value = Min + ((tap.X - bounds.Min.X) / bounds.Dx()) * (Max - Min)`, snapped to Step, clamped. Calls `OnChange`. Swipe events do NOT update slider (bypass via separate code path — no debounce conflict).
- MinSize: (80, 24).

---

### Menu (`widget_menu.go`)

```go
type MenuItem struct {
    Label    string
    OnSelect func()
}

type Menu struct {
    BaseWidget
    Items []MenuItem
    // unexported: offset int, selected int (default -1 = none)
}
func NewMenu(items []MenuItem) *Menu
func (m *Menu) Scroll(dy int) // implements scrollable; clamps offset to valid range
```

- Draw: rows of 20px height. Row `i` = `Items[offset+i]`. If `selected == offset+i`: draw row background filled (inverted). Scroll indicator: 2px bar on right edge, height proportional to visible/total ratio, positioned proportionally.
- Touch (tap): compute row index from `tap.Y`, set `selected = offset+rowIdx`, call `Items[selected].OnSelect()`, marks dirty.
- `Scroll(dy)`: `offset = clamp(offset+dy, 0, max(0, len(Items)-visibleRows))`.
- Swipe up/down from Navigator propagated via `scrollable` interface.
- MinSize: (80, 20).

---

### Alert (`widget_alert.go`)

Defines types only — `ShowAlert` lives in `helpers.go`.

```go
type AlertButton struct {
    Label   string
    OnPress func()
}
```

No widget struct — Alert is composed from existing primitives in `ShowAlert`.

---

### TextInput Scene (`widget_textinput.go`)

Not a widget — implementation of the keyboard scene used by `ShowTextInput` (in `helpers.go`).

**Layout — exactly 122px tall, 250px wide:**

```
┌──────────────────────────────────────────┐
│ hello_                     [⌫]    [OK]  │  22px  ← text + action row
├──────────────────────────────────────────┤
│  A   B   C   D   E   F   G   H   I   J  │  20px
│  K   L   M   N   O   P   Q   R   S   T  │  20px
│  U   V   W   X   Y   Z   !   @   #   $  │  20px
│  0   1   2   3   4   5   6   7   8   9  │  20px
│  _   -   .   /   :   =   ?   +   (   )  │  20px
└──────────────────────────────────────────┘
Total: 22 + 5×20 = 122px ✓
Keys per row: 10, each 25×20px ✓ (at touch minimum)
Space: accessed via long-press on any key (appends " ") OR dedicated row 5 key swap via a [SPC] button replacing last row when a shift-like mode is active — TBD in implementation, simplest is: row 5 has 9 symbols + [SPC] occupying the 10th cell.
```

Row 5 final layout (10 cells exactly):
```
_   -   .   /   :   =   ?   +   (   [SPC]
```
`(` and `)` each occupy one cell. `[SPC]` replaces `)` — parentheses reduced to one.

- Text display: current string + `│` cursor. Left-truncated if overflows 250px.
- `[⌫]`: removes last char, marks dirty.
- `[OK]`: calls `onConfirm(text)`, then `nav.Pop()`.
- Swipe left = cancel: `nav.Pop()` without calling `onConfirm`.
- `maxLen` enforced: when `len(text) >= maxLen`, all character keys are no-ops (drawn with outline only, not filled).
- Implements `Stoppable` (no-op Stop; satisfies interface for consistency).

---

## Helpers (`helpers.go`)

All `Show*` functions live here — no duplicates elsewhere.

```go
func ShowAlert(nav *Navigator, title, message string, buttons ...AlertButton)
func ShowMenu(nav *Navigator, title string, items []MenuItem)
func ShowTextInput(nav *Navigator, placeholder string, maxLen int, onConfirm func(string))
```

**ShowAlert implementation:**
```go
// 1. Build content: VBox(titleLabel, msgLabel, HBox(buttons...))
// 2. Wrap in Overlay{AlignCenter}
// 3. Call overlay.setScreen(250, 122)   ← package-internal, accessible from helpers.go
// 4. nav.Push(&Scene{Widgets: []Widget{overlay}})
// Buttons each call their OnPress then nav.Pop().
```

**ShowMenu:** wraps `NewMenu(items)` in a Scene with optional `StatusBar` title. Swipe left → `nav.Pop()`.

**ShowTextInput:** constructs the keyboard scene defined in `widget_textinput.go`, calls `nav.Push`.

---

## Navigator Changes (`navigator.go`)

### Pop() — Stoppable cleanup (recursive)

```go
func (nav *Navigator) Pop() error {
    // existing stack logic ...
    stopWidgets(poppedScene.Widgets)
    // trigger full refresh ...
}

// stopWidgets recursively walks the widget tree and calls Stop() on any
// widget implementing Stoppable. This handles ClockWidget nested in VBox/HBox.
func stopWidgets(widgets []Widget) {
    for _, w := range widgets {
        if s, ok := w.(Stoppable); ok {
            s.Stop()
        }
        // Recurse into layout containers that expose their children.
        if c, ok := w.(interface{ Children() []Widget }); ok {
            stopWidgets(c.Children())
        }
    }
}
```

`VBox`, `HBox`, `Fixed`, `Overlay`, and `paddingWidget` (returned by `WithPadding`) each gain a package-internal `Children() []Widget` method returning their child slice (unwrapping `layoutHint` or single-child wrappers as needed). This enables recursive cleanup without exposing internal layout state to callers.

### Run() — Swipe detection

State machine entirely within the `Run()` goroutine — all swipe state is single-threaded. The timeout is implemented via a `time.Timer` whose channel is selected alongside the touch event channel, not via `time.AfterFunc` (which fires in a new goroutine and would race on shared state).

```go
swipeStart   *touch.TouchPoint  // nil = no pending first event
swipeStartAt time.Time
swipeTimer   *time.Timer        // nil when no first event buffered
```

On each `TouchEvent` in `Run()`:
1. If `swipeStart == nil`: record position + `time.Now()`. Buffer the event. Start `swipeTimer = time.NewTimer(300ms)`. Do NOT route to `handleTouch` yet.
2. `Run()` select statement:
   ```go
   select {
   case evt := <-touchEvents: // second event
       swipeTimer.Stop()
       // → classify (see below)
   case <-swipeTimer.C: // timeout, no second event
       handleTouch(bufferedEvent) // flush as tap
       swipeStart = nil
   case <-ctx.Done():
       return
   }
   ```
3. On second event (timer cancelled):
   - Compute `ΔX = p2.X - swipeStart.X`, `ΔY = p2.Y - swipeStart.Y`.
   - Classify dominant axis: `if abs(ΔX) >= abs(ΔY)` → horizontal, else vertical.
   - If dominant displacement > 30px → swipe. Do NOT call `handleTouch` for either event.
     - SwipeLeft → `nav.Pop()` (bypasses per-widget debounce — intentional, documented here).
     - SwipeUp → call `scrollable.Scroll(-1)` on top-level widget if it implements `scrollable`.
     - SwipeDown → call `scrollable.Scroll(+1)` on top-level widget if it implements `scrollable`.
     - SwipeRight → no-op.
   - If displacement ≤ 30px → not a swipe: flush buffered first event to `handleTouch`, then route second event to `handleTouch` normally.
3. Reset `swipeStart = nil` after classification.

The 300ms timer ensures a slow single tap is never lost even if no second touch event arrives.

Swipe events explicitly bypass the per-widget debounce map. This is intentional and documented here.

---

## Dependency

```
ui/gui/go.mod:
    require rsc.io/qr v0.2.0
```

Verify availability: `go get rsc.io/qr@v0.2.0` before implementation.

---

## Tests

| Widget | Test focus |
|--------|-----------|
| Toggle | Tap flips state; OnChange called once per tap |
| Checkbox | Tap toggles; Draw produces different Bytes() before/after |
| Slider | SetValue clamps; tap.X maps correctly to value; Step snapping |
| Menu | Scroll clamps at boundaries; selected tracks last tap |
| ClockWidget | Stop() cancels goroutine (no further SetDirty after Stop) |
| ProgressArc | SetProgress clamps [0,1]; Bytes() non-empty at 0 and 1 |
| QRCode | SetData regenerates matrix; nil-safe on empty string |
| ImageWidget | SetImage nil-safe; Draw with nil img = no-op |
| DrawImageScaled | Pixel output for known 2×2 source scaled to 4×4 |
| ShowAlert | Pushes scene; button pops it; no ShowAlert duplicate |
| ShowTextInput | OK calls onConfirm + pops; swipe left pops without onConfirm |
| Swipe detection | ΔX > 30 → SwipeLeft; ΔY > 30 → SwipeUp; both < 30 → tap pass-through |

All tests via `canvas.ToImage()` + pixel assertions or mock Navigator. No hardware required.

---

## Out of Scope

- Drag gesture on Slider (tap-to-set only).
- Multi-line TextInput.
- Animated transitions.
- `SwipeRight` action (reserved).
- AZERTY / locale-specific keyboard layout.
