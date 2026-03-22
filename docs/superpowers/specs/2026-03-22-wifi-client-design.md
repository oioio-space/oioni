# WiFi Client + IP Management — Design Spec

**Goal:** Add WiFi network management and per-interface IP configuration to the oioni device. Users can scan networks, connect with saved credentials, configure DHCP or static IP on any interface, all persisted across reboots.

**Scope:** Sub-project 1 of 2. Sub-project 2 (Access Point mode) follows separately.

**Tech stack:** Go, wpa_supplicant (static ARM64 binary), `github.com/vishvananda/netlink`, `github.com/insomniacslk/dhcp` (CGo-free, confirmed), existing `ui/gui` widget library.

---

## Architecture

Two new backend packages plus UI extension:

```
system/wifi/        — wpa_supplicant lifecycle + control socket protocol
system/netconf/     — per-interface IP configuration (DHCP or static)
cmd/oioni/ui/       — WiFi scene, Network scene, extended Config scene
```

`github.com/gokrazy/wifi` is removed from `oioio/config.json`. The `system/wifi` package replaces it by managing `wpa_supplicant` as a subprocess, following the same pattern as `tools/containers` for podman.

---

## Package: `system/wifi`

### Files

| File | Responsibility |
|------|----------------|
| `wifi.go` | `Manager` public API |
| `wpa.go` | wpa_supplicant control socket protocol (text commands over Unix socket) |
| `config.go` | Read/write `/perm/wifi/wpa_supplicant.conf` |
| `process.go` | Start/stop wpa_supplicant subprocess |
| `hal.go` | `wpaConn` and `processRunner` interfaces for testing |

### API

```go
type Config struct {
    WpaSupplicantBin string // e.g. "/user/wpa_supplicant"
    ConfDir          string // e.g. "/perm/wifi"
    CtrlDir          string // e.g. "/var/run/wpa_supplicant" (tmpfs on gokrazy — writable)
    Iface            string // e.g. "wlan0"
}

type Manager struct { ... }

func New(cfg Config) *Manager
func (m *Manager) Start(ctx context.Context) error // start wpa_supplicant, load conf

func (m *Manager) SetEnabled(enabled bool) error    // via /sys/class/rfkill/ sysfs (no rfkill binary needed)
func (m *Manager) Scan() ([]Network, error)         // blocking ~2s; callers must run in goroutine
func (m *Manager) Connect(ssid, psk string, save bool) error
func (m *Manager) Disconnect() error
func (m *Manager) Status() (Status, error)
func (m *Manager) SavedNetworks() ([]SavedNetwork, error)
func (m *Manager) RemoveSaved(ssid string) error

type Network struct {
    SSID     string
    Signal   int    // dBm
    Security string // "WPA2", "WEP", "Open"
    Saved    bool
}

type SavedNetwork struct {
    SSID string // ID is internal; only SSID exposed publicly
}

type Status struct {
    State   string // wpa_supplicant state: "COMPLETED", "ASSOCIATING", "DISCONNECTED", ...
    SSID    string // from wpa_supplicant STATUS command
    Enabled bool
    // IP is intentionally absent: IP management is system/netconf's responsibility
}
```

**Note on `Stop()`:** not public. The manager listens on `ctx.Done()` internally and terminates wpa_supplicant when the context is cancelled (gokrazy SIGTERM propagation). No explicit caller action required.

**Note on `Scan()`:** always call in a goroutine; communicate results back to the UI via `nav.Dispatch`. Never call from a nav.Dispatch callback directly (would block the event loop for ~2s).

### wpa_supplicant binary

Built as a static ARM64 binary via a Dockerfile in `system/wifi/build/` (same pattern as `system/imgvol/build/`). The binary is **not committed to git**; it is built by `make build-wifi-bins` and placed in `system/wifi/bin/wpa_supplicant` which is `.gitignore`d. Deployed to the device via `ExtraFilePaths` in `oioio/config.json` at path `/user/wpa_supplicant`.

### Persistence

`/perm/wifi/wpa_supplicant.conf` — native wpa_supplicant format. Created empty on first boot if absent. Human-editable.

**Migration:** if `/perm/wifi.json` (gokrazy/wifi legacy) exists at startup, its SSID+PSK are imported into `wpa_supplicant.conf` and the file is renamed to `/perm/wifi.json.migrated`.

### Lifecycle

`Manager.Start(ctx)` launches wpa_supplicant with `-i wlan0 -C /var/run/wpa_supplicant -c /perm/wifi/wpa_supplicant.conf -B` (daemon mode). The `-B` flag causes the process to daemonize and exit, after which the control socket at `/var/run/wpa_supplicant/wlan0` may not be ready immediately. `Start` must poll for the socket existence (e.g. up to 3s with 100ms intervals) before returning. On `ctx.Done()` (gokrazy SIGTERM propagated via context), sends `TERMINATE` to the control socket. `/var/run` is a tmpfs on gokrazy and is writable.

