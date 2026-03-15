package functions

import (
	"fmt"
	"os"
)

type LoopbackFunc struct {
	instance   string
	qLen       uint32
	bulkBufLen uint32
}

// LoopbackOption configures a USB Loopback function.
type LoopbackOption func(*LoopbackFunc)

// WithLoopbackQLen sets the request queue depth (default 32).
func WithLoopbackQLen(n uint32) LoopbackOption { return func(f *LoopbackFunc) { f.qLen = n } }

// WithLoopbackBufLen sets the bulk transfer buffer size in bytes (default 512).
func WithLoopbackBufLen(n uint32) LoopbackOption {
	return func(f *LoopbackFunc) { f.bulkBufLen = n }
}

// Loopback creates a USB SuperSpeed Loopback function (ss_lb).
// Every byte written to the bulk-out endpoint is echoed back on bulk-in.
// Useful for USB bandwidth benchmarking and testing the gadget framework.
// Requires the usb_f_ss_lb kernel module.
func Loopback(opts ...LoopbackOption) *LoopbackFunc {
	f := &LoopbackFunc{
		instance:   "usb0",
		qLen:       32,
		bulkBufLen: 512,
	}
	for _, o := range opts {
		o(f)
	}
	return f
}

func (f *LoopbackFunc) TypeName() string     { return "ss_lb" }
func (f *LoopbackFunc) InstanceName() string { return f.instance }
func (f *LoopbackFunc) Configure(dir string) error {
	write := func(name string, val uint32) error {
		return os.WriteFile(fmt.Sprintf("%s/%s", dir, name),
			[]byte(fmt.Sprintf("%d\n", val)), 0644)
	}
	if err := write("qlen", f.qLen); err != nil {
		return err
	}
	return write("bulk_buflen", f.bulkBufLen)
}
