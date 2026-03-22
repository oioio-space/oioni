# WiFi Client + IP Management Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add WiFi network management and per-interface IP configuration to the oioni device, persisted across reboots.

**Architecture:** Two new Go modules (`system/wifi`, `system/netconf`) provide backend managers wired into `cmd/oioni/epaper.go`; five new UI scenes let the user control WiFi and IP from the e-ink display. `wpa_supplicant` is managed as a subprocess (static ARM64 binary deployed via `ExtraFilePaths`).

**Tech Stack:** Go stdlib (os/exec, net, encoding/json), wpa_supplicant Unix socket protocol, `github.com/vishvananda/netlink`, `github.com/insomniacslk/dhcp/dhcpv4/nclient4`, existing `ui/gui` widget library.

**Spec:** `docs/superpowers/specs/2026-03-22-wifi-client-design.md`

---

## File Map

### New modules
| File | Purpose |
|------|---------|
| `system/wifi/go.mod` | Module `github.com/oioio-space/oioni/system/wifi` |
| `system/wifi/hal.go` | `wpaConn` + `processRunner` interfaces |
| `system/wifi/wpa.go` | wpa_supplicant control socket (real `wpaConn` impl) |
| `system/wifi/config.go` | Read/write `wpa_supplicant.conf` + migration |
| `system/wifi/process.go` | Start/stop wpa_supplicant subprocess (`processRunner` impl) |
| `system/wifi/wifi.go` | `Manager` public API |
| `system/wifi/wifi_test.go` | Manager tests using fake HAL |
| `system/wifi/config_test.go` | Config + migration tests |
| `system/wifi/build/Dockerfile` | Static ARM64 wpa_supplicant build |
| `system/wifi/bin/.gitkeep` | Placeholder (bin/ in .gitignore) |
| `system/netconf/go.mod` | Module `github.com/oioio-space/oioni/system/netconf` |
| `system/netconf/hal.go` | `netlinkClient` interface |
| `system/netconf/static.go` | Static IP via netlink |
| `system/netconf/dhcp.go` | DHCP client via insomniacslk/dhcp |
| `system/netconf/config.go` | Read/write `interfaces.json` |
| `system/netconf/netconf.go` | `Manager` public API + DNS override |
| `system/netconf/netconf_test.go` | Manager tests using fake netlinkClient |
| `system/netconf/config_test.go` | JSON persistence tests |

### Modified files
| File | Change |
|------|--------|
| `go.work` | Add `./system/wifi`, `./system/netconf` |
| `cmd/oioni/go.mod` | Add wifi+netconf requires + replace directives |
| `cmd/oioni/epaper.go` | Create managers, pass to `NewHomeScene` |
| `cmd/oioni/ui/home.go` | `NewHomeScene(nav, wifiMgr, netconfMgr)` |
| `cmd/oioni/ui/scene_config.go` | Replace stub with WiFi+Network items; accept managers |
| `cmd/oioni/ui/scene_helpers_test.go` | Wrap changed signatures in lambdas |
| `Makefile` | Add `build-wifi-bins` target |
| `oioio/config.json` | Remove `github.com/gokrazy/wifi`, add wpa_supplicant ExtraFilePaths |
| `.gitignore` | Add `system/wifi/bin/` |

### New UI files
| File | Purpose |
|------|---------|
| `cmd/oioni/ui/scene_wifi.go` | WiFi scene (toggle + network list + scan) |
| `cmd/oioni/ui/scene_wifi_password.go` | PSK entry scene |
| `cmd/oioni/ui/scene_wifi_connecting.go` | Connection status polling scene |
| `cmd/oioni/ui/scene_network.go` | Interface list scene |
| `cmd/oioni/ui/scene_network_iface.go` | IP Config scene (DHCP/Static mode + fields) |
| `cmd/oioni/ui/widget_modesel.go` | Two-button mode selector widget (DHCP|Static) |

---

## Task 1: Module scaffolding

**Files:**
- Create: `system/wifi/go.mod`
- Create: `system/netconf/go.mod`
- Modify: `go.work`
- Modify: `cmd/oioni/go.mod`
- Modify: `.gitignore`

- [ ] **Step 1: Create `system/wifi/go.mod`**

```
module github.com/oioio-space/oioni/system/wifi

go 1.26
```

- [ ] **Step 2: Create `system/netconf/go.mod`**

```
module github.com/oioio-space/oioni/system/netconf

go 1.26

require (
    github.com/insomniacslk/dhcp v0.0.0-20240710054256-ddd8a41251c9
    github.com/vishvananda/netlink v1.3.0
)
```

Then run `cd system/netconf && go mod tidy` to generate go.sum.

- [ ] **Step 3: Update `go.work` — add the two new modules**

```
go 1.26.0

use (
    ./cmd/oioni
    ./drivers/epd
    ./drivers/touch
    ./drivers/usbgadget
    ./system/imgvol
    ./system/netconf
    ./system/storage
    ./system/wifi
    ./tools
    ./ui/canvas
    ./ui/gui
)
```

- [ ] **Step 4: Update `cmd/oioni/go.mod` — add require + replace**

Add to the `require` block:
```
github.com/oioio-space/oioni/system/netconf v0.0.0-00010101000000-000000000000
github.com/oioio-space/oioni/system/wifi v0.0.0-00010101000000-000000000000
```

Add to the `replace` block:
```
github.com/oioio-space/oioni/system/netconf => ../../system/netconf
github.com/oioio-space/oioni/system/wifi    => ../../system/wifi
```

Then run `cd cmd/oioni && go mod tidy` (workspace resolves the replace).

- [ ] **Step 5: Update `.gitignore` — ignore built binaries**

Add:
```
system/wifi/bin/
```

- [ ] **Step 6: Verify the workspace compiles**

```bash
cd /home/oioio/Documents/GolandProjects/oioni
go build ./system/wifi/... 2>&1
go build ./system/netconf/... 2>&1
```
Expected: errors about missing source files, NOT module errors.

- [ ] **Step 7: Commit**

```bash
git add go.work system/wifi/go.mod system/netconf/go.mod cmd/oioni/go.mod .gitignore
git commit -m "build: scaffold system/wifi and system/netconf modules"
```

---

## Task 2: `system/wifi` — HAL interfaces + wpa control socket

**Files:**
- Create: `system/wifi/hal.go`
- Create: `system/wifi/wpa.go`
- Create: `system/wifi/wpa_test.go`

- [ ] **Step 1: Write `system/wifi/hal.go` — interfaces for testing**

```go
// system/wifi/hal.go — injectable interfaces for wpa_supplicant HAL
package wifi

// wpaConn is the interface over a wpa_supplicant control socket connection.
// Real implementation: realWpaConn (wpa.go). Test implementation: fakeWpaConn.
type wpaConn interface {
    // SendCommand sends a single-line command and returns the full response.
    SendCommand(cmd string) (string, error)
    Close() error
}

// processRunner starts and terminates wpa_supplicant.
type processRunner interface {
    // Start launches wpa_supplicant with the given args. Returns an error if
    // the process fails to start. The process runs detached (daemon mode -B).
    Start(bin string, args []string) error
}
```

- [ ] **Step 2: Write `system/wifi/wpa.go` — real Unix socket client**

```go
// system/wifi/wpa.go — wpa_supplicant control socket protocol
package wifi

import (
    "fmt"
    "net"
    "os"
    "strings"
    "time"
)

// realWpaConn communicates with wpa_supplicant over a Unix socket.
// wpa_supplicant uses UNIX DGRAM sockets: we bind a local path, then
// connect to the ctrl socket path and exchange messages.
type realWpaConn struct {
    conn     *net.UnixConn
    localPath string
}

// dialWpa connects to the wpa_supplicant control socket at ctrlPath.
// localPath is a unique tmp path for our side of the DGRAM socket.
func dialWpa(ctrlPath, localPath string) (*realWpaConn, error) {
    localAddr := &net.UnixAddr{Name: localPath, Net: "unixgram"}
    remoteAddr := &net.UnixAddr{Name: ctrlPath, Net: "unixgram"}

    conn, err := net.DialUnix("unixgram", localAddr, remoteAddr)
    if err != nil {
        os.Remove(localPath)
        return nil, fmt.Errorf("dial wpa socket %s: %w", ctrlPath, err)
    }
    return &realWpaConn{conn: conn, localPath: localPath}, nil
}

func (c *realWpaConn) SendCommand(cmd string) (string, error) {
    if err := c.conn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
        return "", err
    }
    if _, err := c.conn.Write([]byte(cmd)); err != nil {
        return "", fmt.Errorf("wpa send %q: %w", cmd, err)
    }
    buf := make([]byte, 4096)
    n, err := c.conn.Read(buf)
    if err != nil {
        return "", fmt.Errorf("wpa recv %q: %w", cmd, err)
    }
    return strings.TrimRight(string(buf[:n]), "\n"), nil
}

func (c *realWpaConn) Close() error {
    err := c.conn.Close()
    os.Remove(c.localPath)
    return err
}
```

- [ ] **Step 3: Write `system/wifi/wpa_test.go` — parse SCAN_RESULTS**

The most complex parsing is `SCAN_RESULTS`. Test it with a fake response string.

```go
package wifi

import (
    "strings"
    "testing"
)

func TestParseScanResults(t *testing.T) {
    raw := "bssid / frequency / signal level / flags / ssid\n" +
        "aa:bb:cc:dd:ee:ff\t2437\t-55\t[WPA2-PSK-CCMP][ESS]\tMyNet\n" +
        "11:22:33:44:55:66\t2412\t-72\t[ESS]\tOpenNet\n"
    nets := parseScanResults(raw)
    if len(nets) != 2 {
        t.Fatalf("want 2, got %d", len(nets))
    }
    if nets[0].SSID != "MyNet" {
        t.Errorf("want MyNet, got %q", nets[0].SSID)
    }
    if nets[0].Security != "WPA2" {
        t.Errorf("want WPA2, got %q", nets[0].Security)
    }
    if nets[0].Signal != -55 {
        t.Errorf("want -55, got %d", nets[0].Signal)
    }
    if nets[1].Security != "Open" {
        t.Errorf("want Open, got %q", nets[1].Security)
    }
}

func TestParseStatus(t *testing.T) {
    raw := "wpa_state=COMPLETED\nssid=MyNet\nip_address=192.168.1.10\n"
    st := parseWpaStatus(raw)
    if st.State != "COMPLETED" {
        t.Errorf("want COMPLETED, got %q", st.State)
    }
    if st.SSID != "MyNet" {
        t.Errorf("want MyNet, got %q", st.SSID)
    }
}
```

- [ ] **Step 4: Add `parseScanResults` and `parseWpaStatus` to `wpa.go`**

