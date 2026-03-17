# oioni UI — Momentum/Flipper Zero-inspired e-paper Interface Design

## Overview

A touch UI for the Waveshare 2.13" Touch e-Paper HAT V4 (250×122px, 1-bit monochrome, capacitive touch) running on Raspberry Pi Zero 2W via gokrazy. Inspired by Flipper Zero / Momentum firmware aesthetics: clean, icon-driven, snap navigation, adaptive sidebar.

---

## 1. Layout

### Pixel breakdown (logical canvas: 250w × 122h, Rot90)

```
┌─────────────────────────────────────────────────┬──────────┐
│  NavBar — title + breadcrumb        (206×16px)   │          │
├─────────────────────────────────────────────────┤  Sidebar │
│                                                  │  44×122  │
│  IconCarousel — category buttons   (206×78px)    │          │
│  ‹  [⚙ Config] [⚔ Attack] [🔍 DFIR] ›           │  [ oni ] │
│                                                  │          │
│             ● ○ ○ ○ ○   (pagination 8px)         │  [back]  │
├─────────────────────────────────────────────────┤          │
│  StatusBar — dynamic state info     (206×14px)   │          │
└─────────────────────────────────────────────────┴──────────┘
```

### Zones

| Zone | Position | Size | Notes |
|------|----------|------|-------|
| NavBar | top-left | 206×16px | 8pt font, 1-2px padding |
| IconCarousel | center-left | 206×88px | 80px buttons + 8px pagination row |
| StatusBar | bottom-left | 206×18px | matches existing `PreferredSize()=(0,18)` |
| ActionSidebar | right | 44×122px | full height, 2 buttons default |

**Pixel budget:** 16 + 88 + 18 = **122px** ✓

### Carousel buttons

- Size: 80×80px per button, 6px gap between buttons
- Leading indent: 13px → 13+80+6+80+6 = 185px → 21px of 3rd button visible = natural scroll hint
- Content: 32×32px icon (centered) + 8pt label below, ~15px top/bottom padding
- Pagination dots: 8px row at bottom of carousel zone, centered

### Sidebar buttons

- Default: 2 buttons (oni + back) → 61px each — comfortable touch target
- Maximum: 3 buttons → ~40px each (acceptable, adaptive context only)
- Width: 44px (slightly below 48px ideal, acceptable given height constraint)

---

## 2. Hardware Constraints (Waveshare 2.13" V4)

