# oioni UI Redesign — Operator Style — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the home screen carousel + StatusBar with a new operator-style menu list (NetworkStatusBar + HomeMenuWidget) and add an InterfaceDetailPopup accessible from the header badge.

**Architecture:** Three new widgets in `ui/gui/` (`NetworkStatusBar`, `InterfaceDetailPopup`) and one in `cmd/oioni/ui/` (`HomeMenuWidget`). `home.go` is rewritten to wire them together. All widgets follow the existing `BaseWidget` embed pattern with `sync.Mutex` for goroutine-safe state updates.

**Tech Stack:** Go, `ui/gui` framework (Widget/BaseWidget/Navigator/Scene), `ui/canvas` (EmbeddedFont 8pt+12pt, DrawRect, DrawCircle, DrawText, SetPixel), `drivers/touch` (TouchPoint), `drivers/epd` (epd.Width=122, epd.Height=250 logical after Rot90).

---

## File Map

| File | Action | Responsibility |
|------|--------|----------------|
| `ui/gui/widget_networkstatus.go` | **Create** | `NetworkStatusBar` widget: header bar with iface/IP/badge + tool tray |
| `ui/gui/widget_networkstatus_test.go` | **Create** | Unit tests for NetworkStatusBar |
| `ui/gui/widget_ifacepopup.go` | **Create** | `InterfaceDetailPopup` widget: full-screen dithered overlay with iface list |
| `ui/gui/widget_ifacepopup_test.go` | **Create** | Unit tests for InterfaceDetailPopup |
| `cmd/oioni/ui/menu.go` | **Create** | `HomeMenuWidget`: 5-row menu with icon/name/desc/chevron + tap routing |
| `cmd/oioni/ui/menu_test.go` | **Create** | Unit tests for HomeMenuWidget |
| `cmd/oioni/ui/home.go` | **Modify** | Wire NetworkStatusBar + HomeMenuWidget, remove carousel + StatusBar |

---

## Task 1: NetworkStatusBar widget

**Files:**
- Create: `ui/gui/widget_networkstatus.go`
- Create: `ui/gui/widget_networkstatus_test.go`

### Context

`NetworkStatusBar` is a 22px-tall black header bar. Left zone: primary interface name (12pt bold white) + IP (8pt white) + optional `[+N]` badge (white box, black text) when extra interfaces are up. Right zone: up to 2 tool-progress chips (label 8pt + outline progress bar 6px tall), right-aligned with 5px gap between chips. `SetInterfaces` / `SetTools` are goroutine-safe. Badge tap (if `pt` falls in badge bounds) dispatches a push of `InterfaceDetailPopup`.

`IfaceInfo` and `ToolStatus` are the shared data types — they live in this file since they are used by both widgets.

### Steps

- [ ] **Step 1: Write the failing tests**

```go
// ui/gui/widget_networkstatus_test.go
package gui

import (
    "image"
    "testing"

    "github.com/oioio-space/oioni/drivers/touch"
)

func TestNetworkStatusBar_SetInterfacesMarksDirty(t *testing.T) {
    nsb := NewNetworkStatusBar(nil)
    nsb.MarkClean()
    nsb.SetInterfaces([]IfaceInfo{{Name: "eth0", IP: "1.2.3.4", Up: true}})
    if !nsb.IsDirty() {
        t.Error("SetInterfaces should mark dirty")
    }
}

func TestNetworkStatusBar_SetToolsMarksDirty(t *testing.T) {
    nsb := NewNetworkStatusBar(nil)
    nsb.MarkClean()
    nsb.SetTools([]ToolStatus{{Label: "MITM", Progress: 0.5}})
    if !nsb.IsDirty() {
        t.Error("SetTools should mark dirty")
    }
}

func TestNetworkStatusBar_PreferredSize(t *testing.T) {
    nsb := NewNetworkStatusBar(nil)
    sz := nsb.PreferredSize()
    if sz.Y != 22 {
        t.Errorf("PreferredSize().Y = %d, want 22", sz.Y)
    }
}

func TestNetworkStatusBar_DrawDoesNotPanic(t *testing.T) {
    nsb := NewNetworkStatusBar(nil)
    nsb.SetBounds(image.Rect(0, 0, 250, 22))
    nsb.SetInterfaces([]IfaceInfo{
        {Name: "eth0", IP: "192.168.0.33", Up: true},
        {Name: "usb0", IP: "192.168.42.1", Up: true},
    })
    nsb.SetTools([]ToolStatus{
        {Label: "MITM", Progress: 1.0},
        {Label: "SCAN", Progress: 0.6},
    })
    c := newTestCanvas()
    nsb.Draw(c) // must not panic
}

func TestNetworkStatusBar_DrawOfflineDoesNotPanic(t *testing.T) {
    nsb := NewNetworkStatusBar(nil)
    nsb.SetBounds(image.Rect(0, 0, 250, 22))
    // No interfaces set → OFFLINE state
    c := newTestCanvas()
    nsb.Draw(c)
}

func TestNetworkStatusBar_DrawTrayOverflowDoesNotPanic(t *testing.T) {
    nsb := NewNetworkStatusBar(nil)
    nsb.SetBounds(image.Rect(0, 0, 250, 22))
    nsb.SetTools([]ToolStatus{
        {Label: "MITM", Progress: 1.0},
        {Label: "SCAN", Progress: 0.5},
        {Label: "HID", Progress: 0.3}, // 3rd tool → triggers +N badge
    })
    c := newTestCanvas()
    nsb.Draw(c)
}

func TestNetworkStatusBar_BadgeTouchDispatchesWhenNavNil(t *testing.T) {
    // nav=nil means badge tap is a no-op (no panic)
    nsb := NewNetworkStatusBar(nil)
    nsb.SetBounds(image.Rect(0, 0, 250, 22))
    nsb.SetInterfaces([]IfaceInfo{
        {Name: "eth0", IP: "1.2.3.4", Up: true},
        {Name: "usb0", IP: "", Up: false},
    })
    // Tap in badge area (right of IP text, within header)
    nsb.HandleTouch(touch.TouchPoint{X: 80, Y: 5})
}
```

