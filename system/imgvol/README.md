# imgvol

Crée, formate et monte en loop des fichiers image disque (FAT, exFAT, ext4) exposés comme `afero.Fs`.

## Package indépendant

`imgvol` n'a aucune dépendance interne au projet. Il ne dépend que de la bibliothèque standard Go, de `golang.org/x/sys/unix` (ioctls loop) et de `github.com/spf13/afero`.

## Exemple simple

```go
// Créer une image de 64 Mo en FAT et y écrire un fichier.
if err := imgvol.Create("/perm/data.img", 64<<20, imgvol.FAT); err != nil {
    log.Fatal(err)
}

vol, err := imgvol.Open("/perm/data.img")
if err != nil {
    log.Fatal(err)
}
defer vol.Close()

if err := afero.WriteFile(vol.FS, "hello.txt", []byte("hello"), 0644); err != nil {
    log.Fatal(err)
}
```

## Exemple avancé — image ext4 persistante avec vérification d'existence

```go
const imgPath = "/perm/appdata.img"
const imgSize = 256 << 20 // 256 Mo

// Create échoue si l'image existe déjà : créer uniquement au premier démarrage.
if _, err := os.Stat(imgPath); os.IsNotExist(err) {
    if err := imgvol.Create(imgPath, imgSize, imgvol.Ext4); err != nil {
        log.Fatalf("imgvol create: %v", err)
    }
}

vol, err := imgvol.Open(imgPath)
if err != nil {
    log.Fatalf("imgvol open: %v", err)
}
defer vol.Close()

// Le type de filesystem est détecté automatiquement à l'Open.
log.Printf("type détecté : %s", vol.FSType)

// vol.FS est un afero.Fs — utilisable avec toute l'API afero.
files, _ := afero.ReadDir(vol.FS, "/")
for _, f := range files {
    log.Println(f.Name())
}
```

## Points d'attention

- **NTFS non supporté** : le kernel gokrazy n'inclut pas le driver NTFS. Seuls FAT (`vfat`), exFAT et ext4 fonctionnent.
- **Un seul `Open()` par image à la fois** : ouvrir la même image une deuxième fois pendant qu'elle est déjà montée échoue avec `EBUSY` au niveau du mount. Appeler `vol.Close()` avant de ré-ouvrir.
- **Root requis** : les ioctls loop (`LOOP_CTL_GET_FREE`, `LOOP_SET_FD`) et l'appel `mount` nécessitent des privilèges root.
- **`Create` refuse d'écraser** : si le fichier existe déjà, `Create` retourne une erreur. Vérifier avec `os.Stat` avant si nécessaire.
- **Images sparse** : `Create` utilise `os.Truncate` — les blocs non écrits n'occupent pas d'espace disque réel. Le `df` sur l'image montée montrera la taille totale déclarée.
- **Point de montage dans `/tmp/imgvol/`** : sur gokrazy, `/tmp` est un tmpfs recréé à chaque boot. Le répertoire de montage est créé par `Open` et supprimé par `Close`. Un crash sans `Close` laisse un loop device orphelin — utiliser `defer vol.Close()` systématiquement.
- **Les binaires `mkfs` sont embarqués en ARM64 uniquement** : `Create` ne fonctionnera pas sur une architecture différente (x86 en dev local). Formater l'image hors du device si nécessaire.

## API reference

### Types

```go
type FSType string

const (
    FAT   FSType = "vfat"
    ExFAT FSType = "exfat"
    Ext4  FSType = "ext4"
)

type Volume struct {
    Path   string    // chemin du fichier image
    FSType FSType    // type de filesystem détecté
    FS     afero.Fs  // filesystem afero prêt à l'emploi
}
```

### Fonctions

```go
// Create crée un fichier image sparse et le formate.
// Retourne une erreur si le fichier existe déjà.
func Create(path string, size int64, fstype FSType) error

// Open détecte le filesystem, monte l'image en loop, et retourne un Volume.
// Le type de filesystem est détecté automatiquement depuis les magic bytes.
func Open(path string) (*Volume, error)

// Close démonte l'image et libère le loop device.
func (v *Volume) Close() error
```
