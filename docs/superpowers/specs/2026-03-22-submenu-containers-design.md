# Submenu Containers Design

## Goal

Implement reliable, composable submenu containers for the 5 category scenes (Config, System, Attack, DFIR, Info) using the existing widget library. No specific submenu content yet — just clean, extensible shells.

## Context

- Display: 250×122px logical, 1-bit B&W, e-ink (Waveshare 2.13" Touch HAT)
- Navigation: 2 levels max — Home → Category → (future: actions)
- Existing widgets: `NavBar` (breadcrumb), `ActionSidebar` (icon buttons), `ScrollableList`, `NavButton`, `VBox`, `HBox`, `FixedSize`, `Expand`
- Touch routing: fully recursive via `findTouchTarget()` — no need to list leaf widgets separately in `Scene.Widgets`

## Architecture

### Pattern: Factory Functions + Functional Options

Keep the idiomatic Go factory function pattern. The private `newCategoryScene` function becomes the shared shell, extended with functional options for per-scene customization.

```go
// scene_helpers.go
type SubSceneOption func(*subSceneConfig)

type subSceneConfig struct {
    extraSidebarButtons []gui.SidebarButton
}

func withExtraSidebarBtn(icon gui.Icon, onTap func()) SubSceneOption { ... }

func newCategoryScene(nav *gui.Navigator, title string, opts ...SubSceneOption) *gui.Scene
```

The shell layout (top to bottom, left to right):
```
┌─────────────────────────┬────┐
│ NavBar "Home > Title"   │    │  ← 18px
├─────────────────────────┤    │
│                         │Oni │
│      content area       │────│  ← sidebar 44px wide
│      (placeholder now)  │Back│
│                         │    │
└─────────────────────────┴────┘
```

Sidebar default: `[Oni → PopTo(1), Back → Pop()]`
Sidebar extensible: extra buttons inserted above Oni via `SubSceneOption`.

### File Organization

Split `pages.go` into per-scene files as scenes will grow with distinct content:

```
cmd/oioni/ui/
  scene_helpers.go    ← newCategoryScene, SubSceneOption, popToRoot
  scene_config.go     ← NewConfigScene
  scene_system.go     ← NewSystemScene
  scene_attack.go     ← NewAttackScene
  scene_dfir.go       ← NewDFIRScene
  scene_info.go       ← NewInfoScene
  pages.go            ← DELETE (replaced by above)
```

### Key Fix: Remove Redundant Sidebar Listing

Before recursive touch routing, the sidebar had to be listed separately in `Scene.Widgets`. This is now wrong — touch routing is recursive via `root.Children()`:

```go
// Before (wrong, legacy)
Widgets: []gui.Widget{root, sidebar}

// After (correct)
Widgets: []gui.Widget{root}
```

### NavBar Widget

`NavBar` is already a proper breadcrumb widget. No new widget needed. Used as:
```go
navbar := gui.NewNavBar("Home", title)
```
Renders: `Home > Config` with 2px separator line. Height: 18px.

### ActionSidebar Widget

Already modular. Used as:
```go
sidebar := gui.NewActionSidebar(
    // extra buttons from opts prepended here
    gui.SidebarButton{Icon: Icons.Oni,  OnTap: func() { popToRoot(nav) }},
    gui.SidebarButton{Icon: Icons.Back, OnTap: func() { nav.Pop() }},
)
```

### Content Area

For now: `gui.NewLabel("(coming soon)")` centered in the expanded area. Each scene file will replace this as content is added.

Layout assembly:
```go
content := gui.NewVBox(
    gui.FixedSize(navbar, 18),
    gui.Expand(contentWidget),
)
root := gui.NewHBox(gui.Expand(content), gui.FixedSize(sidebar, 44))
root.SetBounds(image.Rect(0, 0, epd.Height, epd.Width))
```

Note: remove the 2px horizontal inset (`Rect(2, 0, epd.Height-2, epd.Width)`) — bounds management belongs to the layout system, not hardcoded in scene constructors.

## Components

| Component | Location | Action |
|-----------|----------|--------|
| `newCategoryScene` | `scene_helpers.go` | Refactor from `pages.go`, add functional options |
| `SubSceneOption` | `scene_helpers.go` | New type for sidebar extensibility |
| `withExtraSidebarBtn` | `scene_helpers.go` | New option constructor |
| `popToRoot` | `scene_helpers.go` | Move from `pages.go` |
| `NewConfigScene` | `scene_config.go` | Move from `pages.go` |
| `NewSystemScene` | `scene_system.go` | Move from `pages.go` |
| `NewAttackScene` | `scene_attack.go` | Move from `pages.go` |
| `NewDFIRScene` | `scene_dfir.go` | Move from `pages.go` |
| `NewInfoScene` | `scene_info.go` | Move from `pages.go` |
| `pages.go` | — | Delete |

## Error Handling

- `nav.Pop()` and `nav.PopTo()` errors are silently ignored (`//nolint:errcheck`) — display errors on e-ink are non-fatal and not user-actionable.
- Scene construction functions do not return errors — layout is deterministic.

## Testing

- Existing `menu_test.go` tests the home menu; no new scene-level tests needed for stub shells.
- The recursive touch routing is already tested in `gui_test.go`.
- Manual verification: deploy via OTA, confirm each of the 5 menu items navigates to the correct category title in the NavBar.

## Constraints

- ASCII only in all text (e-ink font limitation)
- Minimum 12pt font (NavBar already uses 12pt)
- 2px separators everywhere (single pixels can vanish on partial refresh)
- `hScrollable` widgets must be top-level in `Scene.Widgets` if swipe routing needed (not applicable to these scenes)