```go
// parseScanResults parses the SCAN_RESULTS multiline response.
// First line is the header; each subsequent line: bssid\tfreq\tsignal\tflags\tssid
func parseScanResults(raw string) []Network {
    lines := strings.Split(raw, "\n")
    var nets []Network
    for _, line := range lines[1:] { // skip header
        fields := strings.SplitN(line, "\t", 5)
        if len(fields) < 5 {
            continue
        }
        sig := 0
        fmt.Sscanf(fields[2], "%d", &sig)
        sec := "Open"
        if strings.Contains(fields[3], "WPA2") {
            sec = "WPA2"
        } else if strings.Contains(fields[3], "WPA") {
            sec = "WPA"
        } else if strings.Contains(fields[3], "WEP") {
            sec = "WEP"
        }
        nets = append(nets, Network{SSID: fields[4], Signal: sig, Security: sec})
    }
    return nets
}

// parseWpaStatus parses the STATUS command response (key=value lines).
func parseWpaStatus(raw string) Status {
    var st Status
    for _, line := range strings.Split(raw, "\n") {
        k, v, ok := strings.Cut(line, "=")
        if !ok {
            continue
        }
        switch k {
        case "wpa_state":
            st.State = v
        case "ssid":
            st.SSID = v
        }
    }
    return st
}
```

- [ ] **Step 5: Run tests**

```bash
cd /home/oioio/Documents/GolandProjects/oioni/system/wifi
go test ./... -v -run TestParse
```
Expected: PASS (2 tests).

- [ ] **Step 6: Commit**

```bash
git add system/wifi/hal.go system/wifi/wpa.go system/wifi/wpa_test.go
git commit -m "feat(wifi): HAL interfaces + wpa_supplicant socket protocol"
```

---

## Task 3: `system/wifi` — config.go (conf file + migration)

**Files:**
- Create: `system/wifi/config.go`
- Create: `system/wifi/config_test.go`

- [ ] **Step 1: Write failing test**

```go
// system/wifi/config_test.go
package wifi

import (
    "os"
    "path/filepath"
    "testing"
)

func TestWriteReadConf(t *testing.T) {
    dir := t.TempDir()
    cfg := &confManager{dir: dir}
    networks := []savedNetwork{{SSID: "TestNet", PSK: "secret123"}}
    if err := cfg.write(networks); err != nil {
        t.Fatal(err)
    }
    got, err := cfg.read()
    if err != nil {
        t.Fatal(err)
    }
    if len(got) != 1 || got[0].SSID != "TestNet" || got[0].PSK != "secret123" {
        t.Fatalf("unexpected result: %+v", got)
    }
}

func TestMigrateWifiJSON(t *testing.T) {
    dir := t.TempDir()
    // Write legacy wifi.json (gokrazy/wifi format: {"ssid":"OldNet","passphrase":"pw"})
    legacy := filepath.Join(dir, "wifi.json")
    os.WriteFile(legacy, []byte(`{"ssid":"OldNet","passphrase":"oldpass"}`), 0600)

    cfg := &confManager{dir: dir}
    if err := cfg.migrateIfNeeded(); err != nil {
        t.Fatal(err)
    }
    // Legacy file renamed
    if _, err := os.Stat(legacy); !os.IsNotExist(err) {
        t.Error("expected wifi.json to be renamed after migration")
    }
    if _, err := os.Stat(legacy + ".migrated"); err != nil {
        t.Error("expected wifi.json.migrated to exist")
    }
    // conf should now contain OldNet
    nets, err := cfg.read()
    if err != nil {
        t.Fatal(err)
    }
    if len(nets) != 1 || nets[0].SSID != "OldNet" {
        t.Fatalf("unexpected networks after migration: %+v", nets)
    }
}
```

- [ ] **Step 2: Run test — verify it fails**

```bash
cd system/wifi && go test ./... -run TestWriteReadConf
```
Expected: compile error (confManager not defined).

- [ ] **Step 3: Write `system/wifi/config.go`**

```go
// system/wifi/config.go — wpa_supplicant.conf read/write + legacy migration
package wifi

import (
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
    "strings"
)

type savedNetwork struct {
    SSID string
    PSK  string // empty for open networks
}

type confManager struct {
    dir string // e.g. "/perm/wifi"
}

func (c *confManager) confPath() string {
    return filepath.Join(c.dir, "wpa_supplicant.conf")
}

// write serialises networks into wpa_supplicant.conf format.
func (c *confManager) write(networks []savedNetwork) error {
    if err := os.MkdirAll(c.dir, 0755); err != nil {
        return err
    }
    var b strings.Builder
    b.WriteString("ctrl_interface=/var/run/wpa_supplicant\nctrl_interface_group=0\nupdate_config=1\n\n")
    for _, n := range networks {
        b.WriteString("network={\n")
        fmt.Fprintf(&b, "    ssid=%q\n", n.SSID)
        if n.PSK != "" {
            fmt.Fprintf(&b, "    psk=%q\n", n.PSK)
        } else {
            b.WriteString("    key_mgmt=NONE\n")
        }
        b.WriteString("}\n\n")
    }
    return os.WriteFile(c.confPath(), []byte(b.String()), 0600)
}

// read parses the wpa_supplicant.conf and returns saved networks.
// Returns empty slice (not error) if the file does not exist.
func (c *confManager) read() ([]savedNetwork, error) {
    data, err := os.ReadFile(c.confPath())
    if os.IsNotExist(err) {
        return nil, nil
    }
    if err != nil {
        return nil, err
    }
    var nets []savedNetwork
    var cur *savedNetwork
    for _, line := range strings.Split(string(data), "\n") {
        line = strings.TrimSpace(line)
        if line == "network={" {
            cur = &savedNetwork{}
        } else if line == "}" && cur != nil {
            nets = append(nets, *cur)
            cur = nil
        } else if cur != nil {
            k, v, ok := strings.Cut(line, "=")
            if !ok {
                continue
            }
            v = strings.Trim(v, `"`)
            switch strings.TrimSpace(k) {
            case "ssid":
                cur.SSID = v
            case "psk":
                cur.PSK = v
            }
        }
    }
    return nets, nil
}

// migrateIfNeeded imports /perm/wifi.json (gokrazy/wifi legacy) into wpa_supplicant.conf.
func (c *confManager) migrateIfNeeded() error {
    legacyPath := filepath.Join(c.dir, "wifi.json")
    data, err := os.ReadFile(legacyPath)
    if os.IsNotExist(err) {
        return nil
    }
    if err != nil {
        return err
    }
    var legacy struct {
        SSID       string `json:"ssid"`
        Passphrase string `json:"passphrase"`
    }
    if err := json.Unmarshal(data, &legacy); err != nil {
        return fmt.Errorf("parse wifi.json: %w", err)
    }
    existing, err := c.read()
    if err != nil {
        return err
    }
    existing = append(existing, savedNetwork{SSID: legacy.SSID, PSK: legacy.Passphrase})
    if err := c.write(existing); err != nil {
        return err
    }
    return os.Rename(legacyPath, legacyPath+".migrated")
}
```

- [ ] **Step 4: Run tests**

```bash
cd system/wifi && go test ./... -v -run "TestWriteReadConf|TestMigrate"
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add system/wifi/config.go system/wifi/config_test.go
git commit -m "feat(wifi): wpa_supplicant.conf read/write + legacy wifi.json migration"
```

---

## Task 4: `system/wifi` — process.go + Manager (wifi.go)

**Files:**
- Create: `system/wifi/process.go`
- Create: `system/wifi/wifi.go`
- Create: `system/wifi/wifi_test.go`

- [ ] **Step 1: Write failing test for Manager**

```go
// system/wifi/wifi_test.go
package wifi

import (
    "context"
    "os"
    "testing"
    "time"
)

// fakeProcess satisfies processRunner for tests.
type fakeProcess struct{ started bool }
func (f *fakeProcess) Start(_ string, _ []string) error { f.started = true; return nil }

// fakeWpaConn satisfies wpaConn for tests.
type fakeWpa struct {
    responses map[string]string
    commands  []string
}
func (f *fakeWpa) SendCommand(cmd string) (string, error) {
    f.commands = append(f.commands, cmd)
    if r, ok := f.responses[cmd]; ok {
        return r, nil
    }
    return "OK", nil
}
func (f *fakeWpa) Close() error { return nil }

func newTestManager(t *testing.T, wpa *fakeWpa) *Manager {
    dir := t.TempDir()
    proc := &fakeProcess{}
    m := &Manager{
        cfg:     Config{ConfDir: dir, Iface: "wlan0"},
        conf:    &confManager{dir: dir},
        proc:    proc,
        newConn: func(_, _ string) (wpaConn, error) { return wpa, nil },
    }
    return m
}

func TestManager_Scan(t *testing.T) {
    wpa := &fakeWpa{responses: map[string]string{
        "SCAN":         "OK",
        "SCAN_RESULTS": "bssid / frequency / signal level / flags / ssid\naa:bb:cc:dd:ee:ff\t2437\t-60\t[WPA2-PSK-CCMP][ESS]\tTestNet\n",
    }}
    m := newTestManager(t, wpa)
    nets, err := m.Scan()
    if err != nil {
        t.Fatal(err)
    }
    if len(nets) != 1 || nets[0].SSID != "TestNet" {
        t.Fatalf("unexpected scan results: %+v", nets)
    }
}

func TestManager_Connect_Save(t *testing.T) {
    wpa := &fakeWpa{responses: map[string]string{
        "ADD_NETWORK": "0",
    }}
    m := newTestManager(t, wpa)
    if err := m.Connect("MyNet", "mypass", true); err != nil {
        t.Fatal(err)
    }
    // saved to conf
    nets, err := m.conf.read()
    if err != nil {
        t.Fatal(err)
    }
    found := false
    for _, n := range nets {
        if n.SSID == "MyNet" {
            found = true
        }
    }
    if !found {
        t.Error("expected MyNet in saved networks")
    }
}

func TestManager_RemoveSaved(t *testing.T) {
    dir := t.TempDir()
    conf := &confManager{dir: dir}
    conf.write([]savedNetwork{{SSID: "OldNet", PSK: "pw"}})

    wpa := &fakeWpa{}
    m := newTestManager(t, wpa)
    m.conf = conf
    if err := m.RemoveSaved("OldNet"); err != nil {
        t.Fatal(err)
    }
    nets, _ := conf.read()
    if len(nets) != 0 {
        t.Errorf("expected empty after remove, got %+v", nets)
    }
}

func TestManager_Status(t *testing.T) {
    wpa := &fakeWpa{responses: map[string]string{
        "STATUS": "wpa_state=COMPLETED\nssid=MyNet\n",
    }}
    m := newTestManager(t, wpa)
    st, err := m.Status()
    if err != nil {
        t.Fatal(err)
    }
    if st.State != "COMPLETED" || st.SSID != "MyNet" {
        t.Errorf("unexpected status: %+v", st)
    }
}
```

- [ ] **Step 2: Run test — verify it fails**

```bash
cd system/wifi && go test ./... -run TestManager
```
Expected: compile error (Manager not defined).

- [ ] **Step 3: Write `system/wifi/process.go`**

```go
// system/wifi/process.go — wpa_supplicant subprocess management
package wifi

