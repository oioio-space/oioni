# GUI Widgets — Design Spec

**Date:** 2026-03-15
**Module:** `ui/gui`
**Status:** Approved

---

## Context

The existing `ui/gui` package provides a retained-mode GUI framework for the Waveshare 2.13" Touch e-Paper HAT (250×122 px, 1-bit, touch via GT1151). Current widgets: Label, Button, ProgressBar, StatusBar, Spacer, Divider. Layout: VBox, HBox, Fixed, Overlay.

This spec adds 10 new widgets + 3 scene helpers + swipe gesture support.

---

## Approach

**B — Widgets + Scene helpers**: each widget is standalone; complex interactions (Alert, Menu, TextInput) are exposed via `Show*` helper functions that manage push/pop automatically. No new Navigator state machine required.

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
├── widget_alert.go
├── widget_textinput.go
└── helpers.go
```

`ui/gui/navigator.go` modified to add swipe detection.
`ui/gui/go.mod` gains `rsc.io/qr` dependency.

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

- Draw: pill shape (full-width × height). Left half = knob when OFF, right half when ON. Filled = active side.
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

- Draw: calls `canvas.DrawImage` scaled to fit bounds (letterbox, centered).
- No touch handling.
- MinSize: (1, 1) — caller controls size via layout.

---

### ClockWidget (`widget_clock.go`)

```go
type ClockWidget struct {
    BaseWidget
    // unexported: format string, ticker, cancel
}
func NewClock() *ClockWidget           // HH:MM format
func NewClockFull() *ClockWidget       // HH:MM:SS format
func (w *ClockWidget) Stop()
```

- Internal goroutine: `time.NewTicker(1*time.Minute)` for HH:MM, `1*time.Second` for HH:MM:SS.
- On tick: calls `SetDirty()` — Navigator.Run() handles the repaint.
- Draw: centered text using `EmbeddedFont(20)`.
- Stop() cancels the goroutine (call on scene OnLeave).
- MinSize: (60, 24).

---

### QRCode (`widget_qrcode.go`)

```go
type QRCode struct {
    BaseWidget
    // unexported: data string, matrix [][]bool
}
func NewQRCode(data string) *QRCode
func (w *QRCode) SetData(data string)
```

- Uses `rsc.io/qr` to generate QR matrix (Level M error correction).
- Draw: scales module size to `min(bounds.W, bounds.H) / numModules`, renders via `canvas.SetPixel`. Centered in bounds. Quiet zone = 2 modules.
- Regenerates matrix only when data changes (cached).
- MinSize: (40, 40).

---

### ProgressArc (`widget_arc.go`)

```go
type ProgressArc struct {
    BaseWidget
    // unexported: progress float64
}
func NewProgressArc(progress float64) *ProgressArc  // 0.0–1.0
func (w *ProgressArc) SetProgress(v float64)
```

- Draw: background circle (outline) + filled arc from top (−90°) clockwise to `progress*360°`.
- Implemented via `canvas.DrawCircle` + pixel-by-pixel arc fill (polar coordinates, 1° steps).
- Center label: percentage text `"75%"` using `EmbeddedFont(12)`.
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

- Draw: 14×14 px box on left + label text. When checked: X drawn inside box.
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
func (s *Slider) SetValue(v float64)
```

