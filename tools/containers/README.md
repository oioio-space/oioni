# tools/containers

Gestion du cycle de vie d'un container Podman unique et des processus qui s'y executent, avec workaround integre pour le bug de transmission de signaux `podman exec`.

## Package independant

Ce package n'a aucune dependance interne au projet oioni. Il depends uniquement de la bibliotheque standard Go : `os`, `os/exec`, `sync`, `bufio`, `context`, `strconv`, `strings`.

---

## Exemple simple

Demarrer un container a partir d'une image locale, lancer un outil, lire sa sortie.

```go
package main

import (
    "errors"
    "fmt"
    "log"
    "context"
    "github.com/oioio-space/oioni/tools/containers"
)

func main() {
    ctx := context.Background()

    mgr := containers.NewManager(containers.Config{
        Image:          "oioni/impacket:arm64",
        Name:           "oioni-impacket",
        Network:        "host",
        LocalImagePath: "/usr/share/oioni/impacket-arm64.tar.gz",
    })
    defer mgr.Close()

    proc, err := mgr.Start(ctx, "dump-1", "secretsdump.py",
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
}
```

---

## Exemple avance : arret propre, processus paralleles, injection pour tests

```go
// Production : manager avec capabilities reseau.
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

// Construire un Process sans ProcManager pour tester les parseurs de sortie.
ch := make(chan string, 2)
ch <- "admin:500:aad3b435b51404eeaad3b435b51404ee:31d6cfe0d16ae931b73c59d7e0c089c0:::"
close(ch)
proc := containers.NewProcess(ch,
    func() error { return nil }, // wait
    func() error { return nil }, // kill
)
for line := range proc.Lines() {
    fmt.Println(line)
}
```

---

## Points d'attention

**Chargement de l'image au premier demarrage : ~75 secondes.**
La premiere invocation de `Start()` appelle `podman load -i <LocalImagePath>` si l'image n'est pas encore presente. Pour une image ARM64 d'environ 125 Mo compresse, ce chargement prend environ 75 secondes. Les demarrages suivants sont instantanes : `podman image exists` est verifie avant toute tentative de chargement.

**TMPDIR doit pointer sur `/perm/tmp` pendant `podman load`.**
`podman load` decompresse l'archive dans un repertoire temporaire. Le tmpfs `/tmp` de gokrazy est trop petit pour une image ~125 Mo decompresse. Le manager force automatiquement `TMPDIR=/perm/tmp` (carte SD) uniquement pendant `podman load`, en creant le repertoire si necessaire. Ce surchargeage ne s'applique pas aux autres invocations podman.

