# Design Spec — epaper packages

**Date:** 2026-03-14
**Hardware:** Waveshare 2.13inch Touch e-Paper HAT with Case
**Target:** gokrazy on Raspberry Pi Zero 2W

---

## Hardware Reference

| Component | Details |
|-----------|---------|
| Display | 250×122px, black & white, SPI 4-wire |
| Touch controller | GT1151, capacitive, up to 5 points, I2C |
| Full refresh | ~2s |
| Partial refresh | ~0.3s |
| Fast refresh | ~0.5s |
| Operating voltage | 3.3V |

**Pin mapping (BCM):**

| Signal | BCM | Direction |
|--------|-----|-----------|
| EPD RST | configurable | output |
| EPD DC | configurable | output |
| EPD CS | configurable | output |
| EPD BUSY | configurable | input (poll) |
| Touch TRST | 22 | output |
| Touch INT | 27 | input (falling edge) |

---

## Architecture

### Package structure

```
awesomeProject/
└── epaper/
    ├── epd/      # display driver (SPI + GPIO, Linux ioctl)
    ├── touch/    # touch driver (I2C + GPIO interrupt)
    └── canvas/   # drawing canvas (primitives, fonts, clipping, export)
```

### Dependencies

```
canvas  ──► image, image/color, image/png (stdlib)
            golang.org/x/image/font/opentype (TTF — new, requires go get)
epd     ──► golang.org/x/sys/unix (already in go.mod)
touch   ──► golang.org/x/sys/unix (already in go.mod)
```

The three packages are fully independent. Application code (`hello/`) is the only place that combines them:

```go
c := canvas.New(epd.Width, epd.Height, canvas.Rot0)
c.DrawText(0, 0, "Hello Pi!", canvas.EmbeddedFont(16), canvas.Black)
display.DisplayFull(c.Bytes())
```

### Design principles

- **A + interfaces** — raw Linux ioctl, injectable HAL interfaces for testability (same pattern as `storage/manager.go`)
- **No periph.io** — consistent with project style; `golang.org/x/sys/unix` already in `go.mod`
- **No panic** — all I/O errors returned, never `log.Fatalf` (gokrazy robustness rule)
- **Explicit ownership** — methods that return `[]byte` document whether it is a copy or an alias

---

## Package `epaper/epd`

### Constants

```go
// Width is the number of pixels along the fast-scan (horizontal) axis.
// The panel is wired so the 122-pixel axis is horizontal in hardware;
// each row of the framebuffer contains ceil(122/8)=16 bytes.
// Height is the number of rows (vertical axis).
const (
    Width  = 122
    Height = 250
)

// BufferSize is the byte length of a full framebuffer.
// ceil(122/8) = (122+7)/8 = 16 bytes/row × 250 rows = 4000 bytes.
// r.Dx() in DisplayPartial maps to the Width (horizontal) axis;
// r.Dy() maps to the Height (vertical) axis.
const BufferSize = ((Width + 7) / 8) * Height // = 4000

type Mode uint8
const (
    ModeFull    Mode = iota // full refresh ~2s, best quality
    ModePartial             // partial refresh ~0.3s
    ModeFast                // fast refresh ~0.5s
)
```

### HAL interfaces (`hal.go`)

```go
// OutputPin drives a signal (RST, DC, CS).
type OutputPin interface {
    Out(high bool) error
}

// InputPin polls an input signal (BUSY).
type InputPin interface {
    Read() bool
}

// SPIConn is a write-only SPI connection (e-ink has no SPI readback).
type SPIConn interface {
    Tx(w []byte) error
}
```

Real implementations open `/dev/spidev0.0` and control GPIO via Linux chardev or sysfs.

### Config and public API

```go
type Config struct {
    SPIDevice string  // e.g. "/dev/spidev0.0"
    SPISpeed  uint32  // e.g. 4_000_000
    PinRST    int     // BCM pin numbers
    PinDC     int
    PinCS     int
    PinBUSY   int
}

func New(cfg Config) (*Display, error)
```

