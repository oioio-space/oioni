# oioni UI Redesign — Operator Style

**Date:** 2026-03-21
**Scope:** Home screen + category scene template
**Display:** Waveshare 2.13" e-ink, 250×122px landscape (after Rot90), 1-bit (black/white only)
**Existing framework:** `ui/gui` (Navigator, Scene, Widget, canvas)

---

## 1. Design Principles

### 1-bit strict
Every pixel is either **black** (`canvas.Black`) or **white** (`canvas.White`). No grays. Visual hierarchy is expressed through:

| Technique | Use |
|-----------|-----|
| Bold vs normal weight | Primary label vs secondary description |
| Large vs small font | 12pt name vs 8pt description (only sizes available: 8, 12, 16, 20, 24pt) |
| Inversion (black bg) | Active row, header bar, popup title |
| Dithering (checkerboard) | Popup background "dimming" |
| Filled ● vs empty ○ | Active vs inactive state indicators |
| 2px vs 1px lines | Structural borders vs row separators |

### Font sizes
Only bitmap sizes available via `canvas.EmbeddedFont`: **8, 12, 16, 20, 24pt**.
All text in this design uses either **8pt** (secondary, descriptions, hints) or **12pt** (primary names, labels). No other sizes.

### No sidebar
The current `ActionSidebar` (44px) is removed from all scenes. Navigation back is handled by horizontal swipe-left (already implemented in `Navigator.Run`). This recovers 44px of horizontal content space.

---

## 2. Home Screen

### Layout (250×122px)

```
┌──────────────────────────────────────────────────────────────┐  ← y=0
│  eth0  192.168.0.33  [+1]               [MITM ████][SCAN ██░]│  22px header (black)
├──────────────────────────────────────────────────────────────┤  ← y=22 (2px separator)
│  ⚙  Config      reseau · interfaces · device              ›  │  \
│─────────────────────────────────────────────────────────────│   |
│  ≡  System      services · logs · processus               ›  │   |  5 rows × 20px = 100px
│─────────────────────────────────────────────────────────────│   |  (122 - 22 = 100px)
│  ⚡ Attack      MITM · scan · deauth · spoof              ›  │   |
│─────────────────────────────────────────────────────────────│   |
│  ◎  DFIR        capture · pcap · forensics                ›  │   |
│─────────────────────────────────────────────────────────────│   |
│  ⓘ  Info        aide · licences · a propos                ›  │  /
└──────────────────────────────────────────────────────────────┘  ← y=122
```

Row height: `(122 - 22) / 5 = 20px` exactly. No spacer needed.

### Header bar (y=0, h=22, full black background)

**Left zone** — dynamic network status (x=0..~130):

| State | Line 1 | Line 2 |
|-------|--------|--------|
| 1 interface active | `eth0` (12pt bold white) + `  192.168.0.33` (8pt normal white) | — |
| 2+ interfaces active | `eth0` (12pt bold white) + IP (8pt white) + `[+N]` badge | `+ usb0` (8pt normal white) |
| No interface | `OFFLINE` (8pt normal white) | `no link` (8pt normal white) |

`[+N]` badge: white fill box, black 8pt bold text. Width ~16px. Tappable zone (see §5 touch routing).

**Right zone** — active tool tray (x=130..248, max 2 tools):

Each tool chip is 28px wide:
- Label: 8pt normal white, drawn above bar
- Bar: white outline rect (6px tall) + white fill = `(chipW-2) × progress`
- Chips stacked right-to-left, 5px gap between chips
- **Max 2 chips displayed**. If >2 tools active, show 2 + badge `+N` (8pt white) to the left of the chips
- No tools running: right zone empty

Separator: 2px black horizontal line at `y=22`.

### Menu rows (y=22..122, 5 rows × 20px)

Each row (height=20px):
- **Icon** (7px radius, black fill) centered at `x=11, y=row_center`
- **Name** (12pt bold black) at `x=24, y=row_top+2`
- **Description** (8pt normal black) at `x=24, y=row_bottom-9`
- **Chevron** `>` (12pt bold black) right-aligned at `x=246`
- **Row separator**: 1px black horizontal line at `y=row_bottom`, from `x=16` to `x=250`

**Selected/active row** (full inversion):
- Background: black fill for entire row width
- Icon, name, description, chevron: white
- No separator drawn for this row

Touch: tap row → `nav.Dispatch(func() { nav.Push(scene) })`.

---

## 3. Interface Detail Popup

Triggered by tap on the `[+N]` badge in the header.

### Layout (centered overlay, 160×74px logical)

```
┌─ dithered background (checkerboard 1×1px) ──────────────────┐
│                                                               │
│   ╔═══════════════════════════════════════╗                  │
│   ║  Interfaces          (12pt bold white)║  ← black title   │
│   ╠═══════════════════════════════════════╣                  │
│   ║ ●  eth0   192.168.0.33   100Mbps      ║  ← 18px row      │
│   ╠───────────────────────────────────────╣                  │
│   ║ ●  usb0   192.168.42.1   RNDIS        ║                  │
│   ╠───────────────────────────────────────╣                  │
│   ║ ○  wlan0  not connected  off          ║                  │
│   ╠───────────────────────────────────────╣                  │
│   ║  swipe down to close    (8pt normal)  ║  ← hint row      │
│   ╚═══════════════════════════════════════╝                  │
└──────────────────────────────────────────────────────────────┘
```