- [ ] **Step 2: Run tests — expect compile failure**

```bash
cd /home/oioio/Documents/GolandProjects/oioni
go test ./ui/gui/ -run TestNetworkStatusBar -v 2>&1 | head -20
```

Expected: `undefined: NewNetworkStatusBar` or similar compile error.

- [ ] **Step 3: Implement `widget_networkstatus.go`**

```go
// ui/gui/widget_networkstatus.go — NetworkStatusBar: 22px header with iface status + tool tray
package gui

import (
    "fmt"
    "image"
    "sync"

    "github.com/oioio-space/oioni/drivers/touch"
    "github.com/oioio-space/oioni/ui/canvas"
)

// IfaceInfo describes one network interface.
type IfaceInfo struct {
    Name string // "eth0", "wlan0", "usb0"
    IP   string // "192.168.0.33", or "" if not assigned
    Up   bool   // link state — controls filled/empty circle in popup
}

// ToolStatus describes one running tool for the header tray.
type ToolStatus struct {
    Label    string  // max 5 chars: "MITM", "SCAN", "HID"
    Progress float64 // 0.0–1.0
}

// NetworkStatusBar renders the 22px operator-style header bar.
// Left: primary interface + IP + optional [+N] badge.
// Right: up to 2 tool-progress chips (label + progress bar outline+fill).
// SetInterfaces and SetTools are goroutine-safe.
type NetworkStatusBar struct {
    BaseWidget
    mu         sync.Mutex
    nav        *Navigator // may be nil in tests
    interfaces []IfaceInfo
    tools      []ToolStatus
    // badgeBounds is updated during Draw; used for touch routing.
    badgeBounds image.Rectangle
}

// NewNetworkStatusBar creates a NetworkStatusBar. nav may be nil (badge tap is no-op).
func NewNetworkStatusBar(nav *Navigator) *NetworkStatusBar {
    nsb := &NetworkStatusBar{nav: nav}
    nsb.SetDirty()
    return nsb
}

// SetInterfaces updates the displayed interfaces. Goroutine-safe.
func (nsb *NetworkStatusBar) SetInterfaces(ifaces []IfaceInfo) {
    nsb.mu.Lock()
    nsb.interfaces = ifaces
    nsb.mu.Unlock()
    nsb.SetDirty()
}

// SetTools updates the tool tray. Goroutine-safe.
func (nsb *NetworkStatusBar) SetTools(tools []ToolStatus) {
    nsb.mu.Lock()
    nsb.tools = tools
    nsb.mu.Unlock()
    nsb.SetDirty()
}

func (nsb *NetworkStatusBar) PreferredSize() image.Point { return image.Pt(0, 22) }
func (nsb *NetworkStatusBar) MinSize() image.Point       { return image.Pt(0, 22) }

// HandleTouch implements Touchable. Badge tap pushes the interface detail popup.
func (nsb *NetworkStatusBar) HandleTouch(pt touch.TouchPoint) bool {
    p := image.Pt(int(pt.X), int(pt.Y))
    nsb.mu.Lock()
    bb := nsb.badgeBounds
    ifaces := nsb.interfaces
    nsb.mu.Unlock()
    if nsb.nav != nil && !bb.Empty() && p.In(bb) {
        nav := nsb.nav
        nav.Dispatch(func() { //nolint:errcheck
            nav.Push(newInterfaceDetailScene(nav, ifaces)) //nolint:errcheck
        })
        return true
    }
    return false
}

// Draw renders the header. Called from Navigator's render loop (single goroutine).
func (nsb *NetworkStatusBar) Draw(c *canvas.Canvas) {
    r := nsb.Bounds()
    if r.Empty() {
        return
    }

    nsb.mu.Lock()
    ifaces := nsb.interfaces
    tools := nsb.tools
    nsb.mu.Unlock()

    // Black background
    c.DrawRect(r, canvas.Black, true)

    f8 := canvas.EmbeddedFont(8)
    f12 := canvas.EmbeddedFont(12)

    // ── Left zone: interface status ────────────────────────────────────────
    upIfaces := make([]IfaceInfo, 0, len(ifaces))
    for _, iface := range ifaces {
        if iface.Up {
            upIfaces = append(upIfaces, iface)
        }
    }

    nsb.mu.Lock()
    nsb.badgeBounds = image.Rectangle{} // reset
    nsb.mu.Unlock()

    if len(upIfaces) == 0 {
        // OFFLINE state
        c.DrawText(r.Min.X+3, r.Min.Y+2, "OFFLINE", f8, canvas.White)
        c.DrawText(r.Min.X+3, r.Min.Y+12, "no link", f8, canvas.White)
    } else {
        primary := upIfaces[0]
        // Line 1: interface name in 12pt bold
        c.DrawText(r.Min.X+3, r.Min.Y+2, primary.Name, f12, canvas.White)
        // IP in 8pt after the name
        nameW := textWidth(primary.Name, f12)
        if primary.IP != "" {
            c.DrawText(r.Min.X+3+nameW+3, r.Min.Y+4, primary.IP, f8, canvas.White)
        }
        // Badge [+N] if more than 1 up interface
        if len(upIfaces) > 1 {
            extra := len(upIfaces) - 1
            badgeText := fmt.Sprintf("+%d", extra)
            bw := textWidth(badgeText, f8) + 4
            bx := r.Min.X + 3 + nameW + 3 + textWidth(primary.IP, f8) + 4
            badgeR := image.Rect(bx, r.Min.Y+2, bx+bw, r.Min.Y+11)
            c.DrawRect(badgeR, canvas.White, true)
            c.DrawText(bx+2, r.Min.Y+2, badgeText, f8, canvas.Black)
            nsb.mu.Lock()
            nsb.badgeBounds = badgeR
            nsb.mu.Unlock()
        }
    }

    // ── Right zone: tool tray ──────────────────────────────────────────────
    // Show at most 2 chips; if more, show "+N" badge to their left.
    const chipW = 28
    const chipGap = 5
    const barH = 6
    const barY = 13 // y offset within header for progress bar

    visible := tools
    overflow := 0
    if len(tools) > 2 {
        overflow = len(tools) - 2
        visible = tools[len(tools)-2:]
    }

    rx := r.Max.X - 2
    for i := len(visible) - 1; i >= 0; i-- {
        rx -= chipW
        tool := visible[i]
        // Label
        c.DrawText(rx, r.Min.Y+2, tool.Label, f8, canvas.White)
        // Progress bar outline
        barR := image.Rect(rx, r.Min.Y+barY, rx+chipW, r.Min.Y+barY+barH)
        c.DrawRect(barR, canvas.White, false)
        // Fill
        fillW := int(float64(chipW-2) * tool.Progress)
        if fillW > 0 {
            fillR := image.Rect(rx+1, r.Min.Y+barY+1, rx+1+fillW, r.Min.Y+barY+barH-1)
            c.DrawRect(fillR, canvas.White, true)
        }
        if i > 0 {
            rx -= chipGap
        }
    }

    if overflow > 0 {
        rx -= chipGap
        overText := fmt.Sprintf("+%d", overflow)
        rx -= textWidth(overText, f8)
        c.DrawText(rx, r.Min.Y+2, overText, f8, canvas.White)
    }

    // 2px separator at bottom of header
    sepY := r.Max.Y - 2
    c.DrawLine(r.Min.X, sepY, r.Max.X, sepY, canvas.Black)
    c.DrawLine(r.Min.X, sepY+1, r.Max.X, sepY+1, canvas.Black)
}

// newInterfaceDetailScene wraps InterfaceDetailPopup in a Scene and pushes it.
// Defined here (same package) so NetworkStatusBar can reference it.
func newInterfaceDetailScene(nav *Navigator, ifaces []IfaceInfo) *Scene {
    popup := newInterfaceDetailPopup(nav, ifaces)
    return &Scene{
        Title:   "Interfaces",
        Widgets: []Widget{popup},
    }
}
```

