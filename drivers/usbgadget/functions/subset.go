package functions

import (
	"fmt"
	"os"
)

type SubsetFunc struct {
	instance  string
	devAddr   string
	hostAddr  string
	qmult     uint8
	configDir string
}

// SubsetOption configures a CDC Subset function.
type SubsetOption func(*SubsetFunc)

// WithSubsetDevAddr sets the MAC address of the gadget-side interface.
func WithSubsetDevAddr(mac string) SubsetOption { return func(f *SubsetFunc) { f.devAddr = mac } }

// WithSubsetHostAddr sets the MAC address seen by the host.
func WithSubsetHostAddr(mac string) SubsetOption { return func(f *SubsetFunc) { f.hostAddr = mac } }

// WithSubsetQMult sets the TX queue multiplier for high-speed USB (default 5).
func WithSubsetQMult(n uint8) SubsetOption { return func(f *SubsetFunc) { f.qmult = n } }

// Subset creates a CDC Subset network function.
// Lightweight variant of ECM without a union descriptor.
// Supported by older Linux hosts and some embedded stacks.
func Subset(opts ...SubsetOption) *SubsetFunc {
	f := &SubsetFunc{instance: "usb0"}
	for _, o := range opts {
		o(f)
	}
	return f
}

func (f *SubsetFunc) TypeName() string     { return "geth" }
func (f *SubsetFunc) InstanceName() string { return f.instance }

// IfName returns the kernel network interface name on the gadget side.
func (f *SubsetFunc) IfName() (string, error) { return readIfName(f.configDir) }

// ReadStats returns current network counters for this interface.
func (f *SubsetFunc) ReadStats() (NetStats, error) {
	ifname, err := f.IfName()
	if err != nil {
		return NetStats{}, err
	}
	return readNetStats(ifname)
}

func (f *SubsetFunc) Configure(dir string) error {
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
