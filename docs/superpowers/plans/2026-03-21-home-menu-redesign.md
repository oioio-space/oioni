# Home Menu Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remplacer le HomeMenuWidget (20px rangées, cercles, pas de scroll) par un menu tactile e-ink adapté : 2 rangées de 50px avec boutons ∧/∨ et vraies icônes PNG.

**Architecture:** `menu.go` est réécrit entièrement (nouveaux structs, logique scroll, draw). `home.go` est mis à jour pour passer les icônes. Les tests dans `menu_test.go` sont réécrits pour les nouveaux comportements.

**Tech Stack:** Go, `ui/canvas` (DrawRect/DrawText/DrawLine), `ui/gui` (BaseWidget, Icon), `drivers/touch`

---

## File Structure

| Fichier | Action |
|---------|--------|
| `cmd/oioni/ui/menu.go` | Réécriture complète |
| `cmd/oioni/ui/menu_test.go` | Réécriture complète |
| `cmd/oioni/ui/home.go` | Ajout `icon:` dans chaque homeMenuItem |

---

### Task 1: Réécriture menu.go + menu_test.go (TDD)

**Files:**
- Modify: `cmd/oioni/ui/menu.go`
- Modify: `cmd/oioni/ui/menu_test.go`

**Contexte :**
- Display logique 250×122px. Menu widget occupera y=22..121 (100px), x=0..249 (250px).
- Panneau liste : x=0..199. Panneau nav : x=200..249 (50px), ∧ en haut (50px), ∨ en bas (50px).
- `HandleTouch` reçoit des coordonnées **logiques** (le Navigator fait la conversion physique→logique avant d'appeler le widget).
- Font ASCII uniquement → utiliser "^" pour ∧ et "v" pour ∨.
- `gui.Icon.Draw(c *canvas.Canvas, r image.Rectangle)` — render l'icône dans le rectangle donné.
- `canvas.EmbeddedFont(size int) canvas.Font` — renvoie nil si taille non supportée, toujours vérifier.
- `canvas.Font.Glyph(r rune) (data []byte, width, height int)` et `LineHeight() int`.

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

// setBounds place le widget sur y=22..121 comme en production.
func setMenuBounds(m *HomeMenuWidget) {
	m.SetBounds(image.Rect(0, 22, 250, 122))
}

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

func TestHomeMenu_PreferredSize(t *testing.T) {
	m := newTestMenu()
	sz := m.PreferredSize()
	if sz.Y != menuVisible*menuRowH {
		t.Errorf("PreferredSize().Y = %d, want %d", sz.Y, menuVisible*menuRowH)
	}
}

func TestHomeMenu_ScrollDown(t *testing.T) {
	m := newTestMenu()
	setMenuBounds(m)
	// Tap zone ∨ : x=225 (dans nav column), y=97 (dans la moitié basse de 100px menu: 22+50=72 → 97>72)
	m.HandleTouch(touch.TouchPoint{X: 225, Y: 97})
	if m.offset != 1 {
		t.Errorf("offset = %d, want 1", m.offset)
	}
}

func TestHomeMenu_ScrollUp(t *testing.T) {
	m := newTestMenu()
	m.offset = 1
	setMenuBounds(m)
	// Tap zone ∧ : x=225, y=47 (dans la moitié haute: 22 <= 47 < 72)
	m.HandleTouch(touch.TouchPoint{X: 225, Y: 47})
	if m.offset != 0 {
		t.Errorf("offset = %d, want 0", m.offset)
	}
}

func TestHomeMenu_ScrollUpAtTop(t *testing.T) {
	m := newTestMenu()
	m.offset = 0
	setMenuBounds(m)
	m.HandleTouch(touch.TouchPoint{X: 225, Y: 47}) // ∧
	if m.offset != 0 {
		t.Errorf("offset = %d, want 0 (no-op at top)", m.offset)
	}
}

func TestHomeMenu_ScrollDownAtBottom(t *testing.T) {
	// 5 items, menuVisible=2 → max offset = 3
	m := newTestMenu()
	m.offset = 3
	setMenuBounds(m)
	m.HandleTouch(touch.TouchPoint{X: 225, Y: 97}) // ∨
	if m.offset != 3 {
		t.Errorf("offset = %d, want 3 (no-op at bottom)", m.offset)
	}
}

func TestHomeMenu_TapRow1CallsSecondItem(t *testing.T) {
	called := ""
	items := []homeMenuItem{
		{name: "A", desc: "a", onTap: func() { called = "A" }},
		{name: "B", desc: "b", onTap: func() { called = "B" }},
		{name: "C", desc: "c"},
		{name: "D", desc: "d"},
		{name: "E", desc: "e"},
	}
	m := newHomeMenuWidget(items)
	setMenuBounds(m)
	// offset=0, row 1 = y = 22+50..22+99 → center y=97, x dans liste (x<200)
	m.HandleTouch(touch.TouchPoint{X: 100, Y: 97})
	if called != "B" {
		t.Errorf("expected B, got %q", called)
	}
}

func TestHomeMenu_TapNilOnTapIsNoOp(t *testing.T) {
	items := []homeMenuItem{
		{name: "A", desc: "a"}, // onTap nil
		{name: "B", desc: "b"},
		{name: "C", desc: "c"},
		{name: "D", desc: "d"},
		{name: "E", desc: "e"},
	}
	m := newHomeMenuWidget(items)
	setMenuBounds(m)
	// Ne doit pas paniquer
	m.HandleTouch(touch.TouchPoint{X: 100, Y: 47})
}

func TestHomeMenu_DrawDoesNotPanic(t *testing.T) {
	m := newTestMenu()
	setMenuBounds(m)
	c := canvas.New(epd.Width, epd.Height, canvas.Rot90)
	m.Draw(c)
}
```

- [ ] **Step 2: Vérifier que les tests échouent**

```bash
cd /home/oioio/Documents/GolandProjects/oioni
go test ./cmd/oioni/ui/... -run TestHomeMenu -v 2>&1 | head -40
```

Résultat attendu : erreurs de compilation (champs `selected`, constantes inchangées, etc.) ou FAIL.

- [ ] **Step 3: Réécrire menu.go**

Remplacer `cmd/oioni/ui/menu.go` par :

```go
// cmd/oioni/ui/menu.go — HomeMenuWidget: 2-row scrollable menu with ∧/∨ nav buttons
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

// HomeMenuWidget renders 2 rows at a time from a 5-item list,
// with ∧/∨ tap buttons (50×50px each) for scrolling.
type HomeMenuWidget struct {
	gui.BaseWidget
	items  []homeMenuItem
	offset int
}

func newHomeMenuWidget(items []homeMenuItem) *HomeMenuWidget {
	m := &HomeMenuWidget{items: items}
	m.SetDirty()
	return m
}

func (m *HomeMenuWidget) PreferredSize() image.Point { return image.Pt(0, menuVisible*menuRowH) }
func (m *HomeMenuWidget) MinSize() image.Point       { return image.Pt(0, menuVisible*menuRowH) }

func (m *HomeMenuWidget) HandleTouch(pt touch.TouchPoint) bool {
	wb := m.Bounds()
	if wb.Empty() {
		return false
	}
	px, py := int(pt.X), int(pt.Y)
	if !image.Pt(px, py).In(wb) {
		return false
	}

	navX := wb.Max.X - menuNavW
	if px >= navX {
		// Nav column: top half = ∧, bottom half = ∨
		midY := wb.Min.Y + wb.Dy()/2
		if py < midY {
			if m.offset > 0 {
				m.offset--
				m.SetDirty()
			}
		} else {
			maxOff := len(m.items) - menuVisible
			if m.offset < maxOff {
				m.offset++
				m.SetDirty()
			}
		}
		return true
	}

	// List column: which row?
	row := (py - wb.Min.Y) / menuRowH
	if row >= 0 && row < menuVisible {
		actual := m.offset + row
		if actual < len(m.items) && m.items[actual].onTap != nil {
			m.items[actual].onTap()
		}
	}
	return true
}

func (m *HomeMenuWidget) Draw(c *canvas.Canvas) {
	wb := m.Bounds()
	if wb.Empty() {
		return
	}

	c.DrawRect(wb, canvas.White, true)

	f12 := canvas.EmbeddedFont(12)
	f8 := canvas.EmbeddedFont(8)
	navX := wb.Max.X - menuNavW
	midY := wb.Min.Y + wb.Dy()/2

	// Visible rows
	for i := 0; i < menuVisible; i++ {
		idx := m.offset + i
		if idx >= len(m.items) {
			break
		}
		item := m.items[idx]
		rowTop := wb.Min.Y + i*menuRowH

		// Icon
		iconR := image.Rect(
			wb.Min.X+menuIconX,
			rowTop+menuIconYOff,
			wb.Min.X+menuIconX+menuIconSize,
			rowTop+menuIconYOff+menuIconSize,
		)
		item.icon.Draw(c, iconR)

		// Name
		if f12 != nil {
			c.DrawText(wb.Min.X+menuTextX, rowTop+6, item.name, f12, canvas.Black)
		}

		// Description
		if f8 != nil {
			c.DrawText(wb.Min.X+menuTextX, rowTop+28, item.desc, f8, canvas.Black)
		}

		// Row separator (between rows, not after last)
		if i < menuVisible-1 {
			sep := rowTop + menuRowH - 1
			c.DrawLine(wb.Min.X, sep, navX-1, sep, canvas.Black)
		}
	}

	// Nav buttons (borders; left border of upR doubles as the vertical separator)
	upR := image.Rect(navX, wb.Min.Y, wb.Max.X, midY)
	downR := image.Rect(navX, midY, wb.Max.X, wb.Max.Y)
	c.DrawRect(upR, canvas.Black, false)
	c.DrawRect(downR, canvas.Black, false)

	maxOff := len(m.items) - menuVisible
	drawNavBtn(c, upR, "^", m.offset == 0, f12)
	drawNavBtn(c, downR, "v", m.offset >= maxOff, f12)
}

// drawNavBtn draws sym centered in r, or a short horizontal bar if disabled.
func drawNavBtn(c *canvas.Canvas, r image.Rectangle, sym string, disabled bool, f canvas.Font) {
	if disabled {
		cx := r.Min.X + r.Dx()/2
		cy := r.Min.Y + r.Dy()/2
		c.DrawLine(cx-4, cy, cx+4, cy, canvas.Black)
		return
	}
	if f == nil {
		return
	}
	tw := 0
	for _, ch := range sym {
		_, w, _ := f.Glyph(ch)
		tw += w
	}
	tx := r.Min.X + (r.Dx()-tw)/2
	ty := r.Min.Y + (r.Dy()-f.LineHeight())/2
	c.DrawText(tx, ty, sym, f, canvas.Black)
}
```

- [ ] **Step 4: Vérifier que les tests passent**

```bash
cd /home/oioio/Documents/GolandProjects/oioni
go test ./cmd/oioni/ui/... -run TestHomeMenu -v 2>&1
```

Résultat attendu : tous les `TestHomeMenu_*` PASS. Si `home.go` ne compile pas (champ `icon` manquant), c'est normal — on le fixe à la Task 2.

- [ ] **Step 5: Vérifier que le reste du package compile**

```bash
go build ./cmd/oioni/... 2>&1
```

Si erreur sur `home.go` (champ `icon` inconnu dans les items) → c'est attendu, sera corrigé Task 2. Si autre erreur → corriger.

- [ ] **Step 6: Commit**

```bash
git add cmd/oioni/ui/menu.go cmd/oioni/ui/menu_test.go
git commit -m "feat(ui): redesign home menu - 50px rows, scroll buttons, real icons"
```

---

### Task 2: Mettre à jour home.go avec les icônes

**Files:**
- Modify: `cmd/oioni/ui/home.go`

**Contexte :**
- `Icons` est une variable globale dans `cmd/oioni/ui/icons.go`, peuplée par `init()`.
- Champs : `Icons.Config`, `Icons.System`, `Icons.Attack`, `Icons.DFIR`, `Icons.Info` (type `gui.Icon`).
- `homeMenuItem.icon` est du type `gui.Icon` — assignation directe, pas de pointeur.

Cette tâche est une mise à jour de compilation pure (ajout du champ `icon` dans chaque item de `home.go`). Pas de nouveau comportement à tester — les tests de Task 1 couvrent déjà le rendu des icônes (zero-value `gui.Icon` est déjà exercé dans `TestHomeMenu_DrawDoesNotPanic`).

- [ ] **Step 1: Mettre à jour home.go**

Remplacer le contenu de `cmd/oioni/ui/home.go` par :

```go
// cmd/oioni/ui/home.go — HomeScene: operator-style menu home screen
package ui

import (
	"image"

	"github.com/oioio-space/oioni/drivers/epd"
	"github.com/oioio-space/oioni/ui/gui"
)

// NewHomeScene builds the home screen: 22px NetworkStatusBar header + 100px HomeMenuWidget.
// Returns the scene and the NetworkStatusBar so the caller can call SetInterfaces/SetTools.
func NewHomeScene(nav *gui.Navigator) (*gui.Scene, *gui.NetworkStatusBar) {
	nsb := gui.NewNetworkStatusBar(nav)

	menu := newHomeMenuWidget([]homeMenuItem{
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

	content := gui.NewVBox(
		gui.FixedSize(nsb, 22),
		gui.Expand(menu),
	)
	content.SetBounds(image.Rect(0, 0, epd.Height, epd.Width))

	return &gui.Scene{
		Title: "Home",
		// nsb and menu at top level so Navigator finds HandleTouch for badge tap and row taps.
		Widgets: []gui.Widget{content, nsb, menu},
	}, nsb
}
```

Note : descriptions raccourcies pour tenir sur 1 ligne à 8pt dans 152px (200px liste - 48px textX).

- [ ] **Step 2: Build complet**

```bash
cd /home/oioio/Documents/GolandProjects/oioni
go build ./cmd/oioni/... 2>&1
```

Résultat attendu : aucune erreur.

- [ ] **Step 3: Tous les tests**

```bash
go test ./cmd/oioni/... ./ui/gui/... -v 2>&1 | tail -30
```

Résultat attendu : tous PASS.

- [ ] **Step 4: Commit**

```bash
git add cmd/oioni/ui/home.go
git commit -m "feat(ui): wire real icons into home menu items"
```

---

## OTA après implémentation

```bash
podman save localhost/oioni/impacket:arm64 | gzip > /tmp/impacket-arm64.tar.gz
GOWORK=off gok update --parent_dir . -i oioio
```