- [ ] **Step 4: Run tests — expect pass**

```bash
go test ./ui/gui/ -run TestNetworkStatusBar -v
```

Expected: all 6 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add ui/gui/widget_networkstatus.go ui/gui/widget_networkstatus_test.go
git commit -m "feat(ui): add NetworkStatusBar widget with iface status and tool tray"
```

---

## Task 2: InterfaceDetailPopup widget

**Files:**
- Create: `ui/gui/widget_ifacepopup.go`
- Create: `ui/gui/widget_ifacepopup_test.go`

### Context

Full-screen overlay (250×122px bounds). `Draw()` renders:
1. Checkerboard dither fill: `(x+y)%2==0 → black` pixel for every pixel in bounds.
2. White-filled popup box with 2px black border, centered at (45,4)–(205,78).
3. 16px black title bar: "Interfaces" in 12pt bold white, centered.
4. Interface rows (18px each): filled circle `r=2` if `Up`, empty circle if down · name (12pt bold if Up, 8pt normal if down) · IP (8pt) · right-aligned speed/mode placeholder (8pt).
5. Hint row: "swipe down to close" in 8pt, centered in remaining space.

Implements `scrollable`: `Scroll(dy>0)` calls `nav.Dispatch(nav.Pop)`. Bounds span full display so swipe routing finds it.

The constructor is package-private (`newInterfaceDetailPopup`) because it is created by `newInterfaceDetailScene` in `widget_networkstatus.go`.

### Steps

- [ ] **Step 1: Write the failing tests**

```go
// ui/gui/widget_ifacepopup_test.go
package gui

