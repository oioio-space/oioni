# OiOni

[![Go tests](https://github.com/oioio-space/oioni/actions/workflows/test.yml/badge.svg)](https://github.com/oioio-space/oioni/actions/workflows/test.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/oioio-space/oioni.svg)](https://pkg.go.dev/github.com/oioio-space/oioni)

Embedded Go firmware for a **Raspberry Pi Zero 2W** running [gokrazy](https://gokrazy.org/).

The Pi presents itself over USB as a composite gadget (network + HID + storage) and drives a Waveshare 2.13" Touch e-Paper HAT as a local display.

## Hardware

| Component | Part |
|-----------|------|
| SBC | Raspberry Pi Zero 2W |
| Display | [Waveshare 2.13" Touch e-Paper HAT V4](https://www.waveshare.com/wiki/2.13inch_Touch_e-Paper_HAT) — 250×122 px B&W |
| USB | Gadget mode via DWC2 OTG controller |

## Repository layout

```
oioni/
├── cmd/oioni/        # Main gokrazy program
├── drivers/
│   ├── epd/          # Waveshare EPD 2.13" V4 — SPI display driver
│   ├── touch/        # GT1151 capacitive touch — I2C driver
│   └── usbgadget/    # Linux USB composite gadget via configfs
├── system/
│   ├── imgvol/       # Disk image creation + loop-mount (FAT/exFAT/ext4)
│   └── storage/      # USB hotplug + /perm persistent storage
└── ui/
    ├── canvas/       # 1-bit drawing canvas for e-ink
    └── gui/          # Touch GUI framework for the e-paper display
```

Each subdirectory is an independent Go module in a workspace (`go.work`).

## Deploy

```sh
# Flash SD card (first time)
make flash DRIVE=/dev/sdX

# OTA update over Wi-Fi
make update
```

> `GOWORK=off` is set automatically by the Makefile — required because gok's internal `-mod=mod` conflicts with `go.work`.

## Development

```sh
# Run all tests (no hardware needed)
make test

# Cross-compile for ARM64
cd cmd/oioni && GOOS=linux GOARCH=arm64 go build ./...
```

Tests that require root or ARM64 hardware carry `//go:build ignore`.

## USB endpoint budget (DWC2, Pi Zero 2W)

The BCM2835 DWC2 controller has **7 usable endpoints** beyond EP0:

| Config | EPs | Works? |
|--------|-----|--------|
| RNDIS + ECM + HID | 7 | ✓ |
| RNDIS + mass-storage | 5 | ✓ |
| RNDIS + ECM + mass-storage | 8 | ✗ |

## License

MIT — see [LICENSE](LICENSE).
