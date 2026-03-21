# Home Menu Redesign — Spec

## Goal

Remplacer le `HomeMenuWidget` actuel (20px rangées, cercles au lieu d'icônes, pas de scroll, conflit visuel header/sélection) par un menu adapté au tactile e-ink : grandes rangées scrollables via boutons ∧/∨.

## Architecture

**Widget composition** : l'écran home est composé de trois widgets indépendants coordonnés via closures. Chaque widget gère son propre rendu et ses propres touches. Le Navigator route le tactile automatiquement via les bounds (aucune dispatch manuelle).

| Widget | Fichier | Responsabilité |
|--------|---------|----------------|
| `ScrollableMenuList` | `cmd/oioni/ui/menu.go` | Liste 2 rangées + état offset + tap rangée |
| `NavButton` | `cmd/oioni/ui/menu.go` | Bouton ∧ ou ∨ unique, enabled/disabled |
| `gui.NetworkStatusBar` | `ui/gui/widget_networkstatus.go` | Barre header (inchangé) |

`home.go` instancie les trois, les câble via closures, et les place dans le layout (`HBox`/`VBox`) + dans `scene.Widgets` pour le touch routing automatique.

## Contexte hardware

- Display logique : 250×122px (Rot90 depuis 122×250 physique)
- Touch GT1151 : détecte les **taps**, pas les swipes glissés
- Contrainte visuelle : pas de gris, noir strict ou blanc strict (1-bit)

## Layout

```
┌─────────────────────────────────────────────────────┐  y=0
│ NetworkStatusBar — 22px (inchangé)                  │
├────────────────────────────────┬────────────────────┤  y=22
│  [icône 32×32]  Nom (12pt)     │       ∧            │
│                 Desc (8pt)     │   (50×50px)        │  rangée 0
├────────────────────────────────┤                    │  y=72
│  [icône 32×32]  Nom (12pt)     ├────────────────────┤
│                 Desc (8pt)     │       ∨            │
│                                │   (50×50px)        │  rangée 1
└────────────────────────────────┴────────────────────┘  y=122
  x=0                       x=200  x=200          x=250
```

**Panneaux :**
- Liste : x=0..199, y=22..121 (200×100px), 2 rangées × 50px
- Nav : x=200..249, y=22..121 (50×100px), bouton ∧ haut (50×50px) + bouton ∨ bas (50×50px)

## Rangée (200×50px)

```
 8px  │ 32×32 icon │ 8px │ Nom 12pt
      │            │     │ Desc 8pt
```

(`wb` = `m.Bounds()`, `rowTop` = `wb.Min.Y + row*menuRowH` pour chaque rangée visible)

- Icône : `icon.Draw(c, image.Rect(wb.Min.X+menuIconX, rowTop+menuIconYOff, wb.Min.X+menuIconX+menuIconSize, rowTop+menuIconYOff+menuIconSize))`
- Nom : `c.DrawText(wb.Min.X+menuTextX, rowTop+6, item.name, font12, Black)`
- Desc : `c.DrawText(wb.Min.X+menuTextX, rowTop+28, item.desc, font8, Black)`
- Séparateur bas de rangée : ligne 1px noire de x=0 à x=199 (sauf dernière rangée visible)

## Boutons nav (50×50px chacun)

- Fond blanc, bordure 1px noire (`DrawRect` non-filled) sur les 4 côtés du bouton
- La bordure gauche du bouton haut (`x=200`) **constitue** la ligne de séparation verticale — pas de ligne séparée à tracer
- Symbole ∧ ou ∨ centré en 12pt
- Bouton désactivé (offset=0 pour ∧, offset=`len(items)-menuVisible` pour ∨) : trait horizontal de 8px centré à la place du symbole (convention e-ink pour disabled)

## Scroll state

- `offset int` : index de la première rangée visible (0..3 pour 5 items, 2 visibles)
- ∧ : `offset = max(0, offset-1)` + SetDirty
- ∨ : `offset = min(len(items)-menuVisible, offset+1)` + SetDirty
- Initialisation : offset=0

## Feedback tap sur rangée

Pas de "selected" persistant. Tap sur une rangée = appel direct `item.onTap()` sans changement visuel (l'e-ink est lent, le changement de scène est le feedback).

## Contraste header / menu

Fond des rangées = blanc → frontière naturelle avec la barre noire de 22px. Aucune rangée active en noir plein.

## Touch routing

`HandleTouch(pt touch.TouchPoint)` reçoit des coordonnées **logiques** (Navigator fait la conversion physique→logique avant).

```
r := m.Bounds()   // pleine zone du widget (x:0..249, y:22..121)
px, py := int(pt.X), int(pt.Y)

navX := r.Max.X - 50   // = 200

if px >= navX {
    // zone boutons nav
    midY := r.Min.Y + r.Dy()/2   // = 72
    if py < midY { scroll up } else { scroll down }
    return true
}

// zone liste
row := (py - r.Min.Y) / menuRowH   // menuRowH = 50
if row >= 0 && row < menuVisible && offset+row < len(items) {
    items[offset+row].onTap()
}
return true
```

## Structs

```go
type homeMenuItem struct {
    name  string
    desc  string
    icon  gui.Icon
    onTap func()
}

// ScrollableMenuList : liste + état scroll
type ScrollableMenuList struct {
    gui.BaseWidget
    items  []homeMenuItem
    offset int
}

// NavButton : bouton unique ∧ ou ∨
type NavButton struct {
    gui.BaseWidget
    sym      string       // "^" ou "v" (ASCII)
    onTap    func()
    isActive func() bool  // retourne false si bouton désactivé
}
```

**Câblage dans home.go :**
```go
list    := newScrollableMenuList(items)
upBtn   := newNavButton("^", list.ScrollUp,   list.CanScrollUp)
downBtn := newNavButton("v", list.ScrollDown, list.CanScrollDown)
```

`ScrollUp`/`ScrollDown` appellent `SetDirty()` sur la liste. Le Navigator re-rend tout — les boutons appellent `isActive()` à chaque Draw et reflètent l'état courant automatiquement.

## Fichiers modifiés

| Fichier | Changement |
|---------|------------|
| `cmd/oioni/ui/menu.go` | Réécriture : `ScrollableMenuList` + `NavButton` + `homeMenuItem` |
| `cmd/oioni/ui/menu_test.go` | Tests pour les deux widgets |
| `cmd/oioni/ui/home.go` | Composition : câblage list+boutons+layout, icons dans items |

`icons.go`, `epaper.go`, `main.go` : aucun changement.

## Constantes

```go
const (
    menuRowH      = 50
    menuVisible   = 2
    menuNavW      = 50
    menuIconX     = 8         // marge gauche de l'icône
    menuIconSize  = 32        // taille icône
    menuIconYOff  = 9         // = (menuRowH - menuIconSize) / 2, centre verticalement
    menuTextX     = 48        // = menuIconX + menuIconSize + 8, début texte
)
```

## Tests requis

1. `TestHomeMenu_PreferredSize` : 100px height
2. `TestHomeMenu_ScrollDown` : offset 0→1 sur tap ∨
3. `TestHomeMenu_ScrollUp` : offset 1→0 sur tap ∧
4. `TestHomeMenu_ScrollUpAtTop` : tap ∧ quand offset=0 → no-op
5. `TestHomeMenu_ScrollDownAtBottom` : tap ∨ quand offset=3 avec 5 items (max offset = len-menuVisible = 3) → no-op
6. `TestHomeMenu_TapRow` : tap rangée 1 (item index=offset+1) → onTap appelé
7. `TestHomeMenu_DrawDoesNotPanic` : Draw sans crash
