# Composable Widgets Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extraire `ScrollableList` et `NavButton` dans le package `ui/gui/` pour les rendre réutilisables dans toutes les scènes, et remplacer la logique home-menu par `homeListItem` qui implémente `gui.ListItem`.

**Architecture:** `gui.ListItem` est une interface que chaque scène implémente pour définir le rendu de ses rangées. `gui.ScrollableList` gère l'état de scroll et le touch routing de façon responsive (visible = `Bounds().Dy() / RowH`). `gui.NavButton` est un bouton ∧/∨ générique câblé via closures.

**Tech Stack:** Go, `ui/gui/` package, `cmd/oioni/ui/` package, TDD.

---

## Contexte important

- Module Go: `github.com/oioio-space/oioni`
- Package cible widgets réutilisables: `ui/gui/` (package `gui`)
- Package spécifique home: `cmd/oioni/ui/` (package `ui`)
- Display logique: 250×122px. NSB: 22px. Zone menu: 100px.
- `canvas.EmbeddedFont(12)` peut retourner nil — toujours nil-check.
- `textWidth(text, font)` est déjà défini dans `ui/gui/widgets.go` — NE PAS redéfinir dans les nouveaux fichiers gui.
- Les tests gui utilisent `canvas.New(epd.Width, epd.Height, canvas.Rot90)` pour créer un canvas.
- Run tests: `go test ./ui/gui/...` et `go test ./cmd/oioni/ui/...`

---

## Structure des fichiers

| Fichier | Action | Contenu |
|---------|--------|---------|
| `ui/gui/widget_scrolllist.go` | Créer | `ListItem` interface + `ScrollableList` |
| `ui/gui/widget_scrolllist_test.go` | Créer | Tests `ScrollableList` |
| `ui/gui/widget_navbutton.go` | Créer | `NavButton` générique |
| `ui/gui/widget_navbutton_test.go` | Créer | Tests `NavButton` |
| `cmd/oioni/ui/menu.go` | Réécrire | `homeListItem` + constantes seulement |
| `cmd/oioni/ui/menu_test.go` | Réécrire | Tests `homeListItem` |
| `cmd/oioni/ui/home.go` | Modifier | Utiliser `gui.NewScrollableList` + `gui.NewNavButton` |

---

## Task 1 : `gui.ScrollableList` + `gui.ListItem`

**Files:**
- Create: `ui/gui/widget_scrolllist.go`
- Create: `ui/gui/widget_scrolllist_test.go`

