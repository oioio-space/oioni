package functions

import (
	"fmt"
	"os"
	"strings"
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
// It first tries the configfs ifname attribute, then falls back to scanning
// /sys/class/net by MAC address (works around kernels where ifname stays
// "unnamed net_device" after gadget bind).
func (f *ECMFunc) IfName() (string, error) {
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

// HostAddr returns the host-side MAC address as stored in configfs.
// Available only after g.Enable() has been called.
func (f *ECMFunc) HostAddr() (string, error) { return readHostAddr(f.configDir) }

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