```go
// Init initialises the display in the given mode.
// May be called multiple times to switch modes on a live display
// (e.g. ModeFull → ModePartial). No Sleep/Close cycle is required
// between calls; Init re-sends the full LUT for the new mode.
func (d *Display) Init(m Mode) error

// DisplayFull sends a complete 4000-byte framebuffer to the display
// and triggers a full refresh.
func (d *Display) DisplayFull(buf []byte) error

// DisplayPartial updates the rectangular region r on the display.
// buf must contain ceil(r.Dx()/8) × r.Dy() bytes in row-major 1-bit format.
// r must be in physical (hardware) coordinates, matching the values
// returned by canvas.SubRegion.
func (d *Display) DisplayPartial(r image.Rectangle, buf []byte) error

func (d *Display) DisplayFast(buf []byte) error
func (d *Display) Sleep() error
func (d *Display) Close() error
```

### Buffer format

`((Width+7)/8) × Height` = 16 × 250 = **4000 bytes**, 1 bit/pixel, MSB first.
Convention: `0` = black, `1` = white (e-ink standard).

### Internal constructor (tests, in `epd` package)

```go
func newDisplay(spi SPIConn, rst, dc, cs OutputPin, busy InputPin) *Display
```

---

## Package `epaper/touch`

### Public types

```go
type TouchPoint struct {
    ID   uint8
    X, Y uint16  // physical coordinates: X ∈ [0,249], Y ∈ [0,121]
    Size uint8
}

type TouchEvent struct {
    Points []TouchPoint  // 1–5 simultaneous points
    Time   time.Time
}
```

### HAL interfaces (`hal.go`)

```go
// OutputPin drives a signal (TRST reset).
type OutputPin interface {
    Out(high bool) error
}

// InterruptPin waits for a falling edge (INT pin).
// Blocks until an edge occurs or ctx is cancelled.
type InterruptPin interface {
    WaitFalling(ctx context.Context) error
}

// I2CConn performs a write-then-read I2C transaction.
type I2CConn interface {
    Tx(w, r []byte) error
}
```

### Config and public API

```go
type Config struct {
    I2CDevice string  // e.g. "/dev/i2c-1"
    I2CAddr   uint16  // 0x14 (GT1151 default)
    PinTRST   int     // BCM 22
    PinINT    int     // BCM 27
}

func New(cfg Config) (*Detector, error)

// Start initialises the GT1151, then launches the event goroutine.
// Returns the event channel and any initialisation error (I2C reset or
// product-ID read failure). The channel is closed when ctx is cancelled.
// Runtime I2C errors in the goroutine are stored internally and
// accessible via Err() after the channel closes.
func (d *Detector) Start(ctx context.Context) (<-chan TouchEvent, error)

// Err returns the first runtime error encountered by the goroutine,
// or nil if the detector exited cleanly (ctx cancelled).
// Only valid after the channel returned by Start has been closed.
func (d *Detector) Err() error

func (d *Detector) Close() error
```

### Internal goroutine behavior

```
Initialisation (returns error if fails):
  1. TRST: high → 100ms delay → low → 100ms delay → high
  2. Read 4 bytes from 0x8140 → verify product ID

Event loop (runtime errors stored in d.err, channel closed on error or ctx.Done):
  3. intPin.WaitFalling(ctx)               ← zero CPU until touch or cancel
  4. Read 1 byte from 0x814E              → flag (bit 7) + count (bits 3:0)
  5. Validate count ∈ [1,5]; if invalid: write 0x00 to 0x814E, goto 3
  6. Read count×8 bytes starting at 0x814F
  7. Write 0x00 to 0x814E                 → clear interrupt
  8. Parse points: [ID, Xlo, Xhi, Ylo, Yhi, Size, _, _] per point
  9. Push TouchEvent to buffered channel (cap 8)
     If channel full: drop event (non-blocking send)
 10. Goto 3
```

### GT1151 register map

| Register | Size | Purpose |
|----------|------|---------|
| `0x8140` | 4 bytes | Product ID (verified at init) |
| `0x814E` | 1 byte | Touch flag (bit 7) + count (bits 3:0) |
| `0x814F` | count×8 bytes | Touch data: ID, Xlo, Xhi, Ylo, Yhi, Size, _, _ per point |

### Internal constructor (tests, in `touch` package)

