# Submenu Containers Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Refactor the 5 category scene stubs into clean, composable, per-file scenes using a `SubSceneOption` functional options pattern, fixing two latent bugs (redundant sidebar widget listing and 2px inset).

**Architecture:** A private `newCategoryScene(nav, title, contentWidget, ...SubSceneOption)` builder in `scene_helpers.go` assembles NavBar + content + ActionSidebar into a single root HBox. Each category gets its own file. The `pages.go` monolith is deleted.

**Tech Stack:** Go, `ui/gui` widget library (NavBar, ActionSidebar, VBox, HBox, FixedSize, Expand, NewLabel), `cmd/oioni/ui` package.

---

## File Map

| Action | File | Purpose |
|--------|------|---------|
| Create | `cmd/oioni/ui/scene_helpers.go` | `SubSceneOption`, `newCategoryScene`, `popToRoot` |
| Create | `cmd/oioni/ui/scene_helpers_test.go` | Tests for scene structure |
| Create | `cmd/oioni/ui/scene_config.go` | `NewConfigScene` |
| Create | `cmd/oioni/ui/scene_system.go` | `NewSystemScene` |
| Create | `cmd/oioni/ui/scene_attack.go` | `NewAttackScene` |
| Create | `cmd/oioni/ui/scene_dfir.go` | `NewDFIRScene` |
| Create | `cmd/oioni/ui/scene_info.go` | `NewInfoScene` |
| Delete | `cmd/oioni/ui/pages.go` | Replaced by above files |

---

## Task 1: Write failing tests for the new scene structure

**Files:**
- Create: `cmd/oioni/ui/scene_helpers_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// cmd/oioni/ui/scene_helpers_test.go
package ui

import (
	"testing"

	"github.com/oioio-space/oioni/drivers/epd"
	"github.com/oioio-space/oioni/ui/gui"
)

// fakeDisplay satisfies gui.Display with no-op methods for tests.
type fakeDisplay struct{}

func (fakeDisplay) Init(_ epd.Mode) error      { return nil }
func (fakeDisplay) DisplayBase(_ []byte) error { return nil }
func (fakeDisplay) DisplayPartial(_ []byte) error { return nil }
func (fakeDisplay) DisplayFast(_ []byte) error    { return nil }
func (fakeDisplay) DisplayRegenerate() error      { return nil }
func (fakeDisplay) Sleep() error                  { return nil }
func (fakeDisplay) Close() error                  { return nil }

func TestCategoryScene_SingleTopLevelWidget(t *testing.T) {
	nav := gui.NewNavigator(fakeDisplay{})
	for _, tc := range []struct {
		name string
		fn   func(*gui.Navigator) *gui.Scene
	}{
		{"Config", NewConfigScene},
		{"System", NewSystemScene},
		{"Attack", NewAttackScene},
		{"DFIR", NewDFIRScene},
		{"Info", NewInfoScene},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := tc.fn(nav)
			if len(s.Widgets) != 1 {
				t.Fatalf("expected 1 top-level widget, got %d (sidebar must not be listed separately)", len(s.Widgets))
			}
		})
	}
}

func TestCategoryScene_Title(t *testing.T) {
	nav := gui.NewNavigator(fakeDisplay{})
	cases := []struct {
		fn    func(*gui.Navigator) *gui.Scene
		title string
	}{
		{NewConfigScene, "Config"},
		{NewSystemScene, "System"},
		{NewAttackScene, "Attack"},
		{NewDFIRScene, "DFIR"},
		{NewInfoScene, "Info"},
	}
	for _, tc := range cases {
		t.Run(tc.title, func(t *testing.T) {
			s := tc.fn(nav)
			if s.Title != tc.title {
				t.Errorf("expected title %q, got %q", tc.title, s.Title)
			}
		})
	}
}

func TestCategoryScene_ExtraSidebarBtn(t *testing.T) {
	nav := gui.NewNavigator(fakeDisplay{})
	// Just verify it builds without panicking when extra sidebar button is provided.
	// newCategoryScene is private; tested indirectly via its public callers.
	// This exercises the SubSceneOption path.
	called := false
	_ = newCategoryScene(nav, "Test", gui.NewLabel("x"),
		withExtraSidebarBtn(gui.Icon{}, func() { called = true }),
	)
	_ = called // compile check: the closure must be accepted
}
```

