# WiFi Multi-Mode Design

## Overview

Extend `system/wifi` to support three modes — STA (client), AP (access point), and Scanner — with concurrent STA+AP on the BCM43430 via the `uap0` virtual interface.

**Hardware:** BCM43430 on Pi Zero 2W. Supports concurrent STA+AP via `uap0` virtual interface (same chip, separate logical interface). Raspberry Pi firmware enables this out of the box.

**Constraints:** gokrazy (no systemd, no NetworkManager), `/tmp` is tmpfs, persistent storage in `/perm`.

---

## Architecture

```
wlan0  ──── wpa_supplicant ──── wifi.Manager (STA + Scanner)
                                         │
uap0   ──── hostapd         ──── wifi.APManager (AP)
                                         │
uap0   ──── DHCP server Go  ──── insomniacslk/dhcp (server)
```

- `wlan0`: managed by wpa_supplicant (existing). STA and Scanner modes.
- `uap0`: virtual interface created by Manager via `iw` subprocess before hostapd starts.
- `hostapd`: manages AP on `uap0`. Binary at `/user/hostapd`, built as static ARM64.
- DHCP server: Go goroutine in APManager, uses `insomniacslk/dhcp` (new dep for wifi module).
- Both managers live in `system/wifi/`. DHCP server in `ap_dhcp.go`.

---

## Mode Selection

```go
type Mode struct {
    STA bool
    AP  bool
}
```

Modes are independent and additive:
- `{STA:true, AP:false}` — client only (current behavior)
- `{STA:false, AP:true}` — AP only (no client connection)
- `{STA:true, AP:true}` — concurrent (wlan0=client, uap0=AP)

Scanner always works when STA is active (existing SCAN/SCAN_RESULTS commands on wpa_supplicant).

Mode is persisted to `/perm/wifi/mode.json` and restored on `Start()`.

---

## APConfig

```go
type APConfig struct {
    SSID    string   // required
    PSK     string   // empty = open network
    Channel int      // default 6 (BCM43430 is 2.4GHz only; ACS not assumed available)
    IP      string   // CIDR for uap0, e.g. "192.168.4.1/24"
    DNS     []string // DNS servers advertised to clients, e.g. ["8.8.8.8", "8.8.4.4"]
}
```

Defaults: `Channel=6`, `IP="192.168.4.1/24"`, `DNS=["8.8.8.8","8.8.4.4"]`.

APConfig is persisted to `/perm/wifi/apconfig.json`.

---

## APManager

`system/wifi/ap.go`

```go
type APManager struct {
    cfg        APConfig
    hostapdBin string        // "/user/hostapd"
    iwBin      string        // "/user/iw"
    ipBin      string        // "/user/ip" (from iproute2 static binary)
    confDir    string        // "/perm/wifi"
    proc       processRunner // injectable (existing)
    dhcp       *apDHCPServer
    cancel     context.CancelFunc
}
```

### Start(ctx) sequence:
1. Create `uap0` via `iw dev wlan0 interface add uap0 type __ap` (subprocess)
2. Assign IP to `uap0` via `ip addr add <cfg.IP> dev uap0` + `ip link set uap0 up` (subprocess)
3. Generate `/perm/wifi/hostapd.conf` from APConfig
4. Start hostapd **without** `-B` flag — run as foreground process via a goroutine using an extended `processRunner.StartProcess(bin, args) (*os.Process, error)` that does not block. The goroutine waits on the process and restarts it on unexpected exit (with 3s backoff), cancelling on ctx.Done().
5. Start DHCP server goroutine on `uap0`

### Stop() sequence:
1. Stop DHCP server
2. Send SIGTERM to hostapd
3. Delete `uap0` via `iw dev uap0 del` (subprocess)

### Hostapd config template:
```
interface=uap0
driver=nl80211
ssid=<SSID>
hw_mode=g
channel=<channel>  # e.g. 6; BCM43430 is 2.4GHz only, no ACS
wpa=2
wpa_passphrase=<PSK>
wpa_key_mgmt=WPA-PSK
rsn_pairwise=CCMP
```
Open network: omit `wpa`, `wpa_passphrase`, `wpa_key_mgmt`, `rsn_pairwise`.

---

## DHCP Server

`system/wifi/ap_dhcp.go`

Uses `insomniacslk/dhcp` server — new dependency to add to `system/wifi/go.mod`.

```go
type apDHCPServer struct {
    iface   string  // "uap0"
    subnet  string  // "192.168.4.0/24" (derived from APConfig.IP)
    start   net.IP  // .100
    end     net.IP  // .200
    dns     []string // from APConfig.DNS
    server  *dhcpv4.Server
}
```