Source: [Waveshare Hardware Manual](https://www.waveshare.com/wiki/2.13inch_Touch_e-Paper_HAT_Manual#Question_about_Hardware)

| Constraint | Value | Impact on UI |
|-----------|-------|-------------|
| Minimum refresh interval | 180s recommended | Do NOT refresh continuously; partial refreshes on interaction are fine, but avoid tight loops |
| Sleep mode | **Mandatory** when display not updating | Navigator must call `display.Sleep()` after N seconds of inactivity |
| Maximum idle without refresh | 24h | Must wake + refresh at least once per day even with no user interaction |
| Clear before storage | Required | Not a UI concern — operator procedure |
| Voltage floor | ≥ 2.5V | Hardware concern only |

**Practical rules for this UI:**
1. After each interaction batch (swipe, tap), the `refreshManager` handles partial/full refresh, then the display remains powered in idle state.
2. **Idle sleep**: Navigator starts a `time.AfterFunc(idleTimeout, display.Sleep)` reset on every touch event. Default `idleTimeout = 60s`. On next touch, call `display.Init` + full refresh before resuming.
3. **24h keep-alive**: A background goroutine in `cmd/oioni` wakes the display and triggers a full refresh if no interaction has occurred in 24h.
4. Continuous partial refresh in a tight loop (e.g. animation) is explicitly forbidden.

## 3. Refresh Strategy

| Event | Refresh type | Rationale |
|-------|-------------|-----------|
| Scene Push / Pop | Full (forced) | Already handled by Navigator.RenderWith(forced=true) |
| Carousel snap | Partial | Only carousel zone changes |
| StatusBar update | Partial | Small text region |
| NavBar update | Partial | Title change on navigation |
| Every 50 partials | Full (anti-ghost) | Already handled by refreshManager |
| Tap feedback | Partial | Invert button ~100ms before Push |

Partial refresh latency: ~0.3s — acceptable for snap navigation on e-paper.

---

## 4. Navigation Architecture

### Swipe gesture routing (Approach B — HScrollable)

```
Navigator.Run() receives horizontal swipe
    └─ top scene has hScrollable widget?
        ├─ yes → route to widget.ScrollH(delta)   (carousel snap)
        └─ no  → nav.Pop()                         (go back)
```

- `hScrollable` is package-internal (lowercase), matching `scrollable` convention
- Home scene has `IconCarousel` (implements `hScrollable`) → swipe = carousel scroll
- Sub-pages have no `hScrollable` → swipe left = Pop()

### Long-press for future context menu

```
Navigator.Run() detects hold > 500ms
    └─ widget implements ContextMenuProvider?
        ├─ yes → show context menu overlay (future implementation)
        └─ no  → no-op
```

Interface planned now, not implemented:

```go
type ContextMenuProvider interface {
    ContextMenu() []ContextMenuItem
}

type ContextMenuItem struct {
    Label  string
    Icon   image.Image // optional
    Action func()
}
```

### Sidebar action buttons

| Button | Icon | Action |
|--------|------|--------|
| oni | oni.png (Japanese mask) | Pop all → root (home) |
| back | back.png | Pop() one level |

- Home scene: sidebar shows `[oni]` only (nothing to go back to)
- Sub-pages: sidebar shows `[oni, back]`
- Future: up to 3 adaptive buttons via `ActionSidebar.SetButtons([]SidebarButton)`

### Scene title + breadcrumb

- `Scene` gains `Title string` field
- NavBar displays current scene title
- Breadcrumb: `NavBar` receives `[]string` path e.g. `["Home"]`, `["Home", "Config"]`
- Truncation: if path overflows 206px at 8pt, show only last segment with `… >` prefix

---

## 5. Components

### 4.1 `NavBar` — `ui/gui/widget_navbar.go`

```go
type NavBar struct {
    BaseWidget
    path []string // breadcrumb path, e.g. ["Home", "Config"]
}

func NewNavBar(path ...string) *NavBar
func (n *NavBar) SetPath(path ...string)  // triggers SetDirty
func (n *NavBar) Draw(c *canvas.Canvas)   // renders breadcrumb, 8pt font, separator line
func (n *NavBar) PreferredSize() image.Point  // (206, 16)
func (n *NavBar) MinSize() image.Point        // (60, 16)
```

Rendering: left-aligned text `Home › Config`, separator line at bottom. 8pt font, 2px top padding.

### 4.2 `IconCarousel` — `ui/gui/widget_carousel.go`

```go
type CarouselItem struct {
    Icon  Icon
    Label string
    OnTap func() // called after tap feedback + optional Push
}

type IconCarousel struct {
    BaseWidget
    items    []CarouselItem
    index    int // current snap position (leftmost visible)
    selected int // currently highlighted item
}

func NewIconCarousel(items []CarouselItem) *IconCarousel
func (c *IconCarousel) SetIndex(i int)     // restore scroll position
func (c *IconCarousel) Index() int         // save scroll position
func (c *IconCarousel) ScrollH(delta int)  // implements hScrollable: delta=-1 → index++ (swipe left, next item); delta=+1 → index-- (swipe right, previous item)
func (c *IconCarousel) Draw(cv *canvas.Canvas)
func (c *IconCarousel) HandleTouch(pt touch.TouchPoint) bool  // implements Touchable
func (c *IconCarousel) PreferredSize() image.Point  // (206, 78)
func (c *IconCarousel) MinSize() image.Point        // (100, 60)
```

Tap feedback: non-blocking — set `pressed=true`, call `SetDirty()`, schedule `time.AfterFunc(100ms, func(){ pressed=false; SetDirty(); OnTap() })`. Must not block `HandleTouch` (would stall the Navigator event loop and drop touch events during the 100ms window, which conflicts with the 200ms debounce).

Scroll behaviour:
- `ScrollH(-1)` → `index++` (next item enters from right — triggered by swipe-left gesture), snap
- `ScrollH(+1)` → `index--` (previous item — triggered by swipe-right gesture), snap
- Clamps at 0 and `len(items)-1`
- Pagination dots updated on scroll

Rendering details:
- 13px leading indent
- Each button: rounded rect border (radius 4px), icon 32×32 centered, label 8pt below
- Selected/focused button: inverted (black fill, white icon+text)
- Scroll arrows `‹` `›`: drawn at left/right edges, visible only when scroll possible
- Pagination dots: `●` for current, `○` for others, centered in 8px bottom row

### 4.3 `ActionSidebar` — `ui/gui/widget_sidebar.go`

```go
type SidebarButton struct {
    Icon    Icon
    OnTap   func()
}

type ActionSidebar struct {
    BaseWidget
    buttons []SidebarButton
}

func NewActionSidebar(buttons ...SidebarButton) *ActionSidebar
func (s *ActionSidebar) SetButtons(buttons ...SidebarButton)  // adaptive update, SetDirty
func (s *ActionSidebar) Draw(c *canvas.Canvas)
func (s *ActionSidebar) HandleTouch(pt touch.TouchPoint) bool
func (s *ActionSidebar) PreferredSize() image.Point  // (44, 122)
func (s *ActionSidebar) MinSize() image.Point        // (44, 40)
```

Rendering: buttons equally distributed vertically, separator lines between them, icon centered in each button cell, rounded rect border, left separator line for sidebar.

### 4.4 `StatusBar` — extend existing `widgets.go`

The existing `StatusBar` widget is extended with:

```go
func (s *StatusBar) SetLine(i int, text string)  // update line i, triggers SetDirty
```

(The existing `StatusBar` already supports multiple lines — verify and extend if needed.)

---

## 6. Framework changes

### `gui.go`

Add two interfaces:

```go
// hScrollable is package-internal (matches scrollable convention).
// Navigator routes horizontal swipes to the first widget at the TOP LEVEL of
// Scene.Widgets that implements this interface. Widgets nested inside a layout
// container (VBox, HBox…) are NOT found — hScrollable widgets must be direct
// members of Scene.Widgets.
type hScrollable interface {
    ScrollH(delta int)
}

// ContextMenuProvider is an optional exported interface any widget can implement.
// Navigator checks for it on long-press (>500ms). Currently a no-op: Navigator
// will type-assert and silently skip if the feature is not yet wired. Exported
// so widgets in cmd/oioni can implement it for future use.
type ContextMenuProvider interface {
    ContextMenu() []ContextMenuItem
}

type ContextMenuItem struct {
    Label  string
    Icon   image.Image // optional
    Action func()
}
```

### `navigator.go`

Add `Depth() int`:

```go
// Depth returns the number of scenes on the stack.
// Returns 1 when only the root scene is present, 0 when empty.
func (nav *Navigator) Depth() int { return len(nav.stack) }
```

`Scene` gains `Title string` field (metadata only — Navigator does not read it; NavBar is wired manually via `OnEnter` callbacks):

```go
type Scene struct {
    Title   string   // metadata for debugging/testing; not used by Navigator
    Widgets []Widget
    OnEnter func()
    OnLeave func()
}
```

**Idle sleep (hardware protection):**

```go
// NewNavigator accepts an optional idle timeout. Default = 60s.
// After idleTimeout of no touch input, Navigator calls display.Sleep().
// On next touch: display.Init(ModeFull) + full refresh before routing.
func NewNavigatorWithIdle(d Display, idleTimeout time.Duration) *Navigator
```

`NewNavigator` keeps its existing signature; idle = disabled (0 = no sleep). `cmd/oioni` uses `NewNavigatorWithIdle(d, 60*time.Second)`.

Four changes to `Run()`:

**1. Route horizontal swipe to hScrollable (top-level widgets only):**

```go
// In horizontal swipe block, replace current Pop()-only logic:
if adx >= ady && adx > threshold {
    if dx < 0 { // swipe left
        routed := false
        if len(nav.stack) > 0 {
            for _, w := range nav.stack[len(nav.stack)-1].Widgets {
                if hs, ok := w.(hScrollable); ok {
                    hs.ScrollH(-1)
                    routed = true
                    break
                }
            }
        }
        if !routed {
            nav.Pop()
        }
    } else { // swipe right
        if len(nav.stack) > 0 {
            for _, w := range nav.stack[len(nav.stack)-1].Widgets {
                if hs, ok := w.(hScrollable); ok {
                    hs.ScrollH(+1)
                    break
                }
            }
        }
    }
}
```

**2. Idle sleep timer:**

On every touch event: reset idle timer (`time.AfterFunc(idleTimeout, display.Sleep)`).
On idle timer fire: call `display.Sleep()`, set `sleeping = true`.
On next touch while `sleeping`: call `display.Init(ModeFull)` + full refresh before routing touch. Set `sleeping = false`.

**3. Long-press detection (stub for future context menu):**

Add long-press timer (>500ms hold) in the touch event loop. When triggered, check `ContextMenuProvider` — no-op for now, hook point for future implementation.

### `draw.go` (new file in `ui/gui/`)

`helpers.go` already contains `ShowAlert`/`ShowMenu`/`ShowTextInput` — unrelated to drawing primitives. Add `DrawRoundedRect` in a dedicated new file:

```go
// DrawRoundedRect draws a rounded rectangle outline or filled shape.
// radius is the corner radius in pixels. col is canvas.Black or canvas.White.
func DrawRoundedRect(c *canvas.Canvas, r image.Rectangle, radius int, fill bool, col canvas.Color)
```

### `Scene` struct — `navigator.go`

Add `Title string` field:

```go
type Scene struct {
    Title   string   // displayed in NavBar; empty = no title
    Widgets []Widget
    OnEnter func()
    OnLeave func()
}
```

---

## 7. Asset system

Icons are app-level assets, embedded in `cmd/oioni`:

```
cmd/oioni/ui/icons/
├── config.png    32×32px 1-bit PNG
├── system.png
├── attack.png
├── dfir.png
├── info.png
├── oni.png       Japanese oni mask
└── back.png      left arrow / back chevron
```

Loading pattern in `cmd/oioni/ui/icons.go` — icons are pre-loaded once at program startup, not on each scene `OnEnter`:

```go
//go:embed icons/*.png
var iconFS embed.FS

// icons holds all pre-loaded icons. Populated by init().
var icons struct {
    Config, System, Attack, DFIR, Info gui.Icon
    Oni, Back                          gui.Icon
}

func init() {
    icons.Config = mustLoadIcon("config")
    icons.System = mustLoadIcon("system")
    icons.Attack = mustLoadIcon("attack")
    icons.DFIR   = mustLoadIcon("dfir")
    icons.Info   = mustLoadIcon("info")
    icons.Oni    = mustLoadIcon("oni")
    icons.Back   = mustLoadIcon("back")
}

func mustLoadIcon(name string) gui.Icon {
    f, err := iconFS.Open("icons/" + name + ".png")
    if err != nil {
        panic("icon not found: " + name)
    }
    defer f.Close()
    img, err := png.Decode(f)
    if err != nil {
        panic("icon decode failed: " + name + ": " + err.Error())
    }
    return gui.NewImageIcon(img)
}
```

Icons are `gui.Icon` values (existing type) — widgets accept `Icon`, no knowledge of files.

---

## 8. Application scenes — `cmd/oioni/ui/`

### `home.go` — HomeScene

```go
func NewHomeScene(nav *gui.Navigator, status *gui.StatusBar) *gui.Scene {
    carousel := gui.NewIconCarousel([]gui.CarouselItem{
        {Icon: icons.Config, Label: "Config", OnTap: func() { nav.Push(NewConfigScene(nav)) }},
        {Icon: icons.System, Label: "System", OnTap: func() { nav.Push(NewSystemScene(nav)) }},
        {Icon: icons.Attack, Label: "Attack", OnTap: func() { nav.Push(NewAttackScene(nav)) }},
        {Icon: icons.DFIR,   Label: "DFIR",   OnTap: func() { nav.Push(NewDFIRScene(nav)) }},
        {Icon: icons.Info,   Label: "Info",   OnTap: func() { nav.Push(NewInfoScene(nav)) }},
    })

    navbar  := gui.NewNavBar("Home")
    sidebar := gui.NewActionSidebar(
        gui.SidebarButton{Icon: icons.Oni, OnTap: func() { /* already home */ }},
    )

    // Layout: HBox(content_column | sidebar)
    // SetBounds required — Navigator does not set bounds automatically (see helpers.go pattern).
    // epd.Height=250 = logical width, epd.Width=122 = logical height after Rot90.
    content := gui.NewVBox(
        gui.FixedSize(navbar, 16),
        gui.FixedSize(carousel, 88),
        gui.FixedSize(status, 18),
    )
    root := gui.NewHBox(gui.Expand(content), gui.FixedSize(sidebar, 44))
    root.SetBounds(image.Rect(0, 0, epd.Height, epd.Width)) // (0,0,250,122)

    var savedIdx int
    return &gui.Scene{
        Title:   "Home",
        Widgets: []gui.Widget{root, carousel}, // root renders all; carousel listed separately for hScrollable routing
        OnLeave: func() { savedIdx = carousel.Index() },
        OnEnter: func() { carousel.SetIndex(savedIdx) },
    }
}
```

> **Note on hScrollable routing:** `IconCarousel` must appear as a direct member of `Scene.Widgets` for the Navigator's horizontal swipe routing to find it. Since `root` is a layout container, `carousel` is also listed explicitly in the `Widgets` slice. The `root` renders it; the direct slice entry makes it discoverable for interface routing.

### `pages.go` — Category scenes (stub)

Each category scene: NavBar with breadcrumb, empty content area, sidebar with [oni, back].

```go
func newCategoryScene(nav *gui.Navigator, title string) *gui.Scene {
    navbar  := gui.NewNavBar("Home", title)
    sidebar := gui.NewActionSidebar(
        gui.SidebarButton{Icon: icons.Oni,  OnTap: func() { popToRoot(nav) }},
        gui.SidebarButton{Icon: icons.Back, OnTap: func() { nav.Pop() }},
    )
    placeholder := gui.NewLabel("(coming soon)")

    content := gui.NewVBox(
        gui.FixedSize(navbar, 16),
        gui.Expand(placeholder),
    )
    root := gui.NewHBox(gui.Expand(content), gui.FixedSize(sidebar, 44))
    root.SetBounds(image.Rect(0, 0, epd.Height, epd.Width))

    return &gui.Scene{
        Title:   title,
        Widgets: []gui.Widget{root},
    }
}

// popToRoot pops all scenes until only the root (home) remains.
// nav.Depth() returns len(nav.stack): 1 = only home present.
func popToRoot(nav *gui.Navigator) {
    for nav.Depth() > 1 {
        nav.Pop()
    }
}
```

---

## 9. Testing

All new widgets are unit-tested with fake canvases (existing pattern — no hardware required).

| Test | What it verifies |
|------|-----------------|
| `TestIconCarousel_ScrollH` | Snap left/right, clamp at bounds |
| `TestIconCarousel_Pagination` | Dot position matches index |
| `TestIconCarousel_TapFeedback` | OnTap called after inversion delay |
| `TestNavBar_Breadcrumb` | Path renders correctly, truncation on overflow |
| `TestNavBar_SetPath` | SetDirty triggered on update |
| `TestActionSidebar_SetButtons` | Button count changes, SetDirty triggered |
| `TestNavigator_HScrollable_Routing` | Swipe left routes to hScrollable, not Pop |
| `TestNavigator_NoHScrollable_Pop` | Swipe left pops when no hScrollable in scene |
| `TestNavigator_LongPress_Hook` | Long-press detected (500ms), ContextMenuProvider checked |
| `TestDrawRoundedRect` | Corner pixels correct for radius 0, 2, 4 |
| `TestNavigator_IdleSleep` | Sleep called after timeout, re-init on next touch |
| `TestNavigator_IdleReset` | Timeout reset on each touch event |

---

## 10. File summary

### Modified

| File | Change |
|------|--------|
| `ui/gui/gui.go` | +`hScrollable`, +`ContextMenuProvider`, +`ContextMenuItem` |
| `ui/gui/navigator.go` | +`hScrollable` routing, +long-press hook, +`Depth()`, +`Scene.Title`, +idle sleep (`NewNavigatorWithIdle`) |
| `ui/gui/draw.go` *(new)* | `DrawRoundedRect` |
| `ui/gui/widgets.go` | +`StatusBar.SetLine(i, text)` (if not already present) |

### New

| File | Contents |
|------|----------|
| `ui/gui/widget_navbar.go` | `NavBar` widget |
| `ui/gui/widget_carousel.go` | `IconCarousel` widget + `hScrollable` |
| `ui/gui/widget_sidebar.go` | `ActionSidebar` widget |
| `cmd/oioni/ui/home.go` | `NewHomeScene` |
| `cmd/oioni/ui/keepalive.go` | 24h keep-alive goroutine (hardware requirement) |
| `cmd/oioni/ui/pages.go` | 5 category scene stubs + `popToRoot` |
| `cmd/oioni/ui/icons.go` | `init()` pre-loads all icons; `mustLoadIcon` |
| `cmd/oioni/ui/icons/*.png` | 7 PNG icon assets (32×32px 1-bit) |

---

## 10. Out of scope (future)

- Context menu overlay implementation (interface planned, not built)
- Long-press action delivery (hook point planned, not wired)
- Sub-page content (each category scene is a stub)
- Animation substitutes (pulsing indicators, spinners)
- Asset pack / theme switching
