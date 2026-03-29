# usbgadget

Configure des USB composite gadgets Linux via configfs depuis Go — charge les modules kernel, monte configfs, crée l'arbre de répertoires et bind le UDC en un seul appel `Enable()`.

---

## Package indépendant

Ce répertoire contient trois sub-packages sans dépendances croisées entre eux :

| Package | Rôle |
|---------|------|
| `usbgadget` | Orchestration configfs : monte le filesystem, instancie les fonctions, bind le UDC |
| `usbgadget/functions` | Drivers de fonctions USB (RNDIS, ECM, HID, MassStorage, ACM…) — utilisables seuls |
| `usbgadget/ducky` | Parser et executor DuckyScript — dépend uniquement de l'interface `Keyboard` (un `WriteReport`) |

`functions` et `ducky` ne s'importent pas mutuellement. `usbgadget` importe `functions` mais ni `ducky` ni aucune dépendance applicative.

---

## Budget d'endpoints (DWC2 / BCM2835)

Le contrôleur DWC2 présent sur le BCM2835 (Raspberry Pi Zero / Zero 2 W) dispose d'un maximum de **7 endpoints** au-delà de EP0. Chaque USB function consomme un nombre fixe d'EPs :

| Function | EPs consommés |
|----------|:-------------:|
| RNDIS | 3 |
| ECM | 3 |
| NCM | 3 |
| EEM | 2 |
| Subset (geth) | 2 |
| HID (clavier ou souris) | 1 |
| MassStorage | 2 |
| ACM Serial | 3 |
| GSER Serial | 2 |
| MIDI | 2 |
| UAC1 / UAC2 | 2–3 |

### Combinaisons types

| Combinaison | EPs total | Compatible DWC2 |
|-------------|:---------:|:---------------:|
| RNDIS + ECM + HID | 3+3+1 = 7 | Oui |
| RNDIS + MassStorage | 3+2 = 5 | Oui |
| RNDIS + ECM | 3+3 = 6 | Oui |
| RNDIS + ECM + MassStorage | 3+3+2 = 8 | **Non** — dépasse le budget |

