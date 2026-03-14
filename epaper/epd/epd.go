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
	spi         SPIConn
	rst, dc, cs OutputPin
	busy        InputPin
	closers     []func() error
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

// Placeholder stubs — implemented in subsequent tasks.
func (d *Display) Init(m Mode) error                                  { return nil }
func (d *Display) DisplayFull(buf []byte) error                       { return nil }
func (d *Display) DisplayPartial(r image.Rectangle, buf []byte) error { return nil }
func (d *Display) DisplayFast(buf []byte) error                       { return nil }
func (d *Display) Sleep() error                                       { return nil }
func (d *Display) Close() error                                       { return nil }