```go
func newDetector(i2c I2CConn, trst OutputPin, intPin InterruptPin) *Detector
```

---

## Package `epaper/canvas`

### Exported constants

```go
// Black and White are the two display colors.
var Black color.Color = color.Gray{Y: 0}
var White color.Color = color.Gray{Y: 255}

type Rotation int
const (
    Rot0   Rotation = 0
    Rot90  Rotation = 90
    Rot180 Rotation = 180
    Rot270 Rotation = 270
)
```

### Canvas struct

Implements `draw.Image` (stdlib: `Bounds()`, `At()`, `ColorModel()`, `Set()`).

**Coordinate systems:**
- **Physical**: always `physW × physH` (e.g. 122×250), matches hardware layout
- **Logical**: depends on `Rotation`; Rot90/Rot270 swap width and height

**Buffer layout:** `((physW+7)/8) × physH` bytes, physical layout, 1 bit/pixel, MSB first.
`Bytes()` always returns this physical layout regardless of active rotation.
The rotation remaps only the coordinates passed to draw calls.

### Public API

```go
// New creates a canvas for a display of physical size physW × physH.
// rot defines the initial logical orientation seen by draw calls.
func New(physW, physH int, rot Rotation) *Canvas

// Pixels & background
func (c *Canvas) SetPixel(x, y int, col color.Color)
func (c *Canvas) Fill(col color.Color)
func (c *Canvas) Clear()  // Fill(White)

// Shapes
func (c *Canvas) DrawRect(r image.Rectangle, col color.Color, filled bool)
func (c *Canvas) DrawLine(x0, y0, x1, y1 int, col color.Color)         // Bresenham
func (c *Canvas) DrawCircle(cx, cy, radius int, col color.Color, filled bool)

// Text
func (c *Canvas) DrawText(x, y int, text string, f Font, col color.Color)

// Image import: thresholds to 1-bit at 50% luminance
func (c *Canvas) DrawImage(pt image.Point, img image.Image)

// Clipping: all draw calls are silently clipped to the active region
func (c *Canvas) SetClip(r image.Rectangle)
func (c *Canvas) ClearClip()

// Rotation: changes logical coordinate mapping for future draw calls.
// The backing buffer content is NOT transformed; only the coordinate
// remapping changes. Call Clear() first if a blank slate is desired.
func (c *Canvas) SetRotation(r Rotation)
```

### Output methods

```go
// Bytes returns a COPY of the backing buffer in physical layout
// (((physW+7)/8) × physH bytes). Returning a copy means the caller
// can pass it to DisplayFull concurrently with ongoing draw calls
// without a data race. Cost: ~4000 bytes allocated per call.
func (c *Canvas) Bytes() []byte

// SubRegion takes a rectangle in PHYSICAL coordinates (same space as
// the backing buffer, unaffected by rotation) and returns a sub-canvas
// plus the physical rectangle to pass to epd.DisplayPartial.
// SubRegion.Bytes() returns a newly-allocated, re-packed buffer
// containing only the pixels of r (ceil(r.Dx()/8) × r.Dy() bytes),
// where r.Dx() maps to the Width (horizontal) axis.
//
// Always pass physical coordinates — do NOT pass logical/rotated coords.
// To convert a logical rectangle to physical, use canvas.PhysicalRect(r).
//
//   sub, phys := c.SubRegion(c.PhysicalRect(logicalRect))
//   display.DisplayPartial(phys, sub.Bytes())
func (c *Canvas) SubRegion(r image.Rectangle) (*Canvas, image.Rectangle)

// PhysicalRect converts a rectangle in logical (rotated) coordinates
// to physical (hardware) coordinates. Use before calling SubRegion
// when the rectangle was constructed in logical space.
func (c *Canvas) PhysicalRect(r image.Rectangle) image.Rectangle
```

### Screenshot and streaming

