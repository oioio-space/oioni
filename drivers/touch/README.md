# drivers/touch

Pilote Go pour le controleur tactile capacitif GT1151 (5 points, I2C 0x14) sur le Waveshare 2.13" Touch e-Paper HAT.

## Package independant

Ce package n'a aucune dependance interne au projet oioni. Il peut etre importe isolement :

```
go get github.com/oioio-space/oioni/drivers/touch
```

Dependances externes uniquement : `golang.org/x/sys/unix` (ioctls I2C bas niveau), `periph.io/x/conn/v3` et `periph.io/x/host/v3` (GPIO). Aucun import depuis `drivers/epd`, `ui/gui`, ou tout autre package du depot.

---

## Exemple simple

Demarrage de la detection tactile et lecture des evenements dans une boucle.

```go
package main

import (
    "context"
    "log"
    "os/signal"
    "syscall"
    "github.com/oioio-space/oioni/drivers/touch"
)

func main() {
    td, err := touch.New(touch.Config{
        I2CDevice: "/dev/i2c-1",
        I2CAddr:   0x14,
        PinTRST:   22, // BCM 22 — reset hardware
        PinINT:    27, // BCM 27 — interruption falling edge
    })
    if err != nil {
        log.Fatal(err)
    }
    defer td.Close()

    ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
    defer stop()

    events, err := td.Start(ctx)
    if err != nil {
        log.Fatal(err)
    }

    for ev := range events {
        for _, pt := range ev.Points {
            log.Printf("touch id=%d x=%d y=%d taille=%d", pt.ID, pt.X, pt.Y, pt.Size)
        }
    }

    // La goroutine s'arrete quand ctx est annule. Verifier l'erreur residuelle :
    if err := td.Err(); err != nil {
        log.Printf("erreur touch: %v", err)
    }
}
```

---

## Exemple avance : fake HAL pour les tests

Les interfaces `I2CConn`, `OutputPin` et `InterruptPin` sont injectables via le constructeur interne `newDetector` (accessible dans les tests en `package touch`).

```go
// fakeI2C retourne des reponses pre-programmees en sequence.
type fakeI2C struct {
    responses [][]byte
    idx       int
}

func (f *fakeI2C) Tx(w, r []byte) error {
    if len(r) == 0 {
        return nil
    }
    if f.idx < len(f.responses) {
        copy(r, f.responses[f.idx])
        f.idx++
    }
    return nil
}

// fakeOutputPin accepte Out() sans rien faire (TRST).
type fakeOutputPin struct{}
func (f *fakeOutputPin) Out(bool) error { return nil }

// fakeINT simule un falling edge via un channel.
type fakeINT struct{ trigger chan struct{} }
func (f *fakeINT) WaitFalling(ctx context.Context) error {
    select {
    case <-f.trigger: return nil
    case <-ctx.Done(): return ctx.Err()
    }
}

func TestTouchEvent(t *testing.T) {
    intCh := make(chan struct{}, 1)
    i2c := &fakeI2C{
        responses: [][]byte{
            // readReg(0x8140, 4) : product ID != 0
            {0x39, 0x35, 0x30, 0x31},
            // readReg(0x814E, 1) : flag=0x81 (bit7=1, count=1)
            {0x81},
            // readReg(0x814F, 8) : ID=1, X=30, Y=60, Size=8
            {0x01, 0x1E, 0x00, 0x3C, 0x00, 0x08, 0x00, 0x00},
        },
    }

    d := newDetector(i2c, &fakeOutputPin{}, &fakeINT{trigger: intCh})

    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
    defer cancel()

    ch, err := d.Start(ctx)
    if err != nil {
        t.Fatal(err)
    }

    intCh <- struct{}{} // simuler un falling edge sur INT

    evt := <-ch
    if evt.Points[0].X != 30 || evt.Points[0].Y != 60 {
        t.Errorf("coordonnees attendues (30,60), obtenu (%d,%d)",
            evt.Points[0].X, evt.Points[0].Y)
    }
}
```

---

## Points d'attention

**Coordonnees brutes — la rotation appartient a l'appelant.**
Le GT1151 renvoie les coordonnees dans le repere physique du capteur, sans transformation. Sur le montage Waveshare 2.13" Touch HAT, le mapping vers l'espace logique de l'ecran est `logX = pt.Y`, `logY = pt.X` — mais ce remapping est applique par `ui/gui` dans le `Navigator`, pas ici. Le package `touch` retourne toujours les coordonnees brutes.

**Reset materiel au demarrage : 400 ms incompressibles.**
`Start()` appelle `gt1151Reset()` avant toute lecture I2C. La sequence est : TRST haut 100 ms → bas 100 ms → haut 200 ms. Ne pas appeler `Start()` en boucle rapide apres un crash.

