# epaper packages Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement three Go packages (`epaper/epd`, `epaper/touch`, `epaper/canvas`) for the Waveshare 2.13" Touch e-Paper HAT on gokrazy/Raspberry Pi Zero 2W.

**Architecture:** Raw Linux ioctl (sysfs GPIO + `/dev/spidev` + `/dev/i2c`) with injectable HAL interfaces for testability. Each package is independent; application code wires them together. TDD throughout — write the fake-based test first, then implement.

**Tech Stack:** Go 1.26, `golang.org/x/sys/unix` (already in go.mod), `golang.org/x/image/font/opentype` (new, must `go get`), stdlib `image`, `image/color`, `image/png`.

**Spec:** `docs/superpowers/specs/2026-03-14-epaper-packages-design.md`
**Hardware ref:** `github.com/waveshare/Touch_e-Paper_HAT` (C source, `c/lib/EPD/EPD_2in13_V4.c`, `c/lib/Driver/GT1151.c`)

---

## Chunk 1: epaper/epd — Display Driver

### Task 1: Scaffold package + HAL interfaces

**Files:**
- Create: `epaper/epd/hal.go`
- Create: `epaper/epd/epd.go` (skeleton only)

- [ ] **Step 1: Create the package directory and `hal.go`**

```go
// epaper/epd/hal.go
package epd

// SPIConn is a write-only SPI connection.
type SPIConn interface {
	Tx(w []byte) error
}

// OutputPin drives a GPIO signal (RST, DC, CS).
type OutputPin interface {
	Out(high bool) error
}

// InputPin reads a GPIO input (BUSY).
type InputPin interface {
	Read() bool
}
```

- [ ] **Step 2: Create skeleton `epd.go` with constants and empty Display**

```go
// epaper/epd/epd.go
package epd

import "image"

// Width is the horizontal axis (fast-scan): 122 pixels per row = 16 bytes/row.
// Height is the vertical axis: 250 rows.
const (
	Width      = 122
	Height     = 250
	BufferSize = ((Width + 7) / 8) * Height // 4000 bytes
)

// Mode selects the refresh strategy.
type Mode uint8

const (
	ModeFull    Mode = iota // ~2s, best quality
	ModePartial             // ~0.3s, partial update
	ModeFast                // ~0.5s, fast full update
)

// Display drives the EPD_2in13_V4 e-ink panel.
type Display struct {
	spi          SPIConn
	rst, dc, cs  OutputPin
	busy         InputPin
	closers      []func() error
}

// Config holds the Linux device paths and BCM pin numbers.
type Config struct {
	SPIDevice string  // e.g. "/dev/spidev0.0"
	SPISpeed  uint32  // e.g. 4_000_000
	PinRST    int
	PinDC     int
	PinCS     int
	PinBUSY   int
}

// newDisplay creates a Display from injected HAL components (used in tests).
func newDisplay(spi SPIConn, rst, dc, cs OutputPin, busy InputPin) *Display {
	return &Display{spi: spi, rst: rst, dc: dc, cs: cs, busy: busy}
}

// Placeholder stubs — implemented in subsequent tasks.
func (d *Display) Init(m Mode) error                                    { return nil }
func (d *Display) DisplayFull(buf []byte) error                         { return nil }
func (d *Display) DisplayPartial(r image.Rectangle, buf []byte) error   { return nil }
func (d *Display) DisplayFast(buf []byte) error                         { return nil }
func (d *Display) Sleep() error                                         { return nil }
func (d *Display) Close() error                                         { return nil }
```

- [ ] **Step 3: Verify it compiles**

```bash
go build ./epaper/epd/
```
Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add epaper/epd/
git commit -m "feat(epd): scaffold package, HAL interfaces, Display skeleton"
```

---

### Task 2: Linux SPI implementation

**Files:**
- Modify: `epaper/epd/hal.go` (add `linuxSPI`)

- [ ] **Step 1: Write the failing test**

```go
// epaper/epd/epd_test.go
package epd

import (
	"testing"
)

// fakeSPI captures all bytes sent via Tx.
type fakeSPI struct{ log [][]byte }

func (f *fakeSPI) Tx(w []byte) error {
	f.log = append(f.log, append([]byte(nil), w...))
	return nil
}

// fakeOutputPin records the last value written.
type fakeOutputPin struct{ last bool }

func (f *fakeOutputPin) Out(high bool) error { f.last = high; return nil }

// fakeInputPin returns a configurable value.
type fakeInputPin struct{ val bool }

func (f *fakeInputPin) Read() bool { return f.val }

func TestNewDisplay(t *testing.T) {
	spi := &fakeSPI{}
	rst := &fakeOutputPin{}
	dc := &fakeOutputPin{}
	cs := &fakeOutputPin{}
	busy := &fakeInputPin{}

	d := newDisplay(spi, rst, dc, cs, busy)
	if d == nil {
		t.Fatal("newDisplay returned nil")
	}
}
```

- [ ] **Step 2: Run to verify it passes (it's a pure constructor test)**

```bash
go test ./epaper/epd/ -run TestNewDisplay -v
```
Expected: PASS.

- [ ] **Step 3: Add `linuxSPI` to `hal.go`** and a compile-time size check test

```go
// append to epaper/epd/hal.go

import (
	"fmt"
	"unsafe"
	"golang.org/x/sys/unix"
)

// spiIOCTransfer mirrors the kernel struct spi_ioc_transfer (64-bit ARM, 32 bytes).
// The anonymous [5]byte field intentionally zeroes cs_change, tx_nbits, rx_nbits,
// word_delay_usecs and pad — all unused for this write-only e-ink use case.
type spiIOCTransfer struct {
	txBuf       uint64
	rxBuf       uint64
	length      uint32
	speedHz     uint32
	delayUsecs  uint16
	bitsPerWord uint8
	_           [5]byte // cs_change, tx_nbits, rx_nbits, word_delay_usecs, pad
}

// Compile-time assertion: struct must be exactly 32 bytes on ARM64.
var _ [32]byte = [unsafe.Sizeof(spiIOCTransfer{})]byte{}

// spiIOCMessage1 = _IOW('k', 0, spi_ioc_transfer) = (1<<30)|(32<<16)|('k'<<8)|0 = 0x40206b00
// Valid for ARM64 only (sizeof=32). This file is ARM64-targeted (gokrazy on Pi Zero 2W).
const spiIOCMessage1 = 0x40206b00

type linuxSPI struct {
	fd    int
	speed uint32
}

func openSPI(device string, speed uint32) (*linuxSPI, error) {
	fd, err := unix.Open(device, unix.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", device, err)
	}
	mode := uint8(0) // SPI_MODE_0
	if _, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), 0x40016b01, uintptr(unsafe.Pointer(&mode))); errno != 0 {
		unix.Close(fd)
		return nil, fmt.Errorf("set SPI mode: %w", errno)
	}
	bits := uint8(8)
	if _, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), 0x40016b03, uintptr(unsafe.Pointer(&bits))); errno != 0 {
		unix.Close(fd)
		return nil, fmt.Errorf("set SPI bits: %w", errno)
	}
	return &linuxSPI{fd: fd, speed: speed}, nil
}

func (s *linuxSPI) Tx(w []byte) error {
	if len(w) == 0 {
		return nil
	}
	t := spiIOCTransfer{
		txBuf:       uint64(uintptr(unsafe.Pointer(&w[0]))),
		length:      uint32(len(w)),
		speedHz:     s.speed,
		bitsPerWord: 8,
	}
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(s.fd), uintptr(spiIOCMessage1), uintptr(unsafe.Pointer(&t)))
	if errno != 0 {
		return fmt.Errorf("SPI Tx: %w", errno)
	}
	return nil
}

