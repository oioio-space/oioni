# OiOni

[![Go tests](https://github.com/oioio-space/oioni/actions/workflows/test.yml/badge.svg)](https://github.com/oioio-space/oioni/actions/workflows/test.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/oioio-space/oioni.svg)](https://pkg.go.dev/github.com/oioio-space/oioni)

Go firmware for a **Raspberry Pi Zero 2W** running [gokrazy](https://gokrazy.org/) — a minimal Linux OS that runs Go programs directly, with no shell, no package manager, and no init system. The Pi boots in ~5 seconds and all services are live within 170 ms of the kernel starting.

The device serves three roles simultaneously:

- **USB composite gadget** — plugs into any host machine and presents itself as a network adapter (RNDIS for Windows, ECM for Linux/macOS), a HID keyboard, and/or a USB mass-storage drive, all on a single USB cable.
- **Local display** — a Waveshare 2.13" Touch e-Paper HAT shows a UI driven by a touch GUI framework built on a 1-bit drawing canvas.
- **Network security toolkit** — runs [impacket](https://github.com/fortra/impacket) tools (secretsdump, ntlmrelay, Kerberoasting, …) inside a Podman container, controlled from Go.

**TL;DR** — a pocket-sized multi-function device: USB network/HID/storage implant, autonomous security toolkit (impacket), and touch e-paper interface — all in one Pi Zero 2W, fully programmable in Go.

## Use cases

The combination of USB gadget + impacket + e-paper on a device the size of a USB stick opens a number of scenarios for **authorized** penetration testing and red team operations:

- **Network implant via USB** — plugged into any powered USB port (laptop, workstation, docking station), the Pi immediately creates a network interface on the host OS without requiring driver installation. From that interface it can enumerate the local network, relay credentials, or dump hashes from Windows machines reachable on the segment.
- **Automated credential harvesting** — on boot, the device can run secretsdump, Kerberoasting, or AS-REP Roasting against a pre-configured target and store results on the persistent `/perm` partition (SD card), with no interaction needed. Useful for timed drop attacks during physical assessments.
- **NTLM relay station** — `ntlmrelayx` listens for incoming authentications (e.g. triggered by a rogue network share or LLMNR/NBT-NS poisoning) and relays them to a target. The Pi's RNDIS/ECM interface makes the host route traffic through it.
- **HID attack vector** — the HID keyboard function can inject keystrokes on the connected host, combined with the network interface for payload delivery or C2 callback.
- **Virtual USB drive** — the mass-storage function serves a FAT/exFAT/ext4 image that the Pi controls from the inside: it can pre-populate it with payloads, read files the target drops on it, or swap its content between mounts.
- **Autonomous field tool** — the e-paper display shows status (IP, running tool, results summary) and the touch interface lets an operator interact with the device without connecting a laptop, while the Pi stays plugged into the target.

> **All uses require explicit authorization from the owner of the target systems.**

## Hardware

| Component | Part |
|-----------|------|
| SBC | Raspberry Pi Zero 2W — quad-core ARM Cortex-A53 @ 1 GHz, 512 MB RAM |
| OS | [gokrazy](https://gokrazy.org/) — pure-Go, boots from SD card |
| Display | [Waveshare 2.13" Touch e-Paper HAT V4](https://www.waveshare.com/wiki/2.13inch_Touch_e-Paper_HAT) — 250×122 px B&W, SPI, capacitive touch via I2C |
| USB | OTG gadget mode via BCM2835 DWC2 controller |

## How it works

### USB gadget

On boot, the program configures Linux's USB gadget subsystem via `configfs` to expose a composite USB device. The host machine sees network adapters and/or HID/storage devices — no driver installation needed on modern OSes.

The DWC2 controller on the Pi Zero 2W only has **7 usable endpoints** beyond EP0, so not all combinations of functions fit simultaneously:

| Config | Endpoints | Status |
|--------|-----------|--------|
| RNDIS + ECM + HID | 3 + 3 + 1 = 7 | ✓ |
| RNDIS + mass-storage | 3 + 2 = 5 | ✓ |
| RNDIS + ECM + HID + mass-storage | 9 | ✗ controller rejects |

### E-paper display

The Waveshare EPD driver communicates over SPI (4-wire) with the EPD_2in13_V4 display controller. A 1-bit canvas (`ui/canvas`) holds the pixel buffer; the GUI framework (`ui/gui`) handles layout, widgets, and touch events from the GT1151 capacitive touch controller (I2C, addr `0x14`). Partial refresh takes ~0.3 s; full refresh ~2 s.

### Impacket over Podman

Impacket is a Python library — it runs inside a minimal `python:3.13-alpine` container managed by the `tools/containers` package. Rather than pulling from a registry on boot, the image is shipped as a `.tar.gz` alongside the firmware via gokrazy's `ExtraFilePaths` mechanism and loaded with `podman load` on first run.

Signal delivery works around [Podman issue #19486](https://github.com/containers/podman/issues/19486) — `podman exec` does not forward signals — by capturing the container-side PID from `echo $$` and sending signals via `podman exec … kill -TERM/-KILL <pid>`.

### Disk images

The `system/imgvol` package creates and loop-mounts disk image files (FAT32 / exFAT / ext4). These images can be served as USB mass-storage to a connected host, giving the Pi the ability to present a virtual drive that it controls from the inside.

## Repository layout

```
oioni/
├── cmd/oioni/        # Main gokrazy program — wires everything together
│
├── drivers/
│   ├── epd/          # Waveshare EPD 2.13" V4 — SPI display driver
│   │                 #   Init(mode) / DisplayFull / DisplayPartial / Sleep
│   ├── touch/        # GT1151 capacitive touch — I2C, 5-point multitouch
│   │                 #   Start(ctx) → chan TouchEvent
│   └── usbgadget/    # Linux USB composite gadget via configfs
│                     #   WithRNDIS / WithECM / WithHID / WithMassStorage
│
├── system/
│   ├── imgvol/       # Disk image files: Create / Open / Close
│   │                 #   Supports FAT32, exFAT, ext4 via loop-mount
│   └── storage/      # USB hotplug (netlink) + /perm persistent storage
│                     #   Manager auto-mounts USB drives and /perm SD partition
│
├── tools/
│   ├── containers/   # Podman process manager (load image, run, exec, kill)
│   └── impacket/     # Typed Go wrappers: SecretsDump / NTLMRelay /
│                     #   Kerberoast / ASREPRoast / LookupSID / SAMRDump / Exec
│
└── ui/
    ├── canvas/       # 1-bit drawing canvas — implements draw.Image
    └── gui/          # Touch GUI: Navigator, widgets, layout, partial refresh
```

Each directory is an independent Go module. A `go.work` workspace at the root ties them together for local development.

## Configuration

Before deploying, set your own credentials and network settings in the two files below. Both files are tracked in git with placeholder values; local changes are hidden from git via `skip-worktree` so real passwords never get committed accidentally.

### `oioio/wifi.json` — Wi-Fi credentials

```json
{
    "ssid": "YOUR_WIFI_SSID",
    "psk":  "YOUR_WIFI_PASSWORD"
}
```

Edit it directly:
```sh
$EDITOR oioio/wifi.json
```

### `oioio/config.json` — gokrazy instance settings

The relevant fields:

```json
{
  "Hostname": "oioio",
  "Update": {
    "Hostname": "192.168.0.33",
    "HTTPPassword": "CHANGE_ME"
  }
}
```

| Field | Description |
|-------|-------------|
| `Hostname` | Device hostname (used for mDNS: `http://oioio/`) |
| `Update.Hostname` | Pi's IP address — update to match your DHCP lease |
| `Update.HTTPPassword` | Password for the gokrazy web UI and OTA API |

To commit a change to `config.json` without leaking real passwords:

```sh
# 1. Temporarily disable skip-worktree
git update-index --no-skip-worktree oioio/config.json

# 2. Replace real password with placeholder before staging
#    edit the file: set HTTPPassword to "CHANGE_ME"

# 3. Commit
git add oioio/config.json && git commit -m "..."

# 4. Restore real password locally, re-enable skip-worktree
git update-index --skip-worktree oioio/config.json
```

## Deploy

The gokrazy instance is configured in [`oioio/`](oioio/) (config, wifi credentials, builddir).

```sh
# OTA update over Wi-Fi — most common workflow
GOWORK=off gok update --parent_dir . -i oioio

# Flash SD card — first time or after a crash that made the Pi unreachable
sudo setfacl -m u:$USER:rw /dev/sdX
GOWORK=off gok --parent_dir . -i oioio overwrite --full /dev/sdX
```

> `GOWORK=off` is required: gok uses `-mod=mod` internally, which conflicts with a `go.work` workspace.

### Shipping the impacket container image

The image is too large to pull at runtime on a Pi Zero 2W. It is built once on a dev machine and shipped as part of the firmware update:

```sh
# Build the arm64 image
podman build --platform linux/arm64 -t oioni/impacket:arm64 tools/impacket/

# Export as a compressed tar (~40 MB)
podman save oioni/impacket:arm64 | gzip > /tmp/impacket-arm64.tar.gz
```

`oioio/config.json` then maps this file into the firmware via `ExtraFilePaths`:
```json
"ExtraFilePaths": {
  "/usr/share/oioni/impacket-arm64.tar.gz": "/tmp/impacket-arm64.tar.gz"
}
```

On first boot the image loads in ~75 s (written to `/perm/var` which persists across reboots). Subsequent boots start the container in ~6 s.

## Development

```sh
# Run all tests — no hardware required
go test ./...

# Cross-compile the main binary for ARM64
cd cmd/oioni && GOOS=linux GOARCH=arm64 go build .
```

Tests that require root or physical hardware carry `//go:build ignore` and are excluded from `go test`.

## Performance (measured on device)

| Metric | Value |
|--------|-------|
| Boot → all services live | ~5 s (kernel + 170 ms for user services) |
| OTA update (Wi-Fi) | ~85 s (139 MB root FS + 69 MB boot FS + reboot) |
| Impacket — first run (image load) | ~75 s |
| Impacket — subsequent runs (cached) | ~6 s |
| RAM used at idle | ~87 MB / 402 MB total |

## License

MIT — see [LICENSE](LICENSE).
