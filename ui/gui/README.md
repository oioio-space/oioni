# ui/gui

Framework GUI retained-mode pour ecran e-ink Waveshare 2.13" Touch HAT (122x250 px, Rot90).

## Package independant

`ui/gui` n'a **aucune dependance** vers `drivers/epd` ni `drivers/touch`. Il a ete volontairement decouple du hardware. Il definit ses propres types :

- `DisplayMode` (`ModeFull`, `ModePartial`, `ModeFast`) — valeurs intentionnellement identiques a `drivers/epd.Mode` pour simplifier le wrapping
- `TouchPoint`, `TouchEvent` — types locaux independants de `drivers/touch`
- `ScreenWidth = 122`, `ScreenHeight = 250` — dimensions physiques de l'ecran Waveshare 2.13"

Pour utiliser avec le vrai ecran, envelopper `*epd.Display` avec l'adaptateur `epdAdapter` (2 lignes de code, voir `cmd/oioni/epaper.go`). Pour les tests, injecter n'importe quelle implementation fictive de l'interface `Display`.

---

## Exemple simple

```go
package main

import (
    "context"
    "fmt"
    "github.com/oioio-space/oioni/ui/gui"
)

// myDisplay implemente gui.Display (wrapper epd.Display en production, fake en test)
func main() {
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
}
```

---

## Exemple avance : navigation multi-scenes et mise a jour depuis une goroutine

```go
// Sous-scene de reglages poussee depuis un callback OnClick.
settingsBtn.OnClick(func() {
    // OnClick est appele depuis la goroutine de Run() : Push/Pop sont surs ici.
    toggle := gui.NewToggle("WiFi", false)
    toggle.OnChange(func(on bool) {
        applyWifi(on) // appele depuis Run() aussi
    })
    nav.Push(&gui.Scene{
        Title: "Reglages",
        Widgets: []gui.Widget{
            gui.NewVBox(
                gui.NewNavBar("Accueil", "Reglages"),
                gui.NewDivider(),
                toggle,
                gui.FixedSize(gui.NewButton("Retour"), 28),
            ),
        },
        OnLeave: func() {
            // Attention : ne pas appeler Push/Pop depuis OnLeave (non Pop-safe)
            saveSettings()
        },
    })
})

// Mise a jour depuis une goroutine externe (time.AfterFunc, callback reseau...)
go func() {
    time.Sleep(10 * time.Second)
    // Dispatch est OBLIGATOIRE depuis une goroutine externe.
    // Si le dispatchFn est plein, l'appel est abandonne silencieusement.
    nav.Dispatch(func() {
        statusLbl.SetText("Mise a jour depuis goroutine")
        nav.RequestRender()
    })
}()

// Reveil periodique pour eviter la degradation de l'ecran e-ink.
time.AfterFunc(24*time.Hour, func() {
    nav.RequestRegenerate() // cycle noir-blanc ~4s
})

// Navigator avec veille automatique apres 5 minutes d'inactivite.
nav := gui.NewNavigatorWithIdle(myDisplay, 5*time.Minute)
// Reveil depuis n'importe quelle goroutine :
nav.Wake()
```

---

## Points d'attention

**Zero dependance hardware.**
`ui/gui` ne connait pas `drivers/epd` ni `drivers/touch`. Pour brancher le vrai ecran : implementer `gui.Display` en deleguant a `*epd.Display` (type `epdAdapter` dans `cmd/oioni/epaper.go`, environ 5 lignes). Pour les evenements tactiles : lire depuis `<-chan drivers/touch.TouchEvent` et convertir en `gui.TouchEvent` (meme structure).

**`Dispatch()` obligatoire hors de `Run()`.**
`Push()`, `Pop()`, et les mutations d'etat de widgets (`SetText`, `SetDirty`...) ne sont pas concurrent-safe avec `Run()`. Tout appel depuis une goroutine externe doit passer par `nav.Dispatch(fn)`. Le canal `dispatchFn` a une capacite de 1 ; si une fonction est deja en attente, l'appel suivant est silencieusement abandonne.

**`OnLeave` n'est pas Pop-safe.**
Appeler `Push()` ou `Pop()` depuis `OnLeave` provoque une corruption de la pile. `OnLeave` est appele pendant le teardown de la scene ; la pile est dans un etat transitoire.

**`hScrollable` doit etre un element direct de `Scene.Widgets`.**
Un widget implementant `hScrollable` (gestes de balayage horizontal) doit etre a la racine directe du tableau `Scene.Widgets`. S'il est imbrique dans un conteneur (`VBox`, `HBox`...), le Navigator ne le trouvera pas lors de la recherche. Les widgets `Touchable` sont en revanche recherches recursivement dans tout l'arbre.

**`HandleTouch` retourne bool : true = evenement consomme.**
Quand `HandleTouch` retourne `true`, le Navigator stoppe la propagation aux autres widgets. Retourner `false` permet a un widget parent (ou au Navigator lui-meme pour les gestes) de traiter l'evenement.