func (s *linuxSPI) Close() error { return unix.Close(s.fd) }
```

- [ ] **Step 4: Add a TDD test for linuxSPI error path (no hardware needed)**

```go
// append to epd_test.go
func TestOpenSPIFailsOnMissingDevice(t *testing.T) {
	_, err := openSPI("/dev/spidev_nonexistent", 4_000_000)
	if err == nil {
		t.Fatal("expected error opening non-existent SPI device")
	}
}
```

```bash
go test ./epaper/epd/ -run TestOpenSPIFailsOnMissingDevice -v
```
Expected: PASS (error returned, not panic).

- [ ] **Step 5: Verify it compiles**

```bash
go build ./epaper/epd/
```

- [ ] **Step 6: Commit**

```bash
git add epaper/epd/hal.go epaper/epd/epd_test.go
git commit -m "feat(epd): Linux SPI implementation via ioctl"
```

---

### Task 3: Linux GPIO implementation (sysfs)

**Files:**
- Modify: `epaper/epd/hal.go` (add `linuxGPIOOutput`, `linuxGPIOInput`)

- [ ] **Step 1: Add GPIO implementations to `hal.go`**

```go
// append to epaper/epd/hal.go
import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type linuxGPIOOutput struct {
	pin  int
	file *os.File
}

