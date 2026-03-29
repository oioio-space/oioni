# wifi

Gère le WiFi sur gokrazy via wpa_supplicant : connexion STA, scan, point d'accès (AP) et DHCP client, avec support spécifique du chip BCM43430.

## Package indépendant

`wifi` n'a aucune dépendance interne au projet. Il ne dépend que de la bibliothèque standard Go et de `golang.org/x/sys/unix` (chargement de modules kernel).

## Exemple simple

```go
mgr := wifi.New(wifi.Config{
    WpaSupplicantBin: "/user/wpa_supplicant",
    IwBin:            "/user/iw",
    UdhcpcBin:        "/bin/udhcpc",
    ConfDir:          "/perm/wifi",
    CtrlDir:          "/var/run/wpa_supplicant",
    Iface:            "wlan0",
})

ctx := context.Background()
if err := mgr.Start(ctx); err != nil {
    log.Fatalf("wifi start: %v", err)
}

if err := mgr.Connect("MonReseau", "motdepasse", true); err != nil {
    log.Printf("wifi connect: %v", err)
}
```

## Exemple avancé — STA + AP simultanés avec NAT

```go
mgr := wifi.New(wifi.Config{
    WpaSupplicantBin: "/user/wpa_supplicant",
    HostapdBin:       "/user/hostapd",
    IwBin:            "/user/iw",
    UdhcpcBin:        "/bin/udhcpc",
    ConfDir:          "/perm/wifi",
    CtrlDir:          "/var/run/wpa_supplicant",
    Iface:            "wlan0",
    DefaultAPConfig: wifi.APConfig{
        SSID:      "oioni-ap",
        PSK:       "secret123",
        IP:        "192.168.4.1/24",
        DNS:       []string{"8.8.8.8"},
        EnableNAT: true,
        // Channel sera auto-détecté depuis le canal STA (BCM43430 : STA+AP même canal obligatoire)
    },
})

ctx, cancel := context.WithCancel(context.Background())
defer cancel()

if err := mgr.Start(ctx); err != nil {
    log.Fatalf("wifi: %v", err)
}

// Se connecter au réseau domestique en client.
if err := mgr.Connect("Livebox-Home", "wpa2pass", true); err != nil {
    log.Printf("connect: %v", err)
}

// Activer simultanément le mode AP (crée uap0, lance hostapd + DHCP).
// SetMode attend que le STA soit connecté et connaisse son canal.
if err := mgr.SetMode(ctx, wifi.Mode{STA: true, AP: true}); err != nil {
    log.Printf("setmode: %v", err)
}

// Scanner les réseaux disponibles.
nets, err := mgr.Scan()
if err == nil {
    for _, n := range nets {
        log.Printf("  %s (%s) %d dBm saved=%v", n.SSID, n.Security, n.Signal, n.Saved)
    }
}

// Lire l'état courant.
if st, err := mgr.Status(); err == nil {
    log.Printf("STA : %s, IP : %s", st.State, st.IP)
}
if ap := mgr.APStatus(); ap.Running {
    log.Printf("AP : %s, clients : %d", ap.IP, ap.Clients)
}
```

## Points d'attention

- **BCM43430 : `feature_disable=0x82000` obligatoire** : sans ce paramètre, le firmware du BCM43430 (Pi Zero 2W) intercepte la phase d'authentification 802.11 et échoue silencieusement (ASSOC-REJECT sans handshake WPA2), produisant des entrées TEMP-DISABLED permanentes. Le paramètre est passé automatiquement par `Start()` au chargement du module `brcmfmac.ko`.
- **STA + AP sur le même canal** : le BCM43430 n'accepte pas STA et AP sur des canaux différents. `SetMode(AP:true)` attend que le STA soit connecté et extrait son canal pour le passer à hostapd. Ne pas fixer `APConfig.Channel` manuellement sauf si on contrôle aussi le canal du réseau STA.
- **`wpa_supplicant.conf` géré exclusivement par le code** : ne pas éditer ce fichier manuellement. Il est réécrit à chaque `Start()` et à chaque `Connect()`/`RemoveSaved()`. `update_config=0` est positionné pour que wpa_supplicant n'écrase pas le fichier.
- **`ErrNotStarted`** : toutes les méthodes sauf `New()` retournent `ErrNotStarted` si `Start()` n'a pas encore été appelé (ou a échoué). Utiliser `errors.Is(err, wifi.ErrNotStarted)` pour tester.
- **`ErrWPATimeout`** : retourné si le socket de contrôle wpa_supplicant n'est pas prêt dans les 3 secondes après le démarrage. Indique généralement un problème de firmware ou de binaire.
- **Power save désactivé automatiquement** : `Start()` appelle `iw dev wlan0 set power_save off` pour éviter les déconnexions périodiques (~30 s) caractéristiques du BCM43430. Nécessite que `IwBin` soit renseigné dans la config.
- **Script udhcpc écrit dans `/tmp/wifi-udhcpc.sh`** : le script de lease DHCP est généré à chaque boot et doit être exécutable. Sur gokrazy, `/tmp` est un tmpfs — ce fichier est recréé automatiquement par le watcher DHCP.
- **`Start()` charge les modules kernel** : `brcmutil.ko`, `brcmfmac.ko` (avec `feature_disable` et `roamoff=1`), et `brcmfmac-wcc.ko`. Les erreurs de chargement sont non fatales (module déjà chargé = EEXIST ignoré).

