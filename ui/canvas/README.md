# ui/canvas

Surface de dessin 1-bit pour ecrans e-ink, compatible avec `draw.Image`.

## Package independant

`ui/canvas` n'a aucune dependance vers les drivers hardware (`drivers/epd`,
`drivers/touch`) ni vers `ui/gui`. Il peut etre utilise seul pour generer des
images, ecrire des tests unitaires, ou exporter vers PNG.

## Exemple simple

```go
import (
    "image"
    "github.com/oioio-space/oioni/ui/canvas"
)

// Ecran Waveshare 2.13" : physique 122x250, Rot90 -> logique 250x122
c := canvas.New(122, 250, canvas.Rot90)

// Police embarquee (tailles disponibles : 8, 12, 16, 20, 24)
font := canvas.EmbeddedFont(16)

c.Clear() // tout blanc
c.DrawRect(image.Rect(0, 0, 250, 122), canvas.Black, false) // bordure
c.DrawText(10, 20, "Hello world", font, canvas.Black)

// Envoyer le buffer au driver EPD
display.DisplayBase(c.Bytes())
```

## Exemple avance : mise a jour partielle

```go
// Effacer la zone puis redessiner uniquement le texte modifie
c.DrawRect(image.Rect(10, 50, 200, 70), canvas.White, true)
c.DrawText(10, 52, "CPU: 87%", canvas.EmbeddedFont(12), canvas.Black)

// Convertir la zone logique modifiee en coordonnees physiques
physRect := c.PhysicalRect(image.Rect(10, 50, 200, 70))

// SubRegion aligne sur les frontieres d'octets (obligatoire pour DisplayPartial)
sub, _ := c.SubRegion(physRect)
display.DisplayPartial(sub.Bytes()) // ~0.3s au lieu de ~2s
```

## Points d'attention

**Convention bit/couleur** : bit=0 signifie noir, bit=1 signifie blanc. C'est
l'inverse de la convention habituelle des images. Toujours utiliser les
constantes `canvas.Black` et `canvas.White` pour eviter les confusions.

**Rotation Rot90** : rotation standard pour le Waveshare 2.13". Apres
`New(122, 250, canvas.Rot90)`, les coordonnees logiques sont 250 (largeur) x
122 (hauteur). `SetRotation` change le mapping de coordonnees mais ne
transforme pas le buffer physique.

**Layout physique** : `((physW+7)/8) * physH` octets, MSB en premier, 1 bit
par pixel. `Bytes()` renvoie une copie — sans risque de data race avec un
affichage concurrent sur un autre canvas.

**SubRegion** : aligne automatiquement les bords X sur les multiples de 8 px.
Necessaire pour `DisplayPartial` qui adresse la RAM EPD par colonnes d'octets.

**DrawText** : necessite une police obtenue via `EmbeddedFont(size)` ou
`LoadTTF`. Seuls les caracteres ASCII sont garantis avec les polices
embarquees. Les runes inconnues consomment une demi-hauteur de ligne en
largeur.

**ToImage** : convertit en `*image.Gray` (8 bits/px) pour les tests ou
l'export PNG. Non concurrent-safe : ne pas dessiner en parallele.

**Clipping** : `SetClip(r)` restreint tous les `SetPixel` suivants.
`ClearClip()` restaure le rectangle complet.

**Compatibilite draw.Image** : le canvas implemente l'interface `draw.Image`
(methodes `At`, `Set`, `Bounds`, `ColorModel`). Il est utilisable directement
avec le package standard `image/draw`.

## Reference API

### Creation

| Fonction | Description |
|---|---|
| `New(physW, physH int, rot Rotation) *Canvas` | Cree un canvas blanc avec les dimensions physiques et la rotation donnees |

### Constantes et types

| Identifiant | Valeur / Description |
|---|---|
| `Black` | `color.Gray{Y: 0}` — noir (bit=0) |
| `White` | `color.Gray{Y: 255}` — blanc (bit=1) |
| `Rot0`, `Rot90`, `Rot180`, `Rot270` | Valeurs du type `Rotation` |

### Pixels et formes

| Methode | Description |
|---|---|
| `SetPixel(x, y int, col color.Color)` | Pixel unique en coordonnees logiques, silencieusement clippe |
| `Fill(col color.Color)` | Remplit tout le canvas |
| `Clear()` | Equivalent de `Fill(White)` |
| `DrawRect(r image.Rectangle, col color.Color, filled bool)` | Rectangle plein ou contour |
| `DrawLine(x0, y0, x1, y1 int, col color.Color)` | Ligne (algorithme de Bresenham) |
| `DrawCircle(cx, cy, radius int, col color.Color, filled bool)` | Cercle (algorithme du point median) |

### Texte

| Fonction / Methode | Description |
|---|---|
| `EmbeddedFont(size int) Font` | Police bitmap embarquee (tailles : 8, 12, 16, 20, 24) |
| `LoadTTF(data []byte, sizePt, dpi float64) (Font, error)` | Police TrueType avec rendu a la demande |
| `DrawText(x, y int, text string, f Font, col color.Color)` | Rendu texte gauche-droite a partir du point (x, y) |

### Images

| Methode | Description |
|---|---|
| `DrawImage(pt image.Point, img image.Image)` | Rendu par seuillage 50% de luminance |
| `DrawImageScaled(r image.Rectangle, img image.Image)` | Mise a l'echelle nearest-neighbour + seuillage, letterboxe |
| `DrawImageScaledDithered(r image.Rectangle, img image.Image)` | Mise a l'echelle + tramage Floyd-Steinberg |
| `DrawImageScaledFill(r image.Rectangle, img image.Image)` | Remplissage (zoom-crop + flou) avec superposition centree et nette |

### Clipping et coordonnees

| Methode | Description |
|---|---|
| `SetClip(r image.Rectangle)` | Definit le rectangle de clipping (coordonnees logiques) |
| `ClearClip()` | Supprime le clipping (remet les bornes du canvas) |
| `SetRotation(r Rotation)` | Change le mapping de coordonnees (buffer inchange) |
| `PhysicalRect(r image.Rectangle) image.Rectangle` | Convertit un rectangle logique en coordonnees physiques |
| `SubRegion(r image.Rectangle) (*Canvas, image.Rectangle)` | Extrait un sous-canvas aligne sur les frontieres d'octets, renvoie aussi le rectangle physique aligne |

### Acces au buffer

| Methode | Description |
|---|---|
| `Bytes() []byte` | Copie du buffer physique a envoyer au driver EPD |
| `Bounds() image.Rectangle` | Rectangle en coordonnees logiques (interface `draw.Image`) |
| `At(x, y int) color.Color` | Couleur du pixel logique (interface `draw.Image`) |
| `ToImage() *image.Gray` | Conversion en image 8 bits/px pour tests et export PNG, non concurrent-safe |