import "os/exec"

type realProcess struct{}

func (r *realProcess) Start(bin string, args []string) error {
    cmd := exec.Command(bin, args...)
    return cmd.Run() // -B causes immediate exit after daemonising
}
```

- [ ] **Step 4: Write `system/wifi/wifi.go` — Manager**

```go
// system/wifi/wifi.go — WiFi manager public API
package wifi

import (
    "context"
    "fmt"
    "os"
    "path/filepath"
    "strings"
    "time"
)

// Config holds the runtime configuration for the Manager.
type Config struct {
    WpaSupplicantBin string // e.g. "/user/wpa_supplicant"
    ConfDir          string // e.g. "/perm/wifi"
    CtrlDir          string // e.g. "/var/run/wpa_supplicant"
    Iface            string // e.g. "wlan0"
}

// Network is a scanned WiFi access point.
type Network struct {
    SSID     string
    Signal   int    // dBm
    Security string // "WPA2", "WPA", "WEP", "Open"
    Saved    bool
}

// SavedNetwork is a network with persisted credentials.
type SavedNetwork struct {
    SSID string
}

// Status is the current wpa_supplicant state.
type Status struct {
    State   string // "COMPLETED", "ASSOCIATING", "DISCONNECTED", ...
    SSID    string
    Enabled bool
}

// Manager wraps wpa_supplicant to provide WiFi management.
type Manager struct {
    cfg     Config
    conf    *confManager
    proc    processRunner
    conn    wpaConn // nil until Start is called
    newConn func(ctrlPath, localPath string) (wpaConn, error) // injectable for tests
}

// New creates a Manager with the given configuration.
func New(cfg Config) *Manager {
    return &Manager{
        cfg:  cfg,
        conf: &confManager{dir: cfg.ConfDir},
        proc: &realProcess{},
        newConn: func(ctrlPath, localPath string) (wpaConn, error) {
            return dialWpa(ctrlPath, localPath)
        },
    }
}

// Start launches wpa_supplicant, polls until the control socket appears, and
// connects to it. Also runs wifi.json migration. Non-fatal on error.
func (m *Manager) Start(ctx context.Context) error {
    if err := m.conf.migrateIfNeeded(); err != nil {
        // non-fatal — log in caller
        _ = err
    }

    args := []string{
        "-i", m.cfg.Iface,
        "-C", m.cfg.CtrlDir,
        "-c", filepath.Join(m.cfg.ConfDir, "wpa_supplicant.conf"),
        "-B",
    }
    if err := m.proc.Start(m.cfg.WpaSupplicantBin, args); err != nil {
        return fmt.Errorf("wpa_supplicant start: %w", err)
    }

    // Poll for control socket (up to 3s)
    ctrlPath := filepath.Join(m.cfg.CtrlDir, m.cfg.Iface)
    localPath := fmt.Sprintf("/tmp/oioni-wpa-%d", os.Getpid())
    deadline := time.Now().Add(3 * time.Second)
    for time.Now().Before(deadline) {
        conn, err := m.newConn(ctrlPath, localPath)
        if err == nil {
            m.conn = conn
            return nil
        }
        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-time.After(100 * time.Millisecond):
        }
    }
    return fmt.Errorf("wpa_supplicant control socket not ready after 3s")
}

// SetEnabled enables or disables WiFi via /sys/class/rfkill/ sysfs.
func (m *Manager) SetEnabled(enabled bool) error {
    entries, err := os.ReadDir("/sys/class/rfkill")
    if err != nil {
        return fmt.Errorf("rfkill: %w", err)
    }
    val := "0"
    if !enabled {
        val = "1"
    }
    for _, e := range entries {
        typePath := filepath.Join("/sys/class/rfkill", e.Name(), "type")
        data, err := os.ReadFile(typePath)
        if err != nil {
            continue
        }
        if strings.TrimSpace(string(data)) != "wlan" {
            continue
        }
        softPath := filepath.Join("/sys/class/rfkill", e.Name(), "soft")
        if err := os.WriteFile(softPath, []byte(val), 0644); err != nil {
            return fmt.Errorf("rfkill soft: %w", err)
        }
        return nil
    }
    return fmt.Errorf("no wlan rfkill entry found")
}

// Scan triggers a wifi scan and waits ~2s for results. Call in a goroutine.
func (m *Manager) Scan() ([]Network, error) {
    if _, err := m.send("SCAN"); err != nil {
        return nil, err
    }
    time.Sleep(2 * time.Second)
    raw, err := m.send("SCAN_RESULTS")
    if err != nil {
        return nil, err
    }
    nets := parseScanResults(raw)

    // Mark saved networks
    saved, _ := m.conf.read()
    savedSet := make(map[string]bool, len(saved))
    for _, s := range saved {
        savedSet[s.SSID] = true
    }
    for i := range nets {
        nets[i].Saved = savedSet[nets[i].SSID]
    }
    return nets, nil
}

// Connect connects to an SSID with optional PSK. If save is true, persists credentials.
func (m *Manager) Connect(ssid, psk string, save bool) error {
    id, err := m.send("ADD_NETWORK")
    if err != nil {
        return err
    }
    id = strings.TrimSpace(id)
    cmds := []string{
        fmt.Sprintf(`SET_NETWORK %s ssid "%s"`, id, ssid),
    }
    if psk != "" {
        cmds = append(cmds, fmt.Sprintf(`SET_NETWORK %s psk "%s"`, id, psk))
    } else {
        cmds = append(cmds, fmt.Sprintf("SET_NETWORK %s key_mgmt NONE", id))
    }
    cmds = append(cmds,
        "SELECT_NETWORK "+id,
        "RECONNECT",
    )
    for _, cmd := range cmds {
        if _, err := m.send(cmd); err != nil {
            return err
        }
    }
    if save {
        existing, _ := m.conf.read()
        // Remove duplicate if re-saving same SSID
        var filtered []savedNetwork
        for _, n := range existing {
            if n.SSID != ssid {
                filtered = append(filtered, n)
            }
        }
        filtered = append(filtered, savedNetwork{SSID: ssid, PSK: psk})
        if err := m.conf.write(filtered); err != nil {
            return fmt.Errorf("save credentials: %w", err)
        }
    }
    return nil
}

// Disconnect disconnects from the current network.
func (m *Manager) Disconnect() error {
    _, err := m.send("DISCONNECT")
    return err
}

// Status returns the current connection state.
func (m *Manager) Status() (Status, error) {
    raw, err := m.send("STATUS")
    if err != nil {
        return Status{}, err
    }
    return parseWpaStatus(raw), nil
}

// SavedNetworks returns the list of persisted networks.
func (m *Manager) SavedNetworks() ([]SavedNetwork, error) {
    nets, err := m.conf.read()
    if err != nil {
        return nil, err
    }
    var result []SavedNetwork
    for _, n := range nets {
        result = append(result, SavedNetwork{SSID: n.SSID})
    }
    return result, nil
}

// RemoveSaved removes a network from the saved list by SSID.
func (m *Manager) RemoveSaved(ssid string) error {
    existing, err := m.conf.read()
    if err != nil {
        return err
    }
    var filtered []savedNetwork
    for _, n := range existing {
        if n.SSID != ssid {
            filtered = append(filtered, n)
        }
    }
    return m.conf.write(filtered)
}

func (m *Manager) send(cmd string) (string, error) {
    if m.conn == nil {
        return "", fmt.Errorf("wifi manager not started")
    }
    return m.conn.SendCommand(cmd)
}
```

- [ ] **Step 5: Run tests**

```bash
cd system/wifi && go test ./... -v
```
Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add system/wifi/process.go system/wifi/wifi.go system/wifi/wifi_test.go
git commit -m "feat(wifi): Manager API (Scan, Connect, Status, SavedNetworks)"
```

---

## Task 5: `system/netconf` — HAL + static IP

**Files:**
- Create: `system/netconf/hal.go`
- Create: `system/netconf/static.go`
- Create: `system/netconf/static_test.go`

- [ ] **Step 1: Write failing test**

```go
// system/netconf/static_test.go
package netconf

import (
    "net"
    "testing"

    "github.com/vishvananda/netlink"
)

type fakeNetlink struct {
    addedAddrs  []string
    addedRoutes []string
    links       []netlink.Link
}

func (f *fakeNetlink) LinkByName(name string) (netlink.Link, error) {
    for _, l := range f.links {
        if l.Attrs().Name == name {
            return l, nil
        }
    }
    la := netlink.NewLinkAttrs()
    la.Name = name
    return &netlink.Dummy{LinkAttrs: la}, nil
}
func (f *fakeNetlink) AddrAdd(link netlink.Link, addr *netlink.Addr) error {
    f.addedAddrs = append(f.addedAddrs, addr.String())
    return nil
}
func (f *fakeNetlink) AddrDel(link netlink.Link, addr *netlink.Addr) error { return nil }
func (f *fakeNetlink) RouteAdd(route *netlink.Route) error {
    f.addedRoutes = append(f.addedRoutes, route.Gw.String())
    return nil
}
func (f *fakeNetlink) RouteDel(route *netlink.Route) error { return nil }
func (f *fakeNetlink) LinkList() ([]netlink.Link, error)   { return f.links, nil }

func TestApplyStatic(t *testing.T) {
    nl := &fakeNetlink{}
    if err := applyStatic(nl, "eth0", "192.168.1.10/24", "192.168.1.1"); err != nil {
        t.Fatal(err)
    }
    if len(nl.addedAddrs) != 1 || nl.addedAddrs[0] != "192.168.1.10/24" {
        t.Errorf("unexpected addrs: %v", nl.addedAddrs)
    }
    if len(nl.addedRoutes) != 1 || nl.addedRoutes[0] != "192.168.1.1" {
        t.Errorf("unexpected routes: %v", nl.addedRoutes)
    }
}
```

- [ ] **Step 2: Run test — verify fails**

```bash
cd system/netconf && go test ./... -run TestApplyStatic
```
Expected: compile error.

- [ ] **Step 3: Write `system/netconf/hal.go`**

