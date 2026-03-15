package touch

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
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

// --- Linux GPIO output (sysfs) ---

const gpioSysfsDelay = 50 * time.Millisecond

type linuxGPIOOutput struct {
	pin  int
	file *os.File
}

func openGPIOOutput(pin int) (*linuxGPIOOutput, error) {
	_ = os.WriteFile("/sys/class/gpio/export", []byte(strconv.Itoa(pin)), 0)
	time.Sleep(gpioSysfsDelay)
	if err := os.WriteFile(fmt.Sprintf("/sys/class/gpio/gpio%d/direction", pin), []byte("out"), 0); err != nil {
		return nil, fmt.Errorf("gpio%d set direction out: %w", pin, err)
	}
	f, err := os.OpenFile(fmt.Sprintf("/sys/class/gpio/gpio%d/value", pin), os.O_WRONLY, 0)
	if err != nil {
		return nil, fmt.Errorf("gpio%d open value: %w", pin, err)
	}
	return &linuxGPIOOutput{pin: pin, file: f}, nil
}

func (g *linuxGPIOOutput) Out(high bool) error {
	v := []byte("0")
	if high {
		v = []byte("1")
	}
	_, err := g.file.WriteAt(v, 0)
	return err
}

func (g *linuxGPIOOutput) Close() error { return g.file.Close() }

// --- Linux GPIO interrupt (sysfs + epoll, falling edge) ---

type linuxGPIOInterrupt struct {
	pin   int
	epfd  int
	valfd int
}

func openGPIOInterrupt(pin int) (*linuxGPIOInterrupt, error) {
	_ = os.WriteFile("/sys/class/gpio/export", []byte(strconv.Itoa(pin)), 0)
	time.Sleep(gpioSysfsDelay)
	if err := os.WriteFile(fmt.Sprintf("/sys/class/gpio/gpio%d/direction", pin), []byte("in"), 0); err != nil {
		return nil, fmt.Errorf("gpio%d int direction: %w", pin, err)
	}
	if err := os.WriteFile(fmt.Sprintf("/sys/class/gpio/gpio%d/edge", pin), []byte("falling"), 0); err != nil {
		return nil, fmt.Errorf("gpio%d int edge: %w", pin, err)
	}
	valfd, err := unix.Open(fmt.Sprintf("/sys/class/gpio/gpio%d/value", pin), unix.O_RDONLY|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, fmt.Errorf("gpio%d int open: %w", pin, err)
	}
	epfd, err := unix.EpollCreate1(0)
	if err != nil {
		unix.Close(valfd)
		return nil, fmt.Errorf("epoll create: %w", err)
	}
	ev := unix.EpollEvent{Events: unix.EPOLLIN | unix.EPOLLPRI | unix.EPOLLET, Fd: int32(valfd)}
	if err := unix.EpollCtl(epfd, unix.EPOLL_CTL_ADD, valfd, &ev); err != nil {
		unix.Close(valfd)
		unix.Close(epfd)
		return nil, fmt.Errorf("epoll ctl: %w", err)
	}
	buf := make([]byte, 1)
	unix.Read(valfd, buf) //nolint:errcheck — consume initial state
	return &linuxGPIOInterrupt{pin: pin, epfd: epfd, valfd: valfd}, nil
}

// WaitFalling blocks until a falling edge or ctx cancellation.
// Uses 100ms epoll timeout to periodically check ctx.
func (g *linuxGPIOInterrupt) WaitFalling(ctx context.Context) error {
	events := make([]unix.EpollEvent, 1)
	for {
		n, err := unix.EpollWait(g.epfd, events, 100)
		if err != nil && err != unix.EINTR {
			return fmt.Errorf("epoll wait: %w", err)
		}
		if n > 0 {
			buf := make([]byte, 1)
			unix.Pread(g.valfd, buf, 0) //nolint:errcheck
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}
}

func (g *linuxGPIOInterrupt) Close() error {
	unix.Close(g.epfd) //nolint:errcheck
	return unix.Close(g.valfd)
}
