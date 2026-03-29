# storage

Gère le volume persistant `/perm` et les volumes USB hotplug, exposés comme `afero.Fs` avec callbacks de montage/démontage.

## Package indépendant

`storage` ne dépend d'aucun autre package interne au projet en dehors de ses propres sous-packages `storage/mount` et `storage/usbdetect`. Dépendances externes : `github.com/spf13/afero` et `golang.org/x/sys` (netlink/syscall).

## Exemple simple

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

mgr := storage.New(
    storage.WithOnMount(func(v *storage.Volume) {
        log.Printf("monté : %s (%s) -> %s", v.Name, v.FSType, v.MountPath)
    }),
    storage.WithOnUnmount(func(v *storage.Volume) {
        log.Printf("démonté : %s", v.Name)
    }),
)

// Start bloque jusqu'à l'annulation du contexte.
if err := mgr.Start(ctx); err != nil {
    log.Fatal(err)
}
```

## Exemple avancé — accès aux volumes et écriture de fichiers

```go
ctx, cancel := context.WithCancel(context.Background())

mgr := storage.New(
    storage.WithPermPath("/perm"),
    storage.WithMountBase("/tmp/storage"),
    storage.WithOnMount(func(v *storage.Volume) {
        if v.Persistent {
            // Volume /perm disponible — écrire la config si elle n'existe pas.
            if _, err := v.FS.Stat("config.json"); os.IsNotExist(err) {
                afero.WriteFile(v.FS, "config.json", defaultConfig, 0644)
            }
            return
        }
        // Clé USB : lire son contenu.
        files, _ := afero.ReadDir(v.FS, "/")
        for _, f := range files {
            log.Printf("[USB %s] %s", v.Name, f.Name())
        }
    }),
)

go func() {
    if err := mgr.Start(ctx); err != nil {
        log.Printf("storage: %v", err)
    }
}()

// Récupérer un volume par nom à tout moment (thread-safe).
time.Sleep(2 * time.Second)
if usbVol, ok := mgr.Volume("sda1"); ok {
    data, _ := afero.ReadFile(usbVol.FS, "data.csv")
    log.Printf("données USB : %d octets", len(data))
}

cancel() // déclenche le démontage propre de tous les volumes non persistants
```

## Points d'attention

- **`/tmp` est un tmpfs sur gokrazy** : les points de montage sous `/tmp/storage/` sont recréés à chaque boot. Ne pas stocker de chemins absolus vers ces mountpoints entre les reboots.
- **`/perm` est persistant** : le volume `perm` est monté dès le début de `Start()` et n'est jamais démonté par `unmountAll()` (flag `Persistent = true`).
- **Ordre netlink avant sysfs** : `usbdetect` ouvre le socket netlink KOBJECT_UEVENT *avant* de scanner sysfs. Cela évite la race condition où un événement `add` arriverait pendant le scan initial et serait perdu. Les devices déjà présents au démarrage sont émis comme événements synthétiques `"add"`.
- **Lazy unmount** : `mount.Unmount` utilise `MNT_DETACH`. Le démontage est sûr même si des fichiers sont encore ouverts ou si la clé a été retirée physiquement brutalement.
- **`OnMount` appelé dans le goroutine de `Start`** : ne pas bloquer longtemps dans ce callback pour éviter de retarder le traitement des événements hotplug suivants.
- **Thread-safe** : `Volumes()` et `Volume()` sont protégés par un mutex. Les callbacks `OnMount`/`OnUnmount` sont appelés hors du mutex.
- **Détection de filesystem** : basée sur les magic bytes du device (même logique que `imgvol`). Un device non reconnu est ignoré avec un log.

## API reference

### Types

```go
type Volume struct {
    Name       string    // label du volume ou "perm"
    Device     string    // e.g. "/dev/sda1" (vide pour perm)
    MountPath  string    // e.g. "/tmp/storage/sda1" ou "/perm"
    FSType     string    // "vfat", "exfat", "ext4", ou "perm"
    Persistent bool      // true uniquement pour /perm
    FS         afero.Fs  // filesystem prêt à l'emploi
}

type Option func(*Manager)
```

### Constructeur et options

```go
func New(opts ...Option) *Manager

func WithPermPath(path string) Option     // défaut : "/perm"
func WithMountBase(path string) Option    // défaut : "/tmp/storage"
func WithOnMount(fn func(*Volume)) Option
func WithOnUnmount(fn func(*Volume)) Option
```

### Méthodes

```go
// Start monte /perm, scanne les USBs existants, écoute les events hotplug.
// Bloque jusqu'à l'annulation du contexte.
func (m *Manager) Start(ctx context.Context) error

// Volumes retourne un snapshot thread-safe de tous les volumes montés.
func (m *Manager) Volumes() []*Volume

// Volume retourne le volume portant ce nom (e.g. "sda1", "perm").
func (m *Manager) Volume(name string) (*Volume, bool)
```

### Sous-packages

**`storage/mount`** — montage/démontage syscall :
```go
func Mount(device, mountpoint, fstype string) error
func Unmount(mountpoint string) error   // MNT_DETACH
func DetectFSType(device string) (string, error)
```

**`storage/usbdetect`** — détection hotplug USB :
```go
type Event struct {
    Action string // "add" ou "remove"
    Device string // e.g. "/dev/sda1"
}

func New() *Detector
func (d *Detector) Start(ctx context.Context) (<-chan Event, error)
```
