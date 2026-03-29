# tools/containers

Gestion du cycle de vie d'un container Podman unique et des processus qui s'y exécutent.

## Package independant

Ce package n'a aucune dependance interne au projet. Il ne depend que de la bibliotheque standard Go (`os`, `os/exec`, `sync`, `bufio`, `context`).

## Exemple simple

Demarrer un container a partir d'une image locale, lancer un outil, lire sa sortie.

```go
mgr := containers.NewManager(containers.Config{
    Image:          "oioni/impacket:arm64",
    Name:           "oioni-impacket",
    Network:        "host",
    LocalImagePath: "/usr/share/oioni/impacket-arm64.tar.gz",
})
defer mgr.Close()

proc, err := mgr.Start(ctx, "run-1", "secretsdump.py",
    []string{"admin@192.168.1.10", "-p", "Password1"})
if err != nil {
    log.Fatal(err)
}

for line := range proc.Lines() {
    fmt.Println(line)
}

if err := proc.Wait(); err != nil {
    var exitErr *containers.ExitError
    if errors.As(err, &exitErr) {
        log.Printf("exit code %d", exitErr.ExitCode())
    }
}
```

## Exemple avance

Arret propre avec escalade SIGTERM -> SIGKILL, plusieurs processus simultanes, injection du CmdFactory pour les tests.

```go
// Production : manager reel avec capabilities reseau
mgr := containers.NewManager(containers.Config{
    Image:          "oioni/impacket:arm64",
    Name:           "oioni-impacket",
    Network:        "host",
    Caps:           []string{"NET_RAW", "NET_ADMIN"},
    LocalImagePath: "/usr/share/oioni/impacket-arm64.tar.gz",
})

// Plusieurs processus en parallele dans le meme container.
proc1, _ := mgr.Start(ctx, "scan-1", "samrdump.py", []string{"guest@192.168.1.10"})
proc2, _ := mgr.Start(ctx, "scan-2", "lookupsid.py", []string{"guest@192.168.1.11"})

fmt.Println("En cours :", mgr.List()) // ["scan-1", "scan-2"]

// Arret propre : SIGTERM puis attente 10s, puis SIGKILL si necessaire.
stopCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
defer cancel()
_ = mgr.Stop(stopCtx, "scan-1")

// SIGKILL immediat.
_ = mgr.Kill("scan-2")

// Close() tue tout ce qui reste et supprime le container.
_ = mgr.Close()

// ---------------------------------------------------------------
// Tests : remplacer toutes les invocations podman par des fakes.
// ---------------------------------------------------------------
fake := containers.NewManager(containers.Config{Name: "test"},
    containers.WithCmdFactory(func(name string, args ...string) *exec.Cmd {
        return exec.Command("echo", "fake-output")
    }),
)

// Construire un Process sans ProcManager, pour tester les parseurs.
ch := make(chan string, 2)
ch <- "admin:500:aad3b435b51404eeaad3b435b51404ee:31d6cfe0d16ae931b73c59d7e0c089c0:::"
close(ch)
proc := containers.NewProcess(ch,
    func() error { return nil },
    func() error { return nil },
)
for line := range proc.Lines() {
    fmt.Println(line)
}
```

## Points d'attention

**Chargement de l'image : premier demarrage ~75s**
La premiere invocation de `Start()` execute `podman load` si l'image n'est pas encore presente.
Le chargement prend environ 75 secondes pour une image ~125 MiB.
Les demarrages suivants sont instantanes : le manager detecte l'image via `podman image exists` avant de tenter le chargement.

**TMPDIR doit pointer sur /perm/tmp pendant le chargement**
`podman load` decompresse l'archive dans un repertoire temporaire.
Le rootfs tmpfs de gokrazy est trop petit pour une image ARM64 (~125 MiB decompresse).
Le manager force automatiquement `TMPDIR=/perm/tmp` (carte SD) uniquement pendant `podman load`, en creant le repertoire si necessaire.
Ce comportement ne s'applique pas aux autres invocations podman.

