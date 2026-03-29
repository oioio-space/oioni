# netconf

Configure les interfaces réseau (DHCP ou IP statique) via netlink et persiste la configuration dans `confDir/interfaces.json`.

## Package indépendant

`netconf` n'a aucune dépendance interne au projet. Il ne dépend que de la bibliothèque standard Go et de `github.com/vishvananda/netlink`.

## Exemple simple

```go
mgr := netconf.New("/perm/netconf")

// Nettoyer les entrées USB gadget obsolètes avant de démarrer.
mgr.PurgeNonWlan()

ctx := context.Background()
if err := mgr.Start(ctx); err != nil {
    log.Fatalf("netconf: %v", err)
}
```

## Exemple avancé — IP statique + interface USB ECM éphémère

```go
mgr := netconf.New("/perm/netconf")
mgr.PurgeNonWlan() // OBLIGATOIRE avant Start

ctx, cancel := context.WithCancel(context.Background())
defer cancel()

// Appliquer la config persistée pour les interfaces connues (wlan0, eth0...).
if err := mgr.Start(ctx); err != nil {
    log.Printf("netconf start: %v", err)
}

// Configurer eth0 en statique et persister.
if err := mgr.Apply("eth0", netconf.IfaceCfg{
    Mode:    netconf.ModeStatic,
    IP:      "192.168.1.10/24",
    Gateway: "192.168.1.1",
    DNS:     []string{"1.1.1.1", "8.8.8.8"},
}); err != nil {
    log.Printf("netconf apply eth0: %v", err)
}

// Configurer usb0 (gadget ECM) en statique SANS persister :
// cette interface n'existe que pendant la session courante.
if err := mgr.ApplyEphemeral("usb0", netconf.IfaceCfg{
    Mode:    netconf.ModeStatic,
    IP:      "192.168.55.1/24",
    Gateway: "",
}); err != nil {
    log.Printf("netconf apply usb0: %v", err)
}

// Lire l'état runtime d'une interface.
status, err := mgr.Status("wlan0")
if err == nil && status.Up {
    log.Printf("wlan0 : %s", status.IP)
}
```

## Points d'attention

- **Appeler `PurgeNonWlan()` avant `Start()`** : au reboot, les interfaces USB gadget (`usb0`, `rndis0`...) disparaissent mais leur entrée reste dans `interfaces.json`. Sans purge, `Start()` lance un goroutine DHCP inutile sur ces interfaces et peut interférer avec la configuration statique ECM appliquée plus tard.
- **`wlan0` non géré par ce package** : le DHCP sur wlan0 est délégué à `github.com/gokrazy/wifi` (via wpa_supplicant). Ne pas appeler `Apply("wlan0", ...)` avec `ModeDHCP` depuis netconf.
- **`ApplyEphemeral` ne persiste pas** : utiliser uniquement pour les interfaces transientes (gadget USB ECM/RNDIS) qui ne doivent pas survivre un reboot.
- **`Apply` en mode DHCP lance un goroutine** : ce goroutine réessaie toutes les 5 secondes jusqu'à succès. Il est lié au contexte passé à `Start()`. Appeler `Apply` avant `Start()` ne lance pas de goroutine DHCP — le mode est sauvegardé mais pas appliqué jusqu'au prochain `Start()`.
- **DNS statique modifie `net.DefaultResolver`** : l'appel à `Apply` avec des serveurs DNS remplace le resolver global du processus. Protégé par un mutex interne, mais attention aux interactions avec d'autres goroutines qui résouveraient des noms.
- **Les erreurs d'application dans `Start()` sont non fatales** : `Start()` continue d'appliquer les autres interfaces si l'une échoue. Loguer les erreurs côté appelant si nécessaire.

## API reference

### Types

```go
type Mode string

const (
    ModeDHCP   Mode = "dhcp"
    ModeStatic Mode = "static"
)

type IfaceCfg struct {
    Mode    Mode     `json:"mode"`
    IP      string   `json:"ip,omitempty"`      // CIDR, mode statique uniquement
    Gateway string   `json:"gateway,omitempty"` // mode statique uniquement
    DNS     []string `json:"dns,omitempty"`     // mode statique uniquement
}

type IfaceStatus struct {
    IP      string
    Gateway string
    Up      bool
}
```

### Constructeur

```go
// New crée un Manager avec la configuration stockée dans confDir.
// confDir/interfaces.json est créé automatiquement si absent.
func New(confDir string) *Manager
```

### Méthodes

```go
// PurgeNonWlan supprime les entrées non-wlan/eth de interfaces.json.
// DOIT être appelé avant Start() pour nettoyer les interfaces USB gadget obsolètes.
func (m *Manager) PurgeNonWlan()

// Start applique la configuration persistée pour toutes les interfaces connues.
// Les interfaces absentes de la config utilisent DHCP par défaut.
func (m *Manager) Start(ctx context.Context) error

// ListInterfaces retourne les interfaces physiques/USB (exclut lo, veth*, docker*, br-*).
func (m *Manager) ListInterfaces() ([]string, error)

// Get retourne la configuration persistée d'une interface (défaut : DHCP).
func (m *Manager) Get(iface string) (IfaceCfg, error)

// Apply configure une interface et persiste la configuration dans interfaces.json.
func (m *Manager) Apply(iface string, cfg IfaceCfg) error

// ApplyEphemeral configure une interface sans persister (USB gadget ECM, RNDIS...).
func (m *Manager) ApplyEphemeral(iface string, cfg IfaceCfg) error

// Status retourne l'état IP courant d'une interface (via netlink).
func (m *Manager) Status(iface string) (IfaceStatus, error)
```

### Fichier de configuration

`confDir/interfaces.json` — map JSON `iface → IfaceCfg` :

```json
{
  "wlan0": { "mode": "dhcp" },
  "eth0":  { "mode": "static", "ip": "192.168.1.10/24", "gateway": "192.168.1.1", "dns": ["1.1.1.1"] }
}
```