**Les signaux ne sont pas transmis par `podman exec` (bug Podman #19486).**
`podman exec` ne transmet pas les signaux POSIX au processus cible. Contournement integre : la commande est lancee via `sh -c "echo $$; exec tool args"`. La premiere ligne de stdout est le PID du processus dans le container (`containerPID`). `Stop()` et `Kill()` envoient ensuite le signal via `podman exec <container> kill -TERM/-KILL <PID>`, et non au processus `podman exec` lui-meme. Si le PID n'a pas pu etre lu (premiere ligne non numerique), `Kill()` devient un no-op.

**Resolution des binaires via `os.Stat`, pas via `PATH`.**
`exec.LookPath` utilise le PATH du processus parent, qui peut ne pas inclure `/usr/local/bin` sur gokrazy. Le `CmdFactory` par defaut recherche les binaires dans `/user`, `/usr/local/bin`, `/usr/bin`, `/bin` via `os.Stat`. Pour les tests, utiliser `WithCmdFactory` pour injecter un fake.

**Un seul container par ProcManager.**
`ProcManager` gere un unique container (`podman run ... sleep infinity`). Si plusieurs images differentes sont necessaires simultanement, creer plusieurs managers distincts avec des noms differents. La collision de nom de container au redemarrage est geree automatiquement : `podman stop` + `podman rm` sont appeles avant chaque `podman run`.

**Nommage des processus : unicite obligatoire.**
`Start()` retourne `ErrAlreadyRunning` si le nom est deja utilise. Le slot est libere automatiquement via une goroutine watcher qui attend la fin du processus. Reutiliser le meme nom apres la fin du processus est valide.

**Capacite du channel de lignes : 64.**
Les lignes en exces sont silencieusement abandonnees si le consommateur ne lit pas assez vite. Ne pas utiliser `Lines()` pour des outils a sortie tres volumineuse sans consommateur dedie tournant dans une goroutine separee.

**`Close()` est concurrent-safe avec `Start()`.**
Le flag `m.closed` est protege par un mutex. Un `Start()` concurrent avec `Close()` retourne `ErrManagerClosed`. Un `Start()` qui commence avant `Close()` mais n'est pas encore enregistre est detecte par la double verification du flag dans le chemin post-lancement.

**`initContainer` est appele une seule fois via `sync.Once`.**
Si le premier appel a `Start()` echoue (ex. image corrompue), `m.startErr` est fixe et tous les appels subsequents a `Start()` retournent la meme erreur sans retenter. Pour recommencer, creer un nouveau `ProcManager`.

---

## Reference API

### Config

```go
type Config struct {
    Image          string   // nom de l'image, ex. "oioni/impacket:arm64"
    Name           string   // nom du container, unique par instance ProcManager
    Network        string   // ex. "host" pour acces a l'interface USB gadget
    Caps           []string // capabilities Linux, ex. ["NET_RAW", "NET_ADMIN"]
    LocalImagePath string   // si renseigne, charge l'image depuis ce .tar/.tar.gz
}
```

### Constructeurs

```go
// NewManager cree un ProcManager. Aucune E/S a la construction.
func NewManager(cfg Config, opts ...Option) *ProcManager

// WithCmdFactory remplace la factory exec.Cmd pour toutes les invocations podman.
// La factory est appelee une fois par invocation ; le *exec.Cmd retourne est utilise une seule fois.
// Usage principal : tests, sans modifier le comportement de production.
func WithCmdFactory(factory func(name string, args ...string) *exec.Cmd) Option
```

### ProcManager

```go
// Start initialise le container (une seule fois via sync.Once) puis lance l'executable
// via podman exec. name doit etre unique parmi les processus en cours.
// Retourne ErrAlreadyRunning si name est deja utilise, ErrManagerClosed si Close() a ete appele.
func (m *ProcManager) Start(ctx context.Context, name, executable string, args []string) (*Process, error)

// Stop envoie SIGTERM au processus identifie par containerPID, attend jusqu'a 10s,
// puis escalade en SIGKILL si le processus n'est pas encore termine.
func (m *ProcManager) Stop(ctx context.Context, name string) error

// Kill envoie SIGKILL immediatement via "podman exec container kill -KILL <PID>".
func (m *ProcManager) Kill(name string) error

// List retourne les noms de tous les processus actuellement enregistres.
func (m *ProcManager) List() []string

// Close tue tous les processus en cours, attend leur fin (wg.Wait),
// puis supprime le container via "podman rm -f".
func (m *ProcManager) Close() error
```

### Process

```go
// NewProcess construit un Process a partir de composants pre-existants.
// API publique stable : permet de creer des processus fictifs dans les tests
// sans instancier de ProcManager.
//   lines — channel de lignes, ferme quand le processus se termine.
//   wait  — bloque jusqu'a la fin ; retourne nil ou *ExitError. Idempotent via sync.Once.
//   kill  — envoie SIGKILL ; peut retourner une erreur si le processus est deja termine.
func NewProcess(lines <-chan string, wait func() error, kill func() error) *Process

// Lines retourne le channel de lignes stdout. Ferme quand le processus se termine.
func (p *Process) Lines() <-chan string

// Wait bloque jusqu'a la fin du processus. Retourne nil ou *ExitError.
// Idempotent : les appels suivants retournent le resultat mis en cache sans bloquer.
func (p *Process) Wait() error

// Kill envoie SIGKILL immediatement.
func (p *Process) Kill() error

// Running retourne false une fois que Wait() a retourne (process.done ferme).
func (p *Process) Running() bool
```

### Erreurs

```go
// ErrPodmanNotFound : binaire podman introuvable dans les repertoires de recherche.
var ErrPodmanNotFound = errors.New("containers: podman binary not found")

// ErrAlreadyRunning : Start() appele avec un nom deja enregistre dans le registry.
var ErrAlreadyRunning = errors.New("containers: process already running")

// ErrManagerClosed : Start() appele apres Close().
var ErrManagerClosed = errors.New("containers: manager is closed")

// ExitError : retourne par Process.Wait() quand le processus se termine avec un code non nul.
type ExitError struct {
    Err *exec.ExitError // jamais nil
}
func (e *ExitError) Error() string
func (e *ExitError) Unwrap() error
func (e *ExitError) ExitCode() int
```
