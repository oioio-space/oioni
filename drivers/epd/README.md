# epd — Waveshare EPD 2.13" V4 driver

[![Go Reference](https://pkg.go.dev/badge/github.com/oioio-space/oioni/drivers/epd.svg)](https://pkg.go.dev/github.com/oioio-space/oioni/drivers/epd)

Driver for the **Waveshare 2.13inch Touch e-Paper HAT (V4)** — a 122×250 px
black/white e-ink display connected over SPI.

**Hardware:** https://www.waveshare.com/wiki/2.13inch_Touch_e-Paper_HAT

## Install

```sh
go get github.com/oioio-space/oioni/drivers/epd
```

## Wiring (Raspberry Pi Zero 2W)

| EPD pin | BCM GPIO | Function |
|---------|----------|----------|
| RST     | 17       | Reset    |
| DC      | 25       | Data/Command |
| CS      | 8        | Chip Select (SPI CE0) |
| BUSY    | 24       | Busy signal |
| SPI     | /dev/spidev0.0 | 4 MHz |

Enable SPI in `config.txt`: `dtparam=spi=on`

## Quick start

```go
d, err := epd.New(epd.Config{
    SPIDevice: "/dev/spidev0.0",
    SPISpeed:  4_000_000,
    PinRST:    17, PinDC: 25, PinCS: 8, PinBUSY: 24,
})
if err != nil {
    log.Fatal(err)
}
defer d.Close()

buf := make([]byte, epd.BufferSize) // 4000 bytes, 1 bpp
// Fill buf with your image (0=black, 1=white per bit)

d.Init(epd.ModeFull)
d.DisplayBase(buf)          // full refresh + set reference frame
d.DisplayPartial(buf)       // fast partial update (~0.3 s)
d.Sleep()
```

## Refresh modes

| Mode | Duration | Notes |
|------|----------|-------|
| `ModeFull` | ~2 s | Best quality, full redraw |
| `ModeFast` | ~0.5 s | Fast full refresh |
| `ModePartial` | ~0.3 s | No flash, requires prior `DisplayBase` |

## Buffer layout

`BufferSize = ((Width+7)/8) * Height = 4000 bytes`
MSB first: bit 7 of byte 0 = pixel (0,0). `0` = black, `1` = white.