import (
    "image"
    "testing"
)

func TestInterfaceDetailPopup_DrawDoesNotPanic(t *testing.T) {
    popup := newInterfaceDetailPopup(nil, []IfaceInfo{
        {Name: "eth0", IP: "192.168.0.33", Up: true},
        {Name: "usb0", IP: "192.168.42.1", Up: true},
        {Name: "wlan0", IP: "", Up: false},
    })
    popup.SetBounds(image.Rect(0, 0, 250, 122))
    c := newTestCanvas()
    popup.Draw(c)
}

func TestInterfaceDetailPopup_DrawEmptyDoesNotPanic(t *testing.T) {
    popup := newInterfaceDetailPopup(nil, nil)
    popup.SetBounds(image.Rect(0, 0, 250, 122))
    c := newTestCanvas()
    popup.Draw(c)
}

func TestInterfaceDetailPopup_ScrollDownDismisses(t *testing.T) {
    d := &fakeDisplay{}
    nav := NewNavigator(d)
    nav.Push(&Scene{Widgets: []Widget{NewLabel("base")}}) //nolint

    popup := newInterfaceDetailPopup(nav, nil)
    popup.SetBounds(image.Rect(0, 0, 250, 122))

    // Push popup scene so nav has something to pop
    nav.Push(&Scene{Widgets: []Widget{popup}}) //nolint

    depthBefore := nav.Depth()
    popup.Scroll(1) // swipe down — dispatches Pop
    // Drain the dispatch channel so it executes
    nav.drainDispatch()

    if nav.Depth() != depthBefore-1 {
        t.Errorf("depth = %d, want %d", nav.Depth(), depthBefore-1)
    }
}

func TestInterfaceDetailPopup_ScrollUpNoOp(t *testing.T) {
    d := &fakeDisplay{}
    nav := NewNavigator(d)
    nav.Push(&Scene{Widgets: []Widget{NewLabel("base")}}) //nolint
    popup := newInterfaceDetailPopup(nav, nil)
    popup.SetBounds(image.Rect(0, 0, 250, 122))
    nav.Push(&Scene{Widgets: []Widget{popup}}) //nolint

    depthBefore := nav.Depth()
    popup.Scroll(-1) // swipe up — no-op
    nav.drainDispatch()

    if nav.Depth() != depthBefore {
        t.Errorf("scroll up should not pop; depth = %d, want %d", nav.Depth(), depthBefore)
    }
}
```

Note: `nav.drainDispatch()` is a test-only helper — add it to the navigator test file in the next step.

- [ ] **Step 2: Add `drainDispatch` test helper to `gui_test.go`**

Open `ui/gui/gui_test.go` and add at the end (before the last `}`):

```go
// drainDispatch executes one pending Dispatch function synchronously.
// For use in tests only — simulates the Run() event loop draining dispatchFn.
func (nav *Navigator) drainDispatch() {
    select {
    case fn := <-nav.dispatchFn:
        fn()
    default:
    }
}
```

- [ ] **Step 3: Run tests — expect compile failure**

```bash
go test ./ui/gui/ -run TestInterfaceDetailPopup -v 2>&1 | head -20
```

Expected: `undefined: newInterfaceDetailPopup`.

- [ ] **Step 4: Implement `widget_ifacepopup.go`**

```go
// ui/gui/widget_ifacepopup.go — InterfaceDetailPopup: full-screen dithered overlay
package gui

import (
    "image"

    "github.com/oioio-space/oioni/ui/canvas"
)

