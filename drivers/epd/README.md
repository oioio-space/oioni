# drivers/epd

Pilote Go pour l'ecran e-ink Waveshare EPD 2.13" V4 (122x250 px, 1 bit/pixel) via SPI.

## Package independant

Ce package n'a aucune dependance interne au projet oioni. Il peut etre importe dans n'importe quel projet Go :

```
go get github.com/oioio-space/oioni/drivers/epd
```

Dependances externes uniquement : `golang.org/x/sys`, `periph.io/x/conn/v3`, `periph.io/x/host/v3`. Aucun import depuis d'autres packages du depot oioni.

---

## Exemple simple

Affichage d'un buffer blanc en mode full refresh, puis mise en veille.

```go
package main

import (
    "log"
    "github.com/oioio-space/oioni/drivers/epd"
)

func main() {
    d, err := epd.New(epd.Config{
        SPIDevice: "/dev/spidev0.0",
        SPISpeed:  4_000_000,
        PinRST:    17,
        PinDC:     25,
        PinCS:     8,
        PinBUSY:   24,
    })
    if err != nil {
        log.Fatal(err)
    }
    defer d.Close()

    // Buffer blanc : 0xFF = tous les pixels blancs (1=blanc, 0=noir)
    buf := make([]byte, epd.BufferSize)
    for i := range buf {
        buf[i] = 0xFF
    }

    if err := d.Init(epd.ModeFull); err != nil {
        log.Fatal(err)
    }
    if err := d.DisplayFull(buf); err != nil {
        log.Fatal(err)
    }
    d.Sleep()
}
```

---

## Exemple avance : mises a jour partielles avec interfaces injectables

Pour les tests ou les environnements sans materiel, les trois interfaces HAL (`SPIConn`, `OutputPin`, `InputPin`) sont substituables par des fakes dans des tests en `package epd` (acces a `newDisplay` non exportee).

### Cycle production recommande

```go
// 1. Init full + DisplayBase pour etablir le frame de reference (registre 0x26).
//    Cette etape est obligatoire avant tout DisplayPartial.
if err := d.Init(epd.ModeFull); err != nil { log.Fatal(err) }
if err := d.DisplayBase(buf); err != nil { log.Fatal(err) }

// 2. N DisplayPartial consecutifs (~0.3 s chacun, sans flash plein ecran).
//    Le premier appel effectue un mini-reset ; les suivants le sautent.
for _, newBuf := range updates {
    if err := d.DisplayPartial(newBuf); err != nil { log.Fatal(err) }
}

// 3. Apres une longue inactivite (ex. 24h), purger le ghosting profond :
if err := d.DisplayRegenerate(); err != nil { log.Fatal(err) }
// L'ecran est blanc apres DisplayRegenerate : re-rendre l'UI.
```

### Fake HAL pour les tests (package interne)

Les fakes suivants permettent de tester la logique d'affichage sans materiel.
Ils doivent se trouver dans un fichier `_test.go` avec `package epd` (test interne).

```go
// fakeSPI capture tous les octets envoyes via Tx.
type fakeSPI struct{ log [][]byte }
func (f *fakeSPI) Tx(w []byte) error {
    f.log = append(f.log, append([]byte(nil), w...))
    return nil
}

// fakeOutputPin enregistre la derniere valeur ecrite.
type fakeOutputPin struct{ last bool; transitions int }
func (f *fakeOutputPin) Out(high bool) error {
    if high != f.last { f.transitions++ }
    f.last = high
    return nil
}

// fakeInputPin retourne une valeur configurable (false = BUSY bas = pret).
type fakeInputPin struct{ val bool }
func (f *fakeInputPin) Read() bool { return f.val }

func TestCyclePartial(t *testing.T) {
    spi := &fakeSPI{}
    busy := &fakeInputPin{val: false} // BUSY=low = controleur pret
    rst := &fakeOutputPin{}

    d := newDisplay(spi, rst, &fakeOutputPin{}, &fakeOutputPin{}, busy)

    buf := make([]byte, epd.BufferSize)
    d.Init(epd.ModeFull)
    d.DisplayBase(buf)

    // Premier DisplayPartial : mini-reset obligatoire (RST transitions attendus)
    rst.transitions = 0
    d.DisplayPartial(buf)
    if rst.transitions == 0 {
        t.Error("premier partial doit emettre un mini-reset")
    }

    // Deuxieme DisplayPartial consecutif : reset saute
    rst.transitions = 0
    d.DisplayPartial(buf)
    if rst.transitions != 0 {
        t.Errorf("deuxieme partial consecutif ne doit pas resetter RST")
    }
}
```

---

## Points d'attention

**`DisplayBase` est obligatoire avant tout `DisplayPartial`**
Le mode partial compare le nouveau frame au contenu du registre RAM `0x26` (frame de reference). Sans `DisplayBase`, ce registre contient des donnees aleatoires et l'ecran affiche des artefacts visuels. Toujours appeler `Init(ModeFull)` + `DisplayBase(buf)` avant la premiere sequence de partiaux.

