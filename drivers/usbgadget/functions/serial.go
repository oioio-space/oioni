package functions

import (
	"fmt"
	"sync/atomic"
)

var serialPortCounter atomic.Int32

// nextSerialPort returns the next available ttyGS port number.
// ACM, GSER, and OBEX all share the u_serial port pool, so order of
// function creation determines which /dev/ttyGSN device each gets.
func nextSerialPort() int32 {
	return serialPortCounter.Add(1) - 1
}

// — Generic Serial (GSER) ——————————————————————————————————————————

type SerialFunc struct {
	instance string
	portNum  int32
}

// Serial creates a generic CDC serial function (GSER).
// Appears as /dev/ttyGSN on the gadget and as a USB serial device on the host.
// Unlike ACM, it has no modem control signals — simpler and lower overhead.
func Serial() *SerialFunc {
	return &SerialFunc{instance: "usb0", portNum: nextSerialPort()}
}

func (f *SerialFunc) TypeName() string        { return "gser" }
func (f *SerialFunc) InstanceName() string    { return f.instance }
func (f *SerialFunc) Configure(_ string) error { return nil }

// DevPath returns the gadget-side serial device (e.g. /dev/ttyGS0).
func (f *SerialFunc) DevPath() string { return fmt.Sprintf("/dev/ttyGS%d", f.portNum) }

// — ACM Serial ————————————————————————————————————————————————————

type ACMFunc struct {
	instance string
	portNum  int32
}

// ACMSerial creates a USB CDC ACM serial function.
// Appears as /dev/ttyGSN on the gadget and as /dev/ttyUSBN or /dev/ttyACMN on the host.
// ACM includes modem control signals (DTR, RTS) used by many terminal programs.
func ACMSerial() *ACMFunc {
	return &ACMFunc{instance: "usb0", portNum: nextSerialPort()}
}

func (f *ACMFunc) TypeName() string        { return "acm" }
func (f *ACMFunc) InstanceName() string    { return f.instance }
func (f *ACMFunc) Configure(_ string) error { return nil }

// DevPath returns the gadget-side serial device (e.g. /dev/ttyGS0).
func (f *ACMFunc) DevPath() string { return fmt.Sprintf("/dev/ttyGS%d", f.portNum) }