// InterfaceDetailPopup renders a full-screen dithered overlay with a centred
// popup box listing all network interfaces. Swipe-down dismisses it.
// Implements scrollable (package-internal interface).
type InterfaceDetailPopup struct {
    BaseWidget
    nav        *Navigator // may be nil in tests
    interfaces []IfaceInfo
}

func newInterfaceDetailPopup(nav *Navigator, ifaces []IfaceInfo) *InterfaceDetailPopup {
    p := &InterfaceDetailPopup{nav: nav, interfaces: ifaces}
    p.SetDirty()
    return p
}

// Scroll implements scrollable. Swipe-down (dy > 0) pops this scene.
func (p *InterfaceDetailPopup) Scroll(dy int) {
    if dy > 0 && p.nav != nil {
        nav := p.nav
        nav.Dispatch(func() { //nolint:errcheck
            nav.Pop() //nolint:errcheck
        })
    }
}

// Draw renders the popup overlay.
func (p *InterfaceDetailPopup) Draw(c *canvas.Canvas) {
    r := p.Bounds()
    if r.Empty() {
        return
    }

    // 1. Checkerboard dither fill (visual "dimming")
    for y := r.Min.Y; y < r.Max.Y; y++ {
        for x := r.Min.X; x < r.Max.X; x++ {
            if (x+y)%2 == 0 {
                c.SetPixel(x, y, canvas.Black)
            } else {
                c.SetPixel(x, y, canvas.White)
            }
        }
    }

    // 2. Popup box: white fill + 2px black border
    // Centered: x=45..205, y=4..78 (160×74px)
    const boxX0, boxY0, boxX1, boxY1 = 45, 4, 205, 78
    boxR := image.Rect(boxX0, boxY0, boxX1, boxY1)
    c.DrawRect(boxR, canvas.White, true)
    // 2px border
    c.DrawRect(boxR, canvas.Black, false)
    borderInner := image.Rect(boxX0+1, boxY0+1, boxX1-1, boxY1-1)
    c.DrawRect(borderInner, canvas.Black, false)

    // 3. Title bar (16px): black fill, "Interfaces" centered in 12pt bold white
    const titleH = 16
    titleR := image.Rect(boxX0, boxY0, boxX1, boxY0+titleH)
    c.DrawRect(titleR, canvas.Black, true)
    f12 := canvas.EmbeddedFont(12)
    f8 := canvas.EmbeddedFont(8)
    title := "Interfaces"
    tw := textWidth(title, f12)
    c.DrawText(boxX0+(boxX1-boxX0-tw)/2, boxY0+2, title, f12, canvas.White)

    // 4. Interface rows (18px each)
    const rowH = 18
    y := boxY0 + titleH
    for _, iface := range p.interfaces {
        if y+rowH > boxY1-rowH { // leave room for hint
            break
        }
        cx := boxX0 + 6
        cy := y + rowH/2

        // Circle indicator: filled=Up, empty=down
        if iface.Up {
            // Filled circle r=2
            for dy := -2; dy <= 2; dy++ {
                for dx := -2; dx <= 2; dx++ {
                    if dx*dx+dy*dy <= 4 {
                        c.SetPixel(cx+dx, cy+dy, canvas.Black)
                    }
                }
            }
        } else {
            // Empty circle r=2 (outline only)
            for dx := -2; dx <= 2; dx++ {
                for dy := -2; dy <= 2; dy++ {
                    d := dx*dx + dy*dy
                    if d >= 3 && d <= 4 {
                        c.SetPixel(cx+dx, cy+dy, canvas.Black)
                    }
                }
            }
        }

        // Name: 12pt bold if Up, 8pt normal if down
        tx := cx + 6
        if iface.Up {
            c.DrawText(tx, y+2, iface.Name, f12, canvas.Black)
            tx += textWidth(iface.Name, f12) + 4
        } else {
            c.DrawText(tx, y+5, iface.Name, f8, canvas.Black)
            tx += textWidth(iface.Name, f8) + 4
        }

        // IP (8pt)
        if iface.IP != "" {
            c.DrawText(tx, y+5, iface.IP, f8, canvas.Black)
        }

        // 1px separator
        c.DrawLine(boxX0+2, y+rowH-1, boxX1-2, y+rowH-1, canvas.Black)

        y += rowH
    }

    // 5. Hint row: "swipe down to close" centered in 8pt
    hintText := "swipe down to close"
    hw := textWidth(hintText, f8)
    hintY := boxY1 - f8.LineHeight() - 3
    c.DrawText(boxX0+(boxX1-boxX0-hw)/2, hintY, hintText, f8, canvas.Black)
}
```

- [ ] **Step 5: Run tests — expect pass**

```bash
go test ./ui/gui/ -run TestInterfaceDetailPopup -v
```

Expected: all 4 tests PASS.

- [ ] **Step 6: Commit**

```bash
git add ui/gui/widget_ifacepopup.go ui/gui/widget_ifacepopup_test.go ui/gui/gui_test.go
git commit -m "feat(ui): add InterfaceDetailPopup widget with dithered overlay"
```

---

## Task 3: HomeMenuWidget

**Files:**
- Create: `cmd/oioni/ui/menu.go`
- Create: `cmd/oioni/ui/menu_test.go`

### Context

5-row menu occupying 100px (5×20px). Each row:
- Icon: filled circle r=7 at `x=11, y_center`
- Name: 12pt bold black at `x=24, y_top+2`
- Description: 8pt normal black at `x=24, y_bottom-9`
- Chevron `>`: 12pt bold black right-aligned at `x=246`
- 1px separator at `y_bottom` from `x=16` to `x=250`

Selected/active row: black fill, all text white, no separator.

`HandleTouch` maps `pt.Y` to row index (0–4) and calls `item.onTap()`.

This widget lives in package `ui` (cmd/oioni/ui), not in `gui`, because it references the category scene constructors.

### Steps

- [ ] **Step 1: Write the failing tests**

```go
// cmd/oioni/ui/menu_test.go
package ui

