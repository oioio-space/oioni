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
	dir        string // e.g. "/perm/wifi"
	legacyPath string // overridable for tests; defaults to /etc/wifi.json
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
	if err := os.MkdirAll(c.dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(markerPath, []byte(legacyPath+"\n"), 0600)
}

// writeMode persists the current Mode to mode.json.
func (c *confManager) writeMode(m Mode) error {
	if err := os.MkdirAll(c.dir, 0755); err != nil {
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
	if err := os.MkdirAll(c.dir, 0755); err != nil {
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
func (c *confManager) readAPConfig() (APConfig, error) {
	data, err := os.ReadFile(filepath.Join(c.dir, "apconfig.json"))
	if os.IsNotExist(err) {
		return defaultAPConfig(), nil
	}
	if err != nil {
		return APConfig{}, err
	}
	var cfg APConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return APConfig{}, fmt.Errorf("parse apconfig.json: %w", err)
	}
	return cfg, nil
}

// writeHostapdConf generates hostapd.conf from APConfig into confDir.
func (c *confManager) writeHostapdConf(cfg APConfig) error {
	if err := os.MkdirAll(c.dir, 0755); err != nil {
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