**Signaux non transmis par `podman exec` (bug Podman #19486)**
`podman exec` ne transmet pas les signaux POSIX au processus cible.
Contournement integre : la commande est lancee via `sh -c "echo $$; exec tool args"`.
La premiere ligne de stdout est le PID du processus dans le container.
`Stop()` et `Kill()` envoient ensuite le signal via `podman exec kill -TERM/-KILL <PID>`,
et non au processus `podman exec` lui-meme.

**Resolution des binaires sur gokrazy**
`exec.LookPath` utilise le PATH du processus parent, qui peut ne pas inclure `/usr/local/bin`
sur gokrazy (repertoire d'installation de podman).
Le `CmdFactory` par defaut resout les binaires via `os.Stat` dans `/user`, `/usr/local/bin`,
`/usr/bin`, `/bin` — sans passer par PATH.

**Nommage des processus**
Chaque processus enregistre doit avoir un nom unique.
`Start()` retourne `ErrAlreadyRunning` si le nom est deja utilise.
Le slot est libere automatiquement a la fin du processus (goroutine watcher).

**Capacite du channel de lignes**
Le channel interne a une capacite de 64 lignes.
Les lignes en exces sont abandonnees si le consommateur ne lit pas assez vite.
Ne pas utiliser `Lines()` pour des outils a sortie volumineuse sans lecteur dedie.

**Un seul container par ProcManager**
`ProcManager` gere un unique container (`podman run … sleep infinity`).
Si plusieurs images differentes sont necessaires, creer plusieurs managers distincts.
La collision de nom de container au redemarrage est geree automatiquement
(`podman stop` + `podman rm` avant chaque `podman run`).

## Reference API

### Config

```go
type Config struct {
    Image          string   // nom de l'image, ex. "oioni/impacket:arm64"
    Name           string   // nom du container, unique par instance
    Network        string   // ex. "host" pour acces a l'interface USB gadget
    Caps           []string // capabilities Linux, ex. ["NET_RAW", "NET_ADMIN"]
    LocalImagePath string   // si renseigne, charge l'image depuis ce .tar/.tar.gz
}
```

### Constructeurs

```go
// Creer un manager. Aucune E/S a la construction.
func NewManager(cfg Config, opts ...Option) *ProcManager

// Remplacer la factory exec.Cmd pour toutes les invocations podman (tests).
func WithCmdFactory(factory func(name string, args ...string) *exec.Cmd) Option
```

### ProcManager

```go
// Initialise le container (une seule fois) puis lance l'executable via podman exec.
// name doit etre unique parmi les processus en cours.
func (m *ProcManager) Start(ctx context.Context, name, executable string, args []string) (*Process, error)

// Envoie SIGTERM, attend jusqu'a 10s, puis escalade en SIGKILL.
func (m *ProcManager) Stop(ctx context.Context, name string) error

// Envoie SIGKILL immediatement.
func (m *ProcManager) Kill(name string) error

// Retourne les noms de tous les processus en cours.
func (m *ProcManager) List() []string

// Tue tous les processus en cours, attend leur fin, supprime le container.
func (m *ProcManager) Close() error
```

### Process

```go
// Constructeur public — permet de creer des processus fictifs dans les tests
// sans instancier de ProcManager.
func NewProcess(lines <-chan string, wait func() error, kill func() error) *Process

// Channel des lignes stdout. Ferme quand le processus se termine.
func (p *Process) Lines() <-chan string

// Bloque jusqu'a la fin du processus. Idempotent : les appels suivants
// retournent le resultat mis en cache sans bloquer.
func (p *Process) Wait() error

// Envoie SIGKILL immediatement.
func (p *Process) Kill() error

// Retourne false une fois que Wait() a retourne.
func (p *Process) Running() bool
```

### Erreurs

```go
// Binaire podman introuvable dans les repertoires de recherche.
var ErrPodmanNotFound = errors.New("containers: podman binary not found")

// Start() appele avec un nom deja enregistre.
var ErrAlreadyRunning = errors.New("containers: process already running")

// Start() appele apres Close().
var ErrManagerClosed = errors.New("containers: manager is closed")

// Retourne par Process.Wait() quand le processus se termine avec un code non nul.
type ExitError struct {
    Err *exec.ExitError
}
func (e *ExitError) ExitCode() int
func (e *ExitError) Unwrap() error
```