- Leases held in memory (map[MAC]IP). No persistence — clients reconnect after reboot.
- DNS: advertised from `APConfig.DNS` (default `["8.8.8.8","8.8.4.4"]`).
- Gateway: uap0's own IP (e.g. `192.168.4.1`).
- Lease time: 1 hour.

---

## Extended Manager API

`system/wifi/wifi.go`

```go
// New fields in Config:
type Config struct {
    WpaSupplicantBin string
    HostapdBin       string   // e.g. "/user/hostapd"
    ConfDir          string
    CtrlDir          string
    Iface            string   // "wlan0"
    APConfig         APConfig
}

// New methods on Manager:
func (m *Manager) SetMode(ctx context.Context, mode Mode) error
func (m *Manager) GetMode() Mode
func (m *Manager) APStatus() (APStatus, error)
```

```go
type APStatus struct {
    Running  bool
    IP       string   // uap0's IP
    Clients  int      // connected DHCP clients
}
```

`Manager` acquires a `sync.Mutex` for all mode reads/writes and APManager state changes — `SetMode`, `GetMode`, `APStatus` are all goroutine-safe.

`SetMode` persists the mode and applies changes live:
- Switching AP on: calls `apMgr.Start(ctx)`
- Switching AP off: calls `apMgr.Stop()`
- Switching STA off: sends `DISCONNECT` + stops wpa_supplicant
- Switching STA on: restarts wpa_supplicant

---

## Virtual Interface Creation

`system/wifi/vif.go`

Virtual interface creation requires nl80211 genetlink (cfg80211), not rtnetlink — `vishvananda/netlink` does not support this. We use `iw` subprocess instead (same subprocess pattern as wpa_supplicant/hostapd):

```go
func createVirtualAP(proc processRunner, iwBin, parent, name string) error
    // runs: iw dev <parent> interface add <name> type __ap

func deleteVirtualAP(proc processRunner, iwBin, name string) error
    // runs: iw dev <name> del
```

The `iw` binary must be available on gokrazy. It is included in `github.com/gokrazy/wifi` extrafiles or can be built as a static binary alongside hostapd.

---

## Static Binaries

All binaries are built as static ARM64 and deployed via `oioio/config.json` ExtraFilePaths.

| Binary | Path | Source | Size |
|--------|------|--------|------|
| `hostapd` | `/user/hostapd` | `system/wifi/bin/hostapd` | ~1.5MB |
| `iw` | `/user/iw` | `system/wifi/bin/iw` | ~200KB |
| `ip` | `/user/ip` | `system/wifi/bin/ip` (iproute2) | ~700KB |

Build: single Dockerfile in `system/wifi/bin/` builds all three.

## NAT / IP Forwarding

Out of scope for this design. The AP serves as a local network segment only (clients can reach `192.168.4.1` and each other). Clients will not have internet access via the Pi in this implementation. NAT/forwarding can be added in a future iteration if needed.

---

## Error Handling

- `uap0` creation failure → `SetMode` returns error, AP not started.
- hostapd fails to start → `APManager.Start` returns error; STA mode unaffected.
- DHCP server error → logged, AP still operational (clients get no IP but can connect).
- hostapd exits unexpectedly → restart goroutine in APManager (same pattern as wpa_supplicant polling).
- `Stop()` errors (netlink, SIGTERM) → logged but not fatal.

---

## Testing

- `APManager` uses injected `processRunner` (same pattern as existing wifi.Manager); no netlink injection needed (uses subprocess).
- Unit tests: `TestAPManager_Start`, `TestAPManager_Stop`, `TestCreateVirtualAP`.
- DHCP server: `TestAPDHCPServer_Assign` — verify IP assignment from pool.
- Mode persistence: `TestManager_SetMode_Persist` — write/read mode.json roundtrip.
- Integration test: not feasible without hardware; rely on unit tests + manual on-device test.

---

## File Structure

```
system/wifi/
  wifi.go          — Manager, Config, Mode, extended API (modify)
  ap.go            — APManager, APConfig, APStatus (new)
  ap_dhcp.go       — apDHCPServer (new)
  vif.go           — createVirtualAP / deleteVirtualAP via iw subprocess (new)
  conf.go          — confManager: add mode.json + apconfig.json (modify)
  process.go       — processRunner interface: add StartProcess() (modify)
  wpa_conn.go      — wpaConn (existing)
  wifi_test.go     — existing tests
  ap_test.go       — APManager tests (new)
  bin/
    hostapd        — static ARM64 binary (new)
    iw             — static ARM64 binary (new)
    ip             — static ARM64 binary from iproute2 (new)
    Dockerfile     — build for hostapd + iw + ip (new)
```
