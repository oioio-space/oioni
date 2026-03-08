package functions

import "fmt"

type OBEXFunc struct {
	instance string
	portNum  int32
}

// OBEX creates a USB OBEX (Object Exchange) function.
// OBEX over USB is used for file transfer on mobile devices (Nokia, etc.).
// Appears as /dev/ttyGSN on the gadget. The host needs an OBEX client.
func OBEX() *OBEXFunc {
	return &OBEXFunc{instance: "usb0", portNum: nextSerialPort()}
}

func (f *OBEXFunc) TypeName() string        { return "obex" }
func (f *OBEXFunc) InstanceName() string    { return f.instance }
func (f *OBEXFunc) Configure(_ string) error { return nil }

// DevPath returns the gadget-side serial device (e.g. /dev/ttyGS0).
func (f *OBEXFunc) DevPath() string { return fmt.Sprintf("/dev/ttyGS%d", f.portNum) }