import (
    "image"
    "testing"

    "github.com/oioio-space/oioni/drivers/touch"
    "github.com/oioio-space/oioni/ui/canvas"
    "github.com/oioio-space/oioni/ui/gui"
    "github.com/oioio-space/oioni/drivers/epd"
)

func newTestMenu() *HomeMenuWidget {
    items := []homeMenuItem{
        {name: "Config", desc: "reseau"},
        {name: "System", desc: "services"},
        {name: "Attack", desc: "MITM"},
        {name: "DFIR", desc: "capture"},
        {name: "Info", desc: "aide"},
    }
    return newHomeMenuWidget(items)
}

func TestHomeMenuWidget_PreferredSize(t *testing.T) {
    m := newTestMenu()
    sz := m.PreferredSize()
    if sz.Y != 100 {
        t.Errorf("PreferredSize().Y = %d, want 100", sz.Y)
    }
}

func TestHomeMenuWidget_TapCallsOnTap(t *testing.T) {
    called := ""
    items := []homeMenuItem{
        {name: "A", desc: "a", onTap: func() { called = "A" }},
        {name: "B", desc: "b", onTap: func() { called = "B" }},
        {name: "C", desc: "c"},
        {name: "D", desc: "d"},
        {name: "E", desc: "e"},
    }
    m := newHomeMenuWidget(items)
    m.SetBounds(image.Rect(0, 0, 250, 100))
    // Row 1 = y=20..39 → center y=29
    m.HandleTouch(touch.TouchPoint{X: 100, Y: 29})
    if called != "B" {
        t.Errorf("expected B, got %q", called)
    }
}

func TestHomeMenuWidget_TapNilOnTapIsNoOp(t *testing.T) {
    items := []homeMenuItem{
        {name: "A", desc: "a"}, // onTap is nil
    }
    m := newHomeMenuWidget(items)
    m.SetBounds(image.Rect(0, 0, 250, 100))
    // Should not panic
    m.HandleTouch(touch.TouchPoint{X: 100, Y: 5})
}

func TestHomeMenuWidget_DrawDoesNotPanic(t *testing.T) {
    m := newTestMenu()
    m.SetBounds(image.Rect(0, 0, 250, 100))
    c := canvas.New(epd.Width, epd.Height, canvas.Rot90)
    _ = gui.NewLabel("") // ensure gui is imported
    m.Draw(c)
}
```

- [ ] **Step 2: Run tests — expect compile failure**

```bash
go test ./cmd/oioni/ui/ -run TestHomeMenuWidget -v 2>&1 | head -20
```

Expected: `undefined: newHomeMenuWidget` or similar.

- [ ] **Step 3: Implement `cmd/oioni/ui/menu.go`**

```go
// cmd/oioni/ui/menu.go — HomeMenuWidget: 5-row operator-style menu
package ui

import (
    "image"

    "github.com/oioio-space/oioni/drivers/touch"
    "github.com/oioio-space/oioni/ui/canvas"
    "github.com/oioio-space/oioni/ui/gui"
)

const (
    menuRowH    = 20
    menuRows    = 5
    menuIconX   = 11 // x center of icon circle
    menuIconR   = 7  // radius of icon circle
    menuTextX   = 24 // x start of name/desc text
    menuChevX   = 246 // x of chevron (right-aligned at x=246)
)

type homeMenuItem struct {
    name  string
    desc  string
    onTap func()
}

// HomeMenuWidget renders 5 menu rows (5×20px = 100px total).
type HomeMenuWidget struct {
    gui.BaseWidget
    items    []homeMenuItem
    selected int // index of last tapped row, -1 = none
}

