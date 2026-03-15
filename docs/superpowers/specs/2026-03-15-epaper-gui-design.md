# epaper/gui — Design Spec

**Date:** 2026-03-15
**Package:** `awesomeProject/epaper/gui`
**Display:** Waveshare 2.13" Touch e-Paper HAT (EPD_2in13_V4 + GT1151)
**Logical size:** 250×122 px (landscape, canvas.Rot90)
**Colors:** 1-bit black/white only

---

## 1. Goals

Build a versatile GUI library for the e-ink touchscreen following GTK/Qt API conventions:
- Declarative widget construction (`NewButton`, `NewLabel`, etc.) with setter methods and callbacks
- Two-pass layout engine (measure + allocate) — no manual pixel calculation required
- Smart automatic refresh: partial for widget updates, full for scene transitions, with manual override
- Extensible: built-in widgets + user-defined custom widgets via `Widget` interface
- Navigation stack (Flipper Zero / e-reader paradigm): push/pop scenes and overlays
- Automatic touch routing with hit-testing and debounce
- No external dependencies — imports only `epaper/canvas`, `epaper/epd`, `epaper/touch`

---

## 2. Architecture

Five layers, each with one responsibility:

```
Navigator                 ← scene stack, touch routing, Run() event loop
  └── RefreshManager      ← dirty tracking, full/partial decision
        └── Scene         ← ordered widget tree, layout root, gesture handlers
              └── Widget  ← interface: Draw / Bounds / HandleTouch
                    └── canvas.Canvas  ← existing 1-bit drawing surface
```

### File layout

| File | Responsibility |
|------|---------------|
| `gui.go` | `Widget` interface, `Touchable` interface, `BaseWidget` embed, `Display` interface |
| `layout.go` | `HBox`, `VBox`, `Fixed`, `Overlay`, layout hint types |
| `widgets.go` | `Label`, `Button`, `ProgressBar`, `StatusBar`, `Spacer`, `Divider` |
| `refresh.go` | `RefreshManager`: dirty tracking, full/partial strategy |
| `navigator.go` | `Navigator`: scene stack, touch coordinate mapping, debounce, `Run()` |
| `gui_test.go` | Unit tests with fake display and fake touch channel |

---

## 3. Display Interface

`Navigator` depends on a `Display` interface (not the concrete `*epd.Display`) to enable testing without hardware:

```go
// Display is the subset of *epd.Display used by Navigator.
// *epd.Display satisfies this interface.
type Display interface {
    Init(m epd.Mode) error
    DisplayBase(buf []byte) error    // full refresh (writes 0x24 + 0x26 RAM banks)
    DisplayPartial(buf []byte) error // partial refresh (full buffer, self-contained init)
    DisplayFast(buf []byte) error    // fast full refresh
    Sleep() error
    Close() error
}
```

**`DisplayFull` is intentionally excluded.** It writes only to the `0x24` RAM bank without updating the `0x26` reference frame, so subsequent `DisplayPartial` calls would ghost. `Navigator` never calls `DisplayFull` — always use `DisplayBase` for full refreshes. App code that holds a `*epd.Display` directly can still call `DisplayFull` for one-shot renders that will never be followed by partial updates, but this is outside the Navigator's responsibility.

**Display lifetime:** `Navigator` does not own the display. The caller creates the `*epd.Display`, passes it to `NewNavigator`, and is responsible for calling `Close()` after `Run()` returns (or after the last `Render()` call if `Run()` is not used). The Navigator calls `Sleep()` on shutdown (at the end of `Run()`) but not `Close()`.

---

## 4. Widget Interface

Every widget (built-in or custom) implements:

```go
type Widget interface {
    Draw(c *canvas.Canvas)
    Bounds() image.Rectangle
    SetBounds(r image.Rectangle)
    PreferredSize() image.Point
    MinSize() image.Point
    IsDirty() bool
    SetDirty()
}

// Touchable is optional — interactive widgets implement this.
type Touchable interface {
    HandleTouch(pt touch.TouchPoint) bool // true = event consumed
}
```

### BaseWidget

Provides bookkeeping so custom widgets only need to implement `Draw`, `PreferredSize`, and `MinSize`:

```go
type BaseWidget struct {
    bounds image.Rectangle
    dirty  bool
}

func (b *BaseWidget) Bounds() image.Rectangle     { return b.bounds }
func (b *BaseWidget) SetBounds(r image.Rectangle) { b.bounds = r; b.dirty = true }
func (b *BaseWidget) IsDirty() bool               { return b.dirty }
func (b *BaseWidget) SetDirty()                   { b.dirty = true }
func (b *BaseWidget) markClean()                  { b.dirty = false }
// PreferredSize and MinSize return zero; custom widgets override them.
func (b *BaseWidget) PreferredSize() image.Point  { return image.Pt(0, 0) }
func (b *BaseWidget) MinSize() image.Point        { return image.Pt(0, 0) }
```

### MinSize enforcement

The layout engine enforces a minimum allocation of `MinSize()` per widget. For widgets that also implement `Touchable`, the layout engine enforces an additional floor of 20×20 px (minimum reliable touch target). Custom widgets that do NOT implement `Touchable` have no automatic floor — `BaseWidget.MinSize()` returning `(0,0)` is correct for non-interactive widgets (Spacer, Divider).

### Custom widget example

```go
type MyWidget struct {
    gui.BaseWidget
    value int
}

func (w *MyWidget) PreferredSize() image.Point { return image.Pt(60, 20) }
func (w *MyWidget) MinSize() image.Point       { return image.Pt(20, 20) }
func (w *MyWidget) SetValue(v int)             { w.value = v; w.SetDirty() }
func (w *MyWidget) Draw(c *canvas.Canvas) {
    r := w.Bounds()
    c.DrawRect(r, canvas.Black, false)
    f := canvas.EmbeddedFont(12) // EmbeddedFont returns nil for unsupported sizes; Draw must guard against nil
    if f != nil {
        c.DrawText(r.Min.X+2, r.Min.Y+2, fmt.Sprintf("%d", w.value), f, canvas.Black)
    }
}
```

---

## 5. Built-in Widgets

### Label

```go
lbl := gui.NewLabel("USB: actif")
lbl.SetText("USB: inactif")          // → SetDirty() automatic
lbl.SetFont(canvas.EmbeddedFont(12))
lbl.SetAlign(gui.AlignLeft)          // AlignLeft | AlignCenter | AlignRight
lbl.SetWrap(true)                    // wrap long text; false (default) → truncate with "…"
```

`PreferredSize`: `(textPixelWidth, font.LineHeight())`.
`MinSize`: `(0, font.LineHeight())`.

### Button

```go
btn := gui.NewButton("Enable RNDIS")
btn.OnClick(func() { enableRNDIS() })
btn.SetEnabled(false)
```

- Implements `Touchable`
- `OnClick` fires on touch-down (GT1151 has no touch-release event)
- Press state = inverted colors (black fill, white text). State is cleared automatically on the next `Render()` call after a 200 ms delay (same as debounce window), so the inverted state is visible for exactly one partial refresh before the button returns to normal
- `MinSize()` returns `image.Pt(20, 20)`; layout engine additionally enforces 20×20 for Touchable widgets
- Recommended practical size ≥ 30×30 px for reliable touch

### ProgressBar

```go
bar := gui.NewProgressBar()
bar.SetValue(0.75)               // 0.0–1.0, → SetDirty()
bar.SetStyle(gui.BarStyleFill)   // BarStyleFill | BarStyleDots
```

- `BarStyleFill`: solid filled rectangle proportional to value
- `BarStyleDots`: row of small squares — cheaper to partially refresh
- `PreferredSize()` returns `(0, 12)` — width is intentionally 0 (no intrinsic width). Wrap in `Expand()` for full-width allocation: `gui.Expand(gui.NewProgressBar())`. A bare ProgressBar without Expand receives its MinSize.X as minimum.
- `MinSize()` returns `(20, 8)`

### StatusBar

```go
sb := gui.NewStatusBar("oioio")
sb.SetLeft("USB: actif")
sb.SetRight("12:34")
sb.SetAutoTime(true)   // right side shows current time; Navigator.Run() ticks it via time.Ticker every minute → SetDirty()
```

- Fixed height: 18 px; black fill, white text (font 12)
- `PreferredSize`: `(parentWidth, 18)`; `MinSize`: `(0, 18)`
- `SetAutoTime` is driven by Navigator.Run()'s internal ticker — no goroutine started in the widget itself

### Spacer / Divider

```go
gui.NewSpacer()    // zero-size gap; used with Expand() for flexible space
gui.NewDivider()   // 1 px horizontal line (in VBox) or vertical (in HBox)
```

---

## 6. Layout System

Two-pass layout — identical to GTK size-request + size-allocate:

**Pass 1 — Measure:** each widget returns `PreferredSize()` and `MinSize()`.
**Pass 2 — Allocate:** container calls `SetBounds(rect)` on each child.

Layout is triggered by Navigator before the first render and after any scene push/pop. Layout is NOT re-triggered on widget dirty — only on structural changes.

### Layout hint types

```go
type layoutHint struct {
    widget Widget
    fixed  int  // ≥ 0: fixed width (HBox) or height (VBox) in px; 0 = use PreferredSize
    expand bool // true: take remaining space equally with other expand widgets
}

// Helpers — return the same widget with layout hints encoded in the parent container
func Expand(w Widget) layoutHint  { return layoutHint{widget: w, expand: true} }
func Fixed(w Widget, px int) layoutHint { return layoutHint{widget: w, fixed: px} }
```

`HBox` and `VBox` accept `...any` children where each element is either a `Widget` or a `layoutHint`. A bare `Widget` is treated as `Fixed(w, 0)` — use `PreferredSize` for sizing.

### VBox

Stacks children vertically. Distributes height: fixed/preferred children get their size, `Expand` children share remaining height equally.

```go
gui.NewVBox(
    gui.NewStatusBar("oioio"),           // PreferredSize.Y = 18
    gui.NewDivider(),                    // PreferredSize.Y = 1
    gui.NewLabel("USB: actif"),          // PreferredSize.Y = font.LineHeight()
    gui.Expand(gui.NewProgressBar()),    // remaining height
    gui.NewHBox(
        gui.NewButton("RNDIS"),
        gui.NewButton("ECM"),
        gui.NewButton("Mass"),
    ),
)
```

### HBox

Distributes children horizontally. Same expand semantics.

```go
gui.NewHBox(
    gui.Expand(label),      // takes all available width
    gui.Fixed(btn, 60),     // fixed 60 px
)
```

### Fixed

Absolute positioning. Width/height are explicit.

```go
fixed := gui.NewFixed(250, 104)
fixed.Put(label, 4, 4)
fixed.Put(btn,   4, 24)
fixed.Put(bar,   4, 50)
```

### Overlay

A positioned container rendered on top of the current scene. `*Overlay` implements `Widget` and is the type accepted by `nav.PushOverlay`.

```go
type Overlay struct { /* ... */ }
func NewOverlay(content Widget, align Alignment) *Overlay

// Alignment constants
const (
    AlignCenter Alignment = iota
    AlignTop
    AlignBottom
    AlignLeft
    AlignRight
)
```

```go
menu := gui.NewVBox(
    gui.NewButton("Enable RNDIS").OnClick(...),
    gui.NewButton("Sleep").OnClick(func() { nav.PopOverlay() }),
)
overlay := gui.NewOverlay(menu, gui.AlignCenter)
nav.PushOverlay(overlay)
```

### Padding helper

```go
// WithPadding wraps w in a container that adds uniform padding on all 4 sides.
// Returns a Widget.
func WithPadding(px int, w Widget) Widget
```

---

## 7. Scene

A scene is the root of one screen. It holds a single root widget (usually a layout container) and optional gesture handlers.

```go
scene := gui.NewScene(rootWidget)
scene.OnSwipe(gui.SwipeUp,   func() { nav.Push(menuScene) })
scene.OnSwipe(gui.SwipeLeft, func() { nav.Pop() })
```

### Swipe type

```go
type SwipeDir int

const (
    SwipeUp    SwipeDir = iota
    SwipeDown
    SwipeLeft
    SwipeRight
)
```

Swipe recognition: touch delta > 20 px in one axis within 500 ms. The dominant axis wins (larger delta). No visual animation during transition — scene changes are instant.

---

## 8. Navigator

Manages the scene stack, touch event routing, and the main event loop.

```go
nav := gui.NewNavigator(disp Display, c *canvas.Canvas, touchCh <-chan touch.TouchEvent)
```

**Canvas ownership:** the caller creates the canvas and passes it to the Navigator. After `NewNavigator`, the caller must not draw to the canvas directly while the Navigator is running. The canvas is logically owned by the Navigator for the lifetime of `Run()`.

**Display lifetime:** Navigator does not call `Close()` — the caller is responsible for `Close()` after `Run()` returns. Navigator calls `Sleep()` at the end of `Run()`.