```go
// system/netconf/hal.go — injectable netlink interface for testing
package netconf

import "github.com/vishvananda/netlink"

// netlinkClient abstracts vishvananda/netlink for testing.
type netlinkClient interface {
    LinkByName(name string) (netlink.Link, error)
    AddrAdd(link netlink.Link, addr *netlink.Addr) error
    AddrDel(link netlink.Link, addr *netlink.Addr) error
    RouteAdd(route *netlink.Route) error
    RouteDel(route *netlink.Route) error
    LinkList() ([]netlink.Link, error)
}

// realNetlink delegates to vishvananda/netlink package-level functions.
type realNetlink struct{}

func (r *realNetlink) LinkByName(name string) (netlink.Link, error) { return netlink.LinkByName(name) }
func (r *realNetlink) AddrAdd(l netlink.Link, a *netlink.Addr) error  { return netlink.AddrAdd(l, a) }
func (r *realNetlink) AddrDel(l netlink.Link, a *netlink.Addr) error  { return netlink.AddrDel(l, a) }
func (r *realNetlink) RouteAdd(r2 *netlink.Route) error               { return netlink.RouteAdd(r2) }
func (r *realNetlink) RouteDel(r2 *netlink.Route) error               { return netlink.RouteDel(r2) }
func (r *realNetlink) LinkList() ([]netlink.Link, error)              { return netlink.LinkList() }
```

- [ ] **Step 4: Write `system/netconf/static.go`**

```go
// system/netconf/static.go — apply static IP configuration via netlink
package netconf

import (
    "fmt"
    "net"

    "github.com/vishvananda/netlink"
)

// applyStatic sets a static IP and gateway on iface using nl.
// cidr is "192.168.1.10/24", gateway is "192.168.1.1" (empty = no default route).
func applyStatic(nl netlinkClient, iface, cidr, gateway string) error {
    link, err := nl.LinkByName(iface)
    if err != nil {
        return fmt.Errorf("link %s: %w", iface, err)
    }
    addr, err := netlink.ParseAddr(cidr)
    if err != nil {
        return fmt.Errorf("parse addr %s: %w", cidr, err)
    }
    if err := nl.AddrAdd(link, addr); err != nil {
        return fmt.Errorf("addr add: %w", err)
    }
    if gateway != "" {
        gw := net.ParseIP(gateway)
        if gw == nil {
            return fmt.Errorf("invalid gateway: %s", gateway)
        }
        route := &netlink.Route{
            LinkIndex: link.Attrs().Index,
            Gw:        gw,
        }
        if err := nl.RouteAdd(route); err != nil {
            return fmt.Errorf("route add: %w", err)
        }
    }
    return nil
}
```

- [ ] **Step 5: Run tests**

```bash
cd system/netconf && go test ./... -v -run TestApplyStatic
```
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add system/netconf/hal.go system/netconf/static.go system/netconf/static_test.go
git commit -m "feat(netconf): netlinkClient HAL + static IP via netlink"
```

---

## Task 6: `system/netconf` — DHCP client

**Files:**
- Create: `system/netconf/dhcp.go`
- Create: `system/netconf/dhcp_test.go`

- [ ] **Step 1: Write failing test**

```go
// system/netconf/dhcp_test.go
package netconf

import (
    "context"
    "testing"
    "time"
)

// stubDHCP simulates a DHCP lease delivery.
func TestDHCPAppliesLease(t *testing.T) {
    nl := &fakeNetlink{}
    // dhcpApply is the internal function that takes a lease and applies it.
    // We test it directly with a hard-coded lease struct.
    lease := dhcpLease{IP: "10.0.0.5/24", Gateway: "10.0.0.1", DNS: []string{"8.8.8.8"}}
    if err := applyLease(nl, "wlan0", lease); err != nil {
        t.Fatal(err)
    }
    if len(nl.addedAddrs) == 0 {
        t.Error("expected address to be applied")
    }
    if nl.addedAddrs[0] != "10.0.0.5/24" {
        t.Errorf("unexpected addr: %s", nl.addedAddrs[0])
    }
}
```

- [ ] **Step 2: Run test — verify fails**

```bash
cd system/netconf && go test ./... -run TestDHCPAppliesLease
```
Expected: compile error.

- [ ] **Step 3: Write `system/netconf/dhcp.go`**

```go
// system/netconf/dhcp.go — DHCP client using insomniacslk/dhcp (CGo-free)
package netconf

import (
    "context"
    "fmt"
    "net"
    "strings"

    "github.com/insomniacslk/dhcp/dhcpv4"
    "github.com/insomniacslk/dhcp/dhcpv4/nclient4"
)

// dhcpLease holds the result of a DHCP negotiation.
type dhcpLease struct {
    IP      string   // CIDR, e.g. "192.168.1.10/24"
    Gateway string   // e.g. "192.168.1.1"
    DNS     []string // e.g. ["8.8.8.8"]
}

// applyLease applies a lease to an interface via the netlinkClient.
func applyLease(nl netlinkClient, iface string, lease dhcpLease) error {
    return applyStatic(nl, iface, lease.IP, lease.Gateway)
}

// runDHCP runs a DHCP client goroutine for iface. It requests a lease, applies
// it, then exits. Re-call when the lease expires (caller handles renewal timer).
// Results are sent via the returned channel.
func runDHCP(ctx context.Context, nl netlinkClient, iface string) (dhcpLease, error) {
    client, err := nclient4.New(iface)
    if err != nil {
        return dhcpLease{}, fmt.Errorf("dhcp client %s: %w", iface, err)
    }
    defer client.Close()

    lease, err := client.Request(ctx)
    if err != nil {
        return dhcpLease{}, fmt.Errorf("dhcp request %s: %w", iface, err)
    }

    ip := lease.ACK.YourIPAddr
    mask := lease.ACK.SubnetMask()
    cidr := fmt.Sprintf("%s/%d", ip, maskBits(mask))

    var gw string
    if routers := lease.ACK.Options.Get(dhcpv4.OptionRouter); len(routers) >= 4 {
        gw = net.IP(routers[:4]).String()
    }

    var dns []string
    if dnsOpt := lease.ACK.Options.Get(dhcpv4.OptionDomainNameServer); len(dnsOpt) >= 4 {
        for i := 0; i+4 <= len(dnsOpt); i += 4 {
            dns = append(dns, net.IP(dnsOpt[i:i+4]).String())
        }
    }

    result := dhcpLease{IP: cidr, Gateway: gw, DNS: dns}
    if err := applyLease(nl, iface, result); err != nil {
        return dhcpLease{}, err
    }
    return result, nil
}

func maskBits(mask net.IPMask) int {
    ones, _ := mask.Size()
    return ones
}

// stringsContain checks if a string slice contains s.
func stringsContain(ss []string, s string) bool {
    for _, x := range ss {
        if strings.EqualFold(x, s) {
            return true
        }
    }
    return false
}
```

- [ ] **Step 4: Run tests**

```bash
cd system/netconf && go test ./... -v -run TestDHCPAppliesLease
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add system/netconf/dhcp.go system/netconf/dhcp_test.go
git commit -m "feat(netconf): DHCP client via insomniacslk/dhcp"
```

---

## Task 7: `system/netconf` — config.go + Manager (netconf.go)

**Files:**
- Create: `system/netconf/config.go`
- Create: `system/netconf/netconf.go`
- Create: `system/netconf/netconf_test.go`
- Create: `system/netconf/config_test.go`

- [ ] **Step 1: Write failing tests**

```go
// system/netconf/config_test.go
package netconf

import (
    "testing"
)

func TestConfigRoundtrip(t *testing.T) {
    dir := t.TempDir()
    cfg := &ifaceConfig{dir: dir}
    in := map[string]IfaceCfg{
        "wlan0": {Mode: ModeDHCP},
        "usb0":  {Mode: ModeStatic, IP: "10.0.0.1/24", Gateway: "10.0.0.1", DNS: []string{"8.8.8.8"}},
    }
    if err := cfg.write(in); err != nil {
        t.Fatal(err)
    }
    out, err := cfg.read()
    if err != nil {
        t.Fatal(err)
    }
    if out["usb0"].IP != "10.0.0.1/24" {
        t.Errorf("unexpected IP: %s", out["usb0"].IP)
    }
    if out["wlan0"].Mode != ModeDHCP {
        t.Errorf("unexpected mode: %s", out["wlan0"].Mode)
    }
}
```

```go
// system/netconf/netconf_test.go
package netconf

import (
    "strings"
    "testing"

    "github.com/vishvananda/netlink"
)

func TestListInterfaces_FiltersVirtual(t *testing.T) {
    nl := &fakeNetlink{}
    // Add physical + virtual interfaces
    for _, name := range []string{"wlan0", "usb0", "lo", "veth0", "docker0", "br-abc"} {
        la := netlink.NewLinkAttrs()
        la.Name = name
        nl.links = append(nl.links, &netlink.Dummy{LinkAttrs: la})
    }
    m := &Manager{nl: nl, cfg: &ifaceConfig{dir: t.TempDir()}}
    ifaces, err := m.ListInterfaces()
    if err != nil {
        t.Fatal(err)
    }
    for _, iface := range ifaces {
        if iface == "lo" || strings.HasPrefix(iface, "veth") || strings.HasPrefix(iface, "docker") || strings.HasPrefix(iface, "br-") {
            t.Errorf("filtered interface %q should not appear", iface)
        }
    }
    found := false
    for _, iface := range ifaces {
        if iface == "wlan0" {
            found = true
        }
    }
    if !found {
        t.Error("wlan0 should be in list")
    }
}

func TestApply_Static_Persists(t *testing.T) {
    nl := &fakeNetlink{}
    dir := t.TempDir()
    m := &Manager{nl: nl, cfg: &ifaceConfig{dir: dir}}
    cfg := IfaceCfg{Mode: ModeStatic, IP: "10.0.0.2/24", Gateway: "10.0.0.1"}
    if err := m.Apply("eth0", cfg); err != nil {
        t.Fatal(err)
    }
    stored, _ := m.cfg.read()
    if stored["eth0"].IP != "10.0.0.2/24" {
        t.Errorf("not persisted: %+v", stored)
    }
}
```

Add `"strings"` import to `netconf_test.go`.

- [ ] **Step 2: Write `system/netconf/config.go`**

```go
// system/netconf/config.go — read/write /perm/netconf/interfaces.json
package netconf

import (
    "encoding/json"
    "os"
    "path/filepath"
)

// Mode defines whether an interface uses DHCP or static IP.
type Mode string
const (
    ModeDHCP   Mode = "dhcp"
    ModeStatic Mode = "static"
)

// IfaceCfg holds the IP configuration for one interface.
type IfaceCfg struct {
    Mode    Mode     `json:"mode"`
    IP      string   `json:"ip,omitempty"`      // CIDR, static only
    Gateway string   `json:"gateway,omitempty"` // static only
    DNS     []string `json:"dns,omitempty"`     // static only
}

type ifaceConfig struct {
    dir string
}

func (c *ifaceConfig) path() string {
    return filepath.Join(c.dir, "interfaces.json")
}

func (c *ifaceConfig) read() (map[string]IfaceCfg, error) {
    data, err := os.ReadFile(c.path())
    if os.IsNotExist(err) {
        return map[string]IfaceCfg{}, nil
    }
    if err != nil {
        return nil, err
    }
    var m map[string]IfaceCfg
    return m, json.Unmarshal(data, &m)
}

