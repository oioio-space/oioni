# system/imgvol

Cree, formate et monte en loop des fichiers image disque (FAT, exFAT, ext4) exposes comme `afero.Fs`.

## Package independant

`imgvol` n'a aucune dependance interne au projet oioni. Dependances externes uniquement :
- `golang.org/x/sys/unix` — ioctls loop device (`LOOP_CTL_GET_FREE`, `LOOP_SET_FD`, `LOOP_SET_STATUS64`, `LOOP_CLR_FD`)
- `github.com/spf13/afero` — abstraction filesystem

Les binaires `mkfs.vfat`, `mkfs.exfat` et `mkfs.ext4` ARM64 sont embarques via `go:embed` dans `imgvol/bin/`. Ils sont extraits et executes a la demande par `Create()`.

---

## Exemple simple

Creer une image de 64 Mo en FAT et y ecrire un fichier.

```go
package main

import (
    "log"
    "os"
    "github.com/oioio-space/oioni/system/imgvol"
    "github.com/spf13/afero"
)

func main() {
    const path = "/perm/data.img"

    if err := imgvol.Create(path, 64<<20, imgvol.FAT); err != nil {
        log.Fatal(err)
    }

    vol, err := imgvol.Open(path)
    if err != nil {
        log.Fatal(err)
    }
    defer vol.Close()

    if err := afero.WriteFile(vol.FS, "hello.txt", []byte("hello"), 0644); err != nil {
        log.Fatal(err)
    }
}
```

---

## Exemple avance : image ext4 persistante avec verification d'existence

```go
package main

import (
    "log"
    "os"
    "github.com/oioio-space/oioni/system/imgvol"
    "github.com/spf13/afero"
)

const imgPath = "/perm/appdata.img"
const imgSize = 256 << 20 // 256 Mo

func openOrCreate() (*imgvol.Volume, error) {
    // Create echoue si le fichier existe deja : ne creer qu'au premier boot.
    if _, err := os.Stat(imgPath); os.IsNotExist(err) {
        if err := imgvol.Create(imgPath, imgSize, imgvol.Ext4); err != nil {
            return nil, fmt.Errorf("imgvol create: %w", err)
        }
    }

    vol, err := imgvol.Open(imgPath)
    if err != nil {
        return nil, fmt.Errorf("imgvol open: %w", err)
    }
    return vol, nil
}

func main() {
    vol, err := openOrCreate()
    if err != nil {
        log.Fatal(err)
    }
    defer vol.Close()

    // FSType est detecte automatiquement depuis les magic bytes a l'Open.
    log.Printf("type detecte : %s", vol.FSType)

    // vol.FS est un afero.Fs sandbox — les chemins ne peuvent pas sortir du mountpoint.
    files, _ := afero.ReadDir(vol.FS, "/")
    for _, f := range files {
        log.Println(f.Name())
    }
}
```

---

## Points d'attention

**NTFS non supporte.**
Le kernel gokrazy n'inclut pas le driver ntfs3. Toute tentative de montage d'une image NTFS echouera avec une erreur syscall. Les images NTFS ont provoque des kernel panics lors des tests — eviter absolument.

**Filesystems supportes : FAT, exFAT, ext4 uniquement.**
La detection automatique (magic bytes) reconnait exactement ces trois formats. Tout autre filesystem retourne `"imgvol: unrecognized filesystem"`.

**Un seul `Open()` par image a la fois.**
Ouvrir la meme image pendant qu'elle est deja montee echoue avec `EBUSY` au niveau du syscall `mount`. Appeler `vol.Close()` avant de rearranger ou de re-ouvrir l'image.

**Root requis.**
Les ioctls loop (`LOOP_CTL_GET_FREE`, `LOOP_SET_FD`) et le syscall `mount` necessitent des privileges root. Sur gokrazy les processus tournent en root — aucune action supplementaire n'est requise.

**`Create()` refuse d'ecraser un fichier existant.**
Si le chemin existe deja, `Create()` retourne immediatement une erreur. Tester avec `os.Stat` avant si le fichier peut exister (ex. au redemarrage apres un crash).

**Images sparse.**
`Create()` utilise `os.Truncate()` — les blocs non ecrits n'occupent pas d'espace disque reel. Un `du -sh` montrera la taille reelle utilisee ; un `ls -lh` montrera la taille declaree. Sur une partition ext4 de /perm (SD card), les images sparse sont sures et recommandees.

**Point de montage sous `/tmp/imgvol/`.**
Sur gokrazy, `/tmp` est un tmpfs reincarne a chaque boot. Le repertoire de montage (`/tmp/imgvol/<basename>`) est cree par `Open()` et supprime par `Close()`. Un crash sans `Close()` laisse un loop device orphelin dans le kernel. Utiliser `defer vol.Close()` systematiquement.

**Les binaires `mkfs` sont ARM64 uniquement.**
Ils sont embarques comme binaires ARM64 statiques. `Create()` ne fonctionnera pas sur x86 en developpement local. Pour preparer une image de test sur x86, utiliser `dd` + `mkfs` natifs, puis copier le fichier.

**Detection du filesystem : ordre des magic bytes.**
`Open()` detecte d'abord exFAT (OEM ID a l'offset 3 : `"EXFAT   "`), ensuite ext4 (magic `0xEF53` a l'offset `0x438`), enfin FAT (signature `0x55AA` a l'offset 510). Cet ordre est important : une image exFAT a aussi la signature FAT `0x55AA`.

**Unmount lazy (`MNT_DETACH`).**
`Close()` utilise `MNT_DETACH` — le demontage est sur meme si des fichiers sont encore ouverts via `vol.FS` au moment de l'appel. Les operations en cours se terminent ; les nouvelles echouent avec `EIO`.

---

## Reference API

### Types

```go
type FSType string

const (
    FAT   FSType = "vfat"
    ExFAT FSType = "exfat"
    Ext4  FSType = "ext4"
)

type Volume struct {
    Path      string   // chemin absolu du fichier image
    FSType    FSType   // type de filesystem detecte automatiquement
    FS        afero.Fs // filesystem afero sandbox (NewBasePathFs sur le mountpoint)
    // champs internes : loopDev, mountpoint (non exportes)
}
```

### Fonctions

```go
// Create cree un fichier image sparse de size octets et le formate avec fstype.
// Retourne une erreur si le fichier existe deja.
// Extrait et execute le binaire mkfs ARM64 embarque.
func Create(path string, size int64, fstype FSType) error

// Open detecte le type de filesystem depuis les magic bytes, alloue un loop device,
// monte l'image sur /tmp/imgvol/<basename>, et retourne un Volume.
// Un seul Volume par chemin peut etre ouvert a la fois.
func Open(path string) (*Volume, error)

// Close demonte l'image (MNT_DETACH) et libere le loop device.
// Toujours appeler via defer.
func (v *Volume) Close() error
```

### Ioctls loop utilises (reference interne)

| Ioctl | Role |
|-------|------|
| `LOOP_CTL_GET_FREE` | Obtient le numero du premier loop device disponible |
| `LOOP_SET_FD` | Associe un fichier image au loop device |
| `LOOP_SET_STATUS64` | Definit les metadonnees (nom du fichier pour `/proc/mounts`) |
| `LOOP_CLR_FD` | Dissocie le fichier image du loop device (libere le device) |
