// epaper/epd/epd.go
package epd

import (
	"fmt"
	"time"
)

// Width is the horizontal axis (fast-scan): 122 pixels per row = 16 bytes/row.
// Height is the vertical axis: 250 rows.
const (
	Width      = 122
	Height     = 250
	BufferSize = ((Width + 7) / 8) * Height // 4000 bytes
)

// waitBusyTimeout is the maximum time waitBusy() will poll the BUSY pin.
const waitBusyTimeout = 10 * time.Second

// Mode selects the refresh strategy.
type Mode uint8

const (
	ModeFull    Mode = iota // ~2s, best quality
	ModePartial             // ~0.3s, partial update
	ModeFast                // ~0.5s, fast full update
)

// Display drives the EPD_2in13_V4 e-ink panel.
type Display struct {
	spi         SPIConn
	rst, dc, cs OutputPin
	busy        InputPin
	closers     []func() error
	firstErr    error // first I/O error from sendCommand/sendData; reset at start of each public method
}

// Config holds the Linux device paths and BCM pin numbers.
type Config struct {
	SPIDevice string // e.g. "/dev/spidev0.0"
	SPISpeed  uint32 // e.g. 4_000_000
	PinRST    int
	PinDC     int
	PinCS     int
	PinBUSY   int
}

// newDisplay creates a Display from injected HAL components (used in tests).
func newDisplay(spi SPIConn, rst, dc, cs OutputPin, busy InputPin) *Display {
	return &Display{spi: spi, rst: rst, dc: dc, cs: cs, busy: busy}
}

// New creates a Display from a Config, opening all hardware resources.
func New(cfg Config) (*Display, error) {
	spi, err := openSPI(cfg.SPIDevice, cfg.SPISpeed)
	if err != nil {
		return nil, fmt.Errorf("epd New: %w", err)
	}
	rst, err := openGPIOOutput(cfg.PinRST)
	if err != nil {
		spi.Close()
		return nil, fmt.Errorf("epd New RST: %w", err)
	}
	dc, err := openGPIOOutput(cfg.PinDC)
	if err != nil {
		spi.Close()
		rst.Close()
		return nil, fmt.Errorf("epd New DC: %w", err)
	}
	cs, err := openGPIOOutput(cfg.PinCS)
	if err != nil {
		spi.Close()
		rst.Close()
		dc.Close()
		return nil, fmt.Errorf("epd New CS: %w", err)
	}
	busy, err := openGPIOInput(cfg.PinBUSY)
	if err != nil {
		spi.Close()
		rst.Close()
		dc.Close()
		cs.Close()
		return nil, fmt.Errorf("epd New BUSY: %w", err)
	}

	d := newDisplay(spi, rst, dc, cs, busy)
	d.closers = []func() error{spi.Close, rst.Close, dc.Close, cs.Close, busy.Close}
	return d, nil
}

func (d *Display) sendCommand(cmd byte) {
	if d.firstErr != nil {
		return
	}
	if err := d.dc.Out(false); err != nil {
		d.firstErr = err
		return
	}
	if err := d.cs.Out(false); err != nil {
		d.firstErr = err
		return
	}
	if err := d.spi.Tx([]byte{cmd}); err != nil {
		d.firstErr = err
		_ = d.cs.Out(true) // best-effort CS release
		return
	}
	if err := d.cs.Out(true); err != nil {
		d.firstErr = err
	}
}

func (d *Display) sendData(data ...byte) {
	if d.firstErr != nil {
		return
	}
	if err := d.dc.Out(true); err != nil {
		d.firstErr = err
		return
	}
	if err := d.cs.Out(false); err != nil {
		d.firstErr = err
		return
	}
	if err := d.spi.Tx(data); err != nil {
		d.firstErr = err
		_ = d.cs.Out(true) // best-effort CS release
		return
	}
	if err := d.cs.Out(true); err != nil {
		d.firstErr = err
	}
}