`config.go` must call `os.MkdirAll(confDir, 0755)` before writing `wpa_supplicant.conf` — the `/perm/wifi/` directory does not exist on a fresh device.

---

## Package: `system/netconf`

### Files

| File | Responsibility |
|------|----------------|
| `netconf.go` | `Manager` public API |
| `dhcp.go` | Lightweight DHCP client (insomniacslk/dhcp, CGo-free) |
| `static.go` | Static IP via vishvananda/netlink |
| `config.go` | Read/write `/perm/netconf/interfaces.json` |
| `hal.go` | `netlinkClient` interface for testing |

### API

```go
type Mode string
const (
    ModeDHCP   Mode = "dhcp"
    ModeStatic Mode = "static"
)

type IfaceCfg struct {
    Mode    Mode
    IP      string   // CIDR, e.g. "192.168.1.10/24" (static only)
    Gateway string   // e.g. "192.168.1.1" (static only)
    DNS     []string // e.g. ["8.8.8.8"] (static only; see DNS note below)
}

type IfaceStatus struct {
    IP      string
    Gateway string
    Up      bool
}

type Manager struct { ... }

func New(confDir string) *Manager
func (m *Manager) Start(ctx context.Context) error // apply saved config; caller must log error
func (m *Manager) ListInterfaces() ([]string, error) // excludes "lo" and virtual interfaces (veth*, docker*)
func (m *Manager) Get(iface string) (IfaceCfg, error)
func (m *Manager) Apply(iface string, cfg IfaceCfg) error // configure live + persist to disk
func (m *Manager) Status(iface string) (IfaceStatus, error)
```

### Interface filtering

`ListInterfaces()` excludes: `lo`, interfaces matching `veth*`, `docker*`, `br-*`. Shows physical and USB gadget interfaces (wlan0, usb0, eth0, etc.).

### Persistence

`/perm/netconf/interfaces.json`:
```json
{
  "wlan0": {"mode": "dhcp"},
  "usb0":  {"mode": "static", "ip": "10.0.0.1/24", "gateway": "10.0.0.1", "dns": ["8.8.8.8"]}
}
```

### DNS

`/etc/resolv.conf` is **not writable** on gokrazy (read-only squashfs). DNS configuration is persisted in `interfaces.json` and applied to the oioni Go process's default resolver via `net.DefaultResolver` override using a custom `Dial` function. The `Manager` holds a mutex guarding the `net.DefaultResolver` assignment — only one interface at a time provides DNS (last `Apply` with DNS servers wins). Containerized tools (podman/impacket) manage their own DNS separately and are out of scope here.

### DHCP client

A goroutine per DHCP interface sends DISCOVER/REQUEST, handles lease renewal, applies the obtained IP/gateway to the interface via `netlinkClient`. Cancelled on `ctx.Done()`.

### Static IP

Uses `netlinkClient` (wrapping `vishvananda/netlink`): `AddrAdd`, `RouteAdd` for gateway.

---

## UI

### Navigation flow

```
Home → Config → WiFi    → [tap saved network]   → Connecting…
                         → [tap new network]     → Password → Connecting…
               → Network → [tap interface]       → IP Config
```

### Config scene (`scene_config.go`)

