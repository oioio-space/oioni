# touch — Waveshare GT1151 capacitive touch driver

[![Go Reference](https://pkg.go.dev/badge/github.com/oioio-space/oioni/drivers/touch.svg)](https://pkg.go.dev/github.com/oioio-space/oioni/drivers/touch)

Driver for the **GT1151 capacitive touch controller** on the Waveshare 2.13inch
Touch e-Paper HAT. Supports 5 simultaneous touch points over I2C.

**Hardware:** https://www.waveshare.com/wiki/2.13inch_Touch_e-Paper_HAT

## Install

```sh
go get github.com/oioio-space/oioni/drivers/touch
```

## Wiring (Raspberry Pi Zero 2W)

| Touch pin | BCM GPIO | Function |
|-----------|----------|----------|
| TRST      | 22       | Touch reset (output) |
| INT       | 27       | Interrupt (falling edge) |
| I2C       | /dev/i2c-1 | Address 0x14 |

Enable I2C in `config.txt`: `dtparam=i2c_arm=on`

## Quick start

```go
td, err := touch.New(touch.Config{
    I2CDevice: "/dev/i2c-1",
    I2CAddr:   0x14,
    PinTRST:   22,
    PinINT:    27,
})
if err != nil {
    log.Fatal(err)
}

ctx, cancel := context.WithCancel(context.Background())
events, err := td.Start(ctx)
if err != nil {
    log.Fatal(err)
}
defer td.Close()

for ev := range events {
    for _, pt := range ev.Points {
        log.Printf("touch id=%d x=%d y=%d", pt.ID, pt.X, pt.Y)
    }
}
```

## Coordinate system

Physical display pixels: X ∈ [0, 121], Y ∈ [0, 249].
Origin is top-left of the display (not the PCB).
