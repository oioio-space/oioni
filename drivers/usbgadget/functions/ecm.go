package functions

import (
	"fmt"
	"os"
)

type ECMFunc struct {
	instance  string
	devAddr   string
	hostAddr  string
	qmult     uint8
	configDir string
}

type ECMOption func(*ECMFunc)

// WithECMDevAddr sets the MAC address of the gadget-side network interface.
func WithECMDevAddr(mac string) ECMOption {
	return func(f *ECMFunc) { f.devAddr = mac }
}

// WithECMHostAddr sets the MAC address seen by the host.
func WithECMHostAddr(mac string) ECMOption {
	return func(f *ECMFunc) { f.hostAddr = mac }
}

// WithECMQMult sets the TX queue multiplier for high-speed USB (default 5).
func WithECMQMult(n uint8) ECMOption {
	return func(f *ECMFunc) { f.qmult = n }
}

// ECM creates an ECM network function (Linux/macOS USB network).
func ECM(opts ...ECMOption) *ECMFunc {
	f := &ECMFunc{instance: "usb1"}
	for _, o := range opts {
		o(f)
	}
	return f
}

func (f *ECMFunc) TypeName() string     { return "ecm" }
func (f *ECMFunc) InstanceName() string { return f.instance }

// IfName returns the kernel network interface name on the gadget side.
func (f *ECMFunc) IfName() (string, error) { return readIfName(f.configDir) }

// ReadStats returns the current network counters for this interface.
func (f *ECMFunc) ReadStats() (NetStats, error) {
	ifname, err := f.IfName()
	if err != nil {
		return NetStats{}, err
	}
	return readNetStats(ifname)
}

func (f *ECMFunc) Configure(dir string) error {
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