**Anti-ghosting automatique.**
Toutes les 5 refreshs partiels (`antiGhostN = 5`), un refresh complet est force automatiquement. `RequestRegenerate()` declenche en plus un cycle noir-blanc (~4s) pour la purge profonde periodique (keep-alive 24h recommande pour eviter la degradation de l'ecran e-ink).

**Stop() appele recursivement sur les widgets dempiles.**
Quand une scene est depilee via `Pop()`, le Navigator appelle `Stop()` recursivement sur tous les widgets implementant `Stoppable`, y compris ceux imbriques dans des conteneurs de layout (via l'interface interne `hasChildren`). Les widgets avec des goroutines internes doivent implementer `Stoppable`.

**Polices et texte : ASCII uniquement.**
Les polices embedees couvrent uniquement l'ASCII (codes < 128). Les caracteres accentues (e, a, e, o, u...) ne s'affichent pas correctement. Utiliser uniquement des caracteres ASCII dans les labels, ou implementer une police personnalisee.

**Tailles de polices disponibles.**
`canvas.EmbeddedFont(size)` : tailles supportees = 8, 12, 16, 20, 24. Taille minimale recommandee pour la lisibilite sur e-ink : 12 px.

**Separateurs 2 px.**
`NewDivider()` genere un separateur de 2 px d'epaisseur. Un separateur de 1 px peut disparaitre lors d'un refresh partiel e-ink (artefact d'affichage connu).

**Dimensions logiques apres Rot90.**
`ScreenWidth = 122`, `ScreenHeight = 250` sont les dimensions physiques. Apres rotation Rot90 appliquee par le canvas, la largeur logique devient 250 px et la hauteur logique 122 px. Les conteneurs VBox/HBox racine doivent etre dimensionnes en consequence : `SetBounds(image.Rect(0, 0, gui.ScreenHeight, gui.ScreenWidth))`.

---

## Reference API

### Navigator

| Methode | Description |
|---|---|
| `NewNavigator(d Display) *Navigator` | Cree un Navigator avec rafraichissement standard |
| `NewNavigatorWithIdle(d Display, timeout time.Duration) *Navigator` | Comme ci-dessus avec veille automatique apres `timeout` d'inactivite |
| `nav.Push(s *Scene) error` | Empile une scene, declenche un refresh partiel |
| `nav.Pop() error` | Depile la scene du haut ; full refresh si retour a la scene racine |
| `nav.Run(ctx context.Context, events <-chan TouchEvent)` | Boucle principale, bloquante jusqu'a `ctx.Done()` |
| `nav.Dispatch(fn func())` | Enfile `fn` pour execution dans la goroutine de `Run()` (non bloquant) |
| `nav.RequestRender()` | Declenche un re-rendu partiel (non bloquant) |
| `nav.RequestRegenerate()` | Cycle purge noir-blanc ~4s pour keep-alive de l'ecran |
| `nav.Wake()` | Reveille l'ecran si en veille, force un full refresh |
| `nav.Depth() int` | Nombre de scenes dans la pile (0 = vide, 1 = racine seule) |

### Interfaces

```go
// Display est l'interface hardware utilisee par Navigator.
// Implementer avec un wrapper sur *epd.Display ou un fake pour les tests.
type Display interface {
    Init(m DisplayMode) error
    DisplayBase(buf []byte) error    // full refresh : ecrit les banques RAM 0x24 + 0x26
    DisplayPartial(buf []byte) error // partial refresh : buffer 4000 octets, auto-contenu
    DisplayFast(buf []byte) error    // fast full refresh
    DisplayRegenerate() error        // cycle purge noir-blanc ~4s
    Sleep() error
    Close() error
}

// Widget est l'interface de base de tout element graphique.
type Widget interface {
    Draw(c *canvas.Canvas)
    Bounds() image.Rectangle
    SetBounds(r image.Rectangle)
    PreferredSize() image.Point  // taille preferee intrinseque ; (0,0) = pas de preference
    MinSize() image.Point        // taille minimale ; le layout respecte ce plancher
    IsDirty() bool
    SetDirty()
    MarkClean()
}

// Touchable est implemente par les widgets interactifs.
type Touchable interface {
    HandleTouch(pt TouchPoint) bool // true = evenement consomme
}

// Stoppable est implemente par les widgets qui possedent des goroutines internes.
type Stoppable interface {
    Stop()
}
```

### Scene

```go
type Scene struct {
    Title   string   // metadonnees pour NavBar ; non utilise par Navigator
    Widgets []Widget // widgets racine de la scene (layout containers ou widgets directs)
    OnEnter func()   // appele quand la scene devient active (Push ou retour via Pop)
    OnLeave func()   // appele quand la scene est depilee — NE PAS appeler Push/Pop ici
}
```

### Types principaux

```go
type DisplayMode uint8
const (
    ModeFull    DisplayMode = iota // ~2s, qualite maximale
    ModePartial                    // ~0.3s, mise a jour partielle
    ModeFast                       // ~0.5s, full refresh rapide
)

const (
    ScreenWidth  = 122  // pixels physiques horizontaux de l'ecran Waveshare 2.13"
    ScreenHeight = 250  // pixels physiques verticaux
)

type TouchPoint struct {
    ID   uint8
    X, Y uint16
    Size uint8
}

type TouchEvent struct {
    Points []TouchPoint
    Time   time.Time
}
```

### Widget personnalise

```go
type MonWidget struct {
    gui.BaseWidget // fournit Bounds, SetBounds, IsDirty, SetDirty, MarkClean
    valeur string
}

func (w *MonWidget) Draw(c *canvas.Canvas) {
    c.DrawRect(w.Bounds(), canvas.White, true) // effacer le fond
    c.DrawText(w.Bounds().Min.X+2, w.Bounds().Min.Y+2,
               w.valeur, canvas.EmbeddedFont(12), canvas.Black)
}

func (w *MonWidget) PreferredSize() image.Point { return image.Pt(80, 20) }
func (w *MonWidget) MinSize() image.Point       { return image.Pt(20, 16) }

// Touch (optionnel) — implementer Touchable
func (w *MonWidget) HandleTouch(pt gui.TouchPoint) bool {
    w.valeur = "touche"
    w.SetDirty()
    return true // evenement consomme
}

// Stop (optionnel) — implementer Stoppable si le widget a des goroutines
func (w *MonWidget) Stop() { /* fermer les goroutines internes */ }
```

### Widgets integres

| Widget | Constructeur | Description |
|---|---|---|
| Label | `NewLabel(text)` | Texte sur une ligne ; `SetFont(f)`, `SetAlign(a)` |
| Button | `NewButton(label)` | `OnClick(fn)` ; feedback visuel par inversion des couleurs |
| ProgressBar | `NewProgressBar()` | `SetValue(0.0-1.0)` ; utiliser `Expand` pour pleine largeur |
| StatusBar | `NewStatusBar(left, right)` | Barre noire texte blanc ; left = horloge si vide |
| Toggle | `NewToggle(label, on)` | Interrupteur on/off ; `OnChange(fn)` |
| Checkbox | `NewCheckbox(label, checked)` | Case a cocher ; `OnChange(fn)` |
| Slider | `NewSlider(min, max, value)` | Selecteur de valeur horizontal ; `OnChange(fn)` |
| Menu | `NewMenu(items)` | Liste scrollable d'items ; supporte les icones |
| NavButton | `NewNavButton(label)` | Bouton avec etat actif/desactive |
| NavBar | `NewNavBar(path...)` | Fil d'Ariane (ex. "Accueil > Config") |
| Carousel | `NewCarousel(items)` | Carrousel d'icones horizontal ; implemente `hScrollable` |
| ScrollList | `NewScrollList(rows)` | Liste scrollable de lignes texte |
| ActionSidebar | `NewActionSidebar(btns...)` | Barre d'actions laterale |
| ClockWidget | `NewClock()` | Horloge auto-rafraichie chaque minute ; implemente `Stoppable` |
| QRCode | `NewQRCode(content)` | QR code vectoriel |
| NetworkStatusBar | `NewNetworkStatusBar()` | Barre operateur (WiFi, IP...) |
| ImageWidget | `NewImageWidget(img)` | Rendu `image.Image` mis a l'echelle |
| ArcWidget | `NewArcWidget(...)` | Arc / indicateur circulaire |
| Spacer | `NewSpacer()` | Espace vide invisible ; utiliser avec `Expand` |
| Divider | `NewDivider()` | Separateur 2px, horizontal ou vertical |

### Conteneurs de layout

| Conteneur | Constructeur | Description |
|---|---|---|
| VBox | `NewVBox(children...)` | Empilement vertical des enfants |
| HBox | `NewHBox(children...)` | Rangee horizontale des enfants |
| Fixed | `NewFixed(w, h)` + `Put(w, x, y)` | Positionnement absolu en pixels |
| WithPadding | `WithPadding(px, w)` | Marge uniforme sur les 4 cotes |

Modificateurs de layout (enveloppent un widget pour modifier son comportement dans VBox/HBox) :

| Modificateur | Description |
|---|---|
| `Expand(w)` | Le widget prend tout l'espace restant sur l'axe principal |
| `FixedSize(w, px)` | Fixe la taille sur l'axe principal a `px` pixels |

### Helpers de haut niveau

| Fonction | Description |
|---|---|
| `ShowAlert(nav, title, msg, btns...)` | Pousse une scene modale alerte avec boutons |
| `ShowMenu(nav, title, items)` | Pousse une scene menu scrollable |
| `ShowTextInput(nav, placeholder, maxLen, onConfirm)` | Pousse une scene saisie texte avec clavier virtuel |
