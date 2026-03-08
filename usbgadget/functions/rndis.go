package functions

import (
	"fmt"
	"os"
)

type rndisFunc struct {
	instance  string
	devAddr   string
	hostAddr  string
	qmult     uint8
	configDir string
}

type RNDISOption func(*rndisFunc)

// WithRNDISDevAddr sets the MAC address of the gadget-side network interface.
func WithRNDISDevAddr(mac string) RNDISOption {
	return func(f *rndisFunc) { f.devAddr = mac }
}

// WithRNDISHostAddr sets the MAC address seen by the host.
func WithRNDISHostAddr(mac string) RNDISOption {
	return func(f *rndisFunc) { f.hostAddr = mac }
}

// WithRNDISQMult sets the TX queue multiplier for high-speed USB (default 5).
func WithRNDISQMult(n uint8) RNDISOption {
	return func(f *rndisFunc) { f.qmult = n }
}

// RNDIS creates a RNDIS network function (Windows USB network).
// Must be the first function in the composite for Windows compatibility.
func RNDIS(opts ...RNDISOption) Function {
	f := &rndisFunc{instance: "usb0"}
	for _, o := range opts {
		o(f)
	}
	return f
}

func (f *rndisFunc) TypeName() string     { return "rndis" }
func (f *rndisFunc) InstanceName() string { return f.instance }

// IfName returns the kernel network interface name on the gadget side (e.g. "usb0").
// Only valid after the gadget is enabled.
func (f *rndisFunc) IfName() (string, error) { return readIfName(f.configDir) }

// ReadStats returns the current network counters for this interface.
func (f *rndisFunc) ReadStats() (NetStats, error) {
	ifname, err := f.IfName()
	if err != nil {
		return NetStats{}, err
	}
	return readNetStats(ifname)
}

func (f *rndisFunc) Configure(dir string) error {
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
