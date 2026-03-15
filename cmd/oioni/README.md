# cmd/oioni — gokrazy main program

The main program that runs on the Pi Zero 2W under gokrazy.

## What it does

- Configures a USB composite gadget (RNDIS + ECM network, HID keyboard, mass storage)
- Manages disk image files on `/perm` (persistent) and USB drives (hotplug)
- Drives the Waveshare 2.13" Touch e-Paper HAT (display + touch input)

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-rndis` | false | Enable RNDIS network function (Windows, 3 EP) |
| `-ecm` | false | Enable ECM network function (Linux/macOS, 3 EP) |
| `-hid` | false | Enable HID keyboard function (1 EP) |
| `-mass-storage` | false | Enable USB mass storage from `--img` (2 EP) |
| `-epaper` | false | Enable e-paper display |
| `-img` | `/perm/data.img` | Disk image path |
| `-img-fs` | `vfat` | Image filesystem: `vfat`\|`exfat`\|`ext4` |
| `-img-create` | false | Create image if it does not exist |
| `-img-size` | 64 | Image size in MiB |
| `-img-write` | false | Write a test file to the image |
| `-img-read` | false | Read back the test file from the image |

## EP budget (DWC2 BCM2835)

The Pi Zero 2W has only **7 usable endpoints** beyond EP0:

| Config | EPs | Works? |
|--------|-----|--------|
| RNDIS + ECM + HID | 7 | ✓ |
| RNDIS + mass-storage | 5 | ✓ |
| RNDIS + ECM + mass-storage | 8 | ✗ |

## Deploy

```sh
# From repo root
GOWORK=off gok update --parent_dir . -i oioio
```

See [oioio/config.json](../../oioio/config.json) for gokrazy instance configuration.