func openGPIOOutput(pin int) (*linuxGPIOOutput, error) {
	if err := os.WriteFile("/sys/class/gpio/export", []byte(strconv.Itoa(pin)), 0); err != nil {
		// ignore EBUSY (already exported)
		if !os.IsExist(err) {
			// best-effort: continue
		}
	}
	time.Sleep(50 * time.Millisecond) // sysfs node may take a moment
	dir := fmt.Sprintf("/sys/class/gpio/gpio%d/direction", pin)
	if err := os.WriteFile(dir, []byte("out"), 0); err != nil {
		return nil, fmt.Errorf("gpio%d set direction out: %w", pin, err)
	}
	val := fmt.Sprintf("/sys/class/gpio/gpio%d/value", pin)
	f, err := os.OpenFile(val, os.O_WRONLY, 0)
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

type linuxGPIOInput struct {
	pin  int
	file *os.File
}

func openGPIOInput(pin int) (*linuxGPIOInput, error) {
	if err := os.WriteFile("/sys/class/gpio/export", []byte(strconv.Itoa(pin)), 0); err != nil {
		// ignore EBUSY
	}
	time.Sleep(50 * time.Millisecond)
	dir := fmt.Sprintf("/sys/class/gpio/gpio%d/direction", pin)
	if err := os.WriteFile(dir, []byte("in"), 0); err != nil {
		return nil, fmt.Errorf("gpio%d set direction in: %w", pin, err)
	}
	val := fmt.Sprintf("/sys/class/gpio/gpio%d/value", pin)
	f, err := os.OpenFile(val, os.O_RDONLY, 0)
	if err != nil {
		return nil, fmt.Errorf("gpio%d open value: %w", pin, err)
	}
	return &linuxGPIOInput{pin: pin, file: f}, nil
}

func (g *linuxGPIOInput) Read() bool {
	buf := make([]byte, 1)
	g.file.ReadAt(buf, 0)
	return buf[0] == '1'
}

func (g *linuxGPIOInput) Close() error { return g.file.Close() }
```

- [ ] **Step 2: Implement `New(Config)`**

```go
// append to epaper/epd/epd.go

import "fmt"

func New(cfg Config) (*Display, error) {
	spi, err := openSPI(cfg.SPIDevice, cfg.SPISpeed)
	if err != nil {
		return nil, fmt.Errorf("epd New: %w", err)
	}
	rst, err := openGPIOOutput(cfg.PinRST)
	if err != nil { spi.Close(); return nil, fmt.Errorf("epd New RST: %w", err) }
	dc, err := openGPIOOutput(cfg.PinDC)
	if err != nil { spi.Close(); rst.Close(); return nil, fmt.Errorf("epd New DC: %w", err) }
	cs, err := openGPIOOutput(cfg.PinCS)
	if err != nil { spi.Close(); rst.Close(); dc.Close(); return nil, fmt.Errorf("epd New CS: %w", err) }
	busy, err := openGPIOInput(cfg.PinBUSY)
	if err != nil { spi.Close(); rst.Close(); dc.Close(); cs.Close(); return nil, fmt.Errorf("epd New BUSY: %w", err) }

	d := newDisplay(spi, rst, dc, cs, busy)
	d.closers = []func() error{spi.Close, rst.Close, dc.Close, cs.Close, busy.Close}
	return d, nil
}
```

- [ ] **Step 3: Compile check**

```bash
go build ./epaper/epd/
```

- [ ] **Step 4: Commit**

```bash
git add epaper/epd/hal.go epaper/epd/epd.go
git commit -m "feat(epd): Linux GPIO sysfs implementation + New(Config)"
```

---

### Task 4: SPI primitives + Init(ModeFull)

**Files:**
- Modify: `epaper/epd/epd.go`
- Modify: `epaper/epd/epd_test.go`

- [ ] **Step 1: Write the failing test**

```go
// in epd_test.go
func TestInitFullSendsReset(t *testing.T) {
	spi := &fakeSPI{}
	busy := &fakeInputPin{val: false} // BUSY=low means ready
	d := newDisplay(spi, &fakeOutputPin{}, &fakeOutputPin{}, &fakeOutputPin{}, busy)

	if err := d.Init(ModeFull); err != nil {
		t.Fatalf("Init: %v", err)
	}
	// After init, at least one command must have been sent
	if len(spi.log) == 0 {
		t.Fatal("expected SPI commands, got none")
	}
	// First meaningful command should be software reset (0x12)
	found := false
	for _, pkt := range spi.log {
		if len(pkt) == 1 && pkt[0] == 0x12 {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected software reset command 0x12 in SPI log")
	}
}
```

- [ ] **Step 2: Run — expect FAIL**

```bash
go test ./epaper/epd/ -run TestInitFullSendsReset -v
```
Expected: FAIL (Init is a stub).

- [ ] **Step 3: Implement SPI primitives and Init(ModeFull)**

```go
// append internal helpers to epd.go
import "time"

func (d *Display) sendCommand(cmd byte) {
	d.dc.Out(false)
	d.cs.Out(false)
	d.spi.Tx([]byte{cmd})
	d.cs.Out(true)
}

func (d *Display) sendData(data ...byte) {
	d.dc.Out(true)
	d.cs.Out(false)
	d.spi.Tx(data)
	d.cs.Out(true)
}

func (d *Display) waitBusy() {
	for d.busy.Read() {
		time.Sleep(10 * time.Millisecond)
	}
}

func (d *Display) reset() {
	d.rst.Out(true)
	time.Sleep(20 * time.Millisecond)
	d.rst.Out(false)
	time.Sleep(2 * time.Millisecond)
	d.rst.Out(true)
	time.Sleep(20 * time.Millisecond)
}

func (d *Display) setWindow(xStart, yStart, xEnd, yEnd int) {
	d.sendCommand(0x44)
	d.sendData(byte(xStart/8), byte(xEnd/8))
	d.sendCommand(0x45)
	d.sendData(byte(yStart), byte(yStart>>8), byte(yEnd), byte(yEnd>>8))
}

func (d *Display) setCursor(x, y int) {
	d.sendCommand(0x4E)
	d.sendData(byte(x / 8))
	d.sendCommand(0x4F)
	d.sendData(byte(y), byte(y>>8))
}

func (d *Display) Init(m Mode) error {
	switch m {
	case ModeFull:
		d.reset()
		d.waitBusy()
		d.sendCommand(0x12) // software reset
		d.waitBusy()
		d.sendCommand(0x01) // driver output control
		d.sendData(byte(Height-1), byte((Height-1)>>8), 0x00)
		d.sendCommand(0x11) // data entry mode: X inc, Y inc
		d.sendData(0x03)
		d.setWindow(0, 0, Width-1, Height-1)
		d.sendCommand(0x3C) // border waveform
		d.sendData(0x05)
		d.sendCommand(0x21) // display update control
		d.sendData(0x00, 0x80)
		d.sendCommand(0x18) // temperature sensor: internal
		d.sendData(0x80)
		d.setCursor(0, 0)
		d.waitBusy()
	case ModePartial:
		d.reset()
		d.sendCommand(0x3C)
		d.sendData(0x80)
		d.sendCommand(0x01)
		d.sendData(byte(Height-1), byte((Height-1)>>8), 0x00)
		d.sendCommand(0x11)
		d.sendData(0x03)
		d.setWindow(0, 0, Width-1, Height-1)
		d.setCursor(0, 0)
		d.waitBusy()
	case ModeFast:
		d.reset()
		d.waitBusy()
		d.sendCommand(0x12)
		d.waitBusy()
		d.sendCommand(0x18)
		d.sendData(0x80)
		d.sendCommand(0x22)
		d.sendData(0xB1)
		d.sendCommand(0x20)
		d.waitBusy()
		d.sendCommand(0x1A)
		d.sendData(0x64, 0x00)
		d.sendCommand(0x22)
		d.sendData(0x91)
		d.sendCommand(0x20)
		d.waitBusy()
	}
	return nil
}
```

- [ ] **Step 4: Run — expect PASS**

```bash
go test ./epaper/epd/ -run TestInitFullSendsReset -v
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add epaper/epd/epd.go epaper/epd/epd_test.go
git commit -m "feat(epd): SPI primitives + Init(ModeFull/Partial/Fast)"
```

---

### Task 5: DisplayFull, DisplayFast, Sleep, Close

**Files:**
- Modify: `epaper/epd/epd.go`
- Modify: `epaper/epd/epd_test.go`

- [ ] **Step 1: Write failing tests**

```go
// in epd_test.go
func TestDisplayFullSendsBuffer(t *testing.T) {
	spi := &fakeSPI{}
	busy := &fakeInputPin{val: false}
	d := newDisplay(spi, &fakeOutputPin{}, &fakeOutputPin{}, &fakeOutputPin{}, busy)
	d.Init(ModeFull)
	spi.log = nil // reset log

	buf := make([]byte, BufferSize)
	buf[0] = 0xAB
	if err := d.DisplayFull(buf); err != nil {
		t.Fatal(err)
	}
	// The data byte 0xAB must appear in the SPI log
	found := false
	for _, pkt := range spi.log {
		for _, b := range pkt {
			if b == 0xAB {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected buffer content 0xAB in SPI log")
	}
}

func TestSleepSendsCommand(t *testing.T) {
	spi := &fakeSPI{}
	busy := &fakeInputPin{val: false}
	d := newDisplay(spi, &fakeOutputPin{}, &fakeOutputPin{}, &fakeOutputPin{}, busy)
	spi.log = nil
	d.Sleep()
	found := false
	for _, pkt := range spi.log {
		if len(pkt) == 1 && pkt[0] == 0x10 {
			found = true
		}
	}
	if !found {
		t.Error("expected deep sleep command 0x10")
	}
}

func TestCloseCallsClosers(t *testing.T) {
	closed := 0
	closer := func() error { closed++; return nil }
	d := newDisplay(&fakeSPI{}, &fakeOutputPin{}, &fakeOutputPin{}, &fakeOutputPin{}, &fakeInputPin{})
	d.closers = []func() error{closer, closer, closer}
	if err := d.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if closed != 3 {
		t.Errorf("expected 3 closers called, got %d", closed)
	}
}
```

- [ ] **Step 2: Run — expect FAIL**

```bash
go test ./epaper/epd/ -run "TestDisplayFullSendsBuffer|TestSleepSendsCommand" -v
```

- [ ] **Step 3: Implement**

```go
// in epd.go

func (d *Display) DisplayFull(buf []byte) error {
	d.sendCommand(0x24) // write RAM
	d.sendData(buf...)
	d.sendCommand(0x22)
	d.sendData(0xF7)
	d.sendCommand(0x20)
	d.waitBusy()
	return nil
}

func (d *Display) DisplayFast(buf []byte) error {
	d.sendCommand(0x24)
	d.sendData(buf...)
	d.sendCommand(0x22)
	d.sendData(0xC7)
	d.sendCommand(0x20)
	d.waitBusy()
	return nil
}

func (d *Display) Sleep() error {
	d.sendCommand(0x10)
	d.sendData(0x01)
	time.Sleep(100 * time.Millisecond)
	return nil
}

func (d *Display) Close() error {
	var first error
	for _, fn := range d.closers {
		if err := fn(); err != nil && first == nil {
			first = err
		}
	}
	return first
}
```

- [ ] **Step 4: Run — expect PASS**

```bash
go test ./epaper/epd/ -v
```
Expected: all tests PASS.

- [ ] **Step 5: Commit**

```bash
git add epaper/epd/epd.go epaper/epd/epd_test.go
git commit -m "feat(epd): DisplayFull, DisplayFast, Sleep, Close"
```

---

### Task 6: DisplayPartial

**Files:**
- Modify: `epaper/epd/epd.go`
- Modify: `epaper/epd/epd_test.go`

- [ ] **Step 1: Write failing test**

```go
// in epd_test.go
func TestDisplayPartialSendsRegionBuffer(t *testing.T) {
	spi := &fakeSPI{}
	busy := &fakeInputPin{val: false}
	d := newDisplay(spi, &fakeOutputPin{}, &fakeOutputPin{}, &fakeOutputPin{}, busy)
	d.Init(ModePartial)
	spi.log = nil

	// 10×10 pixel region at (0,0): ceil(10/8)=2 bytes/row × 10 rows = 20 bytes
	r := image.Rect(0, 0, 10, 10)
	buf := make([]byte, 2*10)
	buf[0] = 0xCD
	if err := d.DisplayPartial(r, buf); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, pkt := range spi.log {
		for _, b := range pkt {
			if b == 0xCD {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected partial buffer content 0xCD in SPI log")
	}
}
```

- [ ] **Step 2: Run — expect FAIL**

```bash
go test ./epaper/epd/ -run TestDisplayPartialSendsRegionBuffer -v
```

- [ ] **Step 3: Implement**

```go
// in epd.go

func (d *Display) DisplayPartial(r image.Rectangle, buf []byte) error {
	d.sendCommand(0x44) // set RAM X window
	d.sendData(byte(r.Min.X/8), byte((r.Max.X-1)/8))
	d.sendCommand(0x45) // set RAM Y window
	d.sendData(byte(r.Min.Y), byte(r.Min.Y>>8), byte(r.Max.Y-1), byte((r.Max.Y-1)>>8))
	d.sendCommand(0x4E) // cursor X
	d.sendData(byte(r.Min.X / 8))
	d.sendCommand(0x4F) // cursor Y
	d.sendData(byte(r.Min.Y), byte(r.Min.Y>>8))
	d.sendCommand(0x24) // write RAM
	d.sendData(buf...)
	d.sendCommand(0x22)
	d.sendData(0xFF)
	d.sendCommand(0x20)
	d.waitBusy()
	return nil
}
```

- [ ] **Step 4: Run all epd tests**

```bash
go test ./epaper/epd/ -v
```
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add epaper/epd/epd.go epaper/epd/epd_test.go
git commit -m "feat(epd): DisplayPartial with rectangle region"
```

---

## Chunk 2: epaper/touch — GT1151 Driver

### Task 7: Scaffold + HAL interfaces

**Files:**
- Create: `epaper/touch/hal.go`
- Create: `epaper/touch/touch.go`
- Create: `epaper/touch/touch_test.go`

- [ ] **Step 1: Create `hal.go`**

```go
// epaper/touch/hal.go
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

// --- Linux I2C implementation ---

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
	data := i2cRDWRIoctlData{msgs: uintptr(unsafe.Pointer(&msgs[0])), nmsgs: uint32(len(msgs))}
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(c.fd), i2cRDWR, uintptr(unsafe.Pointer(&data)))
	if errno != 0 {
		return fmt.Errorf("i2c Tx: %w", errno)
	}
	return nil
}

func (c *linuxI2C) Close() error { return unix.Close(c.fd) }

// --- Linux GPIO output (sysfs) --- reuse same pattern as epd/hal.go ---

type linuxGPIOOutput struct{ pin int; file *os.File }

func openGPIOOutput(pin int) (*linuxGPIOOutput, error) {
	os.WriteFile("/sys/class/gpio/export", []byte(strconv.Itoa(pin)), 0)
	time.Sleep(50 * time.Millisecond)
	os.WriteFile(fmt.Sprintf("/sys/class/gpio/gpio%d/direction", pin), []byte("out"), 0)
	f, err := os.OpenFile(fmt.Sprintf("/sys/class/gpio/gpio%d/value", pin), os.O_WRONLY, 0)
	if err != nil {
		return nil, fmt.Errorf("gpio%d output: %w", pin, err)
	}
	return &linuxGPIOOutput{pin: pin, file: f}, nil
}

func (g *linuxGPIOOutput) Out(high bool) error {
	v := []byte("0")
	if high { v = []byte("1") }
	_, err := g.file.WriteAt(v, 0)
	return err
}

func (g *linuxGPIOOutput) Close() error { return g.file.Close() }

// --- Linux GPIO interrupt (sysfs + epoll on falling edge) ---

type linuxGPIOInterrupt struct{ pin int; epfd int; valfd int }

func openGPIOInterrupt(pin int) (*linuxGPIOInterrupt, error) {
	os.WriteFile("/sys/class/gpio/export", []byte(strconv.Itoa(pin)), 0)
	time.Sleep(50 * time.Millisecond)
	os.WriteFile(fmt.Sprintf("/sys/class/gpio/gpio%d/direction", pin), []byte("in"), 0)
	os.WriteFile(fmt.Sprintf("/sys/class/gpio/gpio%d/edge", pin), []byte("falling"), 0)

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
		unix.Close(valfd); unix.Close(epfd)
		return nil, fmt.Errorf("epoll ctl: %w", err)
	}
	// consume initial state
	buf := make([]byte, 1)
	unix.Read(valfd, buf)
	return &linuxGPIOInterrupt{pin: pin, epfd: epfd, valfd: valfd}, nil
}

func (g *linuxGPIOInterrupt) WaitFalling(ctx context.Context) error {
	events := make([]unix.EpollEvent, 1)
	for {
		n, err := unix.EpollWait(g.epfd, events, 100) // 100ms timeout to check ctx
		if err != nil && err != unix.EINTR {
			return fmt.Errorf("epoll wait: %w", err)
		}
		if n > 0 {
			buf := make([]byte, 1)
			unix.Pread(g.valfd, buf, 0)
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
	unix.Close(g.epfd)
	return unix.Close(g.valfd)
}
```

- [ ] **Step 2: Create `touch.go` skeleton**

```go
// epaper/touch/touch.go
package touch

import (
	"context"
	"fmt"
	"time"
)

// TouchPoint is a single contact point from the GT1151.
type TouchPoint struct {
	ID   uint8
	X, Y uint16
	Size uint8
}

// TouchEvent carries all active touch points at a moment in time.
type TouchEvent struct {
	Points []TouchPoint
	Time   time.Time
}

// Config holds device paths and BCM pin numbers.
type Config struct {
	I2CDevice string // e.g. "/dev/i2c-1"
	I2CAddr   uint16 // 0x14
	PinTRST   int    // BCM 22
	PinINT    int    // BCM 27
}

// Detector reads touch events from the GT1151.
type Detector struct {
	i2c    I2CConn
	trst   OutputPin
	intPin InterruptPin
	err    error
	closers []func() error
}

func newDetector(i2c I2CConn, trst OutputPin, intPin InterruptPin) *Detector {
	return &Detector{i2c: i2c, trst: trst, intPin: intPin}
}

func New(cfg Config) (*Detector, error) {
	i2c, err := openI2C(cfg.I2CDevice, cfg.I2CAddr)
	if err != nil {
		return nil, fmt.Errorf("touch New: %w", err)
	}
	trst, err := openGPIOOutput(cfg.PinTRST)
	if err != nil { i2c.Close(); return nil, fmt.Errorf("touch New TRST: %w", err) }
	intPin, err := openGPIOInterrupt(cfg.PinINT)
	if err != nil { i2c.Close(); trst.Close(); return nil, fmt.Errorf("touch New INT: %w", err) }

	d := newDetector(i2c, trst, intPin)
	d.closers = []func() error{i2c.Close, trst.Close, intPin.Close}
	return d, nil
}

func (d *Detector) Start(ctx context.Context) (<-chan TouchEvent, error) { return nil, nil }
func (d *Detector) Err() error                                           { return d.err }
func (d *Detector) Close() error                                         { return nil }
```

- [ ] **Step 3: Create `touch_test.go` with fakes**

```go
// epaper/touch/touch_test.go
package touch

import (
	"context"
	"testing"
)

type fakeI2C struct {
	responses [][]byte
	idx       int
}

func (f *fakeI2C) Tx(w, r []byte) error {
	if f.idx < len(f.responses) {
		copy(r, f.responses[f.idx])
		f.idx++
	}
	return nil
}

type fakeOutputPin struct{ last bool }

func (f *fakeOutputPin) Out(high bool) error { f.last = high; return nil }

type fakeINT struct{ trigger chan struct{} }

func (f *fakeINT) WaitFalling(ctx context.Context) error {
	select {
	case <-f.trigger:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func TestNewDetector(t *testing.T) {
	d := newDetector(&fakeI2C{}, &fakeOutputPin{}, &fakeINT{trigger: make(chan struct{})})
	if d == nil {
		t.Fatal("newDetector returned nil")
	}
}
```

- [ ] **Step 4: Verify build**

```bash
go build ./epaper/touch/
go test ./epaper/touch/ -run TestNewDetector -v
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add epaper/touch/
git commit -m "feat(touch): scaffold package, HAL interfaces, Linux I2C/GPIO/interrupt"
```

---

### Task 8: GT1151 init + Start()

**Files:**
- Modify: `epaper/touch/touch.go`
- Modify: `epaper/touch/touch_test.go`

- [ ] **Step 1: Write failing test**

```go
// in touch_test.go
func TestStartReceivesTouchEvent(t *testing.T) {
	intCh := make(chan struct{}, 1)
	i2c := &fakeI2C{
		responses: [][]byte{
			{0x39, 0x35, 0x30, 0x31},  // product ID "9501" (GT1151 standard) at 0x8140
			{0x81},                     // 0x814E: flag=1 (bit7), count=1 (bits 3:0)
			{0x01, 0x1E, 0x00, 0x3C, 0x00, 0x08, 0x00, 0x00}, // point 0: ID=1, X=30, Y=60, S=8
		},
	}
	d := newDetector(i2c, &fakeOutputPin{}, &fakeINT{trigger: intCh})

	ctx, cancel := context.WithCancel(context.Background())
	ch, err := d.Start(ctx)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	intCh <- struct{}{} // simulate falling edge

	select {
	case evt := <-ch:
		if len(evt.Points) != 1 {
			t.Fatalf("expected 1 point, got %d", len(evt.Points))
		}
		if evt.Points[0].X != 30 || evt.Points[0].Y != 60 {
			t.Errorf("expected X=30 Y=60, got X=%d Y=%d", evt.Points[0].X, evt.Points[0].Y)
		}
	case <-ctx.Done():
		t.Fatal("timeout")
	}
	cancel()
}
```

- [ ] **Step 2: Run — expect FAIL**

```bash
go test ./epaper/touch/ -run TestStartReceivesTouchEvent -v
```

- [ ] **Step 3: Implement GT1151 init + event loop**

```go
// replace stubs in touch.go

import "unsafe" // for unsafe.Pointer if needed

const (
	gt1151Addr     = 0x14
	regProductID   = 0x8140
	regTouchFlag   = 0x814E
	regTouchData   = 0x814F
)

func (d *Detector) readReg(reg uint16, n int) ([]byte, error) {
	w := []byte{byte(reg >> 8), byte(reg)}
	r := make([]byte, n)
	if err := d.i2c.Tx(w, r); err != nil {
		return nil, err
	}
	return r, nil
}

func (d *Detector) writeReg(reg uint16, val byte) error {
	return d.i2c.Tx([]byte{byte(reg >> 8), byte(reg), val}, nil)
}

func (d *Detector) gt1151Reset() {
	d.trst.Out(true)
	time.Sleep(100 * time.Millisecond)
	d.trst.Out(false)
	time.Sleep(100 * time.Millisecond)
	d.trst.Out(true)
	time.Sleep(200 * time.Millisecond) // 200ms stabilisation from Waveshare C driver (GT1151.c)
}

func (d *Detector) Start(ctx context.Context) (<-chan TouchEvent, error) {
	// Init: reset + verify product ID
	d.gt1151Reset()
	pid, err := d.readReg(regProductID, 4)
	if err != nil {
		return nil, fmt.Errorf("GT1151 init: read product ID: %w", err)
	}
	// GT1151 product ID is "9501" (bytes 0x39,0x35,0x30,0x31).
	// Reject if all zeros (I2C not responding) but allow other variants.
	if pid[0] == 0 && pid[1] == 0 && pid[2] == 0 && pid[3] == 0 {
		return nil, fmt.Errorf("GT1151 init: no response (product ID all zeros)")
	}

	ch := make(chan TouchEvent, 8)
	go func() {
		defer close(ch)
		for {
			if err := d.intPin.WaitFalling(ctx); err != nil {
				return // ctx cancelled
			}

			flagBuf, err := d.readReg(regTouchFlag, 1)
			if err != nil {
				d.err = err
				return
			}
			flag := flagBuf[0]
			if flag&0x80 == 0 {
				continue
			}
			count := int(flag & 0x0F)
			if count < 1 || count > 5 {
				d.writeReg(regTouchFlag, 0x00)
				continue
			}

			data, err := d.readReg(regTouchData, count*8)
			if err != nil {
				d.err = err
				return
			}
			if err := d.writeReg(regTouchFlag, 0x00); err != nil {
				d.err = err
				return
			}

			evt := TouchEvent{Time: time.Now(), Points: make([]TouchPoint, count)}
			for i := 0; i < count; i++ {
				o := i * 8
				evt.Points[i] = TouchPoint{
					ID:   data[o],
					X:    uint16(data[o+1]) | uint16(data[o+2])<<8,
					Y:    uint16(data[o+3]) | uint16(data[o+4])<<8,
					Size: data[o+5],
				}
			}

			select {
			case ch <- evt:
			default: // drop if full
			}
		}
	}()
	return ch, nil
}

func (d *Detector) Close() error {
	var first error
	for _, fn := range d.closers {
		if err := fn(); err != nil && first == nil {
			first = err
		}
	}
	return first
}
```

- [ ] **Step 4: Run all touch tests**

```bash
go test ./epaper/touch/ -v
```
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add epaper/touch/touch.go epaper/touch/touch_test.go
git commit -m "feat(touch): GT1151 init + event goroutine + Start/Err/Close"
```

---

## Chunk 3: epaper/canvas — Drawing Canvas

### Task 9: Canvas struct + draw.Image + SetPixel/Fill/Bytes

**Files:**
- Create: `epaper/canvas/canvas.go`
- Create: `epaper/canvas/canvas_test.go`

- [ ] **Step 1: Write failing tests**

```go
// epaper/canvas/canvas_test.go
package canvas

import (
	"image"
	"image/color"
	"testing"
)

func TestNewCanvas(t *testing.T) {
	c := New(122, 250, Rot0)
	if c == nil {
		t.Fatal("New returned nil")
	}
	if c.Bounds() != (image.Rectangle{Max: image.Point{X: 122, Y: 250}}) {
		t.Errorf("unexpected bounds: %v", c.Bounds())
	}
}

func TestSetPixelAndBytes(t *testing.T) {
	c := New(122, 250, Rot0)
	c.Clear()

	c.SetPixel(0, 0, Black)
	buf := c.Bytes()
	if len(buf) != 4000 {
		t.Fatalf("expected 4000 bytes, got %d", len(buf))
	}
	// Pixel (0,0) is bit 7 of byte 0. Black = 0, white = 1 after Clear.
	if buf[0]&0x80 != 0 {
		t.Errorf("expected bit7 of byte0 = 0 (black), got 0x%02X", buf[0])
	}
}

func TestFill(t *testing.T) {
	c := New(122, 250, Rot0)
	c.Fill(Black)
	buf := c.Bytes()
	for i, b := range buf {
		if b != 0x00 {
			t.Fatalf("byte %d: expected 0x00 (black), got 0x%02X", i, b)
		}
	}
}
```

- [ ] **Step 2: Run — expect FAIL**

```bash
go test ./epaper/canvas/ -run "TestNewCanvas|TestSetPixelAndBytes|TestFill" -v
```

- [ ] **Step 3: Implement `canvas.go`**

```go
// epaper/canvas/canvas.go
package canvas

import (
	"image"
	"image/color"
)

var (
	Black color.Color = color.Gray{Y: 0}
	White color.Color = color.Gray{Y: 255}
)

type Rotation int

const (
	Rot0   Rotation = 0
	Rot90  Rotation = 90
	Rot180 Rotation = 180
	Rot270 Rotation = 270
)

// Canvas is a 1-bit drawing surface implementing draw.Image.
// Backing store is always in physical layout: ((physW+7)/8) × physH bytes.
// Convention: bit=0 → black, bit=1 → white.
type Canvas struct {
	buf          []byte
	physW, physH int
	rot          Rotation
	clip         image.Rectangle
}

func New(physW, physH int, rot Rotation) *Canvas {
	stride := (physW + 7) / 8
	buf := make([]byte, stride*physH)
	// initialise to white (all bits = 1)
	for i := range buf {
		buf[i] = 0xFF
	}
	c := &Canvas{buf: buf, physW: physW, physH: physH, rot: rot}
	c.clip = image.Rect(0, 0, physW, physH)
	return c
}

// logicalSize returns the canvas dimensions in logical (rotated) coordinates.
func (c *Canvas) logicalSize() (w, h int) {
	if c.rot == Rot90 || c.rot == Rot270 {
		return c.physH, c.physW
	}
	return c.physW, c.physH
}

// toPhysical converts logical (x,y) to physical (px,py).
func (c *Canvas) toPhysical(x, y int) (px, py int) {
	switch c.rot {
	case Rot0:
		return x, y
	case Rot90:
		return c.physW - 1 - y, x
	case Rot180:
		return c.physW - 1 - x, c.physH - 1 - y
	case Rot270:
		return y, c.physH - 1 - x
	}
	return x, y
}

// --- draw.Image interface ---

func (c *Canvas) ColorModel() color.Model { return color.GrayModel }

func (c *Canvas) Bounds() image.Rectangle {
	w, h := c.logicalSize()
	return image.Rect(0, 0, w, h)
}

func (c *Canvas) At(x, y int) color.Color {
	px, py := c.toPhysical(x, y)
	stride := (c.physW + 7) / 8
	idx := py*stride + px/8
	if idx < 0 || idx >= len(c.buf) {
		return White
	}
	if c.buf[idx]>>(7-uint(px%8))&1 == 1 {
		return White
	}
	return Black
}

func (c *Canvas) Set(x, y int, col color.Color) {
	c.SetPixel(x, y, col)
}

// --- Public API ---

func isBlack(col color.Color) bool {
	gray, _, _, _ := col.RGBA()
	return gray < 0x8000
}

func (c *Canvas) SetPixel(x, y int, col color.Color) {
	// Clipping in logical space
	lw, lh := c.logicalSize()
	if x < 0 || y < 0 || x >= lw || y >= lh {
		return
	}
	if !image.Pt(x, y).In(c.clip) {
		return
	}
	px, py := c.toPhysical(x, y)
	stride := (c.physW + 7) / 8
	idx := py*stride + px/8
	bit := uint(7 - px%8)
	if isBlack(col) {
		c.buf[idx] &^= 1 << bit
	} else {
		c.buf[idx] |= 1 << bit
	}
}

func (c *Canvas) Fill(col color.Color) {
	var fill byte
	if isBlack(col) {
		fill = 0x00
	} else {
		fill = 0xFF
	}
	for i := range c.buf {
		c.buf[i] = fill
	}
}

func (c *Canvas) Clear() { c.Fill(White) }

func (c *Canvas) SetClip(r image.Rectangle) { c.clip = r }
func (c *Canvas) ClearClip() {
	lw, lh := c.logicalSize()
	c.clip = image.Rect(0, 0, lw, lh)
}

func (c *Canvas) SetRotation(r Rotation) {
	c.rot = r
	c.ClearClip()
}

// Bytes returns a COPY of the physical backing buffer (4000 bytes for 122×250).
func (c *Canvas) Bytes() []byte {
	out := make([]byte, len(c.buf))
	copy(out, c.buf)
	return out
}

// PhysicalRect converts a logical rectangle to physical coordinates.
func (c *Canvas) PhysicalRect(r image.Rectangle) image.Rectangle {
	p1x, p1y := c.toPhysical(r.Min.X, r.Min.Y)
	p2x, p2y := c.toPhysical(r.Max.X-1, r.Max.Y-1)
	// Normalise so Min < Max
	if p1x > p2x { p1x, p2x = p2x, p1x }
	if p1y > p2y { p1y, p2y = p2y, p1y }
	return image.Rect(p1x, p1y, p2x+1, p2y+1)
}

// SubRegion returns a sub-canvas and physical rectangle for DisplayPartial.
// r must be in physical coordinates.
func (c *Canvas) SubRegion(r image.Rectangle) (*Canvas, image.Rectangle) {
	r = r.Intersect(image.Rect(0, 0, c.physW, c.physH))
	stride := (c.physW + 7) / 8
	subStride := (r.Dx() + 7) / 8
	subBuf := make([]byte, subStride*r.Dy())
	for row := 0; row < r.Dy(); row++ {
		for col := 0; col < r.Dx(); col++ {
			srcIdx := (r.Min.Y+row)*stride + (r.Min.X+col)/8
			srcBit := uint(7 - (r.Min.X+col)%8)
			bit := (c.buf[srcIdx] >> srcBit) & 1
			dstIdx := row*subStride + col/8
			dstBit := uint(7 - col%8)
			subBuf[dstIdx] = (subBuf[dstIdx] &^ (1 << dstBit)) | (bit << dstBit)
		}
	}
	sub := &Canvas{buf: subBuf, physW: r.Dx(), physH: r.Dy(), rot: Rot0}
	sub.clip = image.Rect(0, 0, r.Dx(), r.Dy())
	return sub, r
}

// ToImage converts the canvas to *image.Gray (8 bits/pixel).
// NOT concurrent-safe: caller must not draw concurrently.
func (c *Canvas) ToImage() *image.Gray {
	img := image.NewGray(image.Rect(0, 0, c.physW, c.physH))
	stride := (c.physW + 7) / 8
	for y := 0; y < c.physH; y++ {
		for x := 0; x < c.physW; x++ {
			idx := y*stride + x/8
			bit := uint(7 - x%8)
			if (c.buf[idx]>>bit)&1 == 1 {
				img.SetGray(x, y, color.Gray{Y: 255})
			} else {
				img.SetGray(x, y, color.Gray{Y: 0})
			}
		}
	}
	return img
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./epaper/canvas/ -run "TestNewCanvas|TestSetPixelAndBytes|TestFill" -v
```
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add epaper/canvas/canvas.go epaper/canvas/canvas_test.go
git commit -m "feat(canvas): Canvas struct, draw.Image, SetPixel/Fill/Bytes/SubRegion/ToImage"
```

---

### Task 10: Drawing primitives (DrawRect, DrawLine, DrawCircle)

**Files:**
- Create: `epaper/canvas/draw.go`
- Modify: `epaper/canvas/canvas_test.go`

- [ ] **Step 1: Write failing tests**

```go
// in canvas_test.go
func TestDrawRect(t *testing.T) {
	c := New(122, 250, Rot0)
	c.Clear()
	c.DrawRect(image.Rect(10, 10, 20, 20), Black, false)
	// Top-left corner pixel must be black
	if c.At(10, 10) != Black {
		t.Error("expected pixel (10,10) to be black")
	}
	// Interior pixel must be white (outline only)
	if c.At(15, 15) != White {
		t.Error("expected interior pixel (15,15) to be white")
	}
}

func TestDrawLine(t *testing.T) {
	c := New(122, 250, Rot0)
	c.Clear()
	c.DrawLine(0, 0, 10, 0, Black)
	for x := 0; x <= 10; x++ {
		if c.At(x, 0) != Black {
			t.Errorf("expected pixel (%d,0) to be black", x)
		}
	}
}
```

- [ ] **Step 2: Run — expect FAIL**

```bash
go test ./epaper/canvas/ -run "TestDrawRect|TestDrawLine" -v
```

- [ ] **Step 3: Implement `draw.go`**

```go
// epaper/canvas/draw.go
package canvas

import "image"

func (c *Canvas) DrawRect(r image.Rectangle, col color.Color, filled bool) {
	if filled {
		for y := r.Min.Y; y < r.Max.Y; y++ {
			for x := r.Min.X; x < r.Max.X; x++ {
				c.SetPixel(x, y, col)
			}
		}
		return
	}
	// Top and bottom edges
	for x := r.Min.X; x < r.Max.X; x++ {
		c.SetPixel(x, r.Min.Y, col)
		c.SetPixel(x, r.Max.Y-1, col)
	}
	// Left and right edges
	for y := r.Min.Y; y < r.Max.Y; y++ {
		c.SetPixel(r.Min.X, y, col)
		c.SetPixel(r.Max.X-1, y, col)
	}
}

func (c *Canvas) DrawLine(x0, y0, x1, y1 int, col color.Color) {
	// Bresenham's line algorithm
	dx := x1 - x0
	if dx < 0 { dx = -dx }
	dy := y1 - y0
	if dy < 0 { dy = -dy }
	sx, sy := -1, -1
	if x0 < x1 { sx = 1 }
	if y0 < y1 { sy = 1 }
	err := dx - dy
	for {
		c.SetPixel(x0, y0, col)
		if x0 == x1 && y0 == y1 { break }
		e2 := 2 * err
		if e2 > -dy { err -= dy; x0 += sx }
		if e2 < dx  { err += dx; y0 += sy }
	}
}

func (c *Canvas) DrawCircle(cx, cy, radius int, col color.Color, filled bool) {
	// Midpoint circle algorithm
	x, y := 0, radius
	d := 1 - radius
	for x <= y {
		if filled {
			for i := cx - x; i <= cx+x; i++ { c.SetPixel(i, cy+y, col); c.SetPixel(i, cy-y, col) }
			for i := cx - y; i <= cx+y; i++ { c.SetPixel(i, cy+x, col); c.SetPixel(i, cy-x, col) }
		} else {
			points := [][2]int{{cx+x,cy+y},{cx-x,cy+y},{cx+x,cy-y},{cx-x,cy-y},{cx+y,cy+x},{cx-y,cy+x},{cx+y,cy-x},{cx-y,cy-x}}
			for _, p := range points { c.SetPixel(p[0], p[1], col) }
		}
		if d < 0 { d += 2*x + 3 } else { d += 2*(x-y) + 5; y-- }
		x++
	}
}
```

Note: add `"image/color"` to the imports in `draw.go`.

- [ ] **Step 4: Run tests**

```bash
go test ./epaper/canvas/ -v
```
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add epaper/canvas/draw.go epaper/canvas/canvas_test.go
git commit -m "feat(canvas): DrawRect, DrawLine, DrawCircle"
```

---

### Task 11: Font system + DrawText

**Files:**
- Create: `epaper/canvas/font.go`
- Modify: `epaper/canvas/draw.go`
- Modify: `epaper/canvas/canvas_test.go`

- [ ] **Step 1: Write failing test**

```go
// in canvas_test.go
func TestDrawText(t *testing.T) {
	c := New(122, 250, Rot0)
	c.Clear()
	f := EmbeddedFont(16)
	if f == nil {
		t.Fatal("EmbeddedFont(16) returned nil")
	}
	c.DrawText(0, 0, "A", f, Black)
	// At least one black pixel must exist after drawing "A"
	buf := c.Bytes()
	hasBlack := false
	for _, b := range buf {
		if b != 0xFF { hasBlack = true; break }
	}
	if !hasBlack {
		t.Error("expected at least one black pixel after DrawText")
	}
}
```

- [ ] **Step 2: Run — expect FAIL**

```bash
go test ./epaper/canvas/ -run TestDrawText -v
```

- [ ] **Step 3: Implement `font.go` with Font interface and embedded fonts**

The bitmap data comes from the Waveshare C library (`c/lib/Fonts/font*.c`).
Each font defines a byte array of glyph bitmaps. We embed them as Go byte slices.

```go
// epaper/canvas/font.go
package canvas

import _ "embed"

// Font provides glyph bitmaps for drawing text.
// Glyph data is row-major 1-bit packed (MSB = leftmost pixel).
type Font interface {
	Glyph(r rune) (data []byte, width, height int)
	LineHeight() int
}

// bitmapFont is a fixed-size ASCII bitmap font.
type bitmapFont struct {
	data       []byte
	charW      int
	charH      int
	firstRune  rune
	totalRunes int
}

func (f *bitmapFont) Glyph(r rune) ([]byte, int, int) {
	if r < f.firstRune || int(r-f.firstRune) >= f.totalRunes {
		return nil, f.charW, f.charH
	}
	stride := (f.charW + 7) / 8
	size := stride * f.charH
	idx := int(r-f.firstRune) * size
	return f.data[idx : idx+size], f.charW, f.charH
}

func (f *bitmapFont) LineHeight() int { return f.charH }

// EmbeddedFont returns a built-in bitmap font.
// Available sizes: 8, 12, 16, 20, 24.
// Fonts are sourced from Waveshare C library (lib/Fonts/font*.c).
// Each glyph covers ASCII 0x20–0x7E.
func EmbeddedFont(sizePt int) Font {
	switch sizePt {
	case 8:
		return &bitmapFont{data: font8Data, charW: 5, charH: 8, firstRune: 0x20, totalRunes: 95}
	case 12:
		return &bitmapFont{data: font12Data, charW: 7, charH: 12, firstRune: 0x20, totalRunes: 95}
	case 16:
		return &bitmapFont{data: font16Data, charW: 11, charH: 16, firstRune: 0x20, totalRunes: 95}
	case 20:
		return &bitmapFont{data: font20Data, charW: 14, charH: 20, firstRune: 0x20, totalRunes: 95}
	case 24:
		return &bitmapFont{data: font24Data, charW: 17, charH: 24, firstRune: 0x20, totalRunes: 95}
	}
	return nil
}
```

> **Implementation note:** `font8Data`, `font12Data`, etc. are `[]byte` variables defined in separate files `font8.go`, `font12.go`, etc. Each file contains the glyph bitmap data transcribed from the Waveshare C arrays in `c/lib/Fonts/font8.c` … `font24.c`. Each C array entry is a `uint8` (or `uint16`/`uint32` for wider glyphs) — convert to packed 1-bit row-major format per `Font.Glyph` spec. Start with `font16.go` to make the test pass; add others later.
>
> Example layout for font16 (11px wide, 16px tall, 2 bytes/row × 16 rows = 32 bytes/glyph):
> ```go
> // epaper/canvas/font16.go
> package canvas
> var font16Data = []byte{ /* 95 glyphs × 32 bytes = 3040 bytes */ ... }
> ```

- [ ] **Step 4: Implement `DrawText` in `draw.go`**

```go
// append to draw.go
import "image/color"

func (c *Canvas) DrawText(x, y int, text string, f Font, col color.Color) {
	cx := x
	for _, r := range text {
		data, w, h := f.Glyph(r)
		if data == nil {
			cx += w
			continue
		}
		stride := (w + 7) / 8
		for row := 0; row < h; row++ {
			for col2 := 0; col2 < w; col2++ {
				byteIdx := row*stride + col2/8
				bit := uint(7 - col2%8)
				if (data[byteIdx]>>bit)&1 == 1 {
					c.SetPixel(cx+col2, y+row, col)
				}
			}
		}
		cx += w + 1 // 1px letter spacing
	}
}
```

- [ ] **Step 5: Run tests**

```bash
go test ./epaper/canvas/ -v
```
Expected: all PASS (once font16Data is populated).

- [ ] **Step 6: Commit**

```bash
git add epaper/canvas/font.go epaper/canvas/font16.go epaper/canvas/draw.go epaper/canvas/canvas_test.go
git commit -m "feat(canvas): Font interface, EmbeddedFont(16), DrawText"
```

- [ ] **Step 7: Add remaining font sizes**

Repeat for font8, font12, font20, font24 following the same pattern.

```bash
git add epaper/canvas/font8.go epaper/canvas/font12.go epaper/canvas/font20.go epaper/canvas/font24.go
git commit -m "feat(canvas): add embedded bitmap fonts 8/12/20/24pt"
```

---

### Task 12: DrawImage + LoadTTF

**Files:**
- Modify: `epaper/canvas/draw.go`
- Modify: `epaper/canvas/font.go`
- Modify: `go.mod`

- [ ] **Step 1: Add golang.org/x/image dependency**

```bash
# Root module
go get golang.org/x/image

# gokrazy builddir
GOWORK=off go get golang.org/x/image
```

- [ ] **Step 2: Write failing test**

```go
// in canvas_test.go
import "image/color"
import stdimage "image"

func TestDrawImage(t *testing.T) {
	c := New(122, 250, Rot0)
	c.Clear()
	// Create a 10×10 black image
	src := stdimage.NewGray(stdimage.Rect(0, 0, 10, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			src.SetGray(x, y, color.Gray{Y: 0})
		}
	}
	c.DrawImage(stdimage.Pt(0, 0), src)
	if c.At(0, 0) != Black {
		t.Error("expected (0,0) to be black after DrawImage")
	}
}
```

- [ ] **Step 3: Run — expect FAIL**

```bash
go test ./epaper/canvas/ -run TestDrawImage -v
```

- [ ] **Step 4: Implement DrawImage**

```go
// append to draw.go
import "image"

func (c *Canvas) DrawImage(pt image.Point, img image.Image) {
	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			// Luminance: 0.299R + 0.587G + 0.114B (scaled to 0-65535)
			lum := (299*uint32(r) + 587*uint32(g) + 114*uint32(b)) / 1000
			if lum < 0x8000 {
				c.SetPixel(pt.X+(x-bounds.Min.X), pt.Y+(y-bounds.Min.Y), Black)
			} else {
				c.SetPixel(pt.X+(x-bounds.Min.X), pt.Y+(y-bounds.Min.Y), White)
			}
		}
	}
}
```

- [ ] **Step 5: Implement LoadTTF**

```go
// append to font.go
import (
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
	"image"
	"image/color"
)

type ttfFont struct {
	face       font.Face
	lineHeight int
}

func LoadTTF(data []byte, sizePt float64, dpi float64) (Font, error) {
	parsed, err := opentype.Parse(data)
	if err != nil {
		return nil, err
	}
	face, err := opentype.NewFace(parsed, &opentype.FaceOptions{
		Size: sizePt,
		DPI:  dpi,
	})
	if err != nil {
		return nil, err
	}
	metrics := face.Metrics()
	lh := int(metrics.Height.Ceil())
	return &ttfFont{face: face, lineHeight: lh}, nil
}

func (f *ttfFont) LineHeight() int { return f.lineHeight }

func (f *ttfFont) Glyph(r rune) ([]byte, int, int) {
	bounds, advance, ok := f.face.GlyphBounds(r)
	if !ok {
		return nil, int(advance.Ceil()), f.lineHeight
	}
	w := (bounds.Max.X - bounds.Min.X).Ceil()
	h := (bounds.Max.Y - bounds.Min.Y).Ceil()
	if w <= 0 || h <= 0 {
		return nil, int(advance.Ceil()), f.lineHeight
	}
	dst := image.NewAlpha(image.Rect(0, 0, w, h))
	dot := fixed.Point26_6{X: -bounds.Min.X, Y: -bounds.Min.Y}
	dr, mask, maskp, advance2, ok2 := f.face.Glyph(dot, r)
	if !ok2 {
		return nil, int(advance.Ceil()), f.lineHeight
	}
	_ = advance2
	// Blit alpha mask into Alpha image
	for y := dr.Min.Y; y < dr.Max.Y; y++ {
		for x := dr.Min.X; x < dr.Max.X; x++ {
			_, _, _, a := mask.At(maskp.X+(x-dr.Min.X), maskp.Y+(y-dr.Min.Y)).RGBA()
			if a > 0x8000 {
				dst.SetAlpha(x, y, color.Alpha{A: 255})
			}
		}
	}
	// Convert to packed 1-bit row-major
	stride := (w + 7) / 8
	bits := make([]byte, stride*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if dst.AlphaAt(x, y).A > 0 {
				bits[y*stride+x/8] |= 1 << uint(7-x%8)
			}
		}
	}
	return bits, w, h
}
```

- [ ] **Step 6: Run all canvas tests**

```bash
go test ./epaper/canvas/ -v
```
Expected: all PASS.

- [ ] **Step 7: Commit**

```bash
git add epaper/canvas/draw.go epaper/canvas/font.go go.mod go.sum
git commit -m "feat(canvas): DrawImage + LoadTTF (golang.org/x/image/font/opentype)"
```

---

### Task 13: ToImage screenshot test + final check

**Files:**
- Modify: `epaper/canvas/canvas_test.go`

- [ ] **Step 1: Write test**

```go
// in canvas_test.go
import "image/png"
import "bytes"

func TestToImage(t *testing.T) {
	c := New(122, 250, Rot0)
	c.Clear()
	c.SetPixel(0, 0, Black)

	img := c.ToImage()
	if img.Bounds().Dx() != 122 || img.Bounds().Dy() != 250 {
		t.Errorf("unexpected ToImage bounds: %v", img.Bounds())
	}
	if img.GrayAt(0, 0).Y != 0 {
		t.Error("expected (0,0) to be black in ToImage")
	}

	// Verify it can be PNG-encoded (no panics)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png.Encode: %v", err)
	}
	if buf.Len() == 0 {
		t.Error("expected non-empty PNG")
	}
}
```

- [ ] **Step 2: Run all tests**

```bash
go test ./epaper/... -v
```
Expected: all PASS.

- [ ] **Step 3: Final commit**

```bash
git add epaper/canvas/canvas_test.go
git commit -m "test(canvas): ToImage PNG encoding test + final integration check"
```

---

## Final verification

- [ ] **Build all packages for ARM64 (gokrazy target)**

```bash
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build ./epaper/...
```
Expected: no errors.

- [ ] **Run all tests (host)**

```bash
go test ./epaper/... -v
```
Expected: all PASS.

- [ ] **Close beads issues**

```bash
bd close awesomeProject-ewd awesomeProject-wl8 awesomeProject-36w awesomeProject-q3y
```
