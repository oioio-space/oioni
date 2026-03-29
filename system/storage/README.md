# system/storage

Gere le volume persistant `/perm` et les volumes USB hotplug, exposes comme `afero.Fs` avec callbacks de montage/demontage.

## Package independant

`storage` n'a aucune dependance sur les autres packages applicatifs du projet oioni. Il depend uniquement de ses propres sous-packages :
- `storage/mount` — montage/demontage syscall
- `storage/usbdetect` — detection USB via sysfs + netlink

Dependances externes : `github.com/spf13/afero` et `golang.org/x/sys` (syscall netlink). Aucun import depuis `ui/gui`, `drivers/`, ou `tools/`.

---

## Exemple simple

```go
package main

import (
    "context"
    "log"
    "github.com/oioio-space/oioni/system/storage"
)

func main() {
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    mgr := storage.New(
        storage.WithOnMount(func(v *storage.Volume) {
            log.Printf("monte : %s (%s) -> %s", v.Name, v.FSType, v.MountPath)
        }),
        storage.WithOnUnmount(func(v *storage.Volume) {
            log.Printf("demonte : %s", v.Name)
        }),
    )

    // Start bloque jusqu'a l'annulation du contexte.
    if err := mgr.Start(ctx); err != nil {
        log.Fatal(err)
    }
}
```

---

## Exemple avance : acces aux volumes et ecriture de fichiers

```go
package main

import (
    "context"
    "log"
    "os"
    "time"
    "github.com/oioio-space/oioni/system/storage"
    "github.com/spf13/afero"
)

var defaultConfig = []byte(`{"version":1}`)

func main() {
    ctx, cancel := context.WithCancel(context.Background())

    mgr := storage.New(
        storage.WithPermPath("/perm"),
        storage.WithMountBase("/tmp/storage"),
        storage.WithOnMount(func(v *storage.Volume) {
            if v.Persistent {
                // Volume /perm disponible : initialiser la config si absente.
                if _, err := v.FS.Stat("config.json"); os.IsNotExist(err) {
                    afero.WriteFile(v.FS, "config.json", defaultConfig, 0644)
                }
                return
            }
            // Cle USB : lire son contenu.
            files, _ := afero.ReadDir(v.FS, "/")
            for _, f := range files {
                log.Printf("[USB %s] %s", v.Name, f.Name())
            }
        }),
        storage.WithOnUnmount(func(v *storage.Volume) {
            log.Printf("retrait USB : %s", v.Name)
        }),
    )

    go func() {
        if err := mgr.Start(ctx); err != nil {
            log.Printf("storage: %v", err)
        }
    }()

    // Acceder a un volume par nom a tout moment (thread-safe).
    time.Sleep(2 * time.Second)
    if usbVol, ok := mgr.Volume("sda1"); ok {
        data, _ := afero.ReadFile(usbVol.FS, "data.csv")
        log.Printf("donnees USB : %d octets", len(data))
    }

    // Snapshot de tous les volumes montes.
    for _, v := range mgr.Volumes() {
        log.Printf("volume %s monte sur %s", v.Name, v.MountPath)
    }

    cancel() // demontage propre de tous les volumes non persistants
}
```

---

## Points d'attention

**Netlink ouvert avant le scan sysfs — protection contre la race condition.**
`usbdetect.Start()` ouvre le socket netlink `KOBJECT_UEVENT` **avant** de scanner `/sys/block/sd*`. Cela garantit qu'un evenement `add` arrivant pendant le scan n'est pas perdu. Les devices deja presents au demarrage sont emis comme evenements synthetiques `"add"`.

**Detection USB via le lien symbolique `subsystem`.**
Un block device `sdX` est considere comme USB si le chemin du lien symbolique `/sys/block/sdX/device/subsystem` contient la chaine `"usb"`. Les disques SATA/NVMe internes ne sont pas confondus avec les cles USB.