- **Background**: full-screen dithered (alternating pixels: `(x+y)%2==0 → black`) drawn first
- **Popup box**: white fill, 2px black border, centered at (45, 4)..(205, 78)
- **Title bar** (16px): black fill, `Interfaces` 12pt bold white centered
- **Interface rows** (18px each): `●` (filled circle r=2) if `Up`, `○` (empty circle r=2) if down · name (12pt bold if Up, 8pt normal if down) · IP (8pt) · speed/mode (8pt right-aligned)
- **Hint row**: `swipe down to close` 8pt normal black, centered
- **Dismiss**: swipe-down gesture — `InterfaceDetailPopup` implements `scrollable`; `Scroll(dy>0)` calls `nav.Dispatch(nav.Pop)`

### Scene registration

```go
// InterfaceDetailPopup must be BOTH inside the scene for rendering
// AND at top level for scrollable routing:
popup := newInterfaceDetailPopup(nav, ifaces)
scene := &gui.Scene{
    Widgets: []gui.Widget{popup},
}
nav.Push(scene)
```

The popup widget bounds span the full display (250×122), so `Bounds()` returns the full rect. The `scrollable` interface routes the swipe-down to dismiss.

---

## 4. Category Scene Template

All 5 category scenes share the same template. Sidebar already removed in `pages.go`.

### Layout

```
┌──────────────────────────────────────────────────────────────┐
│  Home > Config                    (12pt bold, black bg)      │  18px NavBar
├──────────────────────────────────────────────────────────────┤  2px separator
│                                                               │
│  (content area — Expand, full width 250px)                    │
│                                                               │
└──────────────────────────────────────────────────────────────┘
```

- **NavBar**: `gui.NewNavBar("Home", title)` — already in place, no change needed
- **Back navigation**: swipe-left → `nav.Pop()` automatically via Navigator (no button needed)

---

## 5. Widget: NetworkStatusBar

Replaces the existing `gui.StatusBar` in `NewHomeScene`. Lives in `ui/gui/widget_networkstatus.go`.

```go
type NetworkStatusBar struct {
    gui.BaseWidget
    mu         sync.Mutex
    nav        *gui.Navigator  // needed to push InterfaceDetailScene on badge tap
    interfaces []IfaceInfo
    tools      []ToolStatus
}

// IfaceInfo describes one network interface.
type IfaceInfo struct {
    Name string  // "eth0", "wlan0", "usb0"
    IP   string  // "192.168.0.33", or "" if not assigned
    Up   bool    // link state — controls ●/○ in popup
}

// ToolStatus describes one running tool for the tray.
type ToolStatus struct {
    Label    string  // max 5 chars: "MITM", "SCAN", "HID"
    Progress float64 // 0.0–1.0
}
```

- `SetInterfaces([]IfaceInfo)` — goroutine-safe, calls `SetDirty()`
- `SetTools([]ToolStatus)` — goroutine-safe, calls `SetDirty()`
- `PreferredSize()` → `(0, 22)` — use `gui.FixedSize(nsb, 22)` in VBox
- `HandleTouch(pt)` — implements `Touchable`; if `pt` falls within badge bounds → `nav.Dispatch(func() { nav.Push(newInterfaceDetailScene(nav, nsb.interfaces)) })`

### Touch routing note

`NetworkStatusBar` must appear at the **top level of `Scene.Widgets`** (not only inside a layout container) so Navigator's hit-test finds it for `Touchable` dispatch. Pattern from existing `home.go`:

```go
// home.go
content := gui.NewVBox(
    gui.FixedSize(nsb, 22),   // renders the header
    gui.Expand(menu),
)
content.SetBounds(...)

return &gui.Scene{
    Widgets: []gui.Widget{content, nsb, menu}, // nsb + menu at top level for touch/hScrollable routing
}
```

---

## 6. Widget: InterfaceDetailPopup

Lives in `ui/gui/widget_ifacepopup.go`.

```go
type InterfaceDetailPopup struct {
    gui.BaseWidget
    nav        *gui.Navigator
    interfaces []IfaceInfo
}

// Scroll implements scrollable. Swipe-down (dy > 0) dismisses the popup.
func (p *InterfaceDetailPopup) Scroll(dy int) {
    if dy > 0 {
        p.nav.Dispatch(func() { p.nav.Pop() }) //nolint:errcheck
    }
}
```

`Draw()` renders: full-screen checkerboard dither → white popup box → title bar → interface rows → hint.

---

## 7. Widget: HomeMenuWidget

Lives in `cmd/oioni/ui/menu.go`. Draws the 5-row menu and handles row taps.

```go
type HomeMenuWidget struct {
    gui.BaseWidget
    items []homeMenuItem
}

type homeMenuItem struct {
    icon    gui.Icon
    name    string   // 12pt bold
    desc    string   // 8pt normal
    onTap   func()   // called via nav.Dispatch
}
```

- `PreferredSize()` → `(0, 100)` (5 × 20px)
- `HandleTouch(pt)` — implements `Touchable`; maps `pt.Y` → row index → calls `item.onTap()`

---

## 8. Files to Create / Modify

| File | Action |
|------|--------|
| `ui/gui/widget_networkstatus.go` | **Create**: `NetworkStatusBar`, `IfaceInfo`, `ToolStatus` |
| `ui/gui/widget_ifacepopup.go` | **Create**: `InterfaceDetailPopup` |
| `cmd/oioni/ui/menu.go` | **Create**: `HomeMenuWidget` |
| `cmd/oioni/ui/home.go` | **Modify**: replace carousel + StatusBar with NetworkStatusBar + HomeMenuWidget |
| `cmd/oioni/ui/pages.go` | No further change (sidebar already removed) |

The `IconCarousel` widget is retired from the home screen but kept in `ui/gui` for future use.

---

## 9. Out of Scope

- Content of individual category scenes — stubs remain
- Real network interface polling wired into `SetInterfaces`
- Real tool progress reporting wired into `SetTools`
- Deletion of `IconCarousel`
