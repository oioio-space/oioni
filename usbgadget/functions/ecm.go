package functions

import (
	"fmt"
	"os"
)

type ecmFunc struct {
	instance string
	devAddr  string
	hostAddr string
	qmult    uint8
}

type ECMOption func(*ecmFunc)

// WithECMDevAddr sets the MAC address of the gadget-side network interface.
func WithECMDevAddr(mac string) ECMOption {
	return func(f *ecmFunc) { f.devAddr = mac }
}

// WithECMHostAddr sets the MAC address seen by the host.
func WithECMHostAddr(mac string) ECMOption {
	return func(f *ecmFunc) { f.hostAddr = mac }
}

// WithECMQMult sets the TX queue multiplier for high-speed USB (default 5).
func WithECMQMult(n uint8) ECMOption {
	return func(f *ecmFunc) { f.qmult = n }
}

// ECM creates an ECM network function (Linux/macOS USB network).
func ECM(opts ...ECMOption) Function {
	f := &ecmFunc{instance: "usb1"}
	for _, o := range opts {
		o(f)
	}
	return f
}

func (f *ecmFunc) TypeName() string     { return "ecm" }
func (f *ecmFunc) InstanceName() string { return f.instance }
func (f *ecmFunc) Configure(dir string) error {
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
