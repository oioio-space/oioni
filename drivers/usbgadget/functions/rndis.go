package functions

import (
	"fmt"
	"os"
	"strings"
)

type RNDISFunc struct {
	instance  string
	devAddr   string
	hostAddr  string
	qmult     uint8
	configDir string
}

type RNDISOption func(*RNDISFunc)

// WithRNDISDevAddr sets the MAC address of the gadget-side network interface.
func WithRNDISDevAddr(mac string) RNDISOption {
	return func(f *RNDISFunc) { f.devAddr = mac }
}

// WithRNDISHostAddr sets the MAC address seen by the host.
func WithRNDISHostAddr(mac string) RNDISOption {
	return func(f *RNDISFunc) { f.hostAddr = mac }
}

// WithRNDISQMult sets the TX queue multiplier for high-speed USB (default 5).
func WithRNDISQMult(n uint8) RNDISOption {
	return func(f *RNDISFunc) { f.qmult = n }
}

// RNDIS creates a RNDIS network function (Windows USB network).
// Must be the first function in the composite for Windows compatibility.
func RNDIS(opts ...RNDISOption) *RNDISFunc {
	f := &RNDISFunc{instance: "usb0"}
	for _, o := range opts {
		o(f)
	}
	return f
}

func (f *RNDISFunc) TypeName() string     { return "rndis" }
func (f *RNDISFunc) InstanceName() string { return f.instance }

// IfName returns the kernel network interface name on the gadget side.
// Falls back to MAC-based scan if configfs ifname is not updated.
func (f *RNDISFunc) IfName() (string, error) {
	name, err := readIfName(f.configDir)
	if err == nil && name != "" && !strings.Contains(name, "unnamed") {
		return name, nil
	}
	if f.devAddr != "" {
		if iface, err2 := findIfaceByMAC(f.devAddr); err2 == nil {
			return iface, nil
		}
	}
	if err != nil {
		return "", err
	}
	return "", fmt.Errorf("ifname not yet assigned (got %q)", name)
}

// ReadStats returns the current network counters for this interface.
func (f *RNDISFunc) ReadStats() (NetStats, error) {
	ifname, err := f.IfName()
	if err != nil {
		return NetStats{}, err
	}
	return readNetStats(ifname)
}

func (f *RNDISFunc) Configure(dir string) error {
	f.configDir = dir
	if f.devAddr != "" {
		if err := os.WriteFile(fmt.Sprintf("%s/dev_addr", dir), []byte(f.devAddr+"\n"), 0644); err != nil {
			return err
		}
	}
	if f.hostAddr != "" {
		if err := os.WriteFile(fmt.Sprintf("%s/host_addr", dir), []byte(f.hostAddr+"\n"), 0644); err != nil {
			return err
		}
	}
	if f.qmult != 0 {
		if err := os.WriteFile(fmt.Sprintf("%s/qmult", dir), []byte(fmt.Sprintf("%d\n", f.qmult)), 0644); err != nil {
			return err
		}
	}
	return nil
}
