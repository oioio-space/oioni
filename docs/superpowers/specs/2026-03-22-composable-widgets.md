# Composable Widgets — Spec

## Goal

Extraire `ScrollableList` et `NavButton` dans le package `ui/gui/` pour les rendre réutilisables dans n'importe quelle scène. Le menu d'accueil utilise ces widgets via une implémentation de `gui.ListItem` spécifique (`homeListItem`).

## Architecture

### Interface `ListItem`

```go
// ui/gui/widget_scrolllist.go
type ListItem interface {
    Draw(c *canvas.Canvas, bounds image.Rectangle)
    OnTap()
}
```

Chaque scène fournit son propre type de rangée en implémentant cette interface.

### Widgets réutilisables (`ui/gui/`)

| Widget | Fichier | Responsabilité |
|--------|---------|----------------|
| `ScrollableList` | `ui/gui/widget_scrolllist.go` | Liste défilable générique, responsive |
| `NavButton` | `ui/gui/widget_navbutton.go` | Bouton ∧ ou ∨ avec état actif/désactivé |

### Code spécifique à l'accueil (`cmd/oioni/ui/`)

| Fichier | Responsabilité |
|---------|----------------|
| `cmd/oioni/ui/menu.go` | `homeListItem` : icône 16×16 + nom 12pt |
| `cmd/oioni/ui/home.go` | Câblage des widgets + layout |

## Contexte hardware

- Display logique : 250×122px (Rot90 depuis 122×250 physique)
- Touch GT1151 : détecte les **taps**, pas les swipes
- Contrainte : 1-bit, noir strict ou blanc strict

## `ScrollableList`

```go
type ScrollableList struct {
    gui.BaseWidget
    items  []ListItem
    offset int
    RowH   int // hauteur d'une rangée en px (défini par le caller)
}

func NewScrollableList(items []ListItem, rowH int) *ScrollableList
```

**Responsive :** `visible() int` = `Bounds().Dy() / RowH` (calculé depuis les bounds réels, jamais hardcodé).

**Scroll API :**
- `CanScrollUp() bool` — `offset > 0`
- `CanScrollDown() bool` — `offset + visible() < len(items)` (pas de underflow)
- `ScrollUp()` — décrémente offset si possible, SetDirty
- `ScrollDown()` — incrémente offset si possible, SetDirty

**HandleTouch :** calcule `row = (pt.Y - bounds.Min.Y) / RowH`, appelle `items[offset+row].OnTap()` si valide.

**Draw :**
1. Fond blanc
2. Pour chaque rangée visible : passe `image.Rect(minX, rowTop, maxX, rowTop+RowH)` à `item.Draw()`
3. Séparateur 2px entre rangées (pas après la dernière rangée visible, ni si la rangée suivante n'existe pas dans items)

## `NavButton`

```go
type NavButton struct {
    gui.BaseWidget
    sym      string
    onTap    func()
    isActive func() bool
}

func NewNavButton(sym string, onTap func(), isActive func() bool) *NavButton
```

- `isActive` nil → défaut `func() bool { return false }`
- `HandleTouch` : appelle toujours `onTap()` (la logique no-op est dans `ScrollUp`/`ScrollDown`)
- `Draw` actif : bordure noire + symbole centré en 12pt dans les bounds réels
- `Draw` désactivé : bordure noire + barre horizontale 8px centrée (convention e-ink disabled)
- **Responsive** : centrage calculé depuis `Bounds()` réels (fonctionne dans n'importe quelle taille)

## `homeListItem`

```go
type homeListItem struct {
    name  string
    icon  gui.Icon
    onTap func()
}
```

Constantes dans `menu.go` :
```go
const (
    homeRowH     = 25   // 4 rangées × 25px = 100px disponibles
    homeIconSize = 16
    homeIconX    = 4    // marge gauche
    homeIconYOff = 4    // (homeRowH - homeIconSize) / 2, centrage vertical
    homeTextX    = 24   // homeIconX + homeIconSize + 4
    homeNavW     = 50   // largeur colonne nav
)
```

**Draw :** icône 16×16 à `(r.Min.X+homeIconX, r.Min.Y+homeIconYOff)`, nom en 12pt centré verticalement à `r.Min.X+homeTextX`.

**OnTap :** nil-guard, appelle `h.onTap()`.

## Layout accueil (inchangé vs précédent design)

```
┌─────────────────────────────────────────────────────┐ y=0
│ NetworkStatusBar — 22px                             │
├────────────────────────────────┬────────────────────┤ y=22
│  [16px icon] Nom 12pt          │  ∧ (NavButton)     │ rangée 0 (25px)
├────────────────────────────────┤  (50×50px)         │ y=47
│  [16px icon] Nom 12pt          ├────────────────────┤ rangée 1 (25px)
├────────────────────────────────┤  ∨ (NavButton)     │ y=72
│  [16px icon] Nom 12pt          │  (50×50px)         │ rangée 2 (25px)
├────────────────────────────────┤                    │ y=97
│  [16px icon] Nom 12pt          │                    │ rangée 3 (25px)
└────────────────────────────────┴────────────────────┘ y=122
 x=0                        x=200 x=200          x=250
```

Avec 5 catégories : `visible()` = 100/25 = 4 → ∧ désactivé au début, ∨ actif.

**Touch routing :** `scene.Widgets: []gui.Widget{content, nsb, list, upBtn, downBtn}` — Navigator route automatiquement par bounds.

**Nav column** : toujours présente (50px). `upBtn` et `downBtn` dans un `VBox` via `Expand()`.

## Fichiers modifiés

| Fichier | Action |
|---------|--------|
| `ui/gui/widget_scrolllist.go` | Créer : `ListItem` + `ScrollableList` |
| `ui/gui/widget_scrolllist_test.go` | Créer : tests `ScrollableList` |
| `ui/gui/widget_navbutton.go` | Créer : `NavButton` |
| `ui/gui/widget_navbutton_test.go` | Créer : tests `NavButton` |
| `cmd/oioni/ui/menu.go` | Réécrire : `homeListItem` + constantes seulement |
| `cmd/oioni/ui/menu_test.go` | Réécrire : tests `homeListItem` |
| `cmd/oioni/ui/home.go` | Modifier : utiliser `gui.NewScrollableList` + `gui.NewNavButton` |