func (d *Display) waitBusy() {
	deadline := time.Now().Add(waitBusyTimeout)
	for d.busy.Read() {
		if time.Now().After(deadline) {
			if d.firstErr == nil {
				d.firstErr = fmt.Errorf("waitBusy: BUSY pin stuck high after %v", waitBusyTimeout)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func (d *Display) reset() {
	if err := d.rst.Out(true); err != nil && d.firstErr == nil {
		d.firstErr = err
	}
	time.Sleep(20 * time.Millisecond)
	if err := d.rst.Out(false); err != nil && d.firstErr == nil {
		d.firstErr = err
	}
	time.Sleep(2 * time.Millisecond)
	if err := d.rst.Out(true); err != nil && d.firstErr == nil {
		d.firstErr = err
	}
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
	d.firstErr = nil
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
		// Minimal reset: RST low 1ms → high (matches Waveshare C library Init_PART).
		if err := d.rst.Out(false); err != nil && d.firstErr == nil {
			d.firstErr = err
		}
		time.Sleep(1 * time.Millisecond)
		if err := d.rst.Out(true); err != nil && d.firstErr == nil {
			d.firstErr = err
		}
		d.sendCommand(0x3C)
		d.sendData(0x80)
		d.sendCommand(0x01)
		d.sendData(byte(Height-1), byte((Height-1)>>8), 0x00)
		d.sendCommand(0x11)
		d.sendData(0x03)
		d.setWindow(0, 0, Width-1, Height-1)
		d.setCursor(0, 0)
	case ModeFast:
		d.reset()
		d.waitBusy()
		d.sendCommand(0x12) // software reset
		d.waitBusy()
		d.sendCommand(0x18) // temperature sensor: internal
		d.sendData(0x80)
		d.sendCommand(0x11) // data entry mode
		d.sendData(0x03)
		d.setWindow(0, 0, Width-1, Height-1)
		d.setCursor(0, 0)
		d.sendCommand(0x22) // load temperature value
		d.sendData(0xB1)
		d.sendCommand(0x20)
		d.waitBusy()
		d.sendCommand(0x1A) // write temperature register (100°C = 0x64)
		d.sendData(0x64, 0x00)
		d.sendCommand(0x22)
		d.sendData(0x91)
		d.sendCommand(0x20)
		d.waitBusy()
	}
	return d.firstErr
}

func (d *Display) DisplayFull(buf []byte) error {
	d.firstErr = nil
	d.sendCommand(0x24) // write RAM
	d.sendData(buf...)
	d.sendCommand(0x22)
	d.sendData(0xF7)
	d.sendCommand(0x20)
	d.waitBusy()
	return d.firstErr
}

func (d *Display) DisplayFast(buf []byte) error {
	d.firstErr = nil
	d.sendCommand(0x24)
	d.sendData(buf...)
	d.sendCommand(0x22)
	d.sendData(0xC7)
	d.sendCommand(0x20)
	d.waitBusy()
	return d.firstErr
}

// DisplayRegenerate performs a full black→white cycle to purge deep e-ink ghosting.
// Intended for use after extended sleep (e.g., 24h keep-alive).
// Leaves the display blank (white); the caller must re-render the UI afterwards.
// Takes ~4s (two full refresh cycles).
func (d *Display) DisplayRegenerate() error {
	black := make([]byte, BufferSize) // 0x00 = all black (0=noir, 1=blanc)
	white := make([]byte, BufferSize)
	for i := range white {
		white[i] = 0xFF
	}
	if err := d.Init(ModeFull); err != nil {
		return err
	}
	if err := d.DisplayBase(black); err != nil {
		return err
	}
	if err := d.Init(ModeFull); err != nil {
		return err
	}
	return d.DisplayBase(white)
}

func (d *Display) Sleep() error {
	d.firstErr = nil
	d.sendCommand(0x10)
	d.sendData(0x01)
	time.Sleep(100 * time.Millisecond)
	return d.firstErr
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

// DisplayBase writes the full framebuffer to both display RAM banks (registers
// 0x24 and 0x26) and triggers a full refresh. This establishes the reference
// frame that the hardware compares against during subsequent DisplayPartial calls.
// Call after Init(ModeFull) before any DisplayPartial sequence.
// Equivalent to Waveshare's Display_Base / displayPartBaseImage.
func (d *Display) DisplayBase(buf []byte) error {
	d.firstErr = nil
	d.sendCommand(0x24) // write new-image RAM
	d.sendData(buf...)
	d.sendCommand(0x26) // write old-image RAM (reference frame)
	d.sendData(buf...)
	d.sendCommand(0x22)
	d.sendData(0xF7) // full update sequence
	d.sendCommand(0x20)
	d.waitBusy()
	return d.firstErr
}

// DisplayPartial sends a full 4000-byte framebuffer and triggers a partial
// (ghost-free fast) refresh. Every call includes its own mini-reset and
// partial-mode init sequence, matching the Waveshare reference implementation.
//
// Typical usage:
//
//	display.Init(epd.ModeFull)
//	display.DisplayBase(buf)        // establish reference frame
//	for each update {
//	    display.DisplayPartial(buf) // ~0.3s, no ghost flush
//	}
//	// Every ~50 partial refreshes, do a full self-refresh:
//	display.Init(epd.ModeFull)
//	display.DisplayBase(buf)
func (d *Display) DisplayPartial(buf []byte) error {
	d.firstErr = nil
	// Mini-reset required before every partial update (Waveshare reference).
	if err := d.rst.Out(false); err != nil && d.firstErr == nil {
		d.firstErr = err
	}
	time.Sleep(1 * time.Millisecond)
	if err := d.rst.Out(true); err != nil && d.firstErr == nil {
		d.firstErr = err
	}
	// Partial-mode init (border, driver output, data entry, window, cursor).
	d.sendCommand(0x3C)
	d.sendData(0x80)
	d.sendCommand(0x01)
	d.sendData(byte(Height-1), byte((Height-1)>>8), 0x00)
	d.sendCommand(0x11)
	d.sendData(0x03)
	d.setWindow(0, 0, Width-1, Height-1)
	d.setCursor(0, 0)
	d.sendCommand(0x24) // write full framebuffer
	d.sendData(buf...)
	d.sendCommand(0x22)
	d.sendData(0xFF) // partial update sequence
	d.sendCommand(0x20)
	d.waitBusy()
	return d.firstErr
}
