// Package wifi manages WiFi on gokrazy via wpa_supplicant.
//
// Usage:
//
//	mgr := wifi.New(wifi.Config{
//	    WpaSupplicantBin: "/user/wpa_supplicant",
//	    HostapdBin:       "/user/hostapd",
//	    IwBin:            "/user/iw",
//	    IpBin:            "/user/ip",
//	    ConfDir:          "/perm/wifi",
//	    CtrlDir:          "/var/run/wpa_supplicant",
//	    Iface:            "wlan0",
//	})
//	if err := mgr.Start(ctx); err != nil { log.Printf("wifi: %v", err) }
//	mgr.Connect("MyNet", "passphrase", true)
//	mgr.SetMode(ctx, wifi.Mode{STA: true, AP: true}) // concurrent STA+AP
//
// Start loads the brcmfmac kernel modules, starts wpa_supplicant in daemon
// mode, and connects to the control socket. Saved credentials live in
// ConfDir/wpa_supplicant.conf; a one-time migration from /etc/wifi.json
// (gokrazy/wifi legacy) runs on first boot.
//
// AP mode creates the uap0 virtual interface and starts hostapd + a DHCP server.
// Scanner always works when STA is active (SCAN/SCAN_RESULTS on wpa_supplicant).
//
// Sentinel errors: ErrNotStarted is returned by any method called before Start;
// ErrWPATimeout is returned when polling exceeds its deadline.
package wifi

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/unix"
)

// kernelRelease caches the running kernel version string for loadModule.
// Computed once at package init — unix.Uname returns immutable data.
var kernelRelease = func() string {
	var uts unix.Utsname
	if err := unix.Uname(&uts); err != nil {
		return ""
	}
	return string(uts.Release[:bytes.IndexByte(uts.Release[:], 0)])
}()

// Mode selects which WiFi roles are active. Modes are independent and additive.
// Scanner always works when STA is active (no separate mode bit needed).
type Mode struct {
	STA bool `json:"sta"` // client mode via wpa_supplicant on wlan0
	AP  bool `json:"ap"`  // access point mode via hostapd on uap0
}

// Config holds the runtime configuration for the Manager.
type Config struct {
	WpaSupplicantBin string // e.g. "/user/wpa_supplicant"
	HostapdBin       string // e.g. "/user/hostapd" (AP mode)
	IwBin            string // e.g. "/user/iw" (AP mode — virtual interface creation)
	UdhcpcBin        string // e.g. "/bin/udhcpc" (empty = no DHCP client on STA iface)
	ConfDir          string // e.g. "/perm/wifi"
	CtrlDir          string // e.g. "/var/run/wpa_supplicant"
	Iface            string // e.g. "wlan0"
	DefaultAPConfig  APConfig
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
	IP      string // e.g. "192.168.0.15" — from wpa_supplicant if DHCP lease is active
	Enabled bool
}

// Manager wraps wpa_supplicant to provide WiFi management.
// Start must be called before any other method.
type Manager struct {
	cfg     Config
	conf    *confManager
	proc    processRunner
	newConn func(ctrlPath, localPath string) (wpaSocket, error) // injectable for tests

	// connMu guards conn. Use RLock for reads in send(), Lock when replacing conn
	// (e.g. during reconnectWPALocked). Kept separate from mu so that concurrent
	// STA commands (Status, Scan) do not block AP mode transitions.
	connMu sync.RWMutex
	conn   wpaSocket // nil until Start is called

	mu    sync.Mutex
	mode  Mode
	apMgr *APManager // non-nil when AP is running
}

// New creates a Manager with the given configuration.
func New(cfg Config) *Manager {
	return &Manager{
		cfg:  cfg,
		conf: &confManager{dir: cfg.ConfDir},
		proc: &realProcess{},
		newConn: func(ctrlPath, localPath string) (wpaSocket, error) {
			return dialWpa(ctrlPath, localPath)
		},
	}
}

// loadModule loads a kernel module by path relative to /lib/modules/<kernelRelease>/.
// params is passed directly to the kernel (space-separated "key=value" pairs).
// Ignores EEXIST/EBUSY (already loaded) and ENODEV/ENOENT (not present/applicable).
func loadModule(mod, params string) error {
	f, err := os.Open(filepath.Join("/lib/modules", kernelRelease, mod))
	if err != nil {
		return err
	}
	defer f.Close()
	if err := unix.FinitModule(int(f.Fd()), params, 0); err != nil {
		if err != unix.EEXIST && err != unix.EBUSY && err != unix.ENODEV && err != unix.ENOENT {
			return fmt.Errorf("FinitModule(%v): %v", mod, err)
		}
	}
	return nil
}

