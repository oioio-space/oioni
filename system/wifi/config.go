// system/wifi/config.go — wpa_supplicant.conf read/write + legacy migration
package wifi

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/pbkdf2"
)

// savedNetwork is a single entry in wpa_supplicant.conf: an SSID with optional PSK.
// Open networks have an empty PSK and key_mgmt=NONE in the config file.
type savedNetwork struct {
	SSID string
	PSK  string // empty for open networks
}

// confManager reads and writes the WiFi configuration directory (ConfDir).
// It manages wpa_supplicant.conf, mode.json, apconfig.json, and hostapd.conf.
type confManager struct {
	dir        string // e.g. "/perm/wifi" — must be writable (use /perm on gokrazy)
	legacyPath string // overridable for tests; defaults to /etc/wifi.json
}

// confPath returns the absolute path to wpa_supplicant.conf.
func (c *confManager) confPath() string {
	return filepath.Join(c.dir, "wpa_supplicant.conf")
}

// ensureDir creates the configuration directory with mode 0755 if absent.
func (c *confManager) ensureDir() error {
	return os.MkdirAll(c.dir, 0755)
}

// write serialises networks into wpa_supplicant.conf format.
func (c *confManager) write(networks []savedNetwork) error {
	if err := c.ensureDir(); err != nil {
		return err
	}
	var b strings.Builder
	b.WriteString("ctrl_interface=/var/run/wpa_supplicant\nctrl_interface_group=0\nupdate_config=1\n\n")
	for _, n := range networks {
		b.WriteString("network={\n")
		fmt.Fprintf(&b, "    ssid=%q\n", n.SSID)
		if n.PSK != "" {
			// Write as pre-computed hex PMK (64 hex chars, no quotes) rather than
			// a quoted passphrase. This avoids wpa_supplicant passphrase-parsing
			// failures observed on BCM43430 and eliminates the TEMP-DISABLED cycle.
			// If PSK is already a 64-char hex PMK (from a previous read), use it
			// directly to avoid double-hashing.
			pmk := n.PSK
			if !isPMK(pmk) {
				pmk = wpa2PMK(n.PSK, n.SSID)
			}
			fmt.Fprintf(&b, "    psk=%s\n", pmk)
			b.WriteString("    key_mgmt=WPA-PSK\n")
			b.WriteString("    proto=RSN\n")
			b.WriteString("    pairwise=CCMP\n")
			b.WriteString("    group=CCMP\n")
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
	for line := range strings.SplitSeq(string(data), "\n") {
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

// migrateIfNeeded imports the gokrazy/wifi legacy config into wpa_supplicant.conf.
// gokrazy/wifi deployed credentials to /etc/wifi.json via ExtraFilePaths.
// The file uses either "psk" or "passphrase" as the password key.
// A marker file in c.dir/.migrated prevents re-running on subsequent boots
// (we cannot rename /etc/wifi.json because /etc is squashfs read-only).
func (c *confManager) migrateIfNeeded() error {
	legacyPath := c.legacyPath
	if legacyPath == "" {
		legacyPath = "/etc/wifi.json"
	}
	data, err := os.ReadFile(legacyPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	// Check marker in /perm (writable) — /etc is squashfs read-only.
	markerPath := filepath.Join(c.dir, ".migrated")
	if _, err := os.Stat(markerPath); err == nil {
		return nil // already migrated
	}
	var legacy struct {
		SSID       string `json:"ssid"`
		PSK        string `json:"psk"`
		Passphrase string `json:"passphrase"`
	}
	if err := json.Unmarshal(data, &legacy); err != nil {
		return fmt.Errorf("parse wifi.json: %w", err)
	}
	psk := legacy.PSK
	if psk == "" {
		psk = legacy.Passphrase
	}
	existing, err := c.read()
	if err != nil {
		return err
	}
	existing = append(existing, savedNetwork{SSID: legacy.SSID, PSK: psk})
	if err := c.write(existing); err != nil {
		return err
	}
	// Write marker so migration doesn't repeat on next boot.
	if err := c.ensureDir(); err != nil {
		return err
	}
	return os.WriteFile(markerPath, []byte(legacyPath+"\n"), 0600)
}

// writeMode persists the current Mode to mode.json.
func (c *confManager) writeMode(m Mode) error {
	if err := c.ensureDir(); err != nil {
		return err
	}
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(c.dir, "mode.json"), data, 0600)
}

// readMode reads the persisted Mode from mode.json.
// Returns zero Mode (STA:false, AP:false) if the file does not exist.
func (c *confManager) readMode() (Mode, error) {
	data, err := os.ReadFile(filepath.Join(c.dir, "mode.json"))
	if os.IsNotExist(err) {
		return Mode{}, nil
	}
	if err != nil {
		return Mode{}, err
	}
	var m Mode
	if err := json.Unmarshal(data, &m); err != nil {
		return Mode{}, fmt.Errorf("parse mode.json: %w", err)
	}
	return m, nil
}

// writeAPConfig persists APConfig to apconfig.json.
func (c *confManager) writeAPConfig(cfg APConfig) error {
	if err := c.ensureDir(); err != nil {
		return err
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(c.dir, "apconfig.json"), data, 0600)
}

// readAPConfig reads the persisted APConfig from apconfig.json.
// Returns the default APConfig if the file does not exist.
// Fields absent from the JSON file (e.g. added in later versions) retain
// their default values so existing configs remain forward-compatible.
func (c *confManager) readAPConfig() (APConfig, error) {
	data, err := os.ReadFile(filepath.Join(c.dir, "apconfig.json"))
	if os.IsNotExist(err) {
		return defaultAPConfig(), nil
	}
	if err != nil {
		return APConfig{}, err
	}
	// Start from defaults so fields added after the config was saved are not zero.
	cfg := defaultAPConfig()
	if err := json.Unmarshal(data, &cfg); err != nil {
		return APConfig{}, fmt.Errorf("parse apconfig.json: %w", err)
	}
	return cfg, nil
}

// isPMK reports whether s is a pre-computed WPA2 PMK (64 lowercase hex characters).
func isPMK(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}

// wpa2PMK computes the WPA2 Pre-shared Master Key (PMK) via PBKDF2-SHA1.
// The result is a 64-character lower-case hex string suitable for use as
// an unquoted psk= value in wpa_supplicant.conf.
func wpa2PMK(passphrase, ssid string) string {
	dk := pbkdf2.Key([]byte(passphrase), []byte(ssid), 4096, 32, sha1.New)
	return hex.EncodeToString(dk)
}

// writeHostapdConf generates hostapd.conf from APConfig into confDir.
func (c *confManager) writeHostapdConf(cfg APConfig) error {
	if err := c.ensureDir(); err != nil {
		return err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "interface=uap0\n")
	fmt.Fprintf(&b, "driver=nl80211\n")
	fmt.Fprintf(&b, "ssid=%s\n", cfg.SSID)
	fmt.Fprintf(&b, "hw_mode=g\n")
	fmt.Fprintf(&b, "channel=%d\n", cfg.Channel)
	if cfg.PSK != "" {
		fmt.Fprintf(&b, "wpa=2\n")
		fmt.Fprintf(&b, "wpa_passphrase=%s\n", cfg.PSK)
		fmt.Fprintf(&b, "wpa_key_mgmt=WPA-PSK\n")
		fmt.Fprintf(&b, "rsn_pairwise=CCMP\n")
	}
	return os.WriteFile(filepath.Join(c.dir, "hostapd.conf"), []byte(b.String()), 0600)
}