func (c *ifaceConfig) write(m map[string]IfaceCfg) error {
    if err := os.MkdirAll(c.dir, 0755); err != nil {
        return err
    }
    data, err := json.MarshalIndent(m, "", "  ")
    if err != nil {
        return err
    }
    return os.WriteFile(c.path(), data, 0644)
}
```

- [ ] **Step 3: Write `system/netconf/netconf.go`**

```go
// system/netconf/netconf.go — per-interface IP configuration manager
package netconf

import (
    "context"
    "fmt"
    "net"
    "strings"
    "sync"
)

// IfaceStatus holds the runtime IP state of an interface.
type IfaceStatus struct {
    IP      string
    Gateway string
    Up      bool
}

// Manager configures network interfaces (DHCP or static) and persists config.
type Manager struct {
    nl  netlinkClient
    cfg *ifaceConfig
    mu  sync.Mutex // guards net.DefaultResolver writes
}

// New creates a Manager with configuration stored in confDir.
func New(confDir string) *Manager {
    return &Manager{
        nl:  &realNetlink{},
        cfg: &ifaceConfig{dir: confDir},
    }
}

// Start applies saved configuration for all known interfaces.
func (m *Manager) Start(ctx context.Context) error {
    saved, err := m.cfg.read()
    if err != nil {
        return fmt.Errorf("netconf load: %w", err)
    }
    for iface, cfg := range saved {
        if err := m.applyNow(iface, cfg); err != nil {
            // non-fatal per spec: log in caller
            _ = err
        }
    }
    return nil
}

// ListInterfaces returns physical/USB interfaces, excluding lo, veth*, docker*, br-*.
func (m *Manager) ListInterfaces() ([]string, error) {
    links, err := m.nl.LinkList()
    if err != nil {
        return nil, err
    }
    var result []string
    for _, l := range links {
        name := l.Attrs().Name
        if name == "lo" ||
            strings.HasPrefix(name, "veth") ||
            strings.HasPrefix(name, "docker") ||
            strings.HasPrefix(name, "br-") {
            continue
        }
        result = append(result, name)
    }
    return result, nil
}

// Get returns the saved configuration for an interface (defaults to DHCP).
func (m *Manager) Get(iface string) (IfaceCfg, error) {
    saved, err := m.cfg.read()
    if err != nil {
        return IfaceCfg{}, err
    }
    cfg, ok := saved[iface]
    if !ok {
        return IfaceCfg{Mode: ModeDHCP}, nil
    }
    return cfg, nil
}

// Apply configures an interface live and persists the configuration.
func (m *Manager) Apply(iface string, cfg IfaceCfg) error {
    if err := m.applyNow(iface, cfg); err != nil {
        return err
    }
    saved, err := m.cfg.read()
    if err != nil {
        return err
    }
    saved[iface] = cfg
    return m.cfg.write(saved)
}

// Status returns the current IP state for iface.
func (m *Manager) Status(iface string) (IfaceStatus, error) {
    link, err := m.nl.LinkByName(iface)
    if err != nil {
        return IfaceStatus{}, err
    }
    _ = link
    // IP state would be read from netlink AddrList — simplified for now.
    return IfaceStatus{Up: true}, nil
}

func (m *Manager) applyNow(iface string, cfg IfaceCfg) error {
    switch cfg.Mode {
    case ModeStatic:
        if err := applyStatic(m.nl, iface, cfg.IP, cfg.Gateway); err != nil {
            return err
        }
        if len(cfg.DNS) > 0 {
            m.setDNS(cfg.DNS)
        }
        return nil
    case ModeDHCP:
        // DHCP runs in a goroutine; Start just records the intent.
        // Actual DHCP is triggered by the caller after Start() returns.
        return nil
    default:
        return fmt.Errorf("unknown mode: %s", cfg.Mode)
    }
}

// setDNS overrides net.DefaultResolver to use the given DNS servers.
// Protected by mu so concurrent Apply calls don't race.
func (m *Manager) setDNS(servers []string) {
    m.mu.Lock()
    defer m.mu.Unlock()
    addrs := make([]string, len(servers))
    for i, s := range servers {
        addrs[i] = s + ":53"
    }
    net.DefaultResolver = &net.Resolver{
        PreferGo: true,
        Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
            d := net.Dialer{}
            return d.DialContext(ctx, "udp", addrs[0])
        },
    }
}
```

- [ ] **Step 4: Run tests**

```bash
cd system/netconf && go test ./... -v
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add system/netconf/config.go system/netconf/netconf.go system/netconf/netconf_test.go system/netconf/config_test.go
git commit -m "feat(netconf): Manager API (Apply, ListInterfaces, DNS override)"
```

---

## Task 8: UI — Signature changes (scene_config, home, helpers_test)

**Files:**
- Modify: `cmd/oioni/ui/scene_config.go`
- Modify: `cmd/oioni/ui/home.go`
- Modify: `cmd/oioni/ui/scene_helpers_test.go`

**Context:** `NewConfigScene` and `NewHomeScene` must accept `wifiMgr` and `netconfMgr`. The test table uses `func(*gui.Navigator) *gui.Scene` as type — update entries to wrap the new signatures in adapter lambdas so the table type stays compatible.

The actual WiFi/Network scene implementations come in later tasks. In this task, `NewConfigScene` just passes nil-safe stubs of the new scenes (use `gui.NewLabel("(coming soon)")` temporarily — the real scenes are wired in Tasks 9–13).

- [ ] **Step 1: Write failing test (compile check)**

The existing `TestCategoryScene_SingleTopLevelWidget` and `TestCategoryScene_Title` will break after the signature change. Run them first to see the failure:

```bash
cd cmd/oioni && go test ./ui/... -v -run TestCategoryScene
```
Expected: currently PASS (will FAIL after Step 2).

- [ ] **Step 2: Update `cmd/oioni/ui/scene_config.go`**

Add imports for the two new packages. Use interface types to avoid hard dependency on the concrete manager types (so nil is valid in tests):

```go
// cmd/oioni/ui/scene_config.go — Config category scene
package ui

import (
    "github.com/oioio-space/oioni/ui/gui"
    wifi "github.com/oioio-space/oioni/system/wifi"
    netconf "github.com/oioio-space/oioni/system/netconf"
)

// NewConfigScene builds the Config category scene: WiFi + Network items.
func NewConfigScene(nav *gui.Navigator, wifiMgr *wifi.Manager, netconfMgr *netconf.Manager) *gui.Scene {
    items := []gui.ListItem{
        &configListItem{name: "WiFi", onTap: func() {
            nav.Dispatch(func() { nav.Push(NewWifiScene(nav, wifiMgr)) }) //nolint:errcheck
        }},
        &configListItem{name: "Network", onTap: func() {
            nav.Dispatch(func() { nav.Push(NewNetworkScene(nav, netconfMgr)) }) //nolint:errcheck
        }},
    }
    list := gui.NewScrollableList(items, 40)
    return newCategoryScene(nav, "Config", list)
}

// configListItem renders a single-line text menu entry.
type configListItem struct {
    name  string
    onTap func()
}

func (c *configListItem) Draw(cv *canvas.Canvas, bounds image.Rectangle) {
    f := canvas.EmbeddedFont(12)
    if f != nil {
        ty := bounds.Min.Y + (bounds.Dy()-f.LineHeight())/2
        cv.DrawText(bounds.Min.X+6, ty, c.name, f, canvas.Black)
    }
}

func (c *configListItem) OnTap() { c.onTap() }
```

**Important:** the `NewWifiScene` and `NewNetworkScene` functions don't exist yet. Add placeholder stubs at the bottom of the same file (will be replaced in Tasks 9 and 12):

```go
// Stubs — replaced in scene_wifi.go and scene_network.go
func NewWifiScene(nav *gui.Navigator, mgr *wifi.Manager) *gui.Scene {
    return newCategoryScene(nav, "WiFi", gui.NewLabel("(coming soon)"))
}
func NewNetworkScene(nav *gui.Navigator, mgr *netconf.Manager) *gui.Scene {
    return newCategoryScene(nav, "Network", gui.NewLabel("(coming soon)"))
}
```

Add the missing imports to `scene_config.go`:

```go
import (
    "image"
    "github.com/oioio-space/oioni/ui/canvas"
    "github.com/oioio-space/oioni/ui/gui"
    "github.com/oioio-space/oioni/ui/gui/font"
    wifi "github.com/oioio-space/oioni/system/wifi"
    netconf "github.com/oioio-space/oioni/system/netconf"
)
```

**Note on fonts:** use the same font package as other scenes. Check existing scene files for the import pattern (e.g. `ui/gui` exposes font rendering through canvas directly — look at how `homeListItem.Draw` works in `menu.go`).

- [ ] **Step 3: Update `cmd/oioni/ui/home.go`**

Change `NewHomeScene` to accept and forward managers:

```go
func NewHomeScene(nav *gui.Navigator, wifiMgr *wifi.Manager, netconfMgr *netconf.Manager) (*gui.Scene, *gui.NetworkStatusBar) {
    nsb := gui.NewNetworkStatusBar(nav)
    items := []gui.ListItem{
        &homeListItem{name: "Config", icon: Icons.Config, onTap: func() {
            nav.Dispatch(func() { nav.Push(NewConfigScene(nav, wifiMgr, netconfMgr)) }) //nolint:errcheck
        }},
        // ... rest unchanged (System, Attack, DFIR, Info)
    }
    // ... rest of function unchanged
}
```

Add imports for the two new packages (same as scene_config.go).

- [ ] **Step 4: Update `cmd/oioni/ui/scene_helpers_test.go`**

The table entries for `NewConfigScene` must wrap the call in a nil-manager lambda:

```go
// In TestCategoryScene_SingleTopLevelWidget:
{"Config", func(nav *gui.Navigator) *gui.Scene { return NewConfigScene(nav, nil, nil) }},

// In TestCategoryScene_Title:
{func(nav *gui.Navigator) *gui.Scene { return NewConfigScene(nav, nil, nil) }, "Config"},
```

Leave all other scene entries (`System`, `Attack`, `DFIR`, `Info`) unchanged — their signatures did not change.

- [ ] **Step 5: Run tests**

```bash
cd cmd/oioni && go test ./ui/... -v
```
Expected: PASS (nil managers handled gracefully in stubs).

- [ ] **Step 6: Commit**

```bash
git add cmd/oioni/ui/scene_config.go cmd/oioni/ui/home.go cmd/oioni/ui/scene_helpers_test.go
git commit -m "feat(ui): thread wifiMgr+netconfMgr through NewHomeScene+NewConfigScene"
```

---

## Task 9: UI — `scene_wifi.go`

**Files:**
- Create: `cmd/oioni/ui/scene_wifi.go`
- Remove the stub `NewWifiScene` from `scene_config.go` (replace with the real implementation)

**Context:** The WiFi scene is a `ScrollableList` with row 0 = WiFi toggle, rows 1–N = network entries (saved first with ★, then scan results). Extra sidebar button: Scan (becomes "…" while scanning). Uses `newCategoryScene` with `withExtraSidebarBtn`.

- [ ] **Step 1: Write failing UI test**

```go
// cmd/oioni/ui/scene_wifi_test.go
package ui

