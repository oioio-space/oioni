package functions

import (
	"fmt"
	"os"
)

type EEMFunc struct {
	instance  string
	devAddr   string
	hostAddr  string
	qmult     uint8
	configDir string
}

// EEMOption configures an EEM function.
type EEMOption func(*EEMFunc)

// WithEEMDevAddr sets the MAC address of the gadget-side interface.
func WithEEMDevAddr(mac string) EEMOption { return func(f *EEMFunc) { f.devAddr = mac } }

// WithEEMHostAddr sets the MAC address seen by the host.
func WithEEMHostAddr(mac string) EEMOption { return func(f *EEMFunc) { f.hostAddr = mac } }

// WithEEMQMult sets the TX queue multiplier for high-speed USB (default 5).
func WithEEMQMult(n uint8) EEMOption { return func(f *EEMFunc) { f.qmult = n } }

// EEM creates a CDC Ethernet Emulation Model network function.
// Simpler than ECM: no control interface, single bulk endpoint pair.
// Best for Linux-to-Linux USB networking where ECM compatibility isn't needed.
func EEM(opts ...EEMOption) *EEMFunc {
	f := &EEMFunc{instance: "usb0"}
	for _, o := range opts {
		o(f)
	}
	return f
}

func (f *EEMFunc) TypeName() string     { return "eem" }
func (f *EEMFunc) InstanceName() string { return f.instance }

// IfName returns the kernel network interface name on the gadget side (e.g. "usb0").
func (f *EEMFunc) IfName() (string, error) { return readIfName(f.configDir) }

// ReadStats returns current network counters for this interface.
func (f *EEMFunc) ReadStats() (NetStats, error) {
	ifname, err := f.IfName()
	if err != nil {
		return NetStats{}, err
	}
	return readNetStats(ifname)
}

func (f *EEMFunc) Configure(dir string) error {
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