**Concurrency contract:** `Render()`, `RenderWith()`, `Push()`, `Pop()`, `PushOverlay()`, and `PopOverlay()` are NOT concurrent-safe. Use either `Run()` (drives all rendering internally) OR call `Render()`/`RenderWith()` manually from a single goroutine — never both simultaneously. In tests, do not start `Run()` — call `Render()` directly.

```go
nav.SetAutoFullInterval(n int)    // full refresh every n partials (default: 50)

nav.Push(s *Scene)                // display scene (full refresh)
nav.Pop()                         // go back (full refresh)
nav.PushOverlay(o *Overlay)       // show overlay (partial refresh)
nav.PopOverlay()                  // remove overlay (partial refresh)

nav.Render()                      // one render cycle (auto mode decision)
nav.RenderWith(m epd.Mode)        // forced mode override

nav.Run(ctx context.Context)      // blocking event loop; calls Sleep() on exit
```

### Startup sequence

`nav.Push(firstScene)` triggers the first full refresh sequence:
1. `disp.Init(epd.ModeFull)`
2. Draw all widgets to canvas
3. `disp.DisplayBase(canvas.Bytes())` — establishes reference frame for partial updates

All subsequent full refreshes follow the same sequence. `DisplayPartial` is called directly for partial updates; it handles its own internal mini-reset and partial-mode init (no external `Init(ModePartial)` call needed or wanted).

### Touch coordinate mapping

The GT1151 reports physical coordinates. With `canvas.Rot90` (physical 122×250, logical 250×122), the inverse mapping from physical touch point to logical canvas point is:

```go
// Physical frame: X ∈ [0, epd.Width-1] = [0,121], Y ∈ [0, epd.Height-1] = [0,249]
// canvas.Rot90 toPhysical: px = physW-1-ly = 121-ly, py = lx
// Inverse:
rawLogX := int(pt.Y)                    // physical Y → logical X (0–249)
rawLogY := (epd.Width - 1) - int(pt.X) // physical X → logical Y (0–121)

// Clamp to logical bounds before hit-testing.
// GT1151 may report coordinates slightly outside the configured resolution
// (e.g. X up to 127 for a 122-pixel axis). Without clamping, logY goes negative.
logX := clamp(rawLogX, 0, epd.Height-1) // Height=250 → logical X max 249
logY := clamp(rawLogY, 0, epd.Width-1)  // Width=122  → logical Y max 121

// clamp(v, lo, hi int) int — a trivial helper in navigator.go
```

**Note:** Verify on first hardware test — if touch axes are swapped or inverted, adjust the formula. The clamp prevents silent hit-test failures regardless of GT1151 calibration edge cases.

### Touch routing (priority order)

1. Map hardware coordinates to logical using formula above
2. If overlay active: hit-test `*Overlay` widget first; if consumed, stop
3. Else: hit-test current scene widgets in reverse Z-order (last added = highest priority)
4. For each widget implementing `Touchable`: if `bounds.Contains(logPt)` → call `HandleTouch(mappedPt)`; if returns true, stop
5. If no widget consumed: check scene swipe gesture handlers

### Touch debounce

After `HandleTouch` fires on a widget, that widget ignores further touch events for 200 ms. Different widgets are independent — simultaneous touches on different widgets both fire.

### Auto-time tick

If any `StatusBar` in the current scene has `SetAutoTime(true)`, `Run()` maintains a `time.Ticker` with 1-minute interval. On each tick, the StatusBar's right text is updated to `time.Now().Format("15:04")` and `SetDirty()` is called, triggering a partial refresh on the next render cycle.

---

## 9. RefreshManager

Decides full vs partial refresh. Called by Navigator on every `Render()`.

### Decision rules (priority order)

| Condition | Refresh type |
|-----------|-------------|
| `RenderWith(ModeFull)` called | Full |
| Scene Push/Pop | Full |
| Overlay Push/Pop | Partial |
| Partial counter ≥ interval (default 50) | Full (anti-ghosting) |
| Any widget dirty | Partial |
| Nothing dirty | No-op |

### Partial refresh

When any widget is dirty, Navigator:
1. Redraws ALL dirty widgets to the canvas (not just the dirty region — full canvas is always maintained as the complete current frame)
2. Calls `disp.DisplayPartial(canvas.Bytes())` with the full 4000-byte buffer

`epd.DisplayPartial` always refreshes the entire display; the dirty tracking decides WHETHER to refresh, not WHICH region. `canvas.SubRegion` is not used in the refresh path — it remains available for direct user use.

### Full refresh sequence