// wlanRfkillSoftPath returns the sysfs "soft" file path for the first wlan
// rfkill entry. Used by SetEnabled and isEnabled to avoid scanning twice.
func wlanRfkillSoftPath() (string, error) {
	entries, err := os.ReadDir("/sys/class/rfkill")
	if err != nil {
		return "", fmt.Errorf("rfkill: %w", err)
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
		return filepath.Join("/sys/class/rfkill", e.Name(), "soft"), nil
	}
	return "", fmt.Errorf("no wlan rfkill entry found")
}

// Start launches wpa_supplicant, polls until the control socket appears, and
// connects to it. Also runs wifi.json migration. Non-fatal on error.
func (m *Manager) Start(ctx context.Context) error {
	if err := m.conf.migrateIfNeeded(); err != nil {
		_ = err // non-fatal — log in caller
	}
	// Rewrite wpa_supplicant.conf to ensure explicit WPA2 crypto params are present.
	// Older conf files lacked key_mgmt/proto/pairwise/group entries, causing
	// auto-detection failures on BCM43430 that result in repeated TEMP-DISABLED.
	if nets, err := m.conf.read(); err == nil && len(nets) > 0 {
		_ = m.conf.write(nets) // best-effort: rewrite with current format
	}

	// Load brcmfmac kernel modules (firmware must be in /lib/firmware/brcm/).
	// feature_disable=0x82000 disables FWAUTH (bit 19) and WOWL_ARP_ND (bit 13),
	// forcing wpa_supplicant to handle WPA2 in userspace. Without this, BCM43430
	// firmware intercepts the 802.11 auth phase and silently fails (ASSOC-REJECT,
	// no 4-way handshake attempted), causing persistent TEMP-DISABLED entries.
	// roamoff=1 disables firmware-based roaming to prevent unexpected disconnects.
	for _, entry := range []struct{ mod, params string }{
		{"kernel/drivers/net/wireless/broadcom/brcm80211/brcmutil/brcmutil.ko", ""},
		{"kernel/drivers/net/wireless/broadcom/brcm80211/brcmfmac/brcmfmac.ko", "feature_disable=0x82000 roamoff=1"},
		{"kernel/drivers/net/wireless/broadcom/brcm80211/brcmfmac/wcc/brcmfmac-wcc.ko", ""},
	} {
		if err := loadModule(entry.mod, entry.params); err != nil {
			_ = err // non-fatal: module may already be present or not applicable
		}
	}

	// Ensure ctrl dir exists on tmpfs (created fresh on every boot).
	if err := os.MkdirAll(m.cfg.CtrlDir, 0755); err != nil {
		return fmt.Errorf("wpa_supplicant ctrl dir: %w", err)
	}

	// Wait for the wireless interface to appear (kernel module may not be ready yet).
	ifacePath := "/sys/class/net/" + m.cfg.Iface
	ifaceDeadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(ifaceDeadline) {
		if _, err := os.Stat(ifacePath); err == nil {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
	if _, err := os.Stat(ifacePath); err != nil {
		// Log all present interfaces to help diagnose driver issues.
		if entries, rerr := os.ReadDir("/sys/class/net"); rerr == nil {
			names := make([]string, 0, len(entries))
			for _, e := range entries {
				names = append(names, e.Name())
			}
			return fmt.Errorf("interface %s not ready after 20s (present: %s)", m.cfg.Iface, strings.Join(names, ","))
		}
		return fmt.Errorf("interface %s not ready after 20s", m.cfg.Iface)
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
			m.connMu.Lock()
			m.conn = conn
			m.connMu.Unlock()
			m.disablePowerSave()
			go m.restoreMode(ctx)
			m.startSTADHCPWatcher(ctx, m.cfg.UdhcpcBin)
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	return fmt.Errorf("wpa_supplicant control socket not ready after 3s: %w", ErrWPATimeout)
}

// disablePowerSave turns off BCM43430 WiFi power management via iw.
// Power save causes periodic beacon misses and results in brief disconnections
// every ~20–30 s on the BCM43430 (Pi Zero 2W). Best-effort: logs on failure.
func (m *Manager) disablePowerSave() {
	if m.cfg.IwBin == "" {
		return
	}
	out, err := exec.Command(m.cfg.IwBin, "dev", m.cfg.Iface, "set", "power_save", "off").CombinedOutput()
	if err != nil {
		log.Printf("wifi: disable power save: %v: %s", err, out)
		return
	}
	log.Printf("wifi: power save disabled on %s", m.cfg.Iface)
}

// restoreMode reads the persisted Mode and re-activates AP if needed.
// Called at the end of Start(). Non-fatal — logs errors.
func (m *Manager) restoreMode(ctx context.Context) {
	mode, err := m.conf.readMode()
	if err != nil {
		return
	}
	if !mode.AP {
		m.mu.Lock()
		m.mode = mode
		m.mu.Unlock()
		return
	}
	// Wait for the STA to provide a channel (freq= in STATUS).
	// BCM43430 requires STA+AP on the same channel; starting AP before the STA
	// reports its frequency would cause a channel conflict and block association.
	if err := m.waitForSTAChannel(ctx, 60*time.Second); err != nil {
		log.Printf("wifi: restoreMode: STA channel unknown after 60 s, skipping AP restore: %v", err)
		return
	}
	// Re-apply AP mode (ignoring error — STA is already working).
	if err := m.SetMode(ctx, mode); err != nil {
		_ = err // non-fatal: AP may not start if config missing or binary absent
	}
}

// waitForSTAChannel polls wpa_supplicant until freq= appears in STATUS
// (i.e. the STA is fully connected and its channel is known).
// Checks immediately on first call, then every 2 s.
func (m *Manager) waitForSTAChannel(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if m.staChannel() > 0 {
			return nil
		}
		if !time.Now().Before(deadline) {
			return fmt.Errorf("timed out after %s: %w", timeout, ErrWPATimeout)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

// waitForSTA polls wpa_supplicant until the STA interface shows an SSID in
// STATUS (indicating 802.11 association is complete).  Checks immediately on
// first call, then every 2 s.
//
// Note: on BCM43430, wpa_state may remain "ASSOCIATED" (not "COMPLETED") even
// when the link is fully functional; SSID presence is the reliable indicator.
func (m *Manager) waitForSTA(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		st, err := m.Status()
		if err == nil && st.SSID != "" {
			return nil
		}
		if !time.Now().Before(deadline) {
			return fmt.Errorf("timed out after %s: %w", timeout, ErrWPATimeout)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

// SetEnabled enables or disables WiFi via /sys/class/rfkill/ sysfs.
func (m *Manager) SetEnabled(enabled bool) error {
	softPath, err := wlanRfkillSoftPath()
	if err != nil {
		return err
	}
	val := "0"
	if !enabled {
		val = "1"
	}
	if err := os.WriteFile(softPath, []byte(val), 0644); err != nil {
		return fmt.Errorf("rfkill soft: %w", err)
	}
	return nil
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

// listNetworkID returns the wpa_supplicant network id for the given SSID,
// or "" if not found. Parses LIST_NETWORKS output (tab-separated lines).
func (m *Manager) listNetworkID(ssid string) string {
	out, err := m.send("LIST_NETWORKS")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(out, "\n") {
		parts := strings.Split(line, "\t")
		if len(parts) >= 2 && parts[1] == ssid {
			return strings.TrimSpace(parts[0])
		}
	}
	return ""
}

// removeNetworksBySSID removes all wpa_supplicant network entries for the given SSID.
// Used to clean up TEMP-DISABLED entries before re-adding a fresh entry.
func (m *Manager) removeNetworksBySSID(ssid string) {
	out, err := m.send("LIST_NETWORKS")
	if err != nil {
		return
	}
	for _, line := range strings.Split(out, "\n") {
		parts := strings.Split(line, "\t")
		if len(parts) >= 2 && parts[1] == ssid {
			_, _ = m.send("REMOVE_NETWORK " + strings.TrimSpace(parts[0]))
		}
	}
}

// Connect connects to an SSID with optional PSK. If save is true, persists credentials.
// When psk is empty and the SSID is already configured in wpa_supplicant (loaded from
// wpa_supplicant.conf on startup), Connect selects the existing network entry directly
// so that the saved WPA2 credentials are used — avoiding the ADD_NETWORK+key_mgmt=NONE
// path which would attempt an open connection.
func (m *Manager) Connect(ssid, psk string, save bool) error {
	// If no PSK given, try to reuse the existing wpa_supplicant network entry.
	if psk == "" {
		if id := m.listNetworkID(ssid); id != "" {
			for _, cmd := range []string{"SELECT_NETWORK " + id, "RECONNECT"} {
				if _, err := m.send(cmd); err != nil {
					return err
				}
			}
			return nil
		}
	}

	// Remove existing entries for this SSID to avoid accumulating TEMP-DISABLED entries.
	// wpa_supplicant marks a network TEMP-DISABLED after repeated auth failures; adding
	// new entries each call compounds the problem rather than recovering from it.
	m.removeNetworksBySSID(ssid)

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
		existing, err := m.conf.read()
		if err != nil {
			return fmt.Errorf("read saved networks: %w", err)
		}
		// Remove duplicate if re-saving same SSID; preserve existing PSK if new one is empty.
		actualPSK := psk
		var filtered []savedNetwork
		for _, n := range existing {
			if n.SSID != ssid {
				filtered = append(filtered, n)
			} else if actualPSK == "" {
				actualPSK = n.PSK // keep the stored PSK
			}
		}
		filtered = append(filtered, savedNetwork{SSID: ssid, PSK: actualPSK})
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
	st := parseWpaStatus(raw)
	st.Enabled = m.isEnabled()
	// ip_address= in STATUS is only set by wpa_supplicant's own DHCP client.
	// When using an external DHCP client (udhcpc), read the IP from the interface.
	if st.IP == "" {
		st.IP = ifaceIPv4(m.cfg.Iface)
	}
	return st, nil
}

// isEnabled checks rfkill state. Returns true if WiFi is not soft-blocked.
func (m *Manager) isEnabled() bool {
	softPath, err := wlanRfkillSoftPath()
	if err != nil {
		return true // assume enabled if rfkill not found
	}
	data, err := os.ReadFile(softPath)
	if err != nil {
		return true
	}
	return strings.TrimSpace(string(data)) == "0"
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

// send sends a single wpa_supplicant control command and returns the response.
// It is safe to call concurrently; connMu prevents races with reconnectWPALocked.
func (m *Manager) send(cmd string) (string, error) {
	m.connMu.RLock()
	conn := m.conn
	m.connMu.RUnlock()
	if conn == nil {
		return "", ErrNotStarted
	}
	return conn.SendCommand(cmd)
}

// DebugCmd sends a raw wpa_supplicant command and returns the response.
// For diagnostic use only.
func (m *Manager) DebugCmd(cmd string) (string, error) {
	return m.send(cmd)
}

// GetMode returns the current operating mode. Goroutine-safe.
func (m *Manager) GetMode() Mode {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.mode
}

// staChannel returns the 2.4 GHz channel that wpa_supplicant is currently using,
// by parsing the freq= field from the STATUS response. Returns 0 if not connected.
// Must NOT be called with m.mu held (send accesses m.conn without locking).
func (m *Manager) staChannel() int {
	status, err := m.send("STATUS")
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(status, "\n") {
		if !strings.HasPrefix(line, "freq=") {
			continue
		}
		var freq int
		if _, err := fmt.Sscanf(line, "freq=%d", &freq); err != nil {
			continue
		}
		if freq >= 2412 && freq <= 2484 { // 2.4 GHz band
			return (freq - 2407) / 5
		}
	}
	return 0
}

// reconnectWPALocked terminates the current wpa_supplicant, restarts it, and
// reconnects the Manager's control socket. MUST be called with m.mu held.
//
// This is needed after AP mode is disabled: destroying the uap0 virtual interface
// leaves the BCM43430 firmware in a state where scans time out (brcmf_escan_timeout).
// Restarting wpa_supplicant forces a firmware reset and allows STA to reconnect.
func (m *Manager) reconnectWPALocked(ctx context.Context) {
	m.connMu.Lock()
	if m.conn != nil {
		_, _ = m.conn.SendCommand("TERMINATE") // ask wpa_supplicant to exit cleanly
		_ = m.conn.Close()
		m.conn = nil
	}
	m.connMu.Unlock()

	ctrlPath := filepath.Join(m.cfg.CtrlDir, m.cfg.Iface)

	// Wait up to 3s for old wpa_supplicant socket to vanish.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(ctrlPath); os.IsNotExist(err) {
			break
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(100 * time.Millisecond):
		}
	}

	// Restart wpa_supplicant (-B daemonises; Start() returns after fork).
	args := []string{
		"-i", m.cfg.Iface,
		"-C", m.cfg.CtrlDir,
		"-c", filepath.Join(m.cfg.ConfDir, "wpa_supplicant.conf"),
		"-B",
	}
	if err := m.proc.Start(m.cfg.WpaSupplicantBin, args); err != nil {
		log.Printf("wifi: reconnectWPA restart: %v", err)
		return
	}

	// Poll for socket + reconnect (up to 5s).
	localPath := fmt.Sprintf("/tmp/oioni-wpa-%d", os.Getpid())
	deadline = time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := m.newConn(ctrlPath, localPath)
		if err == nil {
			m.connMu.Lock()
			m.conn = conn
			m.connMu.Unlock()
			log.Printf("wifi: wpa_supplicant restarted, STA reconnecting")
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(100 * time.Millisecond):
		}
	}
	log.Printf("wifi: reconnectWPA: socket not ready after 5s")
}

// SetMode applies a new operating mode and persists it to mode.json.
// Goroutine-safe. AP transitions happen live; disabling AP restarts wpa_supplicant
// to reset the BCM43430 firmware (brcmf_escan_timeout after uap0 deletion).
func (m *Manager) SetMode(ctx context.Context, mode Mode) error {
	// Validate AP config and detect STA channel BEFORE locking (send is not under m.mu).
	// BCM43430 requires STA and AP to share the same channel for concurrent operation.
	// freq= only appears in wpa_supplicant STATUS when wpa_state=COMPLETED (not ASSOCIATED).
	// Wait up to 15 s for the STA to fully authenticate before reading the channel.
	var (
		apCfg         APConfig
		detectedChannel int
	)
	if mode.AP {
		var err error
		apCfg, err = m.conf.readAPConfig()
		if err != nil {
			return fmt.Errorf("SetMode: read AP config: %w", err)
		}
		if apCfg.SSID == "" {
			return fmt.Errorf("SetMode: AP config has no SSID — call SetAPConfig first")
		}
		_ = m.waitForSTA(ctx, 15*time.Second) // best-effort; ignore timeout error
		detectedChannel = m.staChannel()
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	prev := m.mode

	// AP: turn on
	if mode.AP && !prev.AP {
		cfg := apCfg
		// Use STA channel so AP and STA share the same frequency (BCM43430 constraint).
		if detectedChannel > 0 {
			cfg.Channel = detectedChannel
			log.Printf("wifi: AP channel auto-set to %d (STA channel)", detectedChannel)
		}
		apMgr := newAPManager(cfg, m.cfg.Iface, m.conf, m.proc, m.cfg.HostapdBin, m.cfg.IwBin)
		if err := apMgr.Start(ctx); err != nil {
			return fmt.Errorf("SetMode: start AP: %w", err)
		}
		m.apMgr = apMgr
	}

	// AP: turn off — stop AP and restart wpa_supplicant to reset chip state.
	if !mode.AP && prev.AP && m.apMgr != nil {
		m.apMgr.Stop()
		m.apMgr = nil
		m.reconnectWPALocked(ctx) // blocks ~3–5s while wpa_supplicant restarts
	}

	m.mode = mode
	return m.conf.writeMode(mode)
}

// SetAPConfig validates and persists an APConfig for future AP mode use.
// PSK must be empty (open network) or 8–63 characters (WPA2 requirement).
// Channel must be 0 (auto / keep saved) or 1–14 (2.4 GHz band).
func (m *Manager) SetAPConfig(cfg APConfig) error {
	if cfg.PSK != "" && (len(cfg.PSK) < 8 || len(cfg.PSK) > 63) {
		return fmt.Errorf("wifi: AP PSK must be 8–63 characters (got %d)", len(cfg.PSK))
	}
	if cfg.Channel != 0 && (cfg.Channel < 1 || cfg.Channel > 14) {
		return fmt.Errorf("wifi: AP channel must be 1–14 for 2.4 GHz (got %d)", cfg.Channel)
	}
	return m.conf.writeAPConfig(cfg)
}

// GetAPConfig returns the persisted APConfig (or defaults if not set).
func (m *Manager) GetAPConfig() (APConfig, error) {
	return m.conf.readAPConfig()
}

// APStatus returns the current AP state. Returns zero APStatus when AP is off.
func (m *Manager) APStatus() APStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.apMgr == nil {
		return APStatus{}
	}
	return m.apMgr.Status()
}
