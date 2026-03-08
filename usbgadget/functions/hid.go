package functions

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"
)

var hidCounter atomic.Int32

type hidFunc struct {
	instance   string
	protocol   uint8
	subclass   uint8
	reportLen  uint16
	reportDesc []byte
	devNum     int32 // index assigned at creation → /dev/hidgN
}

type HIDOption func(*hidFunc)

func newHID(opts ...HIDOption) *hidFunc {
	n := hidCounter.Add(1) - 1
	f := &hidFunc{instance: fmt.Sprintf("usb%d", n), devNum: n}
	for _, o := range opts {
		o(f)
	}
	return f
}

// DevPath returns the kernel hidg device path (e.g. /dev/hidg0).
// Use this to write HID input reports or read LED output reports.
func (f *hidFunc) DevPath() string {
	return fmt.Sprintf("/dev/hidg%d", f.devNum)
}

// LEDState holds the keyboard LED indicators sent by the host.
type LEDState struct {
	NumLock    bool
	CapsLock   bool
	ScrollLock bool
	Compose    bool
	Kana       bool
}

// ReadLEDs returns a channel that delivers LED state changes sent by the host
// (e.g. when the user presses NumLock). Blocks until ctx is cancelled.
// Only meaningful for Keyboard HID functions (protocol=1).
func (f *hidFunc) ReadLEDs(ctx context.Context) (<-chan LEDState, error) {
	dev, err := os.OpenFile(f.DevPath(), os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", f.DevPath(), err)
	}
	ch := make(chan LEDState, 4)
	go func() {
		defer dev.Close()
		defer close(ch)
		buf := make([]byte, 1)
		for {
			if ctx.Err() != nil {
				return
			}
			n, err := dev.Read(buf)
			if err != nil || n == 0 {
				return
			}
			b := buf[0]
			ch <- LEDState{
				NumLock:    b&0x01 != 0,
				CapsLock:   b&0x02 != 0,
				ScrollLock: b&0x04 != 0,
				Compose:    b&0x08 != 0,
				Kana:       b&0x10 != 0,
			}
		}
	}()
	return ch, nil
}

// WriteReport writes a raw HID input report to the host.
// For Keyboard: [modifier, 0x00, key1, key2, key3, key4, key5, key6]
// For Mouse:    [buttons, deltaX, deltaY, wheel]
func (f *hidFunc) WriteReport(report []byte) error {
	return os.WriteFile(f.DevPath(), report, 0)
}


// Keyboard creates a standard HID boot keyboard.
func Keyboard(opts ...HIDOption) Function {
	f := newHID(opts...)
	f.protocol = 1
	f.subclass = 1
	f.reportLen = 8
	f.reportDesc = []byte{
		0x05, 0x01, // Usage Page (Generic Desktop)
		0x09, 0x06, // Usage (Keyboard)
		0xa1, 0x01, // Collection (Application)
		0x05, 0x07, // Usage Page (Keyboard)
		0x19, 0xe0, 0x29, 0xe7, // Usage Min/Max (modifier keys)
		0x15, 0x00, 0x25, 0x01, // Logical Min/Max
		0x75, 0x01, 0x95, 0x08, // Report Size 1, Count 8
		0x81, 0x02, // Input (Data, Variable, Absolute)
		0x95, 0x01, 0x75, 0x08, // Report Count 1, Size 8 (padding)
		0x81, 0x03, // Input (Constant)
		0x95, 0x06, 0x75, 0x08, // Report Count 6, Size 8
		0x15, 0x00, 0x25, 0x65, // Logical Min 0, Max 101
		0x05, 0x07, // Usage Page (Keyboard)
		0x19, 0x00, 0x29, 0x65, // Usage Min/Max
		0x81, 0x00, // Input (Data, Array)
		0xc0,       // End Collection
	}
	return f
}

// Mouse creates a standard HID boot mouse.
func Mouse(opts ...HIDOption) Function {
	f := newHID(opts...)
	f.protocol = 2
	f.subclass = 1
	f.reportLen = 4
	f.reportDesc = []byte{
		0x05, 0x01, // Usage Page (Generic Desktop)
		0x09, 0x02, // Usage (Mouse)
		0xa1, 0x01, // Collection (Application)
		0x09, 0x01, // Usage (Pointer)
		0xa1, 0x00, // Collection (Physical)
		0x05, 0x09, // Usage Page (Button)
		0x19, 0x01, 0x29, 0x03, // Usage Min/Max (buttons 1-3)
		0x15, 0x00, 0x25, 0x01, // Logical Min/Max
		0x95, 0x03, 0x75, 0x01, // Count 3, Size 1
		0x81, 0x02, // Input (Data, Variable, Absolute)
		0x95, 0x01, 0x75, 0x05, // Count 1, Size 5 (padding)
		0x81, 0x03, // Input (Constant)
		0x05, 0x01, // Usage Page (Generic Desktop)
		0x09, 0x30, 0x09, 0x31, // Usage X, Y
		0x15, 0x81, 0x25, 0x7f, // Logical Min -127, Max 127
		0x75, 0x08, 0x95, 0x02, // Size 8, Count 2
		0x81, 0x06, // Input (Data, Variable, Relative)
		0xc0, 0xc0, // End Collections
	}
	return f
}

func (f *hidFunc) TypeName() string     { return "hid" }
func (f *hidFunc) InstanceName() string { return f.instance }
func (f *hidFunc) Configure(dir string) error {
	write := func(name string, val uint64) error {
		return os.WriteFile(fmt.Sprintf("%s/%s", dir, name),
			[]byte(fmt.Sprintf("%d\n", val)), 0644)
	}
	if err := write("protocol", uint64(f.protocol)); err != nil {
		return err
	}
	if err := write("subclass", uint64(f.subclass)); err != nil {
		return err
	}
	if err := write("report_length", uint64(f.reportLen)); err != nil {
		return err
	}
	if len(f.reportDesc) > 0 {
		if err := os.WriteFile(dir+"/report_desc", f.reportDesc, 0644); err != nil {
			return err
		}
	}
	return nil
}
