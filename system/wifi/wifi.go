// system/wifi/wifi.go — WiFi manager public API
package wifi

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/unix"
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

// loadModule loads a kernel module by path relative to /lib/modules/<release>/.
// Ignores EEXIST/EBUSY (already loaded) and ENODEV/ENOENT (not present).
func loadModule(mod string) error {
	var uts unix.Utsname
	if err := unix.Uname(&uts); err != nil {
		return err
	}
	release := string(uts.Release[:bytes.IndexByte(uts.Release[:], 0)])
	f, err := os.Open(filepath.Join("/lib/modules", release, mod))
	if err != nil {
		return err
	}
	defer f.Close()
	if err := unix.FinitModule(int(f.Fd()), "", 0); err != nil {
		if err != unix.EEXIST && err != unix.EBUSY && err != unix.ENODEV && err != unix.ENOENT {
			return fmt.Errorf("FinitModule(%v): %v", mod, err)
		}
	}
	return nil
}

// Start launches wpa_supplicant, polls until the control socket appears, and
// connects to it. Also runs wifi.json migration. Non-fatal on error.
func (m *Manager) Start(ctx context.Context) error {
	if err := m.conf.migrateIfNeeded(); err != nil {
		_ = err // non-fatal — log in caller
	}

	// Load brcmfmac kernel modules (firmware must be in /lib/firmware/brcm/).
	for _, mod := range []string{
		"kernel/drivers/net/wireless/broadcom/brcm80211/brcmutil/brcmutil.ko",
		"kernel/drivers/net/wireless/broadcom/brcm80211/brcmfmac/brcmfmac.ko",
		"kernel/drivers/net/wireless/broadcom/brcm80211/brcmfmac/wcc/brcmfmac-wcc.ko",
	} {
		if err := loadModule(mod); err != nil {
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
		existing, err := m.conf.read()
		if err != nil {
			return fmt.Errorf("read saved networks: %w", err)
		}
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
	st := parseWpaStatus(raw)
	st.Enabled = m.isEnabled()
	return st, nil
}

// isEnabled checks rfkill state. Returns true if WiFi is not soft-blocked.
func (m *Manager) isEnabled() bool {
	entries, err := os.ReadDir("/sys/class/rfkill")
	if err != nil {
		return true // assume enabled if rfkill not readable
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
		data, err = os.ReadFile(softPath)
		if err != nil {
			return true
		}
		return strings.TrimSpace(string(data)) == "0"
	}
	return true // no rfkill entry = enabled
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