**Detection de presence : product ID tout-zeros = pas de reponse I2C.**
Apres le reset, `Start()` lit 4 octets a `0x8140`. Si tous sont `0x00`, le GT1151 ne repond pas sur le bus. Verifier l'adresse (par defaut `0x14`), le cablage, et `dtparam=i2c_arm=on` dans `config.txt`.

**Pas d'interruption materielle GPIO sous gokrazy.**
`WaitFalling` poll la pin INT toutes les **10 ms**. Pour un ecran e-ink (refresh >= 300 ms), cette latence est negligeable.

**Evenements droppes si le consommateur est lent.**
Le channel retourne par `Start()` a une capacite de 8. Les nouveaux evenements sont silencieusement ignores quand le channel est plein (`select { case ch <- evt: default: }`). Traiter les evenements rapidement.

**`Err()` n'est valide qu'apres fermeture du channel.**
Appeler `Err()` pendant que la goroutine tourne retourne toujours `nil`. Attendre la sortie de la boucle `for range` avant de consulter `Err()`.

**`Close()` ne cancelle pas le contexte.**
`Close()` libere les ressources hardware (fd I2C, GPIO). Pour arreter la goroutine lancee par `Start()`, annuler le `context.Context` passe en argument. Sequence correcte : `cancel()` puis `Close()`.

**`periph.io host.Init()` est protege par un `sync.Once`.**
Si `drivers/epd` ou un autre package appelle aussi `host.Init()`, il n'y a pas de conflit — la fonction est idempotente au niveau du processus.

**Acquittement apres lecture obligatoire.**
Le driver ecrit `0x00` dans le registre `0x814E` apres chaque lecture valide. Sans cet acquittement, le GT1151 ne signale pas le prochain evenement. En cas d'erreur I2C lors de l'ecriture, la goroutine s'arrete et ferme le channel.

---

## Reference API

### Config

```go
type Config struct {
    I2CDevice string  // chemin du bus I2C, ex. "/dev/i2c-1"
    I2CAddr   uint16  // adresse I2C du GT1151, typiquement 0x14
    PinTRST   int     // numero BCM du pin de reset hardware (ex. 22)
    PinINT    int     // numero BCM du pin d'interruption (ex. 27)
}
```

### TouchPoint

```go
type TouchPoint struct {
    ID   uint8   // identifiant du doigt (0-4, stable pendant le contact)
    X    uint16  // coordonnee horizontale brute dans le repere capteur
    Y    uint16  // coordonnee verticale brute dans le repere capteur
    Size uint8   // surface de contact (unite arbitraire, indicatif)
}
```

### TouchEvent

```go
type TouchEvent struct {
    Points []TouchPoint  // entre 1 et 5 points actifs simultanement
    Time   time.Time     // horodatage de la lecture I2C
}
```

### Interfaces HAL (injectables pour les tests)

| Interface | Methode | Role |
|-----------|---------|------|
| `I2CConn` | `Tx(w, r []byte) error` | Transaction I2C write-then-read atomique |
| `OutputPin` | `Out(high bool) error` | Pin GPIO en sortie : true = HIGH, false = LOW (TRST) |
| `InterruptPin` | `WaitFalling(ctx context.Context) error` | Attend un front descendant ou ctx annule (INT) |

### Constructeur

```go
// New ouvre le peripherique I2C et les deux pins GPIO (TRST en sortie, INT en entree).
// Retourne une erreur si un peripherique est absent ou si periph.io ne s'initialise pas.
func New(cfg Config) (*Detector, error)
```

### Methodes de Detector

```go
// Start reinitialise le GT1151 (sequence reset hardware, verification product ID),
// puis lance la goroutine d'evenements.
// Retourne (chan, nil) si le GT1151 repond. Le channel est ferme quand ctx est
// annule ou sur erreur I2C fatale dans la goroutine.
func (d *Detector) Start(ctx context.Context) (<-chan TouchEvent, error)

// Err retourne la premiere erreur I2C survenue pendant la session.
// Valeur definie uniquement apres fermeture du channel de Start().
func (d *Detector) Err() error

// Close ferme les file descriptors I2C et GPIO.
// Ne stoppe pas la goroutine : annuler le contexte avant d'appeler Close().
func (d *Detector) Close() error
```

### Registres GT1151 (reference interne)

| Registre | Taille | Role |
|----------|--------|------|
| `0x8140` | 4 bytes | Product ID (ex. `"9501"` ou `"1151"` en ASCII) |
| `0x814E` | 1 byte | Flag : bit 7 = buffer valide, bits 3-0 = nombre de points (1-5) |
| `0x814F` | 8 * N  | Donnees de contact : 8 octets par point (ID, X lo, X hi, Y lo, Y hi, Size, padding x2) |

Apres lecture des donnees, ecrire `0x00` dans `0x814E` pour acquitter et autoriser le prochain evenement.