## API reference

### Types

```go
type Mode struct {
    STA bool `json:"sta"` // client wpa_supplicant sur wlan0
    AP  bool `json:"ap"`  // point d'accès hostapd sur uap0
}

type Config struct {
    WpaSupplicantBin string
    HostapdBin       string
    IwBin            string
    UdhcpcBin        string    // vide = pas de DHCP client sur l'interface STA
    ConfDir          string    // e.g. "/perm/wifi"
    CtrlDir          string    // e.g. "/var/run/wpa_supplicant"
    Iface            string    // e.g. "wlan0"
    DefaultAPConfig  APConfig
}

type APConfig struct {
    SSID      string   `json:"ssid"`
    PSK       string   `json:"psk"`       // vide = réseau ouvert
    Channel   int      `json:"channel"`   // 0 = auto-détecté depuis STA
    IP        string   `json:"ip"`        // CIDR pour uap0, e.g. "192.168.4.1/24"
    DNS       []string `json:"dns"`
    EnableNAT bool     `json:"enableNat"`
}

type Network struct {
    SSID     string
    Signal   int    // dBm
    Security string // "WPA2", "WPA", "WEP", "Open"
    Saved    bool
}

type SavedNetwork struct {
    SSID string
}

type Status struct {
    State   string // "COMPLETED", "ASSOCIATING", "DISCONNECTED", ...
    SSID    string
    IP      string
    Enabled bool
}

type APStatus struct {
    Running bool
    IP      string // CIDR de uap0
    Clients int    // clients DHCP connectés
}
```

### Erreurs sentinelles

```go
var ErrNotStarted = errors.New("wifi: not started")
var ErrWPATimeout = errors.New("wifi: timed out waiting for wpa_supplicant")
```

### Constructeur

```go
func New(cfg Config) *Manager
```

### Méthodes

```go
// Start charge les modules kernel, lance wpa_supplicant et connecte au socket de contrôle.
func (m *Manager) Start(ctx context.Context) error

// SetEnabled active/désactive le WiFi via rfkill sysfs.
func (m *Manager) SetEnabled(enabled bool) error

// Connect se connecte à un réseau WiFi. save=true persiste les credentials.
func (m *Manager) Connect(ssid, psk string, save bool) error

// Disconnect se déconnecte du réseau courant.
func (m *Manager) Disconnect() error

// Status retourne l'état courant de wpa_supplicant.
func (m *Manager) Status() (Status, error)

// Scan lance un scan et retourne les réseaux visibles.
func (m *Manager) Scan() ([]Network, error)

// SavedNetworks retourne les réseaux avec credentials persistés.
func (m *Manager) SavedNetworks() ([]SavedNetwork, error)

// RemoveSaved supprime les credentials d'un réseau.
func (m *Manager) RemoveSaved(ssid string) error

// SetMode active/désactive les modes STA et AP.
// En mode AP, attend que le STA soit connecté pour auto-détecter le canal.
func (m *Manager) SetMode(ctx context.Context, mode Mode) error

// GetMode retourne le mode actif courant.
func (m *Manager) GetMode() Mode

// SetAPConfig persiste la configuration AP.
func (m *Manager) SetAPConfig(cfg APConfig) error

// GetAPConfig retourne la configuration AP persistée.
func (m *Manager) GetAPConfig() (APConfig, error)

// APStatus retourne l'état courant du point d'accès.
func (m *Manager) APStatus() APStatus

// DebugCmd envoie une commande brute à wpa_supplicant et retourne la réponse.
func (m *Manager) DebugCmd(cmd string) (string, error)
```
