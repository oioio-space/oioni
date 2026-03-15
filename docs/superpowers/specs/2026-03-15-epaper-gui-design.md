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
  └── RefreshManager      ← dirty tracking, batching, partial/full decision
        └── Scene         ← ordered widget tree, layout root, gesture handlers
              └── Widget  ← interface: Draw / Bounds / HandleTouch
                    └── canvas.Canvas  ← existing 1-bit drawing surface
```

### File layout

| File | Responsibility |
|------|---------------|
| `gui.go` | `Widget` interface, `BaseWidget` embed, `Touchable` interface |
| `layout.go` | `HBox`, `VBox`, `Fixed`, `Overlay` containers |
| `widgets.go` | `Label`, `Button`, `ProgressBar`, `StatusBar`, `Spacer`, `Divider` |
| `refresh.go` | `RefreshManager`: dirty tracking, region batching, full/partial strategy |
| `navigator.go` | `Navigator`: scene stack, touch coordinate mapping, debounce, `Run()` |
| `gui_test.go` | Unit tests with fake display and fake touch channel |

---

## 3. Widget Interface

Every widget (built-in or custom) implements:

```go
type Widget interface {
    Draw(c *canvas.Canvas)
    Bounds() image.Rectangle
    SetBounds(r image.Rectangle)
    PreferredSize() image.Point
    MinSize() image.Point      // minimum tappable size (≥ 20×20)
    IsDirty() bool
    SetDirty()
}

// Optional — interactive widgets implement this
type Touchable interface {
    HandleTouch(pt touch.TouchPoint) bool // true = event consumed
}
```

### BaseWidget

Provides bookkeeping so custom widgets only implement `Draw`:

```go
type BaseWidget struct {
    bounds image.Rectangle
    dirty  bool
}

func (b *BaseWidget) Bounds() image.Rectangle       { return b.bounds }
func (b *BaseWidget) SetBounds(r image.Rectangle)   { b.bounds = r; b.dirty = true }
func (b *BaseWidget) IsDirty() bool                 { return b.dirty }
func (b *BaseWidget) SetDirty()                     { b.dirty = true }
func (b *BaseWidget) markClean()                    { b.dirty = false }
func (b *BaseWidget) PreferredSize() image.Point    { return image.Pt(0, 0) }
func (b *BaseWidget) MinSize() image.Point          { return image.Pt(0, 0) }
```

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
    c.DrawText(r.Min.X+2, r.Min.Y+2,
        fmt.Sprintf("%d", w.value), canvas.EmbeddedFont(12), canvas.Black)
}
```

---

## 4. Built-in Widgets

### Label

```go
lbl := gui.NewLabel("USB: actif")
lbl.SetText("USB: inactif")       // → SetDirty() automatic
lbl.SetFont(canvas.EmbeddedFont(12))
lbl.SetAlign(gui.AlignLeft)       // AlignLeft | AlignCenter | AlignRight
lbl.SetWrap(true)                 // wrap long text; false → truncate with "…"
```

`PreferredSize`: width = text pixel width, height = `font.LineHeight()`.

### Button

```go
btn := gui.NewButton("Enable RNDIS")
btn.OnClick(func() { enableRNDIS() })
btn.SetEnabled(false)
```

- Implements `Touchable`
- Press state = inverted colors (black fill, white text)
- No hover state (e-ink has no cursor)
- `MinSize()` returns `image.Pt(20, 20)` — minimum reliable touch target
- `OnClick` fires on touch-down (GT1151 has no touch-release event)

### ProgressBar

```go
bar := gui.NewProgressBar()
bar.SetValue(0.75)                // 0.0–1.0, → SetDirty()
bar.SetStyle(gui.BarStyleFill)   // BarStyleFill | BarStyleDots
```

- `BarStyleFill`: solid filled rectangle proportional to value
- `BarStyleDots`: series of filled squares — only changed dots redraw (better for partial)
- `PreferredSize`: width = parent width, height = 12px

### StatusBar

```go
sb := gui.NewStatusBar("oioio")
sb.SetLeft("USB: actif")
sb.SetRight("12:34")
sb.SetAutoTime(true)   // right side shows current time, updates every minute
```