Extended from the current "coming soon" stub: `ScrollableList` with items **WiFi** and **Network**. **Breaking change — full call chain:** `NewConfigScene(nav)` becomes `NewConfigScene(nav, wifiMgr, netconfMgr)`. Its only caller is `home.go` (inside `NewHomeScene`'s onTap closure), so `NewHomeScene(nav)` also becomes `NewHomeScene(nav, wifiMgr, netconfMgr)`. Its caller `epaper.go` must pass the managers. Additionally `scene_helpers_test.go` has a table `[]struct{ fn func(*gui.Navigator) *gui.Scene }` with `NewConfigScene` and `NewHomeScene` entries — the table's function type must change to a wrapper, e.g. `func(nav *gui.Navigator) *gui.Scene { return NewConfigScene(nav, nil, nil) }`, using nil managers for test stubs.

### WiFi scene (`scene_wifi.go`)

Built with `newCategoryScene`. Extra sidebar button via `withExtraSidebarBtn(Icons.Scan, onScan)` — the default Back button from `newCategoryScene` is retained; do **not** add a second Back.

Content: `ScrollableList`:
- **Row 1** — WiFi toggle using existing `Toggle` widget.
- **Rows 2–N** — Network entries: saved networks first (marked ★), then scan results. Each row: SSID + 3-level signal indicator (canvas lines) + lock icon if secured.

**Scan flow:** `onScan` callback launches a goroutine that calls `wifi.Scan()`, then calls `nav.Dispatch(func() { list.SetItems(results); nav.RequestRender() })`. The sidebar Scan button shows "…" label while scanning (replaced via `sidebar.SetButtons`). **Important:** do not call `nav.Dispatch` twice in the goroutine (the second call is dropped if the first is still queued); combine list update + render into one closure.

Tap saved network → launches goroutine: `go func() { wifi.Connect(ssid, "", false); nav.Dispatch(func() { nav.Push(connectingScene) }) }()`. (`wifi.Connect` issues socket I/O and must not run on Run()'s goroutine.)
Tap new network → `nav.Push(passwordScene)`.

### Password scene (`scene_wifi_password.go`)

Built with `newCategoryScene(nav, "WiFi", ...)`. NavBar shows "Home > WiFi" (two-level — existing `NewNavBar("Home", "WiFi")` is sufficient; no 4-level breadcrumb needed).

Content: SSID label + PSK field (tapping it calls `gui.ShowTextInput(nav, "Mot de passe", 64, onPSK)` which pushes a full-screen keyboard scene) + `Checkbox` "Mémoriser". Sidebar extra button: Connect (via `withExtraSidebarBtn`). The `gui.ShowTextInput` function already exists in `ui/gui/helpers.go`; do not instantiate `keyboardWidget` directly (it is unexported).

### Connecting scene (`scene_wifi_connecting.go`)

Built with `newCategoryScene(nav, "WiFi", ...)`. Content: status `Label` ("Connexion à <SSID>…" / "Connecté — IP: x.x.x.x" / "Échec").

**Timer cancellation:** the scene holds a `cancel chan struct{}` (closed in `Scene.OnLeave`). A polling goroutine launched in `Scene.OnEnter` calls `wifi.Status()`, dispatches result, then waits 1 second or `cancel`, repeating until cancelled. Pattern:
```go
cancel := make(chan struct{})
go func() {
    for {
        st, _ := wifi.Status()
        nav.Dispatch(func() { label.SetText(...); nav.RequestRender() })
        select {
        case <-cancel:
            return
        case <-time.After(time.Second):
        }
    }
}()
// Scene.OnLeave:
OnLeave: func() { close(cancel) }
```
This is safe: `close(cancel)` unblocks the select immediately, preventing stale dispatches after the user presses Back.

### Network scene (`scene_network.go`)

Built with `newCategoryScene`. Content: `ScrollableList` of interfaces from `netconf.ListInterfaces()`, each showing interface name + current IP (or "down"). Tap → IP Config scene.

### IP Config scene (`scene_network_iface.go`)

Built with `newCategoryScene(nav, "Network", ...)`.

Content layout: two-button mode selector at top (44px) — a side-by-side "DHCP | Static" button pair drawn directly on canvas, the active mode shown with inverted fill. Below (60px): field labels (IP, Gateway, DNS) visible only in Static mode. Tapping a field label calls `gui.ShowTextInput(nav, fieldName, 40, onValue)` which pushes a full-screen keyboard scene.  Sidebar extra button: Save via `withExtraSidebarBtn`.

**`IconCarousel` must not be used here** — its `PreferredSize` is 88px and `MinSize` is 60px, both incompatible with the 44px mode selector row. Use a custom canvas-drawn two-button widget instead.

**Touch routing for the mode selector:** the mode selector widget must be listed in `Scene.Widgets` at the top level (in addition to appearing in the layout) for `findTouchTarget` to reach it — per the established pattern for widgets needing direct touch access.

---

## Boot integration (`cmd/oioni/main.go`)

```go
netconfMgr := netconf.New("/perm/netconf")
if err := netconfMgr.Start(ctx); err != nil {
    log.Printf("netconf start: %v", err) // non-fatal: device still usable
}

wifiMgr := wifi.New(wifi.Config{
    WpaSupplicantBin: "/user/wpa_supplicant",
    ConfDir:          "/perm/wifi",
    CtrlDir:          "/var/run/wpa_supplicant",
    Iface:            "wlan0",
})
if err := wifiMgr.Start(ctx); err != nil {
    log.Printf("wifi start: %v", err) // non-fatal: UI still shows wifi scene with error state
}

// Pass to UI (both NewHomeScene and NewConfigScene signatures change):
homeScene, nsb := ui.NewHomeScene(nav, wifiMgr, netconfMgr)
```

Both managers are torn down by context cancellation when gokrazy sends SIGTERM. No explicit Stop() calls in main.

---

## Testing strategy

- **`system/wifi`:** inject `wpaConn` (fake control socket) and `processRunner` (fake subprocess). Test scan result parsing, connect/save/remove state machine, config read/write, migration from wifi.json.
- **`system/netconf`:** inject `netlinkClient` interface. Test DHCP mock lease apply, static config apply, JSON persistence, interface filtering.
- **`cmd/oioni/ui`:** `fakeDisplay` pattern. Test scene structure (single top-level widget, correct title, correct number of sidebar buttons).

---

## gokrazy config changes

- **Remove** `github.com/gokrazy/wifi` from `Packages` in `oioio/config.json`.
- **Add** `/user/wpa_supplicant` via `ExtraFilePaths` (static ARM64 binary, not in git, built by `make build-wifi-bins`).
- **Keep** `wifi.json` — auto-migrated at first boot.