**Convention de bit : 0 = noir, 1 = blanc**
Un byte `0x00` produit 8 pixels noirs. Un byte `0xFF` produit 8 pixels blancs. C'est l'inverse de l'intuition courante en imagerie.

**`BufferSize` = 4000 octets, pas 3813**
La largeur de 122 pixels necessite 16 bytes par ligne (et non 15,25). Le buffer fait 16 x 250 = 4000 octets. Les 6 bits de rembourrage en fin de chaque ligne sont ignores par le controleur.

**`partialInited` : optimisation du mini-reset**
Lors d'appels `DisplayPartial` consecutifs, le mini-reset (RST bas 1 ms) n'est execute qu'au premier appel (~40 ms economises par appel suivant). Tout appel a `Init()`, quel que soit le mode, remet `partialInited = false` et force un nouveau mini-reset au prochain partial.

**BUSY polling avec timeout de 10 s**
`waitBusy` interroge la pin BUSY toutes les 10 ms. Apres 10 s sans liberation, l'erreur `"waitBusy: BUSY pin stuck high"` remonte via la valeur de retour de la methode en cours. Cela peut indiquer un cablage defectueux ou un controleur bloque.

**`DisplayRegenerate` dure ~4 s et laisse l'ecran blanc**
Deux cycles full refresh successifs (noir puis blanc) purgent le ghosting profond accumule apres une longue inactivite. L'ecran est entierement blanc a la sortie : l'appelant doit re-rendre l'interface.

**`Sleep` ne libere pas les ressources systeme**
`Sleep()` envoie uniquement la commande deep sleep (registre `0x10`) au controleur. Pour fermer les file descriptors SPI et GPIO, appeler `Close()`.

**periph.io initialise une seule fois par processus**
`host.Init()` est protege par un `sync.Once` dans le HAL. Pas de conflit si le package `drivers/touch` est utilise dans le meme binaire.

---

## Reference API

### Constantes

| Nom | Valeur | Description |
|-----|--------|-------------|
| `Width` | 122 | Largeur en pixels (axe fast-scan) |
| `Height` | 250 | Hauteur en pixels |
| `BufferSize` | 4000 | Taille du framebuffer en octets |

### `Mode` — Strategie de rafraichissement

| Constante | Duree approx. | Description |
|-----------|---------------|-------------|
| `ModeFull` | ~2 s | Meilleure qualite, purge le ghosting |
| `ModeFast` | ~0.5 s | Refresh plein ecran rapide |
| `ModePartial` | ~0.3 s | Mise a jour sans flash, necessite un `DisplayBase` prealable |

### `Config` — Configuration materielle

```go
type Config struct {
    SPIDevice string  // ex. "/dev/spidev0.0"
    SPISpeed  uint32  // ex. 4_000_000
    PinRST    int     // BCM 17 par defaut
    PinDC     int     // BCM 25
    PinCS     int     // BCM 8
    PinBUSY   int     // BCM 24
}
```

### Interfaces HAL (injectables pour les tests)

| Interface | Methode | Role |
|-----------|---------|------|
| `SPIConn` | `Tx(w []byte) error` | Connexion SPI write-only |
| `OutputPin` | `Out(high bool) error` | Pin GPIO en sortie (RST, DC, CS) |
| `InputPin` | `Read() bool` | Pin GPIO en entree (BUSY) |

### Fonctions et methodes

**`New(cfg Config) (*Display, error)`**
Ouvre toutes les ressources materiel (SPI + 4 pins GPIO via periph.io). Retourne une erreur si un peripherique est absent ou inaccessible.

**`(*Display) Init(m Mode) error`**
Initialise le controleur dans le mode choisi. A appeler avant toute operation d'affichage, et a chaque changement de mode.

**`(*Display) DisplayFull(buf []byte) error`**
Ecrit le framebuffer dans la RAM `0x24` et declenche un refresh complet (sequence `0xF7`, ~2 s). Utiliser apres `Init(ModeFull)`.

**`(*Display) DisplayFast(buf []byte) error`**
Ecrit le framebuffer et declenche un refresh rapide (sequence `0xC7`, ~0.5 s). Utiliser apres `Init(ModeFast)`.

**`(*Display) DisplayBase(buf []byte) error`**
Ecrit le framebuffer dans les deux banques RAM (`0x24` nouveau frame + `0x26` frame de reference) et declenche un refresh complet. Obligatoire avant toute sequence `DisplayPartial`.

**`(*Display) DisplayPartial(buf []byte) error`**
Ecrit le framebuffer complet et declenche un refresh partiel (~0.3 s). Le premier appel apres un `Init` effectue un mini-reset et configure les registres du mode partial ; les appels consecutifs sautent cette etape.

**`(*Display) DisplayRegenerate() error`**
Cycle noir puis blanc pour purger le ghosting profond. Duree ~4 s. L'ecran est blanc a la sortie.

**`(*Display) Sleep() error`**
Envoie la commande deep sleep (`0x10`). Le controleur consomme environ 0 mA en veille. Ne libere pas les ressources systeme.

**`(*Display) Close() error`**
Ferme tous les file descriptors (SPI + GPIO). Retourne la premiere erreur rencontree.
