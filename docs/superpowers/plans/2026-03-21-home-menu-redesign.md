# Home Menu Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remplacer le HomeMenuWidget monolithique par une composition de widgets indépendants : `ScrollableMenuList` + `NavButton`, avec touch routing automatique via le Navigator.

**Architecture:** Trois widgets dans `scene.Widgets` pour le touch routing automatique (Navigator route par bounds). `content` VBox/HBox pour le layout visuel. Les nav buttons sont câblés à la liste via closures — aucun état partagé global.

**Tech Stack:** Go, `ui/gui` (BaseWidget, Icon, VBox, HBox, FixedSize, Expand), `ui/canvas`, `drivers/touch`

---

## File Structure

| Fichier | Action |
|---------|--------|
| `cmd/oioni/ui/menu.go` | Réécriture : `homeMenuItem` + `ScrollableMenuList` + `NavButton` |
| `cmd/oioni/ui/menu_test.go` | Réécriture : tests des deux widgets |
| `cmd/oioni/ui/home.go` | Composition et câblage |

---

### Task 1: ScrollableMenuList + NavButton (TDD)

**Files:**
- Modify: `cmd/oioni/ui/menu.go`
- Modify: `cmd/oioni/ui/menu_test.go`

**Contexte :**
- Display logique 250×122px. Menu area y=22..121 (100px). Liste : x=0..199. Nav : x=200..249.
- Le Navigator appelle `HandleTouch` avec des coordonnées **logiques** déjà converties.
- Touch routing automatique : chaque widget dans `scene.Widgets` reçoit les touches dont le point est dans ses bounds. Le widget n'a pas besoin de vérifier `In(bounds)` si le Navigator garantit cette précondition — mais vérifier reste défensif.
- `gui.Icon.Draw(c *canvas.Canvas, r image.Rectangle)` — render l'icône dans le rectangle, no-op si zero value.
- `canvas.EmbeddedFont(size)` — retourne nil si taille non supportée, toujours vérifier.
- `canvas.Font.Glyph(r rune) (data []byte, width, height int)` et `LineHeight() int`.
- Utiliser ASCII : "^" pour ∧, "v" pour ∨ (le font bitmap ne supporte que l'ASCII).
- `NavButton.SetDirty()` n'est pas nécessaire après un tap : c'est la liste qui se marque dirty, et le Navigator re-rend tous les widgets (y compris les boutons qui appellent `isActive()` frais à chaque Draw).

- [ ] **Step 1: Écrire les tests qui échouent**

Remplacer `cmd/oioni/ui/menu_test.go` par :

```go
// cmd/oioni/ui/menu_test.go
package ui

import (
	"image"
	"testing"

	"github.com/oioio-space/oioni/drivers/epd"
	"github.com/oioio-space/oioni/drivers/touch"
	"github.com/oioio-space/oioni/ui/canvas"
)

// Bounds en production : liste y=22..121, x=0..199 (menu area minus nav column).
func setListBounds(l *ScrollableMenuList) {
	l.SetBounds(image.Rect(0, 22, 200, 122))
}

// Bounds en production : upBtn y=22..71, x=200..249.
func setUpBtnBounds(b *NavButton) {
	b.SetBounds(image.Rect(200, 22, 250, 72))
}

// Bounds en production : downBtn y=72..121, x=200..249.
func setDownBtnBounds(b *NavButton) {
	b.SetBounds(image.Rect(200, 72, 250, 122))
}

func newTestList() *ScrollableMenuList {
	return newScrollableMenuList([]homeMenuItem{
		{name: "Config", desc: "reseau"},
		{name: "System", desc: "services"},
		{name: "Attack", desc: "MITM"},
		{name: "DFIR", desc: "capture"},
		{name: "Info", desc: "aide"},
	})
}

// ── ScrollableMenuList ────────────────────────────────────────────────────────

func TestScrollableMenuList_PreferredSize(t *testing.T) {
	l := newTestList()
	sz := l.PreferredSize()
	want := menuVisible * menuRowH
	if sz.Y != want {
		t.Errorf("PreferredSize().Y = %d, want %d", sz.Y, want)
	}
}

func TestScrollableMenuList_ScrollDown(t *testing.T) {
	l := newTestList()
	l.ScrollDown()
	if l.offset != 1 {
		t.Errorf("offset = %d, want 1", l.offset)
	}
}

func TestScrollableMenuList_ScrollUp(t *testing.T) {
	l := newTestList()
	l.offset = 1
	l.ScrollUp()
	if l.offset != 0 {
		t.Errorf("offset = %d, want 0", l.offset)
	}
}

func TestScrollableMenuList_ScrollUpAtTop(t *testing.T) {
	l := newTestList()
	l.ScrollUp() // no-op at top
	if l.offset != 0 {
		t.Errorf("offset = %d, want 0 (no-op)", l.offset)
	}
}

func TestScrollableMenuList_ScrollDownAtBottom(t *testing.T) {
	// 5 items, menuVisible=2 → max offset=3
	l := newTestList()
	l.offset = 3
	l.ScrollDown() // no-op at bottom
	if l.offset != 3 {
		t.Errorf("offset = %d, want 3 (no-op)", l.offset)
	}
}

func TestScrollableMenuList_CanScrollUp(t *testing.T) {
	l := newTestList()
	if l.CanScrollUp() {
		t.Error("CanScrollUp() = true at offset 0, want false")
	}
	l.offset = 1
	if !l.CanScrollUp() {
		t.Error("CanScrollUp() = false at offset 1, want true")
	}
}

func TestScrollableMenuList_CanScrollDown(t *testing.T) {
	l := newTestList()
	if !l.CanScrollDown() {
		t.Error("CanScrollDown() = false at offset 0, want true")
	}
	l.offset = 3 // max
	if l.CanScrollDown() {
		t.Error("CanScrollDown() = true at max offset, want false")
	}
}

func TestScrollableMenuList_TapRow0(t *testing.T) {
	called := ""
	items := []homeMenuItem{
		{name: "A", desc: "a", onTap: func() { called = "A" }},
		{name: "B", desc: "b", onTap: func() { called = "B" }},
		{name: "C", desc: "c"},
		{name: "D", desc: "d"},
		{name: "E", desc: "e"},
	}
	l := newScrollableMenuList(items)
	setListBounds(l)
	// row 0: y = 22..71 → center y=47
	l.HandleTouch(touch.TouchPoint{X: 100, Y: 47})
	if called != "A" {
		t.Errorf("expected A, got %q", called)
	}
}

func TestScrollableMenuList_TapRow1(t *testing.T) {
	called := ""
	items := []homeMenuItem{
		{name: "A", desc: "a", onTap: func() { called = "A" }},
		{name: "B", desc: "b", onTap: func() { called = "B" }},
		{name: "C", desc: "c"},
		{name: "D", desc: "d"},
		{name: "E", desc: "e"},
	}
	l := newScrollableMenuList(items)
	setListBounds(l)
	// row 1: y = 72..121 → center y=97
	l.HandleTouch(touch.TouchPoint{X: 100, Y: 97})
	if called != "B" {
		t.Errorf("expected B, got %q", called)
	}
}

func TestScrollableMenuList_TapNilOnTapIsNoOp(t *testing.T) {
	items := []homeMenuItem{
		{name: "A", desc: "a"}, // onTap nil
		{name: "B", desc: "b"},
		{name: "C", desc: "c"},
		{name: "D", desc: "d"},
		{name: "E", desc: "e"},
	}
	l := newScrollableMenuList(items)
	setListBounds(l)
	l.HandleTouch(touch.TouchPoint{X: 100, Y: 47}) // ne doit pas paniquer
}

func TestScrollableMenuList_DrawDoesNotPanic(t *testing.T) {
	l := newTestList()
	setListBounds(l)
	c := canvas.New(epd.Width, epd.Height, canvas.Rot90)
	l.Draw(c)
}

// ── NavButton ─────────────────────────────────────────────────────────────────

func TestNavButton_TapCallsOnTap(t *testing.T) {
	called := false
	b := newNavButton("^", func() { called = true }, func() bool { return true })
	setUpBtnBounds(b)
	b.HandleTouch(touch.TouchPoint{X: 225, Y: 47})
	if !called {
		t.Error("onTap not called")
	}
}

func TestNavButton_TapWhenDisabledStillCallsOnTap(t *testing.T) {
	// Le NavButton appelle toujours onTap — c'est onTap qui décide si c'est no-op.
	called := false
	b := newNavButton("^", func() { called = true }, func() bool { return false })
	setUpBtnBounds(b)
	b.HandleTouch(touch.TouchPoint{X: 225, Y: 47})
	if !called {
		t.Error("onTap not called even when disabled")
	}
}

func TestNavButton_DrawActiveDoesNotPanic(t *testing.T) {
	b := newNavButton("^", func() {}, func() bool { return true })
	setUpBtnBounds(b)
	c := canvas.New(epd.Width, epd.Height, canvas.Rot90)
	b.Draw(c)
}

func TestNavButton_DrawDisabledDoesNotPanic(t *testing.T) {
	b := newNavButton("^", func() {}, func() bool { return false })
	setUpBtnBounds(b)
	c := canvas.New(epd.Width, epd.Height, canvas.Rot90)
	b.Draw(c)
}
```

- [ ] **Step 2: Vérifier que les tests échouent**

```bash
cd /home/oioio/Documents/GolandProjects/oioni
go test ./cmd/oioni/ui/... 2>&1 | head -20
```

Résultat attendu : erreurs de compilation (`ScrollableMenuList`, `NavButton`, etc. inconnus).

- [ ] **Step 3: Réécrire menu.go**

Remplacer `cmd/oioni/ui/menu.go` par :

```go
// cmd/oioni/ui/menu.go — ScrollableMenuList + NavButton for the home screen
package ui

import (
	"image"

	"github.com/oioio-space/oioni/drivers/touch"
	"github.com/oioio-space/oioni/ui/canvas"
	"github.com/oioio-space/oioni/ui/gui"
)

const (
	menuRowH     = 50
	menuVisible  = 2
	menuNavW     = 50
	menuIconX    = 8
	menuIconSize = 32
	menuIconYOff = 9  // = (menuRowH - menuIconSize) / 2
	menuTextX    = 48 // = menuIconX + menuIconSize + 8
)

type homeMenuItem struct {
	name  string
	desc  string
	icon  gui.Icon
	onTap func()
}

// ── ScrollableMenuList ────────────────────────────────────────────────────────

// ScrollableMenuList renders 2 rows at a time from a list of items.
// Scroll state is managed via ScrollUp/ScrollDown called by NavButton closures.
type ScrollableMenuList struct {
	gui.BaseWidget
	items  []homeMenuItem
	offset int
}

func newScrollableMenuList(items []homeMenuItem) *ScrollableMenuList {
	l := &ScrollableMenuList{items: items}
	l.SetDirty()
	return l
}

func (l *ScrollableMenuList) PreferredSize() image.Point { return image.Pt(0, menuVisible*menuRowH) }
func (l *ScrollableMenuList) MinSize() image.Point       { return image.Pt(0, menuVisible*menuRowH) }

func (l *ScrollableMenuList) CanScrollUp() bool { return l.offset > 0 }
func (l *ScrollableMenuList) CanScrollDown() bool {
	return l.offset < len(l.items)-menuVisible
}

func (l *ScrollableMenuList) ScrollUp() {
	if l.CanScrollUp() {
		l.offset--
		l.SetDirty()
	}
}

func (l *ScrollableMenuList) ScrollDown() {
	if l.CanScrollDown() {
		l.offset++
		l.SetDirty()
	}
}

func (l *ScrollableMenuList) HandleTouch(pt touch.TouchPoint) bool {
	wb := l.Bounds()
	row := (int(pt.Y) - wb.Min.Y) / menuRowH
	if row >= 0 && row < menuVisible {
		actual := l.offset + row
		if actual < len(l.items) && l.items[actual].onTap != nil {
			l.items[actual].onTap()
		}
	}
	return true
}

func (l *ScrollableMenuList) Draw(c *canvas.Canvas) {
	wb := l.Bounds()
	if wb.Empty() {
		return
	}
	c.DrawRect(wb, canvas.White, true)

	f12 := canvas.EmbeddedFont(12)
	f8 := canvas.EmbeddedFont(8)

	for i := 0; i < menuVisible; i++ {
		idx := l.offset + i
		if idx >= len(l.items) {
			break
		}
		item := l.items[idx]
		rowTop := wb.Min.Y + i*menuRowH

		// Icon (32×32, centered vertically in row)
		item.icon.Draw(c, image.Rect(
			wb.Min.X+menuIconX,
			rowTop+menuIconYOff,
			wb.Min.X+menuIconX+menuIconSize,
			rowTop+menuIconYOff+menuIconSize,
		))

		// Name
		if f12 != nil {
			c.DrawText(wb.Min.X+menuTextX, rowTop+6, item.name, f12, canvas.Black)
		}

		// Description
		if f8 != nil {
			c.DrawText(wb.Min.X+menuTextX, rowTop+28, item.desc, f8, canvas.Black)
		}

		// Row separator (between rows only)
		if i < menuVisible-1 {
			sep := rowTop + menuRowH - 1
			c.DrawLine(wb.Min.X, sep, wb.Max.X, sep, canvas.Black)
		}
	}
}

// ── NavButton ─────────────────────────────────────────────────────────────────

// NavButton is a 50×50px tap button. isActive controls the rendered state.
// onTap is always called on touch — the caller (ScrollUp/ScrollDown) handles no-op logic.
type NavButton struct {
	gui.BaseWidget
	sym      string
	onTap    func()
	isActive func() bool
}

func newNavButton(sym string, onTap func(), isActive func() bool) *NavButton {
	b := &NavButton{sym: sym, onTap: onTap, isActive: isActive}
	b.SetDirty()
	return b
}

func (b *NavButton) PreferredSize() image.Point { return image.Pt(menuNavW, menuNavW) }
func (b *NavButton) MinSize() image.Point       { return image.Pt(menuNavW, menuNavW) }

func (b *NavButton) HandleTouch(pt touch.TouchPoint) bool {
	if b.onTap != nil {
		b.onTap()
	}
	return true
}

func (b *NavButton) Draw(c *canvas.Canvas) {
	r := b.Bounds()
	if r.Empty() {
		return
	}
	c.DrawRect(r, canvas.White, true)
	c.DrawRect(r, canvas.Black, false)

	f12 := canvas.EmbeddedFont(12)

	if !b.isActive() {
		// Disabled: horizontal bar
		cx := r.Min.X + r.Dx()/2
		cy := r.Min.Y + r.Dy()/2
		c.DrawLine(cx-4, cy, cx+4, cy, canvas.Black)
		return
	}
	if f12 == nil {
		return
	}
	// Center symbol
	tw := 0
	for _, ch := range b.sym {
		_, w, _ := f12.Glyph(ch)
		tw += w
	}
	tx := r.Min.X + (r.Dx()-tw)/2
	ty := r.Min.Y + (r.Dy()-f12.LineHeight())/2
	c.DrawText(tx, ty, b.sym, f12, canvas.Black)
}
```

- [ ] **Step 4: Vérifier que les tests passent**

```bash
cd /home/oioio/Documents/GolandProjects/oioni
go test ./cmd/oioni/ui/... -v 2>&1 | grep -E "PASS|FAIL|---"
```

Résultat attendu : tous les tests `PASS`. Si `home.go` échoue à compiler (ancien `HomeMenuWidget` introuvable), c'est normal — sera corrigé en Task 2.

- [ ] **Step 5: Commit**

```bash
git add cmd/oioni/ui/menu.go cmd/oioni/ui/menu_test.go
git commit -m "feat(ui): split home menu into ScrollableMenuList + NavButton widgets"
```

---

### Task 2: Composition dans home.go

**Files:**
- Modify: `cmd/oioni/ui/home.go`

**Contexte :**
- `gui.NewVBox(...)` et `gui.NewHBox(...)` acceptent des widgets ou des `gui.FixedSize`/`gui.Expand` hints.
- `gui.FixedSize(widget, size)` fixe la taille cross-axis. `gui.Expand(widget)` consomme l'espace restant.
- `content.SetBounds(image.Rect(0, 0, epd.Height, epd.Width))` propage les bounds à tous les enfants — c'est ainsi que les bounds de `list`, `upBtn`, `downBtn` sont définis avant le premier rendu.
- Pattern touch routing : mettre `nsb`, `list`, `upBtn`, `downBtn` dans `scene.Widgets` directement (pas seulement dans `content`) pour que le Navigator les trouve par leurs bounds.
- `Icons.Config` etc. : variable globale `Icons` dans `icons.go`, peuplée par `init()`.
- Descriptions raccourcies : 8pt font ≈ 5px/char, liste 152px → max ~30 chars.

Cette tâche est une mise à jour de compilation + câblage. Le comportement est entièrement testé par Task 1.

- [ ] **Step 1: Mettre à jour home.go**

Remplacer `cmd/oioni/ui/home.go` par :

```go
// cmd/oioni/ui/home.go — HomeScene: operator-style menu home screen
package ui

import (
	"image"

	"github.com/oioio-space/oioni/drivers/epd"
	"github.com/oioio-space/oioni/ui/gui"
)

// NewHomeScene builds the home screen: 22px NetworkStatusBar + 100px scrollable menu.
// Returns the scene and the NetworkStatusBar so the caller can call SetInterfaces/SetTools.
func NewHomeScene(nav *gui.Navigator) (*gui.Scene, *gui.NetworkStatusBar) {
	nsb := gui.NewNetworkStatusBar(nav)

	list := newScrollableMenuList([]homeMenuItem{
		{
			name:  "Config",
			desc:  "reseau - interfaces",
			icon:  Icons.Config,
			onTap: func() { nav.Dispatch(func() { nav.Push(NewConfigScene(nav)) }) }, //nolint:errcheck
		},
		{
			name:  "System",
			desc:  "services - logs",
			icon:  Icons.System,
			onTap: func() { nav.Dispatch(func() { nav.Push(NewSystemScene(nav)) }) }, //nolint:errcheck
		},
		{
			name:  "Attack",
			desc:  "MITM - scan - deauth",
			icon:  Icons.Attack,
			onTap: func() { nav.Dispatch(func() { nav.Push(NewAttackScene(nav)) }) }, //nolint:errcheck
		},
		{
			name:  "DFIR",
			desc:  "capture - forensics",
			icon:  Icons.DFIR,
			onTap: func() { nav.Dispatch(func() { nav.Push(NewDFIRScene(nav)) }) }, //nolint:errcheck
		},
		{
			name:  "Info",
			desc:  "aide - a propos",
			icon:  Icons.Info,
			onTap: func() { nav.Dispatch(func() { nav.Push(NewInfoScene(nav)) }) }, //nolint:errcheck
		},
	})

	upBtn   := newNavButton("^", list.ScrollUp,   list.CanScrollUp)
	downBtn := newNavButton("v", list.ScrollDown, list.CanScrollDown)

	navCol := gui.NewVBox(
		gui.Expand(upBtn),
		gui.Expand(downBtn),
	)
	menuRow := gui.NewHBox(
		gui.Expand(list),
		gui.FixedSize(navCol, menuNavW),
	)
	content := gui.NewVBox(
		gui.FixedSize(nsb, 22),
		gui.Expand(menuRow),
	)
	content.SetBounds(image.Rect(0, 0, epd.Height, epd.Width))

	return &gui.Scene{
		Title: "Home",
		// nsb, list, upBtn, downBtn au top level pour que le Navigator
		// route les touches automatiquement via leurs bounds.
		Widgets: []gui.Widget{content, nsb, list, upBtn, downBtn},
	}, nsb
}
```

- [ ] **Step 2: Build complet**

```bash
cd /home/oioio/Documents/GolandProjects/oioni
go build ./cmd/oioni/... 2>&1
```

Résultat attendu : aucune erreur.

- [ ] **Step 3: Tous les tests**

```bash
go test ./cmd/oioni/... ./ui/gui/... 2>&1 | tail -20
```

Résultat attendu : tous PASS.

- [ ] **Step 4: Commit**

```bash
git add cmd/oioni/ui/home.go
git commit -m "feat(ui): compose home screen from ScrollableMenuList + NavButton + NSB"
```

---

## OTA après implémentation

```bash
podman save localhost/oioni/impacket:arm64 | gzip > /tmp/impacket-arm64.tar.gz
GOWORK=off gok update --parent_dir . -i oioio
```
