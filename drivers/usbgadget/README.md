# usbgadget

Configure des USB composite gadgets Linux via configfs depuis Go — charge les modules kernel ARM64, monte configfs, cree l'arbre de repertoires, bind le UDC en un seul appel `Enable()`.

---

## Package independant

Ce repertoire contient trois sous-packages sans dependances croisees entre eux :

| Package | Role | Dependances internes |
|---------|------|----------------------|
| `usbgadget` | Orchestration configfs : modules, mount, instanciation des fonctions, bind UDC | importe `functions` uniquement |
| `usbgadget/functions` | Drivers de fonctions USB individuels (RNDIS, ECM, EEM, HID, MassStorage, ACM...) | aucune |
| `usbgadget/ducky` | Parser + executor DuckyScript — satisfait par n'importe quelle implementation de `Keyboard` | aucune |

`functions` et `ducky` ne s'importent pas mutuellement. `ducky` peut etre utilise avec n'importe quelle implementation de `WriteReport` ; il ne depasse pas `functions`.

---

## Budget d'endpoints (DWC2 / BCM2835)

Le controleur DWC2 du BCM2835 (Raspberry Pi Zero / Zero 2 W) dispose d'un maximum de **7 endpoints** au-dela de EP0. Chaque USB function consomme un nombre fixe d'EPs :

| Function | EPs consommes |
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
| UAC1 / UAC2 | 2-3 |

### Combinaisons types

| Combinaison | EPs total | Compatible DWC2 |
|-------------|:---------:|:---------------:|
| RNDIS + HID | 3+1 = 4 | Oui |
| RNDIS + ECM | 3+3 = 6 | Oui |
| RNDIS + ECM + HID | 3+3+1 = 7 | Oui (budget exact) |
| RNDIS + MassStorage | 3+2 = 5 | Oui |
| RNDIS + ECM + MassStorage | 3+3+2 = 8 | **Non** — depasse le budget |
| RNDIS + ECM + ACM | 3+3+3 = 9 | **Non** |