Triggered by `RenderWith(ModeFull)`, scene Push/Pop, or anti-ghosting:
```
disp.Init(epd.ModeFull)
redraw all widgets to canvas
disp.DisplayBase(canvas.Bytes())   ← writes to both 0x24 and 0x26 RAM banks
partialCounter = 0
```

### Fast refresh (RenderWith(ModeFast))

Triggered only when the caller explicitly calls `nav.RenderWith(epd.ModeFast)`:
```
disp.Init(epd.ModeFast)
redraw all dirty widgets to canvas
disp.DisplayFast(canvas.Bytes())
partialCounter++   ← counted toward anti-ghosting interval
```
Fast mode does NOT update the 0x26 reference frame. Avoid using it before a partial sequence.

### Anti-ghosting

After `n` partial refreshes (default 50), the next `Render()` performs a full refresh sequence regardless of dirty state. Counter resets to 0.

---

## 10. E-ink / Touch Constraints (explicit)

- **No animations.** Scene transitions are instant (full refresh). Swipes are gesture detection only — no visual motion during the gesture.
- **No hover state.** Buttons have exactly two visual states: Normal and Pressed (inverted colors). No intermediate state.
- **Click = touch-down.** GT1151 interrupt fires on contact; there is no touch-release event from the hardware. `OnClick` fires on touch-down. The pressed visual state lasts ~200 ms (debounce window), then reverts on the next render cycle.
- **Refresh latency.** Full ≈ 2 s, partial ≈ 0.3 s. Design UIs to avoid full refreshes during normal interaction.
- **Minimum touch target.** `Button.MinSize()` = 20×20 px; layout enforces 20×20 for all `Touchable` widgets. Recommended practical minimum: 30×30 px.
- **Recommended font sizes.** StatusBar: 12. Label body: 12 or 16. Button label: 12. Avoid 20/24 in data-dense layouts — the logical screen is only 122 px tall.
- **DisplayBase required before partial sequence.** The full refresh sequence always ends with `DisplayBase` (writes to both 0x24 and 0x26 RAM banks). Without this, subsequent `DisplayPartial` calls may ghost.
- **No external `Init(ModePartial)`.** `epd.DisplayPartial` handles its own mini-reset and partial-mode init internally. The Navigator never calls `Init(ModePartial)` explicitly.
- **`EmbeddedFont` nil guard.** `canvas.EmbeddedFont(size)` returns nil for unsupported sizes. All widget Draw methods must guard against a nil font before calling `DrawText`.

---

## 11. Testing

No hardware required. Tests inject `Display` interface and fake touch channel.

```go
type fakeDisplay struct {
    calls   []string
    lastBuf []byte
    lastMode epd.Mode
}
func (f *fakeDisplay) Init(m epd.Mode) error           { f.calls = append(f.calls, "Init"); f.lastMode = m; return nil }
func (f *fakeDisplay) DisplayBase(buf []byte) error    { f.calls = append(f.calls, "Base"); f.lastBuf = buf; return nil }
func (f *fakeDisplay) DisplayPartial(buf []byte) error { f.calls = append(f.calls, "Partial"); f.lastBuf = buf; return nil }
func (f *fakeDisplay) DisplayFast(buf []byte) error    { f.calls = append(f.calls, "Fast"); f.lastBuf = buf; return nil }
func (f *fakeDisplay) Sleep() error                    { f.calls = append(f.calls, "Sleep"); return nil }
func (f *fakeDisplay) Close() error                    { f.calls = append(f.calls, "Close"); return nil }

// Usage
touchCh := make(chan touch.TouchEvent, 4)
c := canvas.New(epd.Width, epd.Height, canvas.Rot90)
disp := &fakeDisplay{}
nav := gui.NewNavigator(disp, c, touchCh)
```

### Coverage targets

| Area | What to test |
|------|-------------|
| Layout | VBox/HBox measure+allocate with mixed fixed/expand/preferred children; Fixed absolute placement; Overlay alignment |
| RefreshManager | Partial on dirty widget; full on Push; anti-ghosting trigger at interval N; no-op when nothing dirty |
| Touch routing | Hit-test correct widget; miss → no action; overlay priority over scene; debounce suppresses second event within 200 ms |
| Built-in widgets | Draw output (pixel-level via canvas.ToImage); SetDirty on mutation; MinSize; Button press/reset cycle |
| Custom widget | BaseWidget embed compiles; dirty flag works; PreferredSize/MinSize override works |
| Startup sequence | First Push calls Init+Base not just DisplayFull |
