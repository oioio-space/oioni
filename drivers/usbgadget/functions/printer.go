package functions

import (
	"fmt"
	"os"
)

type PrinterFunc struct {
	instance  string
	pnpString string
	qLen      uint32
}

// PrinterOption configures a USB Printer function.
type PrinterOption func(*PrinterFunc)

// WithPrinterPnP sets the IEEE 1284 device ID string reported to the host.
// Example: "MFG:Example;MDL:Gadget Printer;CMD:PCL;"
func WithPrinterPnP(s string) PrinterOption { return func(f *PrinterFunc) { f.pnpString = s } }

// WithPrinterQLen sets the USB request queue depth (default 10).
func WithPrinterQLen(n uint32) PrinterOption { return func(f *PrinterFunc) { f.qLen = n } }

// Printer creates a USB Printer function (IPP-over-USB / USB printing).
// Appears as /dev/usb/lpN on the gadget.
// The host sends print jobs as raw data over the bulk-out endpoint.
func Printer(opts ...PrinterOption) *PrinterFunc {
	f := &PrinterFunc{
		instance: "usb0",
		qLen:     10,
	}
	for _, o := range opts {
		o(f)
	}
	return f
}

func (f *PrinterFunc) TypeName() string     { return "printer" }
func (f *PrinterFunc) InstanceName() string { return f.instance }
func (f *PrinterFunc) Configure(dir string) error {
	if f.pnpString != "" {
		if err := os.WriteFile(fmt.Sprintf("%s/pnp_string", dir), []byte(f.pnpString+"\n"), 0644); err != nil {
			return err
		}
	}
	return os.WriteFile(fmt.Sprintf("%s/q_len", dir),
		[]byte(fmt.Sprintf("%d\n", f.qLen)), 0644)
}