- Fixed height: 18px
- Black fill, white text (font 12)
- `PreferredSize`: width = parent width, height = 18px

### Spacer

```go
gui.Expand(gui.NewSpacer())   // flexible gap, takes remaining space
gui.NewSpacer()               // zero-size spacer
```

### Divider

```go
gui.NewDivider()              // 1px horizontal line (in VBox) or vertical (in HBox)
```

---

## 5. Layout System

Two-pass layout — identical to GTK's size-request + size-allocate:

**Pass 1 — Measure:** each widget reports `PreferredSize()` and `MinSize()`.

**Pass 2 — Allocate:** layout container calls `SetBounds(rect)` on each child.

Layout is triggered by `Navigator` before first render and after any scene push/pop.

### VBox

Stacks children vertically. Fixed-size children get their `PreferredSize().Y`; `Expand()` children share remaining height equally.

```go
gui.NewVBox(
    gui.NewStatusBar("oioio"),           // height: PreferredSize (18px)
    gui.NewDivider(),                    // height: 1px
    gui.NewLabel("USB: actif"),          // height: font line height
    gui.Expand(gui.NewProgressBar()),    // height: all remaining space
    gui.NewHBox(                         // height: max child PreferredSize
        gui.NewButton("RNDIS"),
        gui.NewButton("ECM"),
        gui.NewButton("Mass"),
    ),
)
```

### HBox

Distributes children horizontally. Same Expand semantics as VBox.

```go
gui.NewHBox(
    gui.Expand(label),      // takes all available width
    gui.Fixed(btn, 60),     // fixed 60px width
)
```

`gui.Expand(w)` and `gui.Fixed(w, px)` are wrapper helpers — they return the same widget with layout hints attached.

### Fixed

Absolute positioning. Width/height must be explicit.

```go
fixed := gui.NewFixed(250, 104)
fixed.Put(label, 4, 4)
fixed.Put(btn,   4, 24)
fixed.Put(bar,   4, 50)
```

### Overlay

Positioned relative to the full screen, rendered on top of the current scene.

```go
menu := gui.NewVBox(
    gui.NewButton("Enable RNDIS").OnClick(...),
    gui.NewButton("Sleep").OnClick(func() { nav.PopOverlay() }),
)
overlay := gui.NewOverlay(menu, gui.AlignCenter)
// AlignCenter | AlignBottom | AlignTop | AlignLeft | AlignRight
nav.PushOverlay(overlay)  // partial refresh of overlay region only
```

### Padding helper

```go
gui.WithPadding(4, gui.NewVBox(...))   // 4px inner margin on all sides
```

---

## 6. Scene

A scene is the root of one screen. It holds a single root widget (usually a layout container) and optional gesture handlers.

```go
scene := gui.NewScene(rootWidget)
scene.OnSwipe(gui.SwipeUp,   func() { nav.Push(menuScene) })
scene.OnSwipe(gui.SwipeLeft, func() { nav.Pop() })
```

Swipe detection: gesture recognized when touch delta exceeds 20px in one axis within 500ms. No visual animation during swipe.

---

## 7. Navigator

Manages the scene stack, touch event routing, and the main event loop.

```go
nav := gui.NewNavigator(disp, canvas, touchCh)
nav.SetAutoFullInterval(50)   // full refresh every N partials (default: 50)

nav.Push(mainScene)           // display scene (full refresh)
nav.Push(menuScene)           // navigate forward (full refresh)
nav.Pop()                     // go back (full refresh)
nav.PushOverlay(overlay)      // show overlay (partial refresh)
nav.PopOverlay()              // remove overlay (partial refresh)

nav.Render()                  // manual render (auto mode decision)
nav.RenderWith(epd.ModeFull)  // forced mode override

nav.Run(ctx)                  // blocking event loop until ctx cancelled
```

### Touch coordinate mapping

GT1151 reports hardware coordinates (physical 122×250). Navigator maps to logical coordinates (250×122 after Rot90) before hit-testing:

```go
// Rot90: logX = hwY, logY = (physW-1) - hwX
logX = int(pt.Y)
logY = (epd.Width - 1) - int(pt.X)
```

