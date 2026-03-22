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