**Detection du filesystem via magic bytes.**
La detection est effectuee directement sur le block device (pas sur une image montee). Ordre de verification : exFAT d'abord (OEM ID a l'offset 3 : `"EXFAT   "`), puis ext4 (magic `0xEF53` a `0x438`), puis FAT (signature `0x55AA` a l'offset 510). Un device non reconnu est ignore avec un log — pas d'erreur fatale.

**`/perm` n'est jamais demonte par ce package.**
Le volume `perm` est monte au debut de `Start()` (volume synthetique, `Device` vide, `Persistent = true`). `unmountAll()` saute explicitement les volumes persistants. `/perm` est gere par gokrazy ; ne pas tenter de le demonter manuellement.

**`/tmp` est tmpfs sur gokrazy.**
Les points de montage sous `/tmp/storage/` sont crees par `os.MkdirAll` a chaque boot. Ne pas stocker de chemins absolus vers ces mountpoints entre les reboots. Utiliser le nom du volume (ex. `"sda1"`) et appeler `mgr.Volume("sda1")`.

**Lazy unmount (`MNT_DETACH`).**
`mount.Unmount()` utilise `MNT_DETACH`. Le demontage est sur meme si des fichiers sont encore ouverts ou si la cle a ete arrachee physiquement. Les operations en cours se terminent ; les nouvelles retournent `EIO`.

**`OnMount` est appele dans la goroutine de `Start()`.**
Ne pas bloquer longtemps dans ce callback pour eviter de retarder le traitement des evenements hotplug suivants. Si un traitement long est necessaire, le deleguer a une goroutine separee.

**`Volumes()` et `Volume()` sont thread-safe.**
Les methodes de lecture sont protegees par un mutex interne. Les callbacks `OnMount`/`OnUnmount` sont appeles hors du mutex — les appels a `mgr.Volumes()` depuis ces callbacks ne creent pas de deadlock.

**Interfaces injectables pour les tests.**
`Manager` utilise les interfaces internes `mounter` et `detector`. Construire via `newManager(fakeDetector, fakeMounter, ...)` dans les tests pour simuler des events USB sans hardware ni root.

---

## Reference API

### Volume

```go
type Volume struct {
    Name       string    // label du volume ou "perm"
    Device     string    // ex. "/dev/sda1" ; vide pour perm
    MountPath  string    // ex. "/tmp/storage/sda1" ou "/perm"
    FSType     string    // "vfat", "exfat", "ext4", ou "perm"
    Persistent bool      // true uniquement pour /perm
    FS         afero.Fs  // filesystem afero sandbox (NewBasePathFs sur le mountpoint)
}
```

### Constructeur et options

```go
// New retourne un Manager avec le vrai detecteur USB et le vrai mounter syscall.
func New(opts ...Option) *Manager

func WithPermPath(path string) Option     // defaut : "/perm"
func WithMountBase(path string) Option    // defaut : "/tmp/storage"
func WithOnMount(fn func(*Volume)) Option
func WithOnUnmount(fn func(*Volume)) Option
```

### Methodes de Manager

```go
// Start monte /perm, scanne les USBs existants via sysfs, puis ecoute les
// evenements hotplug netlink jusqu'a l'annulation du contexte.
// Bloque jusqu'a ctx.Done(). Au retour, tous les volumes non persistants sont demontes.
func (m *Manager) Start(ctx context.Context) error

// Volumes retourne un snapshot thread-safe de tous les volumes actuellement montes.
func (m *Manager) Volumes() []*Volume

// Volume retourne le volume portant ce nom (ex. "sda1", "perm").
// Retourne (nil, false) si le volume n'est pas monte.
func (m *Manager) Volume(name string) (*Volume, bool)
```

### Sous-package `storage/mount`

```go
// Mount cree le mountpoint si necessaire, puis monte device sur mountpoint.
func Mount(device, mountpoint, fstype string) error

// Unmount demonte le mountpoint via MNT_DETACH (lazy, sur meme si device arrache).
func Unmount(mountpoint string) error

// DetectFSType lit les magic bytes du block device et retourne "vfat", "exfat" ou "ext4".
func DetectFSType(device string) (string, error)
```

### Sous-package `storage/usbdetect`

```go
type Event struct {
    Action string // "add" ou "remove"
    Device string // ex. "/dev/sda1"
}

// New retourne un Detector utilisant le vrai chemin sysfs (/sys/block).
func New() *Detector

// Start ouvre le socket netlink KOBJECT_UEVENT, scanne sysfs pour les devices
// existants, puis transmet les evenements hotplug jusqu'a ctx.Done().
// Le channel est ferme quand ctx est annule.
func (d *Detector) Start(ctx context.Context) (<-chan Event, error)
```
