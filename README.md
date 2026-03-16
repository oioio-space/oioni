# OiOni

[![Go tests](https://github.com/oioio-space/oioni/actions/workflows/test.yml/badge.svg)](https://github.com/oioio-space/oioni/actions/workflows/test.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/oioio-space/oioni.svg)](https://pkg.go.dev/github.com/oioio-space/oioni)

Embedded Go firmware for a **Raspberry Pi Zero 2W** running [gokrazy](https://gokrazy.org/).

The Pi presents itself over USB as a composite gadget (RNDIS/ECM network, HID keyboard, mass storage) and drives a Waveshare 2.13" Touch e-Paper HAT as a local display. It also runs network security tools from the [impacket](https://github.com/fortra/impacket) suite inside a Podman container.

## Hardware

| Component | Part |
|-----------|------|
| SBC | Raspberry Pi Zero 2W |
| Display | [Waveshare 2.13" Touch e-Paper HAT V4](https://www.waveshare.com/wiki/2.13inch_Touch_e-Paper_HAT) — 250×122 px B&W |
| USB | Gadget mode via DWC2 OTG controller |

## Repository layout

```
oioni/
├── cmd/oioni/        # Main gokrazy program (USB gadget + e-paper + impacket CLI)
├── drivers/
│   ├── epd/          # Waveshare EPD 2.13" V4 — SPI display driver
│   ├── touch/        # GT1151 capacitive touch — I2C driver
│   └── usbgadget/    # Linux USB composite gadget via configfs
├── system/
│   ├── imgvol/       # Disk image creation + loop-mount (FAT/exFAT/ext4)
│   └── storage/      # USB hotplug + /perm persistent storage
├── tools/
│   ├── containers/   # Podman container lifecycle manager
│   └── impacket/     # Typed Go wrappers for impacket scripts
└── ui/
    ├── canvas/       # 1-bit drawing canvas for e-ink
    └── gui/          # Touch GUI framework for the e-paper display
```

Each subdirectory is an independent Go module in a workspace (`go.work`).

## Deploy

```sh
# OTA update over Wi-Fi (most common)
GOWORK=off gok update --parent_dir . -i oioio

# Flash SD card (first time or recovery)
sudo setfacl -m u:$USER:rw /dev/sdX
GOWORK=off gok --parent_dir . -i oioio overwrite --full /dev/sdX
```

> `GOWORK=off` is required because gok's internal `-mod=mod` conflicts with `go.work`.

## Development

```sh
# Run all tests (no hardware needed)
go test ./...

# Cross-compile main binary for ARM64
cd cmd/oioni && GOOS=linux GOARCH=arm64 go build .

# Build the impacket container image (requires podman, builds for arm64)
podman build --platform linux/arm64 -t oioni/impacket:arm64 tools/impacket/
```

Tests that require root or actual hardware carry `//go:build ignore`.

## USB endpoint budget (DWC2, Pi Zero 2W)

The BCM2835 DWC2 controller has **7 usable endpoints** beyond EP0:

| Config | EPs | Works? |
|--------|-----|--------|
| RNDIS + ECM + HID | 7 | ✓ |
| RNDIS + mass-storage | 5 | ✓ |
| RNDIS + ECM + HID + mass-storage | 9 | ✗ |

## License

MIT — see [LICENSE](LICENSE).