import (
    "testing"
    "github.com/oioio-space/oioni/ui/gui"
)

func TestWifiScene_Structure(t *testing.T) {
    nav := gui.NewNavigator(fakeDisplay{})
    s := NewWifiScene(nav, nil)
    if len(s.Widgets) < 1 {
        t.Fatal("expected at least 1 widget")
    }
    if s.Title != "WiFi" {
        t.Errorf("expected title WiFi, got %q", s.Title)
    }
}
```

- [ ] **Step 2: Run test — verify fails**

```bash
cd cmd/oioni && go test ./ui/... -run TestWifiScene
```
Expected: FAIL (stub returns "coming soon").

- [ ] **Step 3: Write `cmd/oioni/ui/scene_wifi.go`**

```go
// cmd/oioni/ui/scene_wifi.go — WiFi management scene
package ui

import (
    "fmt"
    "image"
    "time"

    "github.com/oioio-space/oioni/ui/canvas"
    "github.com/oioio-space/oioni/ui/gui"
    wifi "github.com/oioio-space/oioni/system/wifi"
)

// NewWifiScene builds the WiFi scene: toggle + network list + scan button.
func NewWifiScene(nav *gui.Navigator, mgr *wifi.Manager) *gui.Scene {
    var items []gui.ListItem

    // Row 0: WiFi enable/disable toggle
    toggle := gui.NewToggle(true) // assume enabled by default
    items = append(items, &wifiToggleItem{
        toggle: toggle,
        onTap: func() {
            if mgr == nil {
                return
            }
            _ = mgr.SetEnabled(!toggle.On)
            toggle.On = !toggle.On
            toggle.SetDirty()
        },
    })

    list := gui.NewScrollableList(items, 36)

    var scanBtn gui.SidebarButton
    onScan := func() {
        if mgr == nil {
            return
        }
        go func() {
            nets, err := mgr.Scan()
            nav.Dispatch(func() {
                if err != nil {
                    return
                }
                // Rebuild list: toggle row first, then networks
                newItems := []gui.ListItem{items[0]}
                for _, n := range nets {
                    net := n // capture
                    newItems = append(newItems, &wifiNetItem{
                        net: net,
                        onTap: func() {
                            if net.Saved {
                                go func() {
                                    _ = mgr.Connect(net.SSID, "", false)
                                    nav.Dispatch(func() {
                                        nav.Push(newConnectingScene(nav, mgr, net.SSID)) //nolint:errcheck
                                    })
                                }()
                            } else {
                                nav.Push(newPasswordScene(nav, mgr, net.SSID)) //nolint:errcheck
                            }
                        },
                    })
                }
                list.SetItems(newItems)
                nav.RequestRender()
            })
        }()
    }

    return newCategoryScene(nav, "WiFi", list,
        withExtraSidebarBtn(Icons.Scan, onScan),
    )
}

// wifiToggleItem renders the WiFi enable/disable row.
type wifiToggleItem struct {
    toggle *gui.Toggle
    onTap  func()
}

func (w *wifiToggleItem) Draw(cv *canvas.Canvas, bounds image.Rectangle) {
    f := canvas.EmbeddedFont(12)
    if f != nil {
        ty := bounds.Min.Y + (bounds.Dy()-f.LineHeight())/2
        cv.DrawText(bounds.Min.X+6, ty, "WiFi", f, canvas.Black)
    }
    toggleBounds := image.Rect(bounds.Max.X-46, bounds.Min.Y+4, bounds.Max.X-4, bounds.Max.Y-4)
    w.toggle.SetBounds(toggleBounds)
    w.toggle.Draw(cv)
}

func (w *wifiToggleItem) OnTap() { w.onTap() }

// wifiNetItem renders a single network row: SSID + signal bars + lock icon.
type wifiNetItem struct {
    net   wifi.Network
    onTap func()
}

func (w *wifiNetItem) Draw(cv *canvas.Canvas, bounds image.Rectangle) {
    name := w.net.SSID
    if w.net.Saved {
        name = "★ " + name
    }
    if f := canvas.EmbeddedFont(12); f != nil {
        ty := bounds.Min.Y + (bounds.Dy()-f.LineHeight())/2
        cv.DrawText(bounds.Min.X+6, ty, name, f, canvas.Black)
    }
    // Signal bars (3-level): right side
    drawSignalBars(cv, bounds, w.net.Signal)
    // Lock icon for secured networks
    if w.net.Security != "Open" {
        drawLockIcon(cv, bounds)
    }
}

func (w *wifiNetItem) OnTap() { w.onTap() }

// drawSignalBars renders 3 vertical bars scaled by signal strength.
// Signal is dBm: > -60 = strong, -60 to -75 = medium, < -75 = weak.
func drawSignalBars(cv *canvas.Canvas, bounds image.Rectangle, signal int) {
    x := bounds.Max.X - 20
    y := bounds.Max.Y - 4
    bars := 1
    if signal > -75 {
        bars = 2
    }
    if signal > -60 {
        bars = 3
    }
    for i := 0; i < 3; i++ {
        h := (i + 1) * 4
        r := image.Rect(x+i*5, y-h, x+i*5+3, y)
        if i < bars {
            cv.DrawRect(r, canvas.Black, true)
        } else {
            cv.DrawRect(r, canvas.Black, false)
        }
    }
}

// drawLockIcon draws a small lock symbol at the right side of bounds.
func drawLockIcon(cv *canvas.Canvas, bounds image.Rectangle) {
    // Simple: draw a small filled rectangle at far right
    x := bounds.Max.X - 36
    y := bounds.Min.Y + (bounds.Dy()-8)/2
    cv.DrawRect(image.Rect(x, y, x+6, y+8), canvas.Black, true)
}
```

- [ ] **Step 3b: Add `SetItems` to `ui/gui/widget_scrolllist.go`** (required before Step 3 compiles)

Open `/home/oioio/Documents/GolandProjects/oioni/ui/gui/widget_scrolllist.go` and add after the `ScrollDown` function:

```go
// SetItems replaces the list contents and resets scroll to top.
func (l *ScrollableList) SetItems(items []ListItem) {
    l.items = items
    l.offset = 0
    l.SetDirty()
}
```

- [ ] **Step 4: Remove stub from `scene_config.go`**

Delete the `NewWifiScene` stub at the bottom of `scene_config.go`.

- [ ] **Step 5: Run tests**

```bash
cd cmd/oioni && go test ./ui/... -v -run TestWifiScene
```
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add cmd/oioni/ui/scene_wifi.go cmd/oioni/ui/scene_config.go ui/gui/widget_scrolllist.go
git commit -m "feat(ui): WiFi scene with toggle, network list, and scan button"
```

---

## Task 10: UI — `scene_wifi_password.go`

**Files:**
- Create: `cmd/oioni/ui/scene_wifi_password.go`

**Context:** Shown when tapping a new (unsaved) network. Contains SSID label, a tap-to-edit PSK field (calls `gui.ShowTextInput`), a "Mémoriser" checkbox, and a Connect sidebar button.

- [ ] **Step 1: Write failing test**

```go
// cmd/oioni/ui/scene_wifi_password_test.go
package ui

import (
    "testing"
    "github.com/oioio-space/oioni/ui/gui"
)

func TestPasswordScene_Structure(t *testing.T) {
    nav := gui.NewNavigator(fakeDisplay{})
    s := newPasswordScene(nav, nil, "TestNet")
    if s.Title != "WiFi" {
        t.Errorf("expected title WiFi, got %q", s.Title)
    }
    if len(s.Widgets) < 1 {
        t.Fatal("expected widgets")
    }
}
```

- [ ] **Step 2: Write `cmd/oioni/ui/scene_wifi_password.go`**

```go
// cmd/oioni/ui/scene_wifi_password.go — PSK entry scene for WiFi
package ui

import (
    "github.com/oioio-space/oioni/ui/gui"
    wifi "github.com/oioio-space/oioni/system/wifi"
)

// newPasswordScene builds the password entry scene for a new (unsaved) network.
func newPasswordScene(nav *gui.Navigator, mgr *wifi.Manager, ssid string) *gui.Scene {
    var psk string
    save := gui.NewCheckbox("Mémoriser", false)

    // PSK field: a label that, when tapped, opens ShowTextInput
    pskItem := &pskFieldItem{
        label: "Mot de passe",
        onTap: func() {
            gui.ShowTextInput(nav, "Mot de passe WiFi", 64, func(entered string) {
                psk = entered
            })
        },
    }

    list := gui.NewScrollableList([]gui.ListItem{
        &ssidLabelItem{ssid: ssid},
        pskItem,
        &checkboxItem{cb: save},
    }, 36)

    onConnect := func() {
        if mgr == nil {
            return
        }
        capturedPsk := psk
        capturedSave := save.Checked
        go func() {
            _ = mgr.Connect(ssid, capturedPsk, capturedSave)
            nav.Dispatch(func() {
                nav.Push(newConnectingScene(nav, mgr, ssid)) //nolint:errcheck
            })
        }()
    }

    return newCategoryScene(nav, "WiFi", list,
        withExtraSidebarBtn(Icons.Connect, onConnect),
    )
}

type ssidLabelItem struct{ ssid string }
func (s *ssidLabelItem) Draw(cv *canvas.Canvas, b image.Rectangle) {
    if f := canvas.EmbeddedFont(12); f != nil {
        cv.DrawText(b.Min.X+6, b.Min.Y+(b.Dy()-f.LineHeight())/2, s.ssid, f, canvas.Black)
    }
}
func (s *ssidLabelItem) OnTap() {}

type pskFieldItem struct {
    label string
    onTap func()
}
func (p *pskFieldItem) Draw(cv *canvas.Canvas, b image.Rectangle) {
    if f := canvas.EmbeddedFont(12); f != nil {
        cv.DrawText(b.Min.X+6, b.Min.Y+(b.Dy()-f.LineHeight())/2, p.label+" >", f, canvas.Black)
    }
}
func (p *pskFieldItem) OnTap() { p.onTap() }

type checkboxItem struct{ cb *gui.Checkbox }
func (c *checkboxItem) Draw(cv *canvas.Canvas, b image.Rectangle) {
    c.cb.SetBounds(b)
    c.cb.Draw(cv)
}
// OnTap toggles the checkbox directly (HandleTouch requires the touch package;
// toggle the Checked field instead)
func (c *checkboxItem) OnTap() { c.cb.Checked = !c.cb.Checked; c.cb.SetDirty() }
```

