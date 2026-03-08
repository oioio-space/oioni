package functions

import (
	"fmt"
	"os"
)

type ncmFunc struct {
	instance  string
	devAddr   string
	hostAddr  string
	qmult     uint8
	configDir string
}

type NCMOption func(*ncmFunc)

// WithNCMDevAddr sets the MAC address of the gadget-side network interface.
func WithNCMDevAddr(mac string) NCMOption {
	return func(f *ncmFunc) { f.devAddr = mac }
}

// WithNCMHostAddr sets the MAC address seen by the host.
func WithNCMHostAddr(mac string) NCMOption {
	return func(f *ncmFunc) { f.hostAddr = mac }
}

// WithNCMQMult sets the TX queue multiplier for high-speed USB (default 5).
func WithNCMQMult(n uint8) NCMOption {
	return func(f *ncmFunc) { f.qmult = n }
}

// NCM creates an NCM network function (high-speed USB network, Linux 3.10+).
func NCM(opts ...NCMOption) Function {
	f := &ncmFunc{instance: "usb2"}
	for _, o := range opts {
		o(f)
	}
	return f
}

func (f *ncmFunc) TypeName() string     { return "ncm" }
func (f *ncmFunc) InstanceName() string { return f.instance }

// IfName returns the kernel network interface name on the gadget side.
func (f *ncmFunc) IfName() (string, error) { return readIfName(f.configDir) }

// ReadStats returns the current network counters for this interface.
func (f *ncmFunc) ReadStats() (NetStats, error) {
	ifname, err := f.IfName()
	if err != nil {
		return NetStats{}, err
	}
	return readNetStats(ifname)
}

func (f *ncmFunc) Configure(dir string) error {
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