func newHomeMenuWidget(items []homeMenuItem) *HomeMenuWidget {
    m := &HomeMenuWidget{items: items, selected: -1}
    m.SetDirty()
    return m
}

func (m *HomeMenuWidget) PreferredSize() image.Point { return image.Pt(0, menuRows*menuRowH) }
func (m *HomeMenuWidget) MinSize() image.Point       { return image.Pt(0, menuRows*menuRowH) }

func (m *HomeMenuWidget) HandleTouch(pt touch.TouchPoint) bool {
    r := m.Bounds()
    if r.Empty() {
        return false
    }
    py := int(pt.Y)
    if py < r.Min.Y || py >= r.Max.Y {
        return false
    }
    row := (py - r.Min.Y) / menuRowH
    if row < 0 || row >= len(m.items) {
        return false
    }
    m.selected = row
    m.SetDirty()
    if m.items[row].onTap != nil {
        m.items[row].onTap()
    }
    return true
}

func (m *HomeMenuWidget) Draw(c *canvas.Canvas) {
    r := m.Bounds()
    if r.Empty() {
        return
    }
    c.DrawRect(r, canvas.White, true)

    f12 := canvas.EmbeddedFont(12)
    f8 := canvas.EmbeddedFont(8)

    for i, item := range m.items {
        rowTop := r.Min.Y + i*menuRowH
        rowBot := rowTop + menuRowH
        rowCenter := rowTop + menuRowH/2
        rowR := image.Rect(r.Min.X, rowTop, r.Max.X, rowBot)

        active := i == m.selected
        bg := canvas.White
        fg := canvas.Black
        if active {
            bg = canvas.Black
            fg = canvas.White
            c.DrawRect(rowR, bg, true)
        }

        // Icon: filled circle at (menuIconX, rowCenter)
        ix := r.Min.X + menuIconX
        for dy := -menuIconR; dy <= menuIconR; dy++ {
            for dx := -menuIconR; dx <= menuIconR; dx++ {
                if dx*dx+dy*dy <= menuIconR*menuIconR {
                    c.SetPixel(ix+dx, rowCenter+dy, fg)
                }
            }
        }

        // Name: 12pt bold
        c.DrawText(r.Min.X+menuTextX, rowTop+2, item.name, f12, fg)

        // Description: 8pt at y=rowBot-9
        if f8 != nil {
            c.DrawText(r.Min.X+menuTextX, rowBot-9, item.desc, f8, fg)
        }

        // Chevron: ">" at x=menuChevX (right-aligned means the text ends there)
        if f12 != nil {
            cw := textWidth(">", f12)
            c.DrawText(r.Min.X+menuChevX-cw, rowTop+2, ">", f12, fg)
        }

        // 1px separator (not on active row)
        if !active {
            c.DrawLine(r.Min.X+16, rowBot-1, r.Max.X, rowBot-1, canvas.Black)
        }
    }
}
```

Note: `textWidth` is defined in `ui/gui`. Since `HomeMenuWidget` is in package `ui` (not `gui`), use `canvas.EmbeddedFont(n)` and implement inline width calculation using the font's `Glyph` method, or expose a helper. The simplest approach: copy the one-liner locally.

Add to `cmd/oioni/ui/menu.go` before the struct:

```go
// textWidth returns the pixel width of text in font f.
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
```

- [ ] **Step 4: Run tests — expect pass**

```bash
go test ./cmd/oioni/ui/ -run TestHomeMenuWidget -v
```

Expected: all 4 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/oioni/ui/menu.go cmd/oioni/ui/menu_test.go
git commit -m "feat(ui): add HomeMenuWidget with 5-row operator menu"
```

---

## Task 4: Rewire `home.go`

**Files:**
- Modify: `cmd/oioni/ui/home.go`

### Context

Replace the carousel-based home screen with `NetworkStatusBar` (22px) + `HomeMenuWidget` (100px = remaining space). Total = 22 + 100 = 122px = full logical height.

`NetworkStatusBar` must appear at **top level** of `Scene.Widgets` (alongside the layout `content`) so Navigator's touch routing finds its `HandleTouch` for the badge tap.

The `status *gui.StatusBar` parameter is no longer needed — `NewHomeScene` signature changes. Check that the call site in `cmd/oioni/main.go` (or equivalent) is updated.

### Steps

- [ ] **Step 1: Find the call site**

```bash
grep -rn "NewHomeScene" /home/oioio/Documents/GolandProjects/oioni/cmd/oioni/
```

Note the file and signature so you can update it in Step 3.

- [ ] **Step 2: Rewrite `cmd/oioni/ui/home.go`**

