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
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	if m == nil {
		m = make(map[string]IfaceCfg)
	}
	return m, nil
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
