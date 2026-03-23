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

// migrateIfNeeded imports the gokrazy/wifi legacy config into wpa_supplicant.conf.
// gokrazy/wifi deployed credentials to /etc/wifi.json via ExtraFilePaths.
// The file uses either "psk" or "passphrase" as the password key.
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
	return os.Rename(legacyPath, legacyPath+".migrated")
}