```go
// cmd/oioni/ui/home.go — HomeScene: operator-style menu home screen
package ui

import (
    "image"

    "github.com/oioio-space/oioni/drivers/epd"
    "github.com/oioio-space/oioni/ui/gui"
)

// NewHomeScene builds the home screen: 22px NetworkStatusBar header + 100px HomeMenuWidget.
// nav is passed to NetworkStatusBar for badge-tap routing.
// Returns the scene and the NetworkStatusBar so the caller can call SetInterfaces/SetTools.
func NewHomeScene(nav *gui.Navigator) (*gui.Scene, *gui.NetworkStatusBar) {
    nsb := gui.NewNetworkStatusBar(nav)

    menu := newHomeMenuWidget([]homeMenuItem{
        {
            name:  "Config",
            desc:  "reseau · interfaces · device",
            onTap: func() { nav.Dispatch(func() { nav.Push(NewConfigScene(nav)) }) }, //nolint:errcheck
        },
        {
            name:  "System",
            desc:  "services · logs · processus",
            onTap: func() { nav.Dispatch(func() { nav.Push(NewSystemScene(nav)) }) }, //nolint:errcheck
        },
        {
            name:  "Attack",
            desc:  "MITM · scan · deauth · spoof",
            onTap: func() { nav.Dispatch(func() { nav.Push(NewAttackScene(nav)) }) }, //nolint:errcheck
        },
        {
            name:  "DFIR",
            desc:  "capture · pcap · forensics",
            onTap: func() { nav.Dispatch(func() { nav.Push(NewDFIRScene(nav)) }) }, //nolint:errcheck
        },
        {
            name:  "Info",
            desc:  "aide · licences · a propos",
            onTap: func() { nav.Dispatch(func() { nav.Push(NewInfoScene(nav)) }) }, //nolint:errcheck
        },
    })

    content := gui.NewVBox(
        gui.FixedSize(nsb, 22),  // header
        gui.Expand(menu),        // 100px menu (fills 122-22=100px)
    )
    content.SetBounds(image.Rect(0, 0, epd.Height, epd.Width))

    return &gui.Scene{
        Title: "Home",
        // nsb at top level → Navigator finds HandleTouch for badge tap.
        // menu at top level → Navigator finds HandleTouch for row taps.
        Widgets: []gui.Widget{content, nsb, menu},
    }, nsb
}
```

- [ ] **Step 3: Update the call site in `cmd/oioni/epaper.go`**

The file `cmd/oioni/epaper.go` contains:
- `epaperState` struct with a `status *gui.StatusBar` field
- `startEPaper()` which calls `NewHomeScene(nav, status)`
- `UpdateStatus(left, right string)` method that calls `e.status.SetLeft/SetRight`

`cmd/oioni/main.go` calls `ep.UpdateStatus(...)` to update the status bar.

Make these changes:

1. In `epaperState` struct, remove `status *gui.StatusBar`, add `nsb *gui.NetworkStatusBar`:
```go
type epaperState struct {
    nav    *gui.Navigator
    nsb    *gui.NetworkStatusBar
    cancel context.CancelFunc
}
```

2. In `startEPaper()`, change:
```go
// Before:
status := gui.NewStatusBar("", "")
home := oioniui.NewHomeScene(nav, status)
// ...
return &epaperState{nav: nav, status: status, cancel: cancel}

// After:
home, nsb := oioniui.NewHomeScene(nav)
// ...
return &epaperState{nav: nav, nsb: nsb, cancel: cancel}
```

3. Replace (or remove) the `UpdateStatus` method. For now, replace with a no-op stub so `main.go` callers still compile:
```go
// UpdateStatus is a no-op stub — wire real iface/tool data via nsb.SetInterfaces/SetTools.
func (e *epaperState) UpdateStatus(_, _ string) {}
```

Remove any import of `gui` from `epaper.go` that was only used for `NewStatusBar` if it becomes unused after the change.

- [ ] **Step 4: Build to verify no compile errors**

```bash
go build ./cmd/oioni/...
```

Expected: builds cleanly with no errors.

- [ ] **Step 5: Run full test suite**

```bash
go test ./ui/gui/... ./cmd/oioni/...
```

Expected: all tests pass.

- [ ] **Step 6: Commit**

```bash
git add cmd/oioni/ui/home.go
# Also add any modified call-site file
git commit -m "feat(ui): rewire home screen to NetworkStatusBar + HomeMenuWidget"
```

---

## Task 5: Smoke test on device (optional — skip if no hardware)

This task is for manual verification only; no code changes required.

- [ ] Flash/OTA the build to the device:

```bash
cd /home/oioio/Documents/GolandProjects/oioni
GOWORK=off gok update --parent_dir . -i oioio
```

- [ ] Verify visually:
  - Home screen shows black header with interface name + IP (or "OFFLINE")
  - 5 menu rows visible, each with name + description + chevron
  - Tap a row → navigates to category scene
  - Swipe left → goes back
  - If 2+ interfaces active: `[+N]` badge visible in header; tap badge → interface detail popup; swipe down → popup closes

- [ ] No further action needed — this closes the implementation.