Lorsque le budget est dépassé, `Enable()` retourne une erreur explicite (voir section [Points d'attention](#points-dattention)) au lieu d'un échec silencieux.

---

## Exemple simple : gadget RNDIS

```go
import (
    "github.com/oioio-space/oioni/drivers/usbgadget"
)

g, err := usbgadget.New(
    usbgadget.WithName("netgadget"),
    usbgadget.WithVendorID(0x1d6b, 0x0104),
    usbgadget.WithStrings("0x409", "ACME Corp", "USB Network", "net001"),
    usbgadget.WithRNDIS(),
)
if err != nil {
    log.Fatal(err)
}
if err := g.Enable(); err != nil {
    log.Fatal(err)
}
defer g.Disable()
```

---

## Exemple avancé : gadget composite RNDIS + ECM + HID

Récupération du nom d'interface réseau côté gadget et lecture des LEDs clavier :

```go
import (
    "context"
    "github.com/oioio-space/oioni/drivers/usbgadget"
    "github.com/oioio-space/oioni/drivers/usbgadget/functions"
)

rndis := functions.RNDIS(
    functions.WithRNDISHostAddr("02:00:00:aa:bb:01"), // MAC stable → même bail DHCP
    functions.WithRNDISDevAddr("02:00:00:aa:bb:02"),
)
ecm := functions.ECM(
    functions.WithECMHostAddr("02:00:00:cc:dd:01"),
    functions.WithECMDevAddr("02:00:00:cc:dd:02"),
)
kbd := functions.Keyboard()

g, err := usbgadget.New(
    usbgadget.WithName("composite"),
    usbgadget.WithVendorID(0x1d6b, 0x0104),
    usbgadget.WithStrings("0x409", "ACME", "Composite Device", "comp001"),
    usbgadget.WithFunc(rndis), // RNDIS doit être en premier (compatibilité Windows)
    usbgadget.WithFunc(ecm),
    usbgadget.WithFunc(kbd),
)
if err != nil {
    log.Fatal(err)
}
if err := g.Enable(); err != nil {
    log.Fatal(err)
}
defer g.Disable()

// Nom d'interface réseau côté Pi (ex: "usb0")
if ifname, err := rndis.IfName(); err == nil {
    fmt.Printf("RNDIS interface: %s\n", ifname)
}
if ifname, err := ecm.IfName(); err == nil {
    fmt.Printf("ECM interface: %s\n", ifname)
}

// Compteurs réseau
if stats, err := rndis.ReadStats(); err == nil {
    fmt.Printf("rx=%d tx=%d bytes\n", stats.RxBytes, stats.TxBytes)
}

// LEDs clavier en temps réel (NumLock, CapsLock, ScrollLock)
ctx, cancel := context.WithCancel(context.Background())
defer cancel()
leds, _ := kbd.ReadLEDs(ctx)
for state := range leds {
    fmt.Printf("CapsLock=%v NumLock=%v\n", state.CapsLock, state.NumLock)
}
```

---

## Sub-package : `functions`

Chaque constructeur retourne un `*XxxFunc` qui implémente `Function` (interface `TypeName`, `InstanceName`, `Configure`). Les fonctions réseau implémentent aussi `IfName()` et `ReadStats()`.

### Fonctions réseau

| Constructeur | Protocol | EPs | Plateformes cibles |
|---|---|:---:|---|
| `RNDIS(opts...)` | RNDIS | 3 | Windows (doit être en premier dans un composite) |
| `ECM(opts...)` | CDC ECM | 3 | Linux, macOS |
| `NCM(opts...)` | CDC NCM | 3 | Linux 3.10+ (débit élevé) |
| `EEM(opts...)` | CDC EEM | 2 | Linux-to-Linux (bulk-only, sans interface de contrôle) |
| `Subset(opts...)` | CDC Subset | 2 | Hosts embarqués, anciens Linux |

Options communes réseau : `WithXxxDevAddr(mac)`, `WithXxxHostAddr(mac)`, `WithXxxQMult(n)`.

Méthodes post-`Enable()` : `IfName() (string, error)`, `HostAddr() (string, error)`, `ReadStats() (NetStats, error)`.

### HID

| Constructeur | Protocol | EPs | Device |
|---|---|:---:|---|
| `Keyboard(opts...)` | HID boot keyboard | 1 | `/dev/hidg0` (ou N suivant l'ordre de création) |
| `Mouse(opts...)` | HID boot mouse | 1 | `/dev/hidgN` |

Méthodes : `WriteReport([]byte) error`, `ReadLEDs(ctx) (<-chan LEDState, error)`, `DevPath() string`.

Format rapport clavier : `[modifier, 0x00, key1, key2, key3, key4, key5, key6]`.
Format rapport souris : `[buttons, deltaX, deltaY, wheel]`.

### Storage

| Constructeur | EPs | Remarque |
|---|:---:|---|
| `MassStorage(file, opts...)` | 2 | `WithCDROM(bool)`, `WithReadOnly(bool)`, `WithRemovable(bool)` |

### Serial

| Constructeur | Driver | EPs | Device gadget |
|---|---|:---:|---|
| `ACMSerial()` | CDC ACM | 3 | `/dev/ttyGSN` ; host: `/dev/ttyACMN` |
| `Serial()` | GSER | 2 | `/dev/ttyGSN` ; host: USB serial |
| `OBEX()` | OBEX | 2 | `/dev/ttyGSN` |

Les numéros de port `ttyGSN` sont attribués dans l'ordre de création entre ACM, GSER et OBEX.

### Audio / MIDI

| Constructeur | EPs | Remarque |
|---|:---:|---|
| `UAC2(opts...)` | 2–3 | USB Audio Class 2 ; Windows 10+, Linux, macOS sans driver |
| `UAC1(opts...)` | 2–3 | USB Audio Class 1 ; compatibilité maximale |
| `MIDI(opts...)` | 2 | USB MIDI ; accessible via `/dev/snd/midiC0D0` |

### Printer / Loopback

| Constructeur | EPs | Remarque |
|---|:---:|---|
| `Printer(opts...)` | 2 | Print jobs sur `/dev/usb/lp0` |
| `Loopback(opts...)` | 2 | Benchmark USB (usbtest côté host) |

---

## Sub-package : `ducky`

Parser et executor **DuckyScript V2** pour USB HID, basé sur une grammaire PEG ([pigeon](https://github.com/mna/pigeon)).

### Layouts disponibles

| Variable | Disposition |
|---|---|
| `ducky.LayoutEN` | QWERTY US (par défaut) |
| `ducky.LayoutFR` | AZERTY FR |

### API principale

```go
// ParseScript parse un script en []Instruction sans l'exécuter.
func ParseScript(script string) ([]Instruction, error)

// ExecuteScript parse et exécute un script sur le clavier donné.
// layout : LayoutEN ou LayoutFR selon la disposition du host cible.
func ExecuteScript(ctx context.Context, kbd Keyboard, script string, layout *Layout) error

// TypeString tape une chaîne caractère par caractère (délai 5 ms entre touches).
func TypeString(ctx context.Context, kbd Keyboard, text string, layout *Layout) error

// PressKeys appuie simultanément sur une combinaison de touches nommées puis les relâche.
// Exemples : ["CTRL","ALT","DELETE"], ["GUI","r"], ["ENTER"]
func PressKeys(ctx context.Context, kbd Keyboard, keys []string) error

// MouseMove envoie un mouvement relatif (dx, dy dans [-127, 127]).
func MouseMove(ctx context.Context, m Mouse, dx, dy int8) error

// MouseClick envoie un clic bouton (0x01=gauche, 0x02=droit, 0x04=milieu).
func MouseClick(ctx context.Context, m Mouse, btn byte) error
```

### Interface `Keyboard`

L'interface `ducky.Keyboard` est satisfaite par `*functions.HIDFunc` créé via `functions.Keyboard()` :

```go
type Keyboard interface {
    WriteReport(report []byte) error
}
```

### Commandes DuckyScript supportées

| Commande | Description |
|---|---|
| `STRING <texte>` | Tape le texte sans Enter |
| `STRINGLN <texte>` | Tape le texte puis appuie sur Enter |
| `DELAY <ms>` | Pause en millisecondes |
| `DEFAULT_DELAY <ms>` | Délai inter-commandes pour toute la suite du script |
| `ENTER`, `TAB`, `ESC`… | Touche nominative seule |
| `CTRL ALT DELETE` | Combinaison de touches (espace = simultané) |
| `GUI r`, `SHIFT F10`… | Combos modificateur + touche |
| `REM <commentaire>` | Ligne ignorée |

Touches nommées complètes : CTRL/CONTROL, SHIFT, ALT, GUI/WINDOWS/COMMAND, ENTER/RETURN, ESC/ESCAPE, TAB, BACKSPACE, DELETE/DEL, INSERT, HOME, END, PAGEUP, PAGEDOWN, UPARROW, DOWNARROW, LEFTARROW, RIGHTARROW, SPACE, CAPSLOCK, NUMLOCK, SCROLLLOCK, PRINTSCREEN, PAUSE/BREAK, MENU/APP, F1–F12.

### Exemple

```go
import (
    "context"
    "github.com/oioio-space/oioni/drivers/usbgadget/functions"
    "github.com/oioio-space/oioni/drivers/usbgadget/ducky"
)

kb := functions.Keyboard() // *HIDFunc, satisfies ducky.Keyboard interface

// kb doit être ajouté à un gadget et Enable() appelé avant d'envoyer des reports.

script := "STRING Hello World\nENTER"
if err := ducky.ExecuteScript(ctx, kb, script, ducky.LayoutEN); err != nil {
    log.Fatal(err)
}
```

---

## Points d'attention

### configfs est monté automatiquement

`Enable()` monte configfs sur `/sys/kernel/config` si ce n'est pas déjà fait. Aucune configuration système préalable n'est requise.

### RNDIS doit être la première fonction

Pour que Windows reconnaisse correctement un gadget composite, RNDIS **doit être déclaré en premier**. `priority.go` trie automatiquement les fonctions dans le bon ordre (rndis=0, ecm=1, hid=3…), même si elles ont été ajoutées dans un ordre différent via `WithFunc`.

### Dépassement du budget d'endpoints : erreur explicite

Le kernel Linux accepte silencieusement l'écriture du UDC même quand le bind échoue faute d'endpoints disponibles — le fichier `UDC` revient simplement à vide. `udc.go` détecte ce cas et retourne une erreur claire :

```
UDC bind failed: gadget did not attach to fe980000.usb
(too many functions for the controller's endpoint budget?)
```

### ECM : hw_type=14 et ARP

Sur le kernel embarqué utilisé par ce projet, ECM est exposé avec `hw_type=14` (non standard) au lieu de `hw_type=1` (Ethernet). Certains composants réseau (notamment le noyau Linux côté host) ne traitent pas l'ARP correctement dans ce cas. Contournement : injecter les entrées ARP via netlink (`ip neigh add` ou l'API netlink Go) plutôt que de laisser le kernel gérer la résolution ARP automatiquement.

### Modules kernel embarqués

Les modules `.ko` ARM64 (dwc2, libcomposite, usb_f_rndis, usb_f_ecm, usb_f_hid…) sont embarqués dans le binaire via `modules/embed.go`. `Enable()` les charge automatiquement. Les erreurs `EEXIST` sont ignorées (module déjà chargé ou compilé en dur dans le kernel gokrazy). Les modules manquants sont sautés silencieusement.

---

## Référence API

### Package `usbgadget`

```go
// Création du gadget
func New(opts ...Option) (*Gadget, error)

// Options de configuration
func WithName(name string) Option
func WithVendorID(vendor, product uint16) Option
func WithStrings(langID, manufacturer, product, serial string) Option
func WithUSBVersion(major, minor uint8) Option

// Ajout d'une fonction avec référence (pour appeler des méthodes post-Enable)
func WithFunc(f functions.Function) Option

// Raccourcis (sans référence à la fonction)
func WithRNDIS(opts ...functions.RNDISOption) Option
func WithECM(opts ...functions.ECMOption) Option
func WithNCM(opts ...functions.NCMOption) Option
func WithEEM(opts ...functions.EEMOption) Option
func WithSubset(opts ...functions.SubsetOption) Option
func WithMassStorage(file string, opts ...functions.MassStorageOption) Option
func WithACMSerial() Option
func WithSerial() Option
func WithOBEX() Option
func WithMIDI(opts ...functions.MIDIOption) Option
func WithUAC1(opts ...functions.UAC1Option) Option
func WithUAC2(opts ...functions.UAC2Option) Option
func WithPrinter(opts ...functions.PrinterOption) Option
func WithLoopback(opts ...functions.LoopbackOption) Option

// Cycle de vie
func (g *Gadget) Enable() error   // charge modules, monte configfs, bind UDC
func (g *Gadget) Disable() error  // unbind UDC, nettoie configfs
func (g *Gadget) Reload() error   // Disable() puis Enable()
```

### Package `functions`

```go
type Function interface {
    TypeName() string
    InstanceName() string
    Configure(dir string) error
}

// Méthodes communes aux fonctions réseau (RNDIS, ECM, NCM, EEM, Subset)
IfName() (string, error)
HostAddr() (string, error)
ReadStats() (NetStats, error)

type NetStats struct {
    RxBytes, TxBytes     uint64
    RxPackets, TxPackets uint64
    RxErrors, TxErrors   uint64
    RxDropped, TxDropped uint64
}

// Méthodes HID
WriteReport(report []byte) error
ReadLEDs(ctx context.Context) (<-chan LEDState, error)
DevPath() string

type LEDState struct {
    NumLock, CapsLock, ScrollLock, Compose, Kana bool
}

// Méthodes Serial (ACMFunc, SerialFunc, OBEXFunc)
DevPath() string  // ex: "/dev/ttyGS0"
```

### Package `ducky`

```go
type Keyboard interface { WriteReport([]byte) error }
type Mouse    interface { WriteReport([]byte) error }

func ParseScript(script string) ([]Instruction, error)
func ExecuteScript(ctx context.Context, kbd Keyboard, script string, layout *Layout) error
func TypeString(ctx context.Context, kbd Keyboard, text string, layout *Layout) error
func PressKeys(ctx context.Context, kbd Keyboard, keys []string) error
func MouseMove(ctx context.Context, m Mouse, dx, dy int8) error
func MouseClick(ctx context.Context, m Mouse, btn byte) error

var LayoutEN *Layout  // QWERTY US
var LayoutFR *Layout  // AZERTY FR

// Constantes modifier byte
const (
    ModLCtrl, ModLShift, ModLAlt, ModLGUI byte = 0x01, 0x02, 0x04, 0x08
    ModRCtrl, ModRShift, ModRAlt, ModRGUI byte = 0x10, 0x20, 0x40, 0x80
)
```