Quand le budget est depasse, `Enable()` retourne une erreur explicite au lieu d'un echec silencieux (voir [Points d'attention](#points-dattention)).

---

## Exemple simple : gadget RNDIS

```go
import "github.com/oioio-space/oioni/drivers/usbgadget"

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

## Exemple avance : gadget composite RNDIS + ECM + HID

Recuperation du nom d'interface reseau cote gadget, lecture de la MAC host assignee par le kernel, et reception des LEDs clavier :

```go
import (
    "context"
    "github.com/oioio-space/oioni/drivers/usbgadget"
    "github.com/oioio-space/oioni/drivers/usbgadget/functions"
)

rndis := functions.RNDIS(
    functions.WithRNDISHostAddr("02:00:00:aa:bb:01"), // MAC stable -> meme bail DHCP
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
    usbgadget.WithStrings("0x409", "ACME", "Composite", "comp001"),
    usbgadget.WithFunc(rndis), // RNDIS doit etre en premier (compatibilite Windows)
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

// Nom d'interface reseau cote Pi (ex. "usb0")
if ifname, err := rndis.IfName(); err == nil {
    fmt.Printf("RNDIS interface: %s\n", ifname)
}
// MAC host telle qu'assignee par le kernel dans configfs (disponible apres Enable())
if mac, err := rndis.HostAddr(); err == nil {
    fmt.Printf("RNDIS host MAC: %s\n", mac)
}

// Compteurs reseau
if stats, err := rndis.ReadStats(); err == nil {
    fmt.Printf("rx=%d tx=%d bytes\n", stats.RxBytes, stats.TxBytes)
}

// LEDs clavier en temps reel (NumLock, CapsLock, ScrollLock)
ctx, cancel := context.WithCancel(context.Background())
defer cancel()
leds, _ := kbd.ReadLEDs(ctx)
for state := range leds {
    fmt.Printf("CapsLock=%v NumLock=%v\n", state.CapsLock, state.NumLock)
}
```

---

## Sub-package : `functions`

Chaque constructeur retourne un `*XxxFunc` qui implemente l'interface `Function`.

### Fonctions reseau

| Constructeur | Protocole | EPs | Plateformes cibles |
|---|---|:---:|---|
| `RNDIS(opts...)` | RNDIS | 3 | Windows (doit etre en premier dans un composite) |
| `ECM(opts...)` | CDC ECM | 3 | Linux, macOS |
| `NCM(opts...)` | CDC NCM | 3 | Linux 3.10+ (debit eleve) |
| `EEM(opts...)` | CDC EEM | 2 | Linux-to-Linux (bulk-only, sans interface de controle) |
| `Subset(opts...)` | CDC Subset | 2 | Hosts embarques, anciens Linux |

Options communes : `WithXxxDevAddr(mac)`, `WithXxxHostAddr(mac)`, `WithXxxQMult(n)`.

Methodes disponibles apres `Enable()` :
- `IfName() (string, error)` — nom d'interface kernel cote gadget (ex. `"usb0"`) ; fallback par scan MAC si configfs retourne `"unnamed"`
- `HostAddr() (string, error)` — MAC host telle que stockee dans configfs ; disponible uniquement apres `Enable()`
- `ReadStats() (NetStats, error)` — compteurs reseau de l'interface

Note sur `IfName()` pour EEM/Subset : si `ifname` n'est pas encore assigne par le kernel, le driver lit `dev_addr` depuis configfs et scanne `/sys/class/net` par MAC.

### HID

| Constructeur | Protocole | EPs | Device |
|---|---|:---:|---|
| `Keyboard(opts...)` | HID boot keyboard | 1 | `/dev/hidg0` (ou N selon l'ordre de creation) |
| `Mouse(opts...)` | HID boot mouse | 1 | `/dev/hidgN` |

Methodes :
- `WriteReport([]byte) error` — envoie un rapport HID brut au host
- `ReadLEDs(ctx) (<-chan LEDState, error)` — reception des indicateurs LED envoy es par le host (NumLock, CapsLock...) ; uniquement pour Keyboard (protocol=1)
- `DevPath() string` — chemin du device kernel, ex. `"/dev/hidg0"`

Format rapport clavier : `[modifier, 0x00, key1, key2, key3, key4, key5, key6]`.
Format rapport souris : `[buttons, deltaX, deltaY, wheel]`.

### Storage

| Constructeur | EPs | Remarque |
|---|:---:|---|
| `MassStorage(file, opts...)` | 2 | `WithCDROM(bool)`, `WithReadOnly(bool)`, `WithRemovable(bool)` |

### Serial

| Constructeur | Driver | EPs | Device gadget |
|---|---|:---:|---|
| `ACMSerial()` | CDC ACM | 3 | `/dev/ttyGSN` ; host : `/dev/ttyACMN` |
| `Serial()` | GSER | 2 | `/dev/ttyGSN` ; host : serial USB |
| `OBEX()` | OBEX | 2 | `/dev/ttyGSN` |

Les numeros `ttyGSN` sont attribues dans l'ordre de creation entre ACM, GSER et OBEX.

### Audio / MIDI

| Constructeur | EPs | Remarque |
|---|:---:|---|
| `UAC2(opts...)` | 2-3 | USB Audio Class 2 ; Windows 10+, Linux, macOS sans driver |
| `UAC1(opts...)` | 2-3 | USB Audio Class 1 ; compatibilite maximale |
| `MIDI(opts...)` | 2 | USB MIDI ; `/dev/snd/midiC0D0` |

### Printer / Loopback

| Constructeur | EPs | Remarque |
|---|:---:|---|
| `Printer(opts...)` | 2 | Print jobs sur `/dev/usb/lp0` |
| `Loopback(opts...)` | 2 | Benchmark USB (usbtest cote host) |

---

## Sub-package : `ducky`

Parser et executor **DuckyScript V2** pour USB HID, base sur une grammaire PEG (pigeon).

### Layouts disponibles

| Variable | Disposition |
|---|---|
| `ducky.LayoutEN` | QWERTY US (par defaut) |
| `ducky.LayoutFR` | AZERTY FR |

### API principale

```go
// ParseScript parse un script en []Instruction sans l'executer.
func ParseScript(script string) ([]Instruction, error)

// ExecuteScript parse et execute un script sur le clavier donne.
// ctx permet d'interrompre l'execution entre deux instructions.
func ExecuteScript(ctx context.Context, kbd Keyboard, script string, layout *Layout) error

// TypeString tape une chaine caractere par caractere (delai 5 ms entre touches).
func TypeString(ctx context.Context, kbd Keyboard, text string, layout *Layout) error

// PressKeys appuie simultanement sur une combinaison de touches nommees puis les relache.
// Exemples : ["CTRL","ALT","DELETE"], ["GUI","r"], ["ENTER"]
func PressKeys(ctx context.Context, kbd Keyboard, keys []string) error

// MouseMove envoie un mouvement relatif (dx, dy dans [-127, 127]).
func MouseMove(ctx context.Context, m Mouse, dx, dy int8) error

// MouseClick envoie un clic bouton : 0x01=gauche, 0x02=droit, 0x04=milieu.
func MouseClick(ctx context.Context, m Mouse, btn byte) error
```

### Commandes DuckyScript supportees

| Commande | Description |
|---|---|
| `STRING <texte>` | Tape le texte sans Enter |
| `STRINGLN <texte>` | Tape le texte puis appuie sur Enter |
| `DELAY <ms>` | Pause en millisecondes |
| `DEFAULT_DELAY <ms>` | Delai inter-commandes pour toute la suite du script |
| `ENTER`, `TAB`, `ESC`, `BACKSPACE`... | Touche nominative seule |
| `CTRL ALT DELETE` | Combinaison de touches (espace = simultanee) |
| `GUI r`, `SHIFT F10`... | Combos modificateur + touche |
| `REM <commentaire>` | Ligne ignoree |

Touches nommees : CTRL/CONTROL, SHIFT, ALT, GUI/WINDOWS/COMMAND, ENTER/RETURN, ESC/ESCAPE, TAB, BACKSPACE, DELETE/DEL, INSERT, HOME, END, PAGEUP, PAGEDOWN, UPARROW, DOWNARROW, LEFTARROW, RIGHTARROW, SPACE, CAPSLOCK, NUMLOCK, SCROLLLOCK, PRINTSCREEN, PAUSE/BREAK, MENU/APP, F1-F12.

### Exemple DuckyScript

```go
import (
    "context"
    "github.com/oioio-space/oioni/drivers/usbgadget/functions"
    "github.com/oioio-space/oioni/drivers/usbgadget/ducky"
)

kb := functions.Keyboard() // satisfait ducky.Keyboard via WriteReport

// kb doit etre ajoute a un gadget et Enable() appele avant l'envoi.

script := `DEFAULT_DELAY 50
GUI r
DELAY 500
STRING notepad
ENTER
DELAY 1000
STRING Hello World
`
if err := ducky.ExecuteScript(ctx, kb, script, ducky.LayoutEN); err != nil {
    log.Fatal(err)
}
```

---

## Points d'attention

**RNDIS doit etre la premiere fonction.**
Pour qu'un composite soit reconnu par Windows, RNDIS doit etre declare en premier. `priority.go` trie automatiquement les fonctions dans le bon ordre lors de `Enable()`, meme si elles ont ete ajoutees dans le desordre via `WithFunc`.

**Budget d'endpoints : erreur explicite au lieu d'un echec silencieux.**
Le kernel accepte l'ecriture du fichier UDC meme si le bind echoue faute d'EPs. `udc.go` detecte que le gadget n'est pas attache et retourne une erreur claire :
```
UDC bind failed: gadget did not attach to fe980000.usb
(too many functions for the controller's endpoint budget?)
```

**configfs est monte automatiquement.**
`Enable()` monte configfs sur `/sys/kernel/config` si ce n'est pas deja fait. Aucune preparation systeme n'est requise.

**Modules kernel embarques en ARM64.**
Les modules `.ko` (dwc2, libcomposite, usb_f_rndis, usb_f_ecm, usb_f_hid...) sont embarques dans le binaire. `Enable()` les charge via `init_module`. Les erreurs `EEXIST` (module deja present ou compile en dur) sont ignorees.

**`Enable()` nettoie l'etat residuel.**
Au debut de `Enable()`, `unbindUDC()` et `teardownConfigfs()` sont appeles sur les erreurs pour effacer tout etat laisse par un crash precedent. Les erreurs de nettoyage sont ignorees.

**ECM : hw_type=14 et ARP.**
Sur le kernel embarque de ce projet, ECM est expose avec `hw_type=14` (non standard) au lieu de `hw_type=1` (Ethernet). L'ARP automatique ne fonctionne pas correctement cote host dans ce cas. Contournement : injecter les entrees ARP manuellement via `ip neigh add` ou l'API netlink Go.

**`HostAddr()` disponible uniquement apres `Enable()`.**
La methode lit `host_addr` depuis le repertoire configfs de la fonction, qui n'est cree que lors de `Enable()`. Appeler avant retourne une erreur.

**`IfName()` avec fallback MAC.**
Pour RNDIS, ECM, EEM et Subset, si l'attribut `ifname` dans configfs contient `"unnamed"` (kernel pas encore mis a jour), le driver scanne `/sys/class/net` par adresse MAC (`dev_addr`). Fournir `WithXxxDevAddr` pour que ce fallback fonctionne.

**Le package `ducky` ignore silencieusement les caracteres inconnus.**
Si un caractere n'est pas dans le layout choisi, il est saute sans erreur (comportement identique au Rubber Ducky original). S'assurer que le layout correspond a la disposition clavier du host cible.

---

## Reference API

### Package `usbgadget`

```go
// Constructeur
func New(opts ...Option) (*Gadget, error)

// Options de configuration du gadget
func WithName(name string) Option
func WithVendorID(vendor, product uint16) Option
func WithStrings(langID, manufacturer, product, serial string) Option
func WithUSBVersion(major, minor uint8) Option

// Ajout d'une fonction avec reference (pour appeler des methodes apres Enable)
func WithFunc(f functions.Function) Option

// Raccourcis sans reference a la fonction
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
// Interface implementee par tous les drivers de fonctions
type Function interface {
    TypeName() string
    InstanceName() string
    Configure(dir string) error
}

// Methodes communes aux fonctions reseau (RNDIS, ECM, NCM, EEM, Subset)
func (f *XxxFunc) IfName() (string, error)
func (f *XxxFunc) HostAddr() (string, error)   // RNDIS et ECM uniquement
func (f *XxxFunc) ReadStats() (NetStats, error)

type NetStats struct {
    RxBytes, TxBytes     uint64
    RxPackets, TxPackets uint64
    RxErrors, TxErrors   uint64
    RxDropped, TxDropped uint64
}

// Methodes HID
func (f *HIDFunc) WriteReport(report []byte) error
func (f *HIDFunc) ReadLEDs(ctx context.Context) (<-chan LEDState, error)
func (f *HIDFunc) DevPath() string

type LEDState struct {
    NumLock, CapsLock, ScrollLock, Compose, Kana bool
}
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

// Constantes pour le byte modificateur HID
const (
    ModLCtrl  byte = 0x01
    ModLShift byte = 0x02
    ModLAlt   byte = 0x04
    ModLGUI   byte = 0x08
    ModRCtrl  byte = 0x10
    ModRShift byte = 0x20
    ModRAlt   byte = 0x40
    ModRGUI   byte = 0x80
)
```