Required imports for `scene_wifi_password.go`:
```go
import (
    "image"
    "github.com/oioio-space/oioni/ui/canvas"
    "github.com/oioio-space/oioni/ui/gui"
    wifi "github.com/oioio-space/oioni/system/wifi"
)
```

**Note:** `Icons.Connect` and `Icons.Scan` must exist in `icons.go`. Add them if missing (32×32px icon files in `cmd/oioni/ui/icons/`). If no icon exists yet for these actions, reuse `Icons.Back` as a placeholder and note the TODO.

- [ ] **Step 3: Run tests**

```bash
cd cmd/oioni && go test ./ui/... -v -run TestPasswordScene
```
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add cmd/oioni/ui/scene_wifi_password.go
git commit -m "feat(ui): WiFi password entry scene"
```

---

## Task 11: UI — `scene_wifi_connecting.go`

**Files:**
- Create: `cmd/oioni/ui/scene_wifi_connecting.go`

**Context:** Polls `wifi.Status()` every second via a goroutine + cancel channel. Updates a `Label` widget with connection state. Cancels on `Scene.OnLeave`.

- [ ] **Step 1: Write failing test**

```go
// cmd/oioni/ui/scene_wifi_connecting_test.go
package ui

import (
    "testing"
    "github.com/oioio-space/oioni/ui/gui"
)

func TestConnectingScene_Structure(t *testing.T) {
    nav := gui.NewNavigator(fakeDisplay{})
    s := newConnectingScene(nav, nil, "TestNet")
    if s.Title != "WiFi" {
        t.Errorf("expected title WiFi, got %q", s.Title)
    }
    if s.OnLeave == nil {
        t.Error("OnLeave must be set to cancel the polling goroutine")
    }
}
```

- [ ] **Step 2: Write `cmd/oioni/ui/scene_wifi_connecting.go`**

```go
// cmd/oioni/ui/scene_wifi_connecting.go — connection status polling scene
package ui

import (
    "fmt"
    "time"

    "github.com/oioio-space/oioni/ui/gui"
    wifi "github.com/oioio-space/oioni/system/wifi"
)

// newConnectingScene builds the WiFi connecting/status scene.
// It polls wifi.Status() every second in a goroutine.
// The goroutine is cancelled when Scene.OnLeave fires.
func newConnectingScene(nav *gui.Navigator, mgr *wifi.Manager, ssid string) *gui.Scene {
    statusLabel := gui.NewLabel(fmt.Sprintf("Connexion à %s…", ssid))
    cancel := make(chan struct{})

    var s *gui.Scene
    s = newCategoryScene(nav, "WiFi", statusLabel)
    s.OnLeave = func() {
        close(cancel)
    }
    s.OnEnter = func() {
        if mgr == nil {
            return
        }
        go func() {
            for {
                st, err := mgr.Status()
                nav.Dispatch(func() {
                    if err != nil {
                        statusLabel.SetText("Erreur de connexion")
                    } else {
                        switch st.State {
                        case "COMPLETED":
                            statusLabel.SetText(fmt.Sprintf("Connecté — %s", st.SSID))
                        case "ASSOCIATING", "AUTHENTICATING":
                            statusLabel.SetText(fmt.Sprintf("Connexion à %s…", ssid))
                        case "DISCONNECTED", "INACTIVE":
                            statusLabel.SetText("Échec de connexion")
                        default:
                            statusLabel.SetText(st.State)
                        }
                    }
                    nav.RequestRender()
                })
                select {
                case <-cancel:
                    return
                case <-time.After(time.Second):
                }
            }
        }()
    }
    return s
}
```

**Note:** `gui.NewLabel` must have a `SetText(string)` method. Check `widgets.go`. If missing, add it:
```go
func (l *Label) SetText(text string) { l.text = text; l.SetDirty() }
```

- [ ] **Step 3: Run tests**

```bash
cd cmd/oioni && go test ./ui/... -v -run TestConnectingScene
```
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add cmd/oioni/ui/scene_wifi_connecting.go
git commit -m "feat(ui): WiFi connecting scene with polling goroutine"
```

---

## Task 12: UI — `scene_network.go`

**Files:**
- Create: `cmd/oioni/ui/scene_network.go`
- Remove the stub `NewNetworkScene` from `scene_config.go`

**Context:** Lists physical interfaces from `netconf.ListInterfaces()`. Tapping an interface pushes `newIPConfigScene`.

- [ ] **Step 1: Write failing test**

```go
// cmd/oioni/ui/scene_network_test.go
package ui

import (
    "testing"
    "github.com/oioio-space/oioni/ui/gui"
)

func TestNetworkScene_Structure(t *testing.T) {
    nav := gui.NewNavigator(fakeDisplay{})
    s := NewNetworkScene(nav, nil)
    if s.Title != "Network" {
        t.Errorf("expected title Network, got %q", s.Title)
    }
}
```

- [ ] **Step 2: Write `cmd/oioni/ui/scene_network.go`**

```go
// cmd/oioni/ui/scene_network.go — network interface list scene
package ui

import (
    "image"

    "github.com/oioio-space/oioni/ui/canvas"
    "github.com/oioio-space/oioni/ui/gui"
    netconf "github.com/oioio-space/oioni/system/netconf"
)

// NewNetworkScene lists physical interfaces; tapping one opens IP Config.
func NewNetworkScene(nav *gui.Navigator, mgr *netconf.Manager) *gui.Scene {
    var ifaces []string
    if mgr != nil {
        ifaces, _ = mgr.ListInterfaces()
    }

    items := make([]gui.ListItem, len(ifaces))
    for i, name := range ifaces {
        name := name // capture
        var ip string
        if mgr != nil {
            if st, err := mgr.Status(name); err == nil && st.IP != "" {
                ip = st.IP
            }
        }
        items[i] = &ifaceListItem{
            name: name,
            ip:   ip,
            onTap: func() {
                nav.Push(newIPConfigScene(nav, mgr, name)) //nolint:errcheck
            },
        }
    }

    list := gui.NewScrollableList(items, 36)
    return newCategoryScene(nav, "Network", list)
}

type ifaceListItem struct {
    name  string
    ip    string
    onTap func()
}

func (f *ifaceListItem) Draw(cv *canvas.Canvas, b image.Rectangle) {
    text := f.name
    if f.ip != "" {
        text += " " + f.ip
    }
    if f := canvas.EmbeddedFont(12); f != nil {
        cv.DrawText(b.Min.X+6, b.Min.Y+(b.Dy()-f.LineHeight())/2, text, f, canvas.Black)
    }
}

func (f *ifaceListItem) OnTap() { f.onTap() }
```

- [ ] **Step 3: Remove stub from `scene_config.go`**

Delete the `NewNetworkScene` stub at the bottom of `scene_config.go`.

- [ ] **Step 4: Run tests**

```bash
cd cmd/oioni && go test ./ui/... -v -run TestNetworkScene
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/oioni/ui/scene_network.go cmd/oioni/ui/scene_config.go
git commit -m "feat(ui): Network interface list scene"
```

---

## Task 13: UI — `widget_modesel.go` + `scene_network_iface.go`

**Files:**
- Create: `cmd/oioni/ui/widget_modesel.go`
- Create: `cmd/oioni/ui/scene_network_iface.go`

**Context:** IP Config scene. The mode selector is a 44px-tall widget with two buttons (DHCP | Static) drawn on canvas. Tapping a button changes the active mode. Below (60px): IP/Gateway/DNS field rows (tap → `gui.ShowTextInput`). Save sidebar button.

- [ ] **Step 1: Write failing test**

```go
// cmd/oioni/ui/scene_network_iface_test.go
package ui

import (
    "testing"
    "github.com/oioio-space/oioni/ui/gui"
    "github.com/oioio-space/oioni/drivers/touch"
)

func TestModeSelector_TapSwitches(t *testing.T) {
    sel := newModeSelector(modeDHCP)
    if sel.Mode() != modeDHCP {
        t.Error("initial mode should be DHCP")
    }
    // Simulate tap on the Static button (right half)
    sel.SetBounds(image.Rect(0, 0, 150, 44))
    sel.HandleTouch(touch.TouchPoint{X: 100, Y: 22})
    if sel.Mode() != modeStatic {
        t.Error("expected Static after tapping right side")
    }
}

func TestIPConfigScene_Structure(t *testing.T) {
    nav := gui.NewNavigator(fakeDisplay{})
    s := newIPConfigScene(nav, nil, "wlan0")
    if s.Title != "Network" {
        t.Errorf("expected title Network, got %q", s.Title)
    }
    // The mode selector must be in Scene.Widgets for touch routing
    if len(s.Widgets) < 2 {
        t.Errorf("expected mode selector in Widgets, got %d", len(s.Widgets))
    }
}
```

- [ ] **Step 2: Write `cmd/oioni/ui/widget_modesel.go`**

```go
// cmd/oioni/ui/widget_modesel.go — two-button DHCP/Static mode selector
package ui

import (
    "image"

    "github.com/oioio-space/oioni/drivers/touch"
    "github.com/oioio-space/oioni/ui/canvas"
    "github.com/oioio-space/oioni/ui/gui"
)

type ipMode int

const (
    modeDHCP   ipMode = iota
    modeStatic ipMode = iota
)

// modeSelector is a 44px-tall two-button widget for selecting DHCP or Static.
type modeSelector struct {
    gui.BaseWidget
    mode    ipMode
    onChange func(ipMode)
}

func newModeSelector(initial ipMode) *modeSelector {
    return &modeSelector{mode: initial}
}

func (m *modeSelector) Mode() ipMode { return m.mode }

func (m *modeSelector) SetOnChange(fn func(ipMode)) { m.onChange = fn }

func (m *modeSelector) HandleTouch(pt touch.TouchPoint) bool {
    b := m.Bounds()
    mid := b.Min.X + b.Dx()/2
    if int(pt.X) < mid {
        m.mode = modeDHCP
    } else {
        m.mode = modeStatic
    }
    m.SetDirty()
    if m.onChange != nil {
        m.onChange(m.mode)
    }
    return true
}

func (m *modeSelector) Draw(cv *canvas.Canvas) {
    b := m.Bounds()
    mid := b.Min.X + b.Dx()/2

    left := image.Rect(b.Min.X, b.Min.Y, mid, b.Max.Y)
    right := image.Rect(mid, b.Min.Y, b.Max.X, b.Max.Y)

    // Active mode: filled (inverted). Inactive: outlined.
    if m.mode == modeDHCP {
        cv.DrawRect(left, canvas.Black, true)
        cv.DrawRect(right, canvas.White, true)
        cv.DrawRect(right, canvas.Black, false)
    } else {
        cv.DrawRect(left, canvas.White, true)
        cv.DrawRect(left, canvas.Black, false)
        cv.DrawRect(right, canvas.Black, true)
    }

    // Labels — use canvas.EmbeddedFont (the only public font API in this codebase)
    f := canvas.EmbeddedFont(12)
    if f == nil {
        return
    }
    // Approximate centering: 6px left margin within each half
    drawLabel := func(rect image.Rectangle, text string, fg canvas.Color) {
        x := rect.Min.X + 6
        y := rect.Min.Y + (rect.Dy()-f.LineHeight())/2
        cv.DrawText(x, y, text, f, fg)
    }
    if m.mode == modeDHCP {
        drawLabel(left, "DHCP", canvas.White)
        drawLabel(right, "Static", canvas.Black)
    } else {
        drawLabel(left, "DHCP", canvas.Black)
        drawLabel(right, "Static", canvas.White)
    }
}
```