### Touch routing

1. Map hardware coordinates to logical
2. If overlay active: hit-test overlay widgets first; consume event if matched
3. Else: hit-test current scene widgets in reverse Z-order (last added = top)
4. If `Touchable` and bounds contain touch point → call `HandleTouch()`
5. If no widget consumed → check scene gesture handlers (swipe)

### Touch debounce

Events on the same widget within 200ms are ignored. Prevents double-firing from GT1151 multi-event bursts during a single tap.

---

## 8. RefreshManager

Decides full vs partial refresh. Called by Navigator on every `Render()`.

### Decision rules (in priority order)

| Condition | Refresh |
|-----------|---------|
| User called `RenderWith(ModeFull)` | Full |
| Scene change (Push/Pop) | Full |
| Overlay Push/Pop | Partial (overlay region) |
| Partial counter ≥ interval (default 50) | Full (anti-ghosting) |
| Any widget dirty | Partial (union of dirty bounds) |
| Nothing dirty | No-op |

### Dirty region batching

All dirty widget bounds are unioned into a single physical region before calling `epd.DisplayPartial`. Only one SPI transaction per `Render()` call, regardless of how many widgets changed.

```
dirty widgets: label(0,20,100,32), bar(0,50,250,62)
→ union → (0,20,250,62)
→ canvas.SubRegion((0,20,250,62)) → one DisplayPartial call
```

The physical region is byte-aligned (snapped to 8-pixel X boundaries) by `canvas.SubRegion` automatically.

### Anti-ghosting

After 50 partial refreshes, the next `Render()` does a full refresh automatically: `Init(ModeFull)` + `DisplayBase(buf)`. The partial counter resets to zero.

---

## 9. E-ink / Touch Constraints (explicit)

These are design constraints, not bugs:

- **No animations.** Scene transitions are instant (full refresh). Swipes are gesture recognition only, no visual motion.
- **No hover state.** Buttons have two states: Normal and Pressed (inverted colors). No intermediate state.
- **Click = touch-down.** GT1151 interrupt fires on contact; there is no touch-release event. `OnClick` fires immediately on touch-down.
- **Refresh latency.** Full refresh ≈ 2s, partial ≈ 0.3s. UI design should minimize full refreshes during normal interaction.
- **Minimum touch target.** `Button.MinSize()` = 20×20px. Layout will not allocate less. Recommended practical minimum: 30×30px.
- **Recommended font sizes.** StatusBar: 12. Label: 12 or 16. Button label: 12. Avoid 20/24 in data-dense layouts.
- **DisplayBase required before partial.** Navigator calls `DisplayBase` after every full refresh to establish the reference frame for subsequent partial updates.

---

## 10. Testing

No hardware required. Tests inject fake display and fake touch channel.

```go
type fakeDisplay struct{ lastBuf []byte; lastMode epd.Mode }
func (f *fakeDisplay) Init(m epd.Mode) error          { f.lastMode = m; return nil }
func (f *fakeDisplay) DisplayFull(buf []byte) error   { f.lastBuf = buf; return nil }
func (f *fakeDisplay) DisplayBase(buf []byte) error   { f.lastBuf = buf; return nil }
func (f *fakeDisplay) DisplayPartial(buf []byte) error{ f.lastBuf = buf; return nil }

touchCh := make(chan touch.TouchEvent, 1)
c := canvas.New(epd.Width, epd.Height, canvas.Rot90)
nav := gui.NewNavigator(fakeDisp, c, touchCh)

scene := gui.NewScene(gui.NewVBox(
    gui.NewLabel("hello"),
    gui.NewProgressBar(),
))
nav.Push(scene)
nav.Render()
// assert canvas pixels, assert fakeDisp.lastMode, etc.
```

Coverage targets:
- Layout: VBox/HBox measure+allocate with mixed fixed/expand children
- RefreshManager: partial→full transition at interval, dirty batching
- Touch routing: hit-test correct widget, debounce, overlay priority
- Each built-in widget: Draw output, SetDirty on mutation, MinSize
- Custom widget: BaseWidget embed works correctly
