# ui/gui

Framework GUI retained-mode pour ecran e-ink Waveshare 2.13" Touch HAT.

## Package independant

`ui/gui` n'a **aucune dependance** vers `drivers/epd` ni `drivers/touch`. Il
definit ses propres types (`DisplayMode`, `TouchPoint`, `TouchEvent`,
`ScreenWidth/Height`) et expose l'interface `Display` pour le hardware.

Pour utiliser avec le vrai ecran, il suffit d'envelopper `*epd.Display` avec
un adaptateur de 5 lignes (voir `cmd/oioni/epaper.go`, type `epdAdapter`).
Cela permet de tester `ui/gui` sans hardware en injectant un `Display` fictif.

## Exemple simple

```go
import (
    "context"
    "fmt"
    "github.com/oioio-space/oioni/ui/gui"
)

// myDisplay implemente gui.Display (wrapping *epd.Display en production)
nav := gui.NewNavigator(myDisplay)

count := 0
lbl := gui.NewLabel("Tap me!")
lbl.SetAlign(gui.AlignCenter)

btn := gui.NewButton("Incrementer")
btn.OnClick(func() {
    count++
    lbl.SetText(fmt.Sprintf("Compteur : %d", count))
})

scene := &gui.Scene{
    Widgets: []gui.Widget{
        gui.NewVBox(
            gui.NewStatusBar("Demo", ""),
            gui.NewDivider(),
            gui.Expand(lbl),
            gui.FixedSize(btn, 32),
        ),
    },
}

nav.Push(scene)

ctx, cancel := context.WithCancel(context.Background())
defer cancel()
nav.Run(ctx, touchEvents) // bloque jusqu'a cancel()
```

## Exemple avance : navigation multi-scenes et goroutine

```go
// Scene racine avec bouton qui pousse une sous-scene
root := &gui.Scene{
    Widgets: []gui.Widget{
        gui.NewVBox(
            gui.NewStatusBar("Accueil", ""),
            gui.Expand(gui.NewLabel("Ecran principal")),
            gui.FixedSize(settingsBtn, 30),
        ),
    },
}

// Sous-scene de reglages
settingsBtn.OnClick(func() {
    toggle := gui.NewToggle("WiFi", false)
    toggle.OnChange(func(on bool) {
        // Appel depuis callback OnClick -> dans la goroutine de Run, pas de race
        applyWifi(on)
    })
    nav.Push(&gui.Scene{
        Widgets: []gui.Widget{
            gui.NewVBox(
                gui.NewStatusBar("Reglages", ""),
                gui.NewDivider(),
                toggle,
                gui.FixedSize(gui.NewButton("Retour"), 28),
            ),
        },
        OnLeave: func() { saveSettings() },
    })
})

// Mise a jour depuis une goroutine externe (time.AfterFunc, etc.)
go func() {
    time.Sleep(10 * time.Second)
    // Dispatch est obligatoire depuis une goroutine externe
    nav.Dispatch(func() {
        lbl.SetText("Mise a jour depuis goroutine")
        nav.RequestRender()
    })
}()

// Idle sleep apres 5 minutes d'inactivite
nav = gui.NewNavigatorWithIdle(myDisplay, 5*time.Minute)
// Reveil depuis une goroutine :
nav.Wake()
```

## Points d'attention

**Zero dependance hardware** : `ui/gui` ne connait pas `drivers/epd` ni
`drivers/touch`. Pour brancher le vrai ecran, implementer l'interface
`gui.Display` (generalement 5 lignes de delegation) et produire des
`gui.TouchEvent` a partir des evenements du touch controller.

**Dispatch obligatoire hors de Run** : `Push`, `Pop`, et les mutations d'etat
des widgets ne sont pas concurrent-safe avec `Run`. Tout appel depuis une
goroutine externe doit passer par `nav.Dispatch(fn)`.

**hScrollable et top-level** : un widget implementant `hScrollable` (gestes
horizontaux) doit etre un element direct de `Scene.Widgets`. S'il est emballe
dans un conteneur (`VBox`, `HBox`...), le Navigator ne le trouvera pas. Les
widgets `Touchable`, en revanche, sont cherches recursivement dans l'arbre.

**SetBounds sur les conteneurs racine** : le Navigator n'appelle pas
`SetBounds` automatiquement sur les widgets de `Scene.Widgets`. Les conteneurs
racine doivent recevoir leurs bounds explicitement, par exemple :
```go
vbox.SetBounds(image.Rect(0, 0, gui.ScreenHeight, gui.ScreenWidth))
// Apres Rot90 : ScreenHeight=250 (largeur logique), ScreenWidth=122 (hauteur logique)
```
Les helpers `ShowAlert`, `ShowMenu`, et `ShowTextInput` s'en chargent
automatiquement.

**Divider 2px** : les separateurs mesurent 2 px d'epaisseur. 1 px disparait
souvent lors d'un refresh partiel e-ink.

**anti-ghosting** : toutes les 5 refreshs partiels (`antiGhostN=5`), un
refresh complet est force automatiquement. `RequestRegenerate()` declenche un
cycle noir-blanc (~4s) pour le keep-alive quotidien.

**Polices et texte** : les polices embedees ne couvrent que l'ASCII. Utiliser
`canvas.EmbeddedFont(size)` avec les tailles 8, 12, 16, 20 ou 24.

**Widgets Stoppable** : les widgets qui possedent des goroutines internes
doivent implementer `Stop()`. Le Navigator appelle `Stop()` recursivement sur
tous les widgets d'une scene depilee.

## Reference API

### Navigator