**Imports for `widget_modesel.go`:** `image`, `github.com/oioio-space/oioni/drivers/touch`, `github.com/oioio-space/oioni/ui/canvas`, `github.com/oioio-space/oioni/ui/gui`. Do NOT import a `font` sub-package — it does not exist. Font rendering uses `canvas.EmbeddedFont(12)` as shown above.

- [ ] **Step 3: Write `cmd/oioni/ui/scene_network_iface.go`**

```go
// cmd/oioni/ui/scene_network_iface.go — IP configuration scene for one interface
package ui

import (
    "image"

    "github.com/oioio-space/oioni/ui/canvas"
    "github.com/oioio-space/oioni/ui/gui"
    netconf "github.com/oioio-space/oioni/system/netconf"
)

// newIPConfigScene builds the IP Config scene for a single interface.
func newIPConfigScene(nav *gui.Navigator, mgr *netconf.Manager, iface string) *gui.Scene {
    // Load current config
    var current netconf.IfaceCfg
    if mgr != nil {
        current, _ = mgr.Get(iface)
    }

    initialMode := modeDHCP
    if current.Mode == netconf.ModeStatic {
        initialMode = modeStatic
    }

    modeSel := newModeSelector(initialMode)

    // Editable fields (static mode only)
    ipVal := current.IP
    gwVal := current.Gateway
    dnsVal := ""
    if len(current.DNS) > 0 {
        dnsVal = current.DNS[0]
    }

    makeFieldItem := func(label string, valPtr *string) *fieldItem {
        return &fieldItem{
            label: label,
            valPtr: valPtr,
            onTap: func() {
                gui.ShowTextInput(nav, label, 40, func(v string) {
                    *valPtr = v
                })
            },
        }
    }

    ipItem  := makeFieldItem("IP (CIDR)", &ipVal)
    gwItem  := makeFieldItem("Passerelle", &gwVal)
    dnsItem := makeFieldItem("DNS", &dnsVal)

    list := gui.NewScrollableList([]gui.ListItem{ipItem, gwItem, dnsItem}, 34)

    onSave := func() {
        if mgr == nil {
            return
        }
        cfg := netconf.IfaceCfg{Mode: netconf.ModeDHCP}
        if modeSel.Mode() == modeStatic {
            cfg = netconf.IfaceCfg{
                Mode:    netconf.ModeStatic,
                IP:      ipVal,
                Gateway: gwVal,
            }
            if dnsVal != "" {
                cfg.DNS = []string{dnsVal}
            }
        }
        _ = mgr.Apply(iface, cfg)
        nav.Pop() //nolint:errcheck
    }

    s := newCategoryScene(nav, "Network", list,
        withExtraSidebarBtn(Icons.Save, onSave),
    )

    // The modeSelector must be in Scene.Widgets at top level for touch routing.
    // It also needs SetBounds to match its visual position (top 44px of content area).
    // The layout places the root widget as s.Widgets[0]; we append modeSel as [1].
    modeSel.SetBounds(image.Rect(0, 18, 206, 62)) // below 18px navbar, 44px tall
    s.Widgets = append(s.Widgets, modeSel)

    return s
}

type fieldItem struct {
    label  string
    valPtr *string
    onTap  func()
}

func (f *fieldItem) Draw(cv *canvas.Canvas, b image.Rectangle) {
    val := *f.valPtr
    if val == "" {
        val = "—"
    }
    text := f.label + ": " + val
    if f := canvas.EmbeddedFont(12); f != nil {
        cv.DrawText(b.Min.X+6, b.Min.Y+(b.Dy()-f.LineHeight())/2, text, f, canvas.Black)
    }
}

func (f *fieldItem) OnTap() { f.onTap() }
```

**Note:** `Icons.Save` needs to exist in `icons.go`. Add if missing (or use `Icons.Back` as placeholder).

- [ ] **Step 4: Run tests**

```bash
cd cmd/oioni && go test ./ui/... -v -run "TestModeSelector|TestIPConfig"
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/oioni/ui/widget_modesel.go cmd/oioni/ui/scene_network_iface.go
git commit -m "feat(ui): IP Config scene with DHCP/Static mode selector"
```

---

## Task 14: Boot integration + gokrazy config + Makefile

**Files:**
- Modify: `cmd/oioni/epaper.go`
- Modify: `oioio/config.json`
- Modify: `Makefile`
- Create: `system/wifi/build/Dockerfile`
- Create: `system/wifi/bin/.gitkeep`

**Context:** Wire managers into `startEPaper`. Update gokrazy config to remove `gokrazy/wifi` and add `wpa_supplicant` via `ExtraFilePaths`. Add `build-wifi-bins` Makefile target.

- [ ] **Step 1: Update `cmd/oioni/epaper.go`**

```go
// Add to imports:
wifi "github.com/oioio-space/oioni/system/wifi"
netconf "github.com/oioio-space/oioni/system/netconf"

// Modify startEPaper signature to accept ctx only; create managers internally:
func startEPaper(ctx context.Context) *epaperState {
    // ... existing display + touch setup unchanged ...

    netconfMgr := netconf.New("/perm/netconf")
    if err := netconfMgr.Start(ctx); err != nil {
        log.Printf("netconf: %v", err) // non-fatal
    }

    wifiMgr := wifi.New(wifi.Config{
        WpaSupplicantBin: "/user/wpa_supplicant",
        ConfDir:          "/perm/wifi",
        CtrlDir:          "/var/run/wpa_supplicant",
        Iface:            "wlan0",
    })
    if err := wifiMgr.Start(ctx); err != nil {
        log.Printf("wifi: %v", err) // non-fatal
    }

    nav := gui.NewNavigatorWithIdle(d, idleTimeout)
    home, nsb := oioniui.NewHomeScene(nav, wifiMgr, netconfMgr) // pass managers
    // ... rest unchanged ...
}
```

- [ ] **Step 2: Update `oioio/config.json`**

a) Remove `"github.com/gokrazy/wifi"` from the `Packages` array.

b) Remove the entire `"github.com/gokrazy/wifi"` key from `PackageConfig`.

c) Add to `PackageConfig["github.com/oioio-space/oioni/cmd/oioni"]` under `ExtraFilePaths`:
```json
"/user/wpa_supplicant": "system/wifi/bin/wpa_supplicant"
```

**Note:** `oioio/config.json` has skip-worktree set (per memory). Edit carefully:
```bash
git update-index --no-skip-worktree oioio/config.json
# edit the file
git update-index --skip-worktree oioio/config.json
```

- [ ] **Step 3: Add `build-wifi-bins` to Makefile**

```makefile
build-wifi-bins: ## Compile wpa_supplicant static ARM64 binary for system/wifi
	podman build --platform linux/arm64 \
	    --output type=local,dest=system/wifi/bin \
	    system/wifi/build/
	@echo "Binary generated in system/wifi/bin/:"
	@ls -lh system/wifi/bin/wpa_supplicant 2>/dev/null || echo "(not found — check Dockerfile)"
	@file system/wifi/bin/wpa_supplicant 2>/dev/null || true
```

Also update `build-all`:
```makefile
build-all: build-modules build-imgvol-bins build-wifi-bins build
```

- [ ] **Step 4: Create `system/wifi/build/Dockerfile`**

```dockerfile
# system/wifi/build/Dockerfile — builds static wpa_supplicant for ARM64
# Output: /out/wpa_supplicant (static ARM64 ELF)
FROM --platform=linux/arm64 alpine:3.21 AS builder

RUN apk add --no-cache \
    build-base \
    openssl-dev \
    openssl-libs-static \
    libnl3-dev \
    libnl3-static \
    dbus-dev \
    linux-headers

ARG WPA_VERSION=2.11
RUN wget -q https://w1.fi/releases/wpa_supplicant-${WPA_VERSION}.tar.gz \
    && tar xf wpa_supplicant-${WPA_VERSION}.tar.gz

WORKDIR /wpa_supplicant-${WPA_VERSION}/wpa_supplicant

COPY defconfig .config

RUN make -j$(nproc) LDFLAGS="-static" CC="gcc" \
    && strip wpa_supplicant

FROM scratch
COPY --from=builder /wpa_supplicant-${WPA_VERSION}/wpa_supplicant/wpa_supplicant /wpa_supplicant
```

Create `system/wifi/build/defconfig`:
```
CONFIG_DRIVER_NL80211=y
CONFIG_DRIVER_WEXT=y
CONFIG_WPS=y
CONFIG_EAP_PEAP=y
CONFIG_EAP_TTLS=y
CONFIG_EAP_TLS=y
CONFIG_EAP_MSCHAPV2=y
CONFIG_CTRL_IFACE=y
CONFIG_CTRL_IFACE_UNIX=y
CONFIG_BACKEND=file
CONFIG_IEEE8021X_EAPOL=y
CONFIG_CRYPTO=openssl
CONFIG_TLS=openssl
CONFIG_IPV6=y
```

- [ ] **Step 5: Create placeholder and verify build**

```bash
touch system/wifi/bin/.gitkeep
```

Verify the whole project still compiles:
```bash
cd /home/oioio/Documents/GolandProjects/oioni
make build  # gokrazy cross-compilation check
```

- [ ] **Step 6: Run all tests**

```bash
cd /home/oioio/Documents/GolandProjects/oioni
make test
```
Expected: all PASS.

- [ ] **Step 7: Commit**

```bash
git add cmd/oioni/epaper.go Makefile system/wifi/build/ system/wifi/bin/.gitkeep
git commit -m "feat: boot integration — wire wifi+netconf managers into UI"
# Note: oioio/config.json is skip-worktree; commit separately if changed:
# git update-index --no-skip-worktree oioio/config.json
# git add oioio/config.json
# git commit -m "feat(gokrazy): remove gokrazy/wifi, add wpa_supplicant ExtraFilePaths"
# git update-index --skip-worktree oioio/config.json
```

---

## Final check

After all tasks are complete, run:

```bash
cd /home/oioio/Documents/GolandProjects/oioni
make test
make build
```

Both must pass before declaring the implementation complete.