- [ ] **Step 2: Run tests — expect compile failure (functions don't exist yet)**

```bash
go test ./cmd/oioni/ui/... 2>&1
```
Expected: compile error — `newCategoryScene`, `withExtraSidebarBtn`, `NewConfigScene` etc. undefined.

---

## Task 2: Create scene_helpers.go

**Files:**
- Create: `cmd/oioni/ui/scene_helpers.go`

- [ ] **Step 1: Create the file**

```go
// cmd/oioni/ui/scene_helpers.go — shared category scene builder
package ui

import (
	"image"

	"github.com/oioio-space/oioni/drivers/epd"
	"github.com/oioio-space/oioni/ui/gui"
)

// SubSceneOption configures optional behavior of a category scene.
type SubSceneOption func(*subSceneConfig)

type subSceneConfig struct {
	extraSidebarButtons []gui.SidebarButton
}

// withExtraSidebarBtn adds an extra icon button above the default Oni/Back sidebar buttons.
func withExtraSidebarBtn(icon gui.Icon, onTap func()) SubSceneOption {
	return func(cfg *subSceneConfig) {
		cfg.extraSidebarButtons = append(cfg.extraSidebarButtons, gui.SidebarButton{
			Icon:  icon,
			OnTap: onTap,
		})
	}
}

// newCategoryScene builds a category scene: NavBar breadcrumb + content area + ActionSidebar.
//
// Layout (250×122px logical):
//
//	┌─────────────────────────┬────┐
//	│ Home > title   (18px)   │    │
//	├─────────────────────────┤Oni │ 44px wide
//	│      contentWidget      │────│
//	│      (expands)          │Back│
//	└─────────────────────────┴────┘
//
// Default sidebar: [Oni → home, Back → pop one level].
// Extra buttons prepended above Oni via SubSceneOption.
//
// Touch routing: root's Children() traversal reaches the sidebar recursively,
// so sidebar must NOT be listed separately in Scene.Widgets.
//
// Swipe-scroll: if contentWidget needs swipe-to-scroll, the caller must add it
// as an additional top-level Scene.Widget (Navigator's swipe handler is not recursive).
func newCategoryScene(nav *gui.Navigator, title string, contentWidget gui.Widget, opts ...SubSceneOption) *gui.Scene {
	cfg := &subSceneConfig{}
	for _, o := range opts {
		o(cfg)
	}

	navbar := gui.NewNavBar("Home", title)

	// Build sidebar: extra buttons first, then default Oni + Back.
	sidebarBtns := make([]gui.SidebarButton, 0, len(cfg.extraSidebarButtons)+2)
	sidebarBtns = append(sidebarBtns, cfg.extraSidebarButtons...)
	sidebarBtns = append(sidebarBtns,
		gui.SidebarButton{Icon: Icons.Oni, OnTap: func() { popToRoot(nav) }},
		gui.SidebarButton{Icon: Icons.Back, OnTap: func() { nav.Pop() }}, //nolint:errcheck
	)
	sidebar := gui.NewActionSidebar(sidebarBtns...)

	// NavBar.Draw() renders its 2px separator within its own 18px bounds —
	// no external spacer needed between navbar and content.
	content := gui.NewVBox(
		gui.FixedSize(navbar, 18),
		gui.Expand(contentWidget),
	)
	root := gui.NewHBox(gui.Expand(content), gui.FixedSize(sidebar, 44))
	root.SetBounds(image.Rect(0, 0, epd.Height, epd.Width))

	return &gui.Scene{
		Title:   title,
		Widgets: []gui.Widget{root},
	}
}

// popToRoot pops all scenes until only the root (home) scene remains.
func popToRoot(nav *gui.Navigator) {
	nav.PopTo(1) //nolint:errcheck
}
```

- [ ] **Step 2: Run tests — expect compile errors for missing NewXxxScene functions**

```bash
go test ./cmd/oioni/ui/... 2>&1
```
Expected: compile errors — `NewConfigScene`, `NewSystemScene`, etc. undefined.

---

## Task 3: Create the 5 individual scene files

**Files:**
- Create: `cmd/oioni/ui/scene_config.go`
- Create: `cmd/oioni/ui/scene_system.go`
- Create: `cmd/oioni/ui/scene_attack.go`
- Create: `cmd/oioni/ui/scene_dfir.go`
- Create: `cmd/oioni/ui/scene_info.go`

- [ ] **Step 1: Create scene_config.go**

```go
// cmd/oioni/ui/scene_config.go — Config category scene
package ui

import "github.com/oioio-space/oioni/ui/gui"

// NewConfigScene builds the Config category scene.
func NewConfigScene(nav *gui.Navigator) *gui.Scene {
	return newCategoryScene(nav, "Config", gui.NewLabel("(coming soon)"))
}
```

- [ ] **Step 2: Create scene_system.go**

```go
// cmd/oioni/ui/scene_system.go — System category scene
package ui

import "github.com/oioio-space/oioni/ui/gui"

// NewSystemScene builds the System category scene.
func NewSystemScene(nav *gui.Navigator) *gui.Scene {
	return newCategoryScene(nav, "System", gui.NewLabel("(coming soon)"))
}
```

- [ ] **Step 3: Create scene_attack.go**

```go
// cmd/oioni/ui/scene_attack.go — Attack category scene
package ui

import "github.com/oioio-space/oioni/ui/gui"

// NewAttackScene builds the Attack category scene.
func NewAttackScene(nav *gui.Navigator) *gui.Scene {
	return newCategoryScene(nav, "Attack", gui.NewLabel("(coming soon)"))
}
```

- [ ] **Step 4: Create scene_dfir.go**

```go
// cmd/oioni/ui/scene_dfir.go — DFIR category scene
package ui

import "github.com/oioio-space/oioni/ui/gui"

// NewDFIRScene builds the DFIR category scene.
func NewDFIRScene(nav *gui.Navigator) *gui.Scene {
	return newCategoryScene(nav, "DFIR", gui.NewLabel("(coming soon)"))
}
```

- [ ] **Step 5: Create scene_info.go**

```go
// cmd/oioni/ui/scene_info.go — Info category scene
package ui

import "github.com/oioio-space/oioni/ui/gui"

// NewInfoScene builds the Info category scene.
func NewInfoScene(nav *gui.Navigator) *gui.Scene {
	return newCategoryScene(nav, "Info", gui.NewLabel("(coming soon)"))
}
```

- [ ] **Step 6: Run tests — expect compile error (pages.go duplicates symbols)**

```bash
go test ./cmd/oioni/ui/... 2>&1
```
Expected: compile error — `NewConfigScene` etc. redeclared (both pages.go and new files define them).

---

## Task 4: Delete pages.go and verify

**Files:**
- Delete: `cmd/oioni/ui/pages.go`

- [ ] **Step 1: Delete pages.go**

```bash
rm cmd/oioni/ui/pages.go
```

- [ ] **Step 2: Run all tests — expect PASS**

```bash
go test ./cmd/oioni/ui/... ./ui/gui/... 2>&1
```
Expected:
```
ok  	github.com/oioio-space/oioni/cmd/oioni/ui	...
ok  	github.com/oioio-space/oioni/ui/gui	...
```

- [ ] **Step 3: Build the full binary to confirm no import errors**

```bash
go build ./cmd/oioni/... 2>&1
```
Expected: no output (clean build).

- [ ] **Step 4: Commit**

```bash
git add cmd/oioni/ui/scene_helpers.go \
        cmd/oioni/ui/scene_helpers_test.go \
        cmd/oioni/ui/scene_config.go \
        cmd/oioni/ui/scene_system.go \
        cmd/oioni/ui/scene_attack.go \
        cmd/oioni/ui/scene_dfir.go \
        cmd/oioni/ui/scene_info.go
git rm cmd/oioni/ui/pages.go
git commit -m "refactor(ui): split pages.go into per-scene files with SubSceneOption pattern"
```