| Fonction / Methode | Description |
|---|---|
| `NewNavigator(d Display) *Navigator` | Cree un Navigator avec rafraichissement standard |
| `NewNavigatorWithIdle(d Display, timeout time.Duration) *Navigator` | Comme ci-dessus avec sleep apres inactivite |
| `nav.Push(s *Scene) error` | Empile une scene, declenche un refresh partial |
| `nav.Pop() error` | Depile la scene du haut, full refresh si retour a la racine |
| `nav.PopTo(depth int) error` | Depile jusqu'a la profondeur donnee en une seule passe |
| `nav.Run(ctx context.Context, events <-chan TouchEvent)` | Boucle principale (bloquant) |
| `nav.Dispatch(fn func())` | Enfile fn pour execution dans la goroutine de Run |
| `nav.RequestRender()` | Declenche un re-rendu (non bloquant) |
| `nav.RequestRegenerate()` | Cycle purge noir-blanc (~4s) pour keep-alive |
| `nav.Wake()` | Reveille l'ecran si en veille, force un full refresh |
| `nav.Depth() int` | Nombre de scenes dans la pile |

### Types principaux

| Type | Description |
|---|---|
| `Scene` | `Widgets []Widget`, `OnEnter func()`, `OnLeave func()`, `Title string` |
| `DisplayMode` | `ModeFull`, `ModePartial`, `ModeFast` |
| `TouchPoint` | `ID uint8`, `X, Y uint16`, `Size uint8` |
| `TouchEvent` | `Points []TouchPoint`, `Time time.Time` |

### Widgets integres

| Widget | Constructeur | Description |
|---|---|---|
| Label | `NewLabel(text)` | Texte sur une ligne ; `SetFont`, `SetAlign` |
| Button | `NewButton(label)` | `OnClick(fn)` ; feedback visuel par inversion |
| ProgressBar | `NewProgressBar()` | `SetValue(0.0-1.0)` ; utiliser `Expand` pour pleine largeur |
| StatusBar | `NewStatusBar(left, right)` | Barre noire texte blanc ; left = horloge si vide |
| Toggle | `NewToggle(label, on)` | Interrupteur on/off ; `OnChange(fn)` |
| Checkbox | `NewCheckbox(label, checked)` | Case a cocher ; `OnChange(fn)` |
| Slider | `NewSlider(min, max, value)` | Selecteur de valeur horizontal ; `OnChange(fn)` |
| Menu | `NewMenu(items)` | Liste scrollable d'items ; supporte les icones |
| NavButton | `NewNavButton(label)` | Bouton avec etat actif/desactive |
| NavBar | `NewNavBar(path...)` | Fil d'Ariane (ex. "Accueil > Config") |
| Carousel | `NewCarousel(items)` | Carrousel d'icones horizontal |
| ScrollList | `NewScrollList(rows)` | Liste scrollable de lignes texte |
| ActionSidebar | `NewActionSidebar(btns...)` | Barre d'actions laterale |
| ClockWidget | `NewClock()` | Horloge auto-rafraichie chaque minute |
| QRCode | `NewQRCode(content)` | QR code vectoriel |
| NetworkStatusBar | `NewNetworkStatusBar()` | Barre operateur (WiFi, IP...) |
| ImageWidget | `NewImageWidget(img)` | Rendu `image.Image` mis a l'echelle |
| ArcWidget | `NewArcWidget(...)` | Arc / indicateur circulaire |
| Spacer | `NewSpacer()` | Vide invisible ; utiliser avec `Expand` |
| Divider | `NewDivider()` | Separateur 2px, horizontal ou vertical |

### Conteneurs de layout

| Conteneur | Constructeur | Description |
|---|---|---|
| VBox | `NewVBox(children...)` | Empilement vertical |
| HBox | `NewHBox(children...)` | Rangee horizontale |
| Fixed | `NewFixed(w, h)` + `Put(w, x, y)` | Positionnement absolu en pixels |
| Overlay | `NewOverlay(content, align)` | Flottant par-dessus la scene |
| WithPadding | `WithPadding(px, w)` | Marge uniforme sur les 4 cotes |

Modificateurs de layout pour enfants de VBox/HBox :

| Modificateur | Description |
|---|---|
| `Expand(w)` | Le widget prend tout l'espace restant sur l'axe principal |
| `FixedSize(w, px)` | Fixe la taille sur l'axe principal a px pixels |

### Widget personnalise

```go
type MonWidget struct {
    gui.BaseWidget
    valeur string
}

func (w *MonWidget) Draw(c *canvas.Canvas) {
    c.DrawRect(w.Bounds(), canvas.White, true)
    c.DrawText(w.Bounds().Min.X+2, w.Bounds().Min.Y+2,
               w.valeur, canvas.EmbeddedFont(12), canvas.Black)
}
func (w *MonWidget) PreferredSize() image.Point { return image.Pt(80, 20) }
func (w *MonWidget) MinSize() image.Point       { return image.Pt(20, 16) }

// Touch (optionnel)
func (w *MonWidget) HandleTouch(pt gui.TouchPoint) bool {
    w.valeur = "touche"
    w.SetDirty()
    return true
}

// Stop (optionnel, si goroutines internes)
func (w *MonWidget) Stop() { /* arreter les goroutines */ }
```

### Helpers de haut niveau

| Fonction | Description |
|---|---|
| `ShowAlert(nav, title, msg, btns...)` | Pousse une scene modale alerte avec boutons |
| `ShowMenu(nav, title, items)` | Pousse une scene menu scrollable |
| `ShowTextInput(nav, placeholder, maxLen, onConfirm)` | Pousse une scene saisie texte avec clavier |