- [ ] **Step 1: Écrire les tests (fichier de test d'abord)**

```go
// ui/gui/widget_scrolllist_test.go
package gui

import (
	"image"
	"testing"

	"github.com/oioio-space/oioni/drivers/epd"
	"github.com/oioio-space/oioni/drivers/touch"
	"github.com/oioio-space/oioni/ui/canvas"
)

// stubItem est un ListItem de test qui enregistre les taps.
type stubItem struct{ tapped bool }

func (s *stubItem) Draw(_ *canvas.Canvas, _ image.Rectangle) {}
func (s *stubItem) OnTap()                                    { s.tapped = true }

// newTestList5 crée une ScrollableList avec 5 stubItems et rowH=25.
func newTestList5() (*ScrollableList, []*stubItem) {
	stubs := make([]*stubItem, 5)
	items := make([]ListItem, 5)
	for i := range stubs {
		stubs[i] = &stubItem{}
		items[i] = stubs[i]
	}
	return NewScrollableList(items, 25), stubs
}

// setBoundsMenu simule les bounds de production : liste y=22..122, x=0..200 (100px height)
func setBoundsMenu(l *ScrollableList) {
	l.SetBounds(image.Rect(0, 22, 200, 122))
}

// ── visible() responsive ──────────────────────────────────────────────────────

func TestScrollableList_Visible_100px(t *testing.T) {
	l, _ := newTestList5()
	setBoundsMenu(l) // 100px height / rowH=25 → 4
	if got := l.visible(); got != 4 {
		t.Errorf("visible() = %d, want 4", got)
	}
}

func TestScrollableList_Visible_50px(t *testing.T) {
	l, _ := newTestList5()
	l.SetBounds(image.Rect(0, 0, 200, 50)) // 50px height / rowH=25 → 2
	if got := l.visible(); got != 2 {
		t.Errorf("visible() = %d, want 2", got)
	}
}

func TestScrollableList_Visible_NoBounds(t *testing.T) {
	l, _ := newTestList5()
	// bounds zero → visible() must return 0, not divide by zero
	if got := l.visible(); got != 0 {
		t.Errorf("visible() = %d, want 0 with empty bounds", got)
	}
}

// ── CanScroll ────────────────────────────────────────────────────────────────

func TestScrollableList_CanScrollDown_5items_4visible(t *testing.T) {
	l, _ := newTestList5()
	setBoundsMenu(l) // visible=4, items=5 → can scroll
	if !l.CanScrollDown() {
		t.Error("CanScrollDown() = false with 5 items and 4 visible")
	}
}

func TestScrollableList_CannotScrollDown_WhenAllFit(t *testing.T) {
	items := []ListItem{&stubItem{}, &stubItem{}, &stubItem{}, &stubItem{}}
	l := NewScrollableList(items, 25)
	setBoundsMenu(l) // 4 items, visible=4 → no scroll
	if l.CanScrollDown() {
		t.Error("CanScrollDown() = true when all items fit")
	}
}

func TestScrollableList_CannotScrollDown_ShortList(t *testing.T) {
	for _, n := range []int{0, 1} {
		items := make([]ListItem, n)
		l := NewScrollableList(items, 25)
		setBoundsMenu(l)
		if l.CanScrollDown() {
			t.Errorf("CanScrollDown() = true for %d-item list", n)
		}
	}
}

func TestScrollableList_CanScrollUp_AtTop(t *testing.T) {
	l, _ := newTestList5()
	if l.CanScrollUp() {
		t.Error("CanScrollUp() = true at offset 0")
	}
}

func TestScrollableList_CanScrollUp_WithOffset(t *testing.T) {
	l, _ := newTestList5()
	l.offset = 1
	if !l.CanScrollUp() {
		t.Error("CanScrollUp() = false with offset=1")
	}
}

// ── Scroll ───────────────────────────────────────────────────────────────────

func TestScrollableList_ScrollDown(t *testing.T) {
	l, _ := newTestList5()
	setBoundsMenu(l)
	l.ScrollDown()
	if l.offset != 1 {
		t.Errorf("offset = %d, want 1", l.offset)
	}
}

func TestScrollableList_ScrollUp(t *testing.T) {
	l, _ := newTestList5()
	setBoundsMenu(l)
	l.offset = 1
	l.ScrollUp()
	if l.offset != 0 {
		t.Errorf("offset = %d, want 0", l.offset)
	}
}

func TestScrollableList_ScrollUpAtTop_Noop(t *testing.T) {
	l, _ := newTestList5()
	l.ScrollUp() // no-op
	if l.offset != 0 {
		t.Errorf("offset = %d, want 0 (no-op)", l.offset)
	}
}

func TestScrollableList_ScrollDownAtBottom_Noop(t *testing.T) {
	l, _ := newTestList5()
	setBoundsMenu(l)  // visible=4, max offset = 5-4 = 1
	l.offset = 1
	l.ScrollDown() // no-op
	if l.offset != 1 {
		t.Errorf("offset = %d, want 1 (no-op)", l.offset)
	}
}

// ── HandleTouch ──────────────────────────────────────────────────────────────

func TestScrollableList_TapRow0(t *testing.T) {
	l, stubs := newTestList5()
	setBoundsMenu(l) // row 0: y=22..46
	l.HandleTouch(touch.TouchPoint{X: 100, Y: 34}) // y=34 → row 0
	if !stubs[0].tapped {
		t.Error("row 0 not tapped")
	}
}

func TestScrollableList_TapRow3(t *testing.T) {
	l, stubs := newTestList5()
	setBoundsMenu(l) // row 3: y=97..121
	l.HandleTouch(touch.TouchPoint{X: 100, Y: 109}) // y=109 → row 3
	if !stubs[3].tapped {
		t.Error("row 3 not tapped")
	}
}

func TestScrollableList_TapWithOffset(t *testing.T) {
	l, stubs := newTestList5()
	setBoundsMenu(l)
	l.offset = 1 // showing items 1..4
	l.HandleTouch(touch.TouchPoint{X: 100, Y: 34}) // row 0 → item index 1
	if !stubs[1].tapped {
		t.Error("item 1 not tapped with offset=1")
	}
}

// ── Draw ─────────────────────────────────────────────────────────────────────

func TestScrollableList_DrawDoesNotPanic(t *testing.T) {
	l, _ := newTestList5()
	setBoundsMenu(l)
	c := canvas.New(epd.Width, epd.Height, canvas.Rot90)
	l.Draw(c)
}

func TestScrollableList_DrawEmptyList(t *testing.T) {
	l := NewScrollableList(nil, 25)
	setBoundsMenu(l)
	c := canvas.New(epd.Width, epd.Height, canvas.Rot90)
	l.Draw(c) // must not panic
}

func TestScrollableList_DrawEmptyBounds(t *testing.T) {
	l, _ := newTestList5()
	// bounds zero → early return, no panic
	c := canvas.New(epd.Width, epd.Height, canvas.Rot90)
	l.Draw(c)
}
```

- [ ] **Step 2: Vérifier que les tests échouent**

```
go test ./ui/gui/... -run TestScrollableList
```
Expected: FAIL — `ScrollableList` undefined.

- [ ] **Step 3: Implémenter `widget_scrolllist.go`**

```go
// ui/gui/widget_scrolllist.go — generic composable scrollable list widget
package gui

import (
	"image"

	"github.com/oioio-space/oioni/drivers/touch"
	"github.com/oioio-space/oioni/ui/canvas"
)

// ListItem is implemented by each row in a ScrollableList.
// The caller defines rendering; ScrollableList handles scroll state and touch routing.
type ListItem interface {
	Draw(c *canvas.Canvas, bounds image.Rectangle)
	OnTap()
}

// ScrollableList is a responsive composable scrollable list widget.
// The number of visible rows is computed dynamically: visible = Bounds().Dy() / RowH.
// Pair with NavButton for ∧/∨ scroll controls.
//
// Usage:
//
//	list    := gui.NewScrollableList(items, 25)
//	upBtn   := gui.NewNavButton("^", list.ScrollUp, list.CanScrollUp)
//	downBtn := gui.NewNavButton("v", list.ScrollDown, list.CanScrollDown)
type ScrollableList struct {
	BaseWidget
	items  []ListItem
	offset int
	RowH   int // row height in pixels; set by caller
}

// NewScrollableList creates a ScrollableList with the given items and row height.
func NewScrollableList(items []ListItem, rowH int) *ScrollableList {
	l := &ScrollableList{items: items, RowH: rowH}
	l.SetDirty()
	return l
}

// visible returns the number of rows that fit in current bounds.
func (l *ScrollableList) visible() int {
	if l.RowH <= 0 {
		return 0
	}
	return l.Bounds().Dy() / l.RowH
}

// CanScrollUp returns true when the list is not at the top.
func (l *ScrollableList) CanScrollUp() bool { return l.offset > 0 }

// CanScrollDown returns true when items exist beyond the visible window.
// Uses addition to avoid underflow when len(items) < visible().
func (l *ScrollableList) CanScrollDown() bool {
	return l.offset+l.visible() < len(l.items)
}

// ScrollUp decrements the offset by one (no-op at top).
func (l *ScrollableList) ScrollUp() {
	if l.CanScrollUp() {
		l.offset--
		l.SetDirty()
	}
}

// ScrollDown increments the offset by one (no-op at bottom).
func (l *ScrollableList) ScrollDown() {
	if l.CanScrollDown() {
		l.offset++
		l.SetDirty()
	}
}

// HandleTouch routes the touch to the correct item by row index.
func (l *ScrollableList) HandleTouch(pt touch.TouchPoint) bool {
	wb := l.Bounds()
	if l.RowH <= 0 {
		return true
	}
	row := (int(pt.Y) - wb.Min.Y) / l.RowH
	vis := l.visible()
	if row >= 0 && row < vis {
		actual := l.offset + row
		if actual < len(l.items) {
			l.items[actual].OnTap()
		}
	}
	return true
}

// Draw renders the visible rows. Each item draws itself in its row bounds.
// A 2px separator is drawn between rows (e-ink safe: 1px lines can disappear).
func (l *ScrollableList) Draw(c *canvas.Canvas) {
	wb := l.Bounds()
	if wb.Empty() || l.RowH <= 0 {
		return
	}
	c.DrawRect(wb, canvas.White, true)
	vis := l.visible()
	for i := 0; i < vis; i++ {
		idx := l.offset + i
		if idx >= len(l.items) {
			break
		}
		rowBounds := image.Rect(
			wb.Min.X, wb.Min.Y+i*l.RowH,
			wb.Max.X, wb.Min.Y+(i+1)*l.RowH,
		)
		l.items[idx].Draw(c, rowBounds)
		// 2px separator between rows (not after last visible row, not if no next item)
		if i < vis-1 && idx+1 < len(l.items) {
			sep := rowBounds.Max.Y - 2
			c.DrawLine(wb.Min.X, sep, wb.Max.X, sep, canvas.Black)
			c.DrawLine(wb.Min.X, sep+1, wb.Max.X, sep+1, canvas.Black)
		}
	}
}
```

- [ ] **Step 4: Vérifier que les tests passent**

```
go test ./ui/gui/... -run TestScrollableList -v
```
Expected: tous PASS.

- [ ] **Step 5: Commit**

```bash
git add ui/gui/widget_scrolllist.go ui/gui/widget_scrolllist_test.go
git commit -m "feat(gui): add composable ScrollableList widget with ListItem interface"
```

---

## Task 2 : `gui.NavButton`

**Files:**
- Create: `ui/gui/widget_navbutton.go`
- Create: `ui/gui/widget_navbutton_test.go`

⚠️ `textWidth()` est déjà défini dans `ui/gui/widgets.go` — ne pas le redéfinir.

- [ ] **Step 1: Écrire les tests**

```go
// ui/gui/widget_navbutton_test.go
package gui

import (
	"image"
	"testing"

	"github.com/oioio-space/oioni/drivers/epd"
	"github.com/oioio-space/oioni/drivers/touch"
	"github.com/oioio-space/oioni/ui/canvas"
)

func setNavBtnBounds(b *NavButton) {
	b.SetBounds(image.Rect(200, 22, 250, 72)) // 50×50px
}

func TestNavButton_TapCallsOnTap(t *testing.T) {
	called := false
	b := NewNavButton("^", func() { called = true }, func() bool { return true })
	setNavBtnBounds(b)
	b.HandleTouch(touch.TouchPoint{X: 225, Y: 47})
	if !called {
		t.Error("onTap not called")
	}
}

func TestNavButton_TapWhenDisabledStillCallsOnTap(t *testing.T) {
	// NavButton fires onTap regardless of active state — onTap decides the no-op logic.
	called := false
	b := NewNavButton("^", func() { called = true }, func() bool { return false })
	setNavBtnBounds(b)
	b.HandleTouch(touch.TouchPoint{X: 225, Y: 47})
	if !called {
		t.Error("onTap not called when disabled")
	}
}

func TestNavButton_NilOnTap_Noop(t *testing.T) {
	b := NewNavButton("^", nil, func() bool { return true })
	setNavBtnBounds(b)
	b.HandleTouch(touch.TouchPoint{X: 225, Y: 47}) // must not panic
}

func TestNavButton_NilIsActive_DefaultFalse(t *testing.T) {
	b := NewNavButton("^", func() {}, nil) // nil isActive → default to false
	b.SetBounds(image.Rect(0, 0, 50, 50))
	c := canvas.New(epd.Width, epd.Height, canvas.Rot90)
	b.Draw(c) // must not panic (disabled path)
}

func TestNavButton_DrawActiveDoesNotPanic(t *testing.T) {
	b := NewNavButton("^", func() {}, func() bool { return true })
	setNavBtnBounds(b)
	c := canvas.New(epd.Width, epd.Height, canvas.Rot90)
	b.Draw(c)
}

func TestNavButton_DrawDisabledDoesNotPanic(t *testing.T) {
	b := NewNavButton("v", func() {}, func() bool { return false })
	setNavBtnBounds(b)
	c := canvas.New(epd.Width, epd.Height, canvas.Rot90)
	b.Draw(c)
}

func TestNavButton_DrawEmptyBounds(t *testing.T) {
	b := NewNavButton("^", func() {}, func() bool { return true })
	// bounds zero → early return, no panic
	c := canvas.New(epd.Width, epd.Height, canvas.Rot90)
	b.Draw(c)
}
```

- [ ] **Step 2: Vérifier que les tests échouent**

```
go test ./ui/gui/... -run TestNavButton
```
Expected: FAIL — `NavButton` undefined.

- [ ] **Step 3: Implémenter `widget_navbutton.go`**

```go
// ui/gui/widget_navbutton.go — NavButton: tap button with active/disabled state
package gui

import (
	"github.com/oioio-space/oioni/drivers/touch"
	"github.com/oioio-space/oioni/ui/canvas"
)

// NavButton is a tap button with an active/disabled visual state.
// Typically used for scroll controls (∧/∨) alongside a ScrollableList.
//
// The button always fires onTap on touch — the disabled state is visual only.
// The caller's onTap function (e.g. ScrollableList.ScrollUp) handles no-op logic.
// Rendering is responsive: symbol and disabled bar are centered in actual Bounds().
type NavButton struct {
	BaseWidget
	sym      string
	onTap    func()
	isActive func() bool
}

// NewNavButton creates a NavButton.
//   - sym: ASCII symbol displayed when active (e.g. "^" or "v")
//   - onTap: called on every touch, regardless of active state; nil = no-op
//   - isActive: returns true when button appears enabled; nil → always disabled
func NewNavButton(sym string, onTap func(), isActive func() bool) *NavButton {
	if isActive == nil {
		isActive = func() bool { return false }
	}
	b := &NavButton{sym: sym, onTap: onTap, isActive: isActive}
	b.SetDirty()
	return b
}

// HandleTouch fires onTap. Touch routing is handled by the Navigator.
func (b *NavButton) HandleTouch(_ touch.TouchPoint) bool {
	if b.onTap != nil {
		b.onTap()
	}
	return true
}

// Draw renders the button: border + symbol (active) or 8px bar (disabled).
// All positioning is relative to Bounds(), so the widget works in any container.
func (b *NavButton) Draw(c *canvas.Canvas) {
	r := b.Bounds()
	if r.Empty() {
		return
	}
	c.DrawRect(r, canvas.White, true)
	c.DrawRect(r, canvas.Black, false)

	cx := r.Min.X + r.Dx()/2
	cy := r.Min.Y + r.Dy()/2

	if !b.isActive() {
		// Disabled: 8px horizontal bar (e-ink disabled convention)
		c.DrawLine(cx-4, cy, cx+4, cy, canvas.Black)
		return
	}
	f := canvas.EmbeddedFont(12)
	if f == nil {
		return
	}
	// Center symbol in bounds
	tw := textWidth(b.sym, f) // textWidth is in ui/gui/widgets.go, same package
	tx := r.Min.X + (r.Dx()-tw)/2
	ty := r.Min.Y + (r.Dy()-f.LineHeight())/2
	c.DrawText(tx, ty, b.sym, f, canvas.Black)
}
```

- [ ] **Step 4: Vérifier que les tests passent**

```
go test ./ui/gui/... -run TestNavButton -v
```
Expected: tous PASS.

- [ ] **Step 5: Vérifier que tous les tests `ui/gui/` passent encore**

```
go test ./ui/gui/...
```
Expected: PASS (aucune régression).

- [ ] **Step 6: Commit**

```bash
git add ui/gui/widget_navbutton.go ui/gui/widget_navbutton_test.go
git commit -m "feat(gui): add NavButton widget with active/disabled state"
```

---

## Task 3 : `homeListItem` + câblage `home.go`

**Files:**
- Rewrite: `cmd/oioni/ui/menu.go`
- Rewrite: `cmd/oioni/ui/menu_test.go`
- Modify: `cmd/oioni/ui/home.go`

- [ ] **Step 1: Réécrire `menu_test.go`**

```go
// cmd/oioni/ui/menu_test.go
package ui

import (
	"image"
	"testing"

	"github.com/oioio-space/oioni/drivers/epd"
	"github.com/oioio-space/oioni/ui/canvas"
)

func TestHomeListItem_OnTap(t *testing.T) {
	called := false
	item := &homeListItem{name: "Config", onTap: func() { called = true }}
	item.OnTap()
	if !called {
		t.Error("onTap not called")
	}
}

func TestHomeListItem_OnTapNilIsNoOp(t *testing.T) {
	item := &homeListItem{name: "Config"} // onTap nil
	item.OnTap()                          // must not panic
}

func TestHomeListItem_DrawDoesNotPanic(t *testing.T) {
	item := &homeListItem{name: "Config"}
	r := image.Rect(0, 22, 200, 47) // one homeRowH=25px slot
	c := canvas.New(epd.Width, epd.Height, canvas.Rot90)
	item.Draw(c, r)
}

func TestHomeListItem_DrawWithIcon(t *testing.T) {
	item := &homeListItem{name: "Attack", icon: Icons.Attack}
	r := image.Rect(0, 22, 200, 47)
	c := canvas.New(epd.Width, epd.Height, canvas.Rot90)
	item.Draw(c, r) // must not panic even with real icon
}
```

- [ ] **Step 2: Vérifier que les tests échouent**

```
go test ./cmd/oioni/ui/... -run TestHomeListItem
```
Expected: FAIL — `homeListItem` pas encore implémenté avec la bonne API.

- [ ] **Step 3: Réécrire `menu.go`**

```go
// cmd/oioni/ui/menu.go — homeListItem: gui.ListItem implementation for the home menu
package ui

import (
	"image"

	"github.com/oioio-space/oioni/ui/canvas"
	"github.com/oioio-space/oioni/ui/gui"
)

const (
	homeRowH     = 25 // 4 rows × 25px = 100px (122px total - 22px NSB)
	homeNavW     = 50 // nav column width (∧/∨ buttons)
	homeIconSize = 16 // icon size in px (scaled from 32×32 source)
	homeIconX    = 4  // left margin for icon
	homeIconYOff = 4  // (homeRowH - homeIconSize) / 2 — vertical center
	homeTextX    = 24 // homeIconX + homeIconSize + 4 — text start
)

// homeListItem implements gui.ListItem for the home menu.
// Renders a 16×16 icon on the left and the item name vertically centered.
type homeListItem struct {
	name  string
	icon  gui.Icon
	onTap func()
}

// Draw renders the item within its row bounds.
func (h *homeListItem) Draw(c *canvas.Canvas, r image.Rectangle) {
	// Icon (16×16, vertically centered in row)
	h.icon.Draw(c, image.Rect(
		r.Min.X+homeIconX,
		r.Min.Y+homeIconYOff,
		r.Min.X+homeIconX+homeIconSize,
		r.Min.Y+homeIconYOff+homeIconSize,
	))

	// Name — vertically centered
	f := canvas.EmbeddedFont(12)
	if f != nil {
		ty := r.Min.Y + (r.Dy()-f.LineHeight())/2
		c.DrawText(r.Min.X+homeTextX, ty, h.name, f, canvas.Black)
	}
}

// OnTap fires the item's action (no-op if nil).
func (h *homeListItem) OnTap() {
	if h.onTap != nil {
		h.onTap()
	}
}
```

- [ ] **Step 4: Vérifier que les tests `homeListItem` passent**

```
go test ./cmd/oioni/ui/... -run TestHomeListItem -v
```
Expected: tous PASS.

- [ ] **Step 5: Modifier `home.go`**

// NOTE: items sont des `*homeListItem` (pointeur vers struct qui implémente gui.ListItem).
// NE PAS utiliser de struct literal sur gui.ListItem — c'est une interface, pas un struct.

```go
// cmd/oioni/ui/home.go — HomeScene: operator-style menu home screen
package ui

import (
	"image"

	"github.com/oioio-space/oioni/drivers/epd"
	"github.com/oioio-space/oioni/ui/gui"
)

// NewHomeScene builds the home screen: 22px NetworkStatusBar + scrollable menu.
// Returns the scene and the NetworkStatusBar so the caller can call SetInterfaces/SetTools.
func NewHomeScene(nav *gui.Navigator) (*gui.Scene, *gui.NetworkStatusBar) {
	nsb := gui.NewNetworkStatusBar(nav)

	items := []gui.ListItem{
		&homeListItem{name: "Config", icon: Icons.Config, onTap: func() { nav.Dispatch(func() { nav.Push(NewConfigScene(nav)) }) }}, //nolint:errcheck
		&homeListItem{name: "System", icon: Icons.System, onTap: func() { nav.Dispatch(func() { nav.Push(NewSystemScene(nav)) }) }}, //nolint:errcheck
		&homeListItem{name: "Attack", icon: Icons.Attack, onTap: func() { nav.Dispatch(func() { nav.Push(NewAttackScene(nav)) }) }}, //nolint:errcheck
		&homeListItem{name: "DFIR",   icon: Icons.DFIR,   onTap: func() { nav.Dispatch(func() { nav.Push(NewDFIRScene(nav)) }) }}, //nolint:errcheck
		&homeListItem{name: "Info",   icon: Icons.Info,   onTap: func() { nav.Dispatch(func() { nav.Push(NewInfoScene(nav)) }) }}, //nolint:errcheck
	}

	list    := gui.NewScrollableList(items, homeRowH)
	upBtn   := gui.NewNavButton("^", list.ScrollUp, list.CanScrollUp)
	downBtn := gui.NewNavButton("v", list.ScrollDown, list.CanScrollDown)

	navCol  := gui.NewVBox(gui.Expand(upBtn), gui.Expand(downBtn))
	menuRow := gui.NewHBox(gui.Expand(list), gui.FixedSize(navCol, homeNavW))
	content := gui.NewVBox(gui.FixedSize(nsb, 22), gui.Expand(menuRow))
	content.SetBounds(image.Rect(0, 0, epd.Height, epd.Width))

	return &gui.Scene{
		Title:   "Home",
		// nsb, list, upBtn, downBtn at top level for Navigator automatic touch routing by bounds.
		Widgets: []gui.Widget{content, nsb, list, upBtn, downBtn},
	}, nsb
}
```

- [ ] **Step 6: Vérifier que tout compile et que tous les tests passent**

```
go test ./cmd/oioni/ui/... -v
go test ./ui/gui/...
```
Expected: tous PASS.

- [ ] **Step 7: Commit**

```bash
git add cmd/oioni/ui/menu.go cmd/oioni/ui/menu_test.go cmd/oioni/ui/home.go
git commit -m "refactor(home): homeListItem implements gui.ListItem, use gui.ScrollableList+NavButton"
```

---

## Vérification finale

```
go test ./...
```
Expected: tous PASS, aucune régression.
