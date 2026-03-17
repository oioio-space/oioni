package touch

import (
	"context"
	"fmt"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/gpio/gpioreg"
	host "periph.io/x/host/v3"
)

// I2CConn performs a write-then-read I2C transaction.
type I2CConn interface {
	Tx(w, r []byte) error
}

// OutputPin drives a GPIO signal (TRST).
type OutputPin interface {
	Out(high bool) error
}

// InterruptPin waits for a falling edge (INT).
type InterruptPin interface {
	WaitFalling(ctx context.Context) error
}

// --- Linux I2C ---

type i2cRDWRIoctlData struct {
	msgs  uintptr
	nmsgs uint32
}

type i2cMsg struct {
	addr  uint16
	flags uint16
	len   uint16
	buf   uintptr
}

const (
	i2cRDWR    = 0x0707
	i2cMFlagRD = 0x0001
)

type linuxI2C struct {
	fd   int
	addr uint16
}

func openI2C(device string, addr uint16) (*linuxI2C, error) {
	fd, err := unix.Open(device, unix.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", device, err)
	}
	return &linuxI2C{fd: fd, addr: addr}, nil
}

func (c *linuxI2C) Tx(w, r []byte) error {
	msgs := make([]i2cMsg, 0, 2)
	if len(w) > 0 {
		msgs = append(msgs, i2cMsg{addr: c.addr, flags: 0, len: uint16(len(w)), buf: uintptr(unsafe.Pointer(&w[0]))})
	}
	if len(r) > 0 {
		msgs = append(msgs, i2cMsg{addr: c.addr, flags: i2cMFlagRD, len: uint16(len(r)), buf: uintptr(unsafe.Pointer(&r[0]))})
	}
	if len(msgs) == 0 {
		return nil
	}
	data := i2cRDWRIoctlData{msgs: uintptr(unsafe.Pointer(&msgs[0])), nmsgs: uint32(len(msgs))}
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(c.fd), i2cRDWR, uintptr(unsafe.Pointer(&data)))
	if errno != 0 {
		return fmt.Errorf("i2c Tx: %w", errno)
	}
	return nil
}

func (c *linuxI2C) Close() error { return unix.Close(c.fd) }

// --- periph.io GPIO ---

// hostOnce ensures periph.io host.Init() is called exactly once per process.
var (
	hostOnce    sync.Once
	hostInitErr error
)

func ensureHostInit() error {
	hostOnce.Do(func() {
		if _, err := host.Init(); err != nil {
			hostInitErr = fmt.Errorf("periph host.Init: %w", err)
		}
	})
	return hostInitErr
}

// periphOutput wraps a periph.io pin as an OutputPin (TRST).
type periphOutput struct{ pin gpio.PinIO }

func (p *periphOutput) Out(high bool) error { return p.pin.Out(gpio.Level(high)) }
func (p *periphOutput) Close() error        { return nil }

// periphInterrupt wraps a periph.io input pin as an InterruptPin (INT).
// Edge detection is not available on gokrazy, so WaitFalling polls at 10 ms.
// For a touch UI on an e-paper display (300 ms refresh), 10 ms latency is negligible.
type periphInterrupt struct {
	pin  gpio.PinIn
	last gpio.Level
}

// WaitFalling blocks until a High→Low transition is detected or ctx is cancelled.
func (p *periphInterrupt) WaitFalling(ctx context.Context) error {
	t := time.NewTicker(10 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			current := p.pin.Read()
			if p.last == gpio.High && current == gpio.Low {
				p.last = current
				return nil
			}
			p.last = current
		}
	}
}

func (p *periphInterrupt) Close() error { return nil }

// openGPIOOutput opens a BCM-numbered pin as an output via periph.io.
func openGPIOOutput(pin int) (*periphOutput, error) {
	if err := ensureHostInit(); err != nil {
		return nil, err
	}
	p := gpioreg.ByName(fmt.Sprintf("GPIO%d", pin))
	if p == nil {
		return nil, fmt.Errorf("gpio%d: pin not found (periph.io)", pin)
	}
	if err := p.Out(gpio.Low); err != nil {
		return nil, fmt.Errorf("gpio%d set output: %w", pin, err)
	}
	return &periphOutput{pin: p}, nil
}

// openGPIOInterrupt opens a BCM-numbered pin as a polled interrupt input via periph.io.
func openGPIOInterrupt(pin int) (*periphInterrupt, error) {
	if err := ensureHostInit(); err != nil {
		return nil, err
	}
	p := gpioreg.ByName(fmt.Sprintf("GPIO%d", pin))
	if p == nil {
		return nil, fmt.Errorf("gpio%d: pin not found (periph.io)", pin)
	}
	if err := p.In(gpio.PullNoChange, gpio.NoEdge); err != nil {
		return nil, fmt.Errorf("gpio%d set input: %w", pin, err)
	}
	return &periphInterrupt{pin: p, last: p.Read()}, nil
}