```go
// ToImage converts the canvas to a *image.Gray (8 bits/pixel, stdlib-compatible).
// Returns a NEW image allocated from a snapshot of the current backing buffer.
// This is NOT concurrent-safe: the caller must ensure no draw calls are
// in progress on this canvas while ToImage is executing (e.g. use a mutex
// or call only from the same goroutine that drives drawing).
//
// Usage — save to file:
//   f, _ := os.Create("/perm/screenshot.png")
//   png.Encode(f, c.ToImage())
//
// Usage — stream over HTTP:
//   http.HandleFunc("/screen", func(w http.ResponseWriter, r *http.Request) {
//       mu.Lock()
//       img := canvas.ToImage()
//       mu.Unlock()
//       w.Header().Set("Content-Type", "image/png")
//       png.Encode(w, img)
//   })
func (c *Canvas) ToImage() *image.Gray
```

### Font system

```go
type Font interface {
    // Glyph returns the bitmap for rune r.
    // data is a row-major, 1-bit-per-pixel packed byte slice:
    //   byte index = (row * ceil(width/8)) + (col/8)
    //   bit  index = 7 - (col % 8)   (MSB = leftmost pixel)
    // width and height are in pixels.
    // Returns nil data if the rune is not in the font.
    Glyph(r rune) (data []byte, width, height int)
    LineHeight() int
}

// EmbeddedFont returns a built-in bitmap font sourced from the Waveshare
// C library (lib/Fonts/font*.c). Glyphs are converted to the 1-bit
// row-major format described by Font.Glyph at package init time.
// Available sizes: 8, 12, 16, 20, 24 pt.
func EmbeddedFont(sizePt int) Font

// LoadTTF parses a TrueType font at the given size and DPI.
// Requires golang.org/x/image/font/opentype (must be go-get'd into builddir).
func LoadTTF(data []byte, sizePt float64, dpi float64) (Font, error)
```

### draw.Image interop

Implementing `draw.Image` enables:
- `golang.org/x/image/draw` for scaling/rotating imported images
- The future GUI package builds on `draw.Image` without tight coupling

### New dependency

`golang.org/x/image` required for `LoadTTF` (`golang.org/x/image/font/opentype`).
Must be added to **two** `go.mod` files:

```bash
# 1. Root module (awesomeProject/go.mod)
go get golang.org/x/image

# 2. gokrazy builddir (oioio/builddir/github.com/oioio/awesomeProject/go.mod)
GOWORK=off go get golang.org/x/image
```

---

## Testing strategy

### Per-package approach

| Package | Strategy |
|---------|----------|
| `epd` | Inject `fakeSPI`, `fakeOutputPin`, `fakeInputPin` via `newDisplay` — assert SPI command sequences |
| `touch` | Inject `fakeI2C`, `fakeOutputPin`, `fakeINT` via `newDetector` — simulate touches via channel |
| `canvas` | Pure memory — assert pixel values in `Bytes()` and `ToImage()` pixels |

### Fake implementations (defined in `_test.go` of their respective packages)

```go
// epd package fakes

type fakeSPI struct{ log [][]byte }
func (f *fakeSPI) Tx(w []byte) error {
    f.log = append(f.log, append([]byte(nil), w...))
    return nil
}

type fakeOutputPin struct{ last bool }
func (f *fakeOutputPin) Out(high bool) error { f.last = high; return nil }

type fakeInputPin struct{ val bool }
func (f *fakeInputPin) Read() bool { return f.val }

// touch package fakes

type fakeOutputPin struct{ last bool }
func (f *fakeOutputPin) Out(high bool) error { f.last = high; return nil }

type fakeI2C struct{ responses [][]byte; idx int }
func (f *fakeI2C) Tx(w, r []byte) error {
    if f.idx < len(f.responses) {
        copy(r, f.responses[f.idx]); f.idx++
    }
    return nil
}

type fakeINT struct{ trigger chan struct{} }
func (f *fakeINT) WaitFalling(ctx context.Context) error {
    select {
    case <-f.trigger: return nil
    case <-ctx.Done(): return ctx.Err()
    }
}
```

---

## Future: GUI package

The future `epaper/gui/` package will import:
- `epaper/canvas` — draw widgets
- `epaper/touch` — receive input events
- `epaper/epd` — flush frames (full or partial via `SubRegion`)

It is NOT part of this implementation. The `draw.Image` interface, `SubRegion`, `ToImage`, and `<-chan TouchEvent` are all designed to support it naturally.