- Draw: full-width horizontal line + 6px-wide filled thumb rectangle centered at value position. Value text right-aligned above thumb.
- Touch: `tap.X` position within bounds maps linearly to [Min, Max], snapped to Step. Calls `OnChange`. No drag required (tap-to-set).
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
    // unexported: offset int (first visible item index)
}
func NewMenu(items []MenuItem) *Menu
```

- Draw: items rendered as rows of fixed height (20px). Selected item (last tapped) = inverted. Scroll indicator: 2px bar on right edge if `len(Items)*20 > bounds.H`.
- Touch (tap): selects item, calls `OnSelect`.
- Scroll (Scrollable interface): `Scroll(dy int)` adjusts `offset`.
- Swipe up/down in Navigator propagated to Menu via `Scrollable`.
- MinSize: (80, 20).

**Scrollable interface** (internal to `ui/gui`):
```go
type scrollable interface {
    Scroll(dy int)
}
```

---

### Alert (`widget_alert.go`)

No new widget type — Alert composes existing primitives.

```go
type AlertButton struct {
    Label   string
    OnPress func()
}
func ShowAlert(nav *Navigator, title, message string, buttons ...AlertButton)
```

- Pushes a Scene containing an `Overlay{AlignCenter}` with:
  - Border rect (2px)
  - Title: `Label` with `EmbeddedFont(12)`, bold via inversion
  - Message: `Label` with `EmbeddedFont(8)`, wrapped to width
  - Buttons: `HBox` of `Button` widgets, each calling its `OnPress` then `nav.Pop()`
- If no buttons provided: single "OK" button that calls `nav.Pop()`.

---

### TextInput Scene (`widget_textinput.go`)

Not a widget — a full Scene pushed onto the Navigator stack.

```go
func ShowTextInput(nav *Navigator, placeholder string, maxLen int, onConfirm func(string))
```

**Layout (250×122 px):**

```
┌──────────────────────────────────────────┐
│ hello_                        [⌫]  [OK] │  24px  (text display row)
├──────────────────────────────────────────┤
│ A  B  C  D  E  F  G  H  I  J            │  20px
│ K  L  M  N  O  P  Q  R  S  T            │  20px
│ U  V  W  X  Y  Z  !  @  #  $            │  20px
│ 0  1  2  3  4  5  6  7  8  9            │  20px
│ _  -  .  /  :  =  ?  +  ( )  [SPC]      │  20px (SPC = 2 cells wide)
└──────────────────────────────────────────┘
```

- 10 columns × 5 rows = 50 touch targets, each 25×20 px (at touch minimum).
- Text display: current string + `_` cursor. Truncated left if > width.
- `[⌫]` removes last character. `[OK]` calls `onConfirm(text)` then `nav.Pop()`.
- Swipe left = cancel (`nav.Pop()` without calling onConfirm).
- `maxLen` enforced: keys disabled (drawn gray-ish via outline-only) when reached.

---

### Helpers (`helpers.go`)

```go
func ShowAlert(nav *Navigator, title, message string, buttons ...AlertButton)
func ShowMenu(nav *Navigator, title string, items []MenuItem)
func ShowTextInput(nav *Navigator, placeholder string, maxLen int, onConfirm func(string))
```

`ShowMenu` wraps a `Menu` widget in a Scene with optional title `StatusBar`. Swipe left = `nav.Pop()`.

---

## Navigator Changes (`navigator.go`)

**Swipe detection:**
- Track last touch event (position + timestamp).
- On new touch: if Δt < 300ms and |ΔX| > 30 or |ΔY| > 30 → classify as swipe.
- SwipeLeft: `nav.Pop()`.
- SwipeUp / SwipeDown: find top-level widget in current scene implementing `scrollable`; call `Scroll(±1)`.
- SwipeRight: no-op (reserved).

---

## Dependency

```
ui/gui/go.mod:
    require rsc.io/qr v1.0.0
```

Pure Go, no CGo, ~400 lines. Used only in `widget_qrcode.go`.

---

## Tests

Each widget gets a test in `ui/gui/gui_test.go` or a dedicated `*_test.go`:

- Toggle: tap flips state, OnChange called.
- Checkbox: tap toggles, draws correctly.
- Slider: SetValue clamps to [Min,Max], tap.X maps correctly.
- Menu: Scroll adjusts offset, wraps at boundaries.
- ClockWidget: SetDirty called after tick (mock ticker).
- ProgressArc: SetProgress clamps to [0,1], Bytes() non-empty.
- QRCode: SetData regenerates matrix, Draw doesn't panic.
- ImageWidget: SetImage nil-safe.
- Alert/Menu/TextInput: ShowAlert pushes scene, buttons pop it.

Hardware-dependent draws are verified via `canvas.ToImage()` pixel assertions.

---

## Out of Scope

- Drag gesture on Slider (tap-to-set only — e-ink refresh too slow for smooth drag).
- Multi-line TextInput (single line, maxLen enforced).
- Animated transitions between scenes.
- Font rendering for TextInput beyond `EmbeddedFont(12)`.
