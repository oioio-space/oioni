# canvas — 1-bit drawing surface for e-ink displays

[![Go Reference](https://pkg.go.dev/badge/github.com/oioio-space/oioni/ui/canvas.svg)](https://pkg.go.dev/github.com/oioio-space/oioni/ui/canvas)

`canvas` is a 1-bit drawing surface designed for e-ink / e-paper displays.
It implements `draw.Image` so every standard-library draw function works
out of the box, and it produces a packed byte buffer that can be handed
directly to an EPD driver.

**Buffer layout:** `((physW+7)/8) × physH` bytes, MSB-first.
`0` = black, `1` = white — matching the Waveshare EPD hardware convention.

## Install

```sh
go get github.com/oioio-space/oioni/ui/canvas
```

## Quick start

```go
package main

import (
    "image"
    stdDraw "image/draw"

    "github.com/oioio-space/oioni/ui/canvas"
    "github.com/oioio-space/oioni/drivers/epd"
)

func main() {
    // 1. Open the EPD display (Waveshare 2.13" HAT, physW=122 physH=250).
    d, err := epd.New(epd.Config{
        SPIDevice: "/dev/spidev0.0", SPISpeed: 4_000_000,
        PinRST: 17, PinDC: 25, PinCS: 8, PinBUSY: 24,
    })
    if err != nil {
        panic(err)
    }
    defer d.Close()

    // 2. Create a canvas. Rot90 makes logical (0,0) the top-left corner
    //    when the display is mounted in landscape orientation.
    c := canvas.New(122, 250, canvas.Rot90)

    // 3. Draw using canvas primitives or any standard-library function.
    stdDraw.Draw(c, c.Bounds(), &image.Uniform{canvas.White}, image.Point{}, stdDraw.Src)
    c.DrawRect(image.Rect(0, 0, 250, 122), canvas.Black, false) // border
    c.DrawText(8, 8, "Hello, e-ink!", canvas.EmbeddedFont(16), canvas.Black)

    // 4. Push the buffer to the display.
    d.Init(epd.ModeFull)
    if err := d.DisplayBase(c.Bytes()); err != nil {
        panic(err)
    }

    // 5. Partial update: change a label and refresh only the dirty region.
    c.DrawRect(image.Rect(8, 8, 100, 24), canvas.White, true) // clear label area
    c.DrawText(8, 8, "Updated!", canvas.EmbeddedFont(16), canvas.Black)
    sub, _ := c.SubRegion(image.Rect(0, 0, 122, 32)) // physical rect
    d.DisplayPartial(sub.Bytes())

    d.Sleep()
}
```

## Color convention

`canvas.Black` and `canvas.White` are the two valid colors. Any `color.Color`
with luminance below 50 % is treated as black; at or above 50 % as white.
Standard library colors (`color.Black`, `color.White`, `color.RGBA`, etc.) all
work transparently through the color model conversion.

## Embedded fonts

`canvas.EmbeddedFont(sizePt)` returns a built-in bitmap font. Available sizes
are **8, 12, 16, 20, 24** points. For TrueType fonts use `canvas.LoadTTF`.

## Rotation

Pass `canvas.Rot0 / Rot90 / Rot180 / Rot270` to `New` (or call
`SetRotation` later) to map logical draw coordinates to the physical buffer.
Only coordinate remapping changes; the buffer layout is always physical.

## License

MIT — see [LICENSE](../../LICENSE) at the repository root.
