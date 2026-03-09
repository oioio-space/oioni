# imgvol — Disk Image Management Design

## Overview

New package `imgvol` for creating, formatting, and loop-mounting disk images on gokrazy (Pi Zero 2W). Images are served to a USB host via the existing `MassStorage` gadget function. Access is exclusive: either the Pi or the host owns the image at any given time.

## Architecture

```
awesomeProject/
  imgvol/
    imgvol.go      — public API: FSType, Create, Open, Volume
    loop.go        — loopback device management (syscall Linux)
    format.go      — invoke static mkfs binaries via go:embed
    build/
      Dockerfile   — cross-compile static ARM64 mkfs binaries
    bin/
      mkfs.fat     — dosfstools static ARM64
      mkfs.exfat   — exfatprogs static ARM64
      mkfs.ntfs    — ntfsprogs static ARM64
      mkfs.ext4    — e2fsprogs static ARM64
```

Existing packages unchanged: `storage` (USB hotplug), `usbgadget` (gadget control).

## Ownership Model

```
Pi: Create+Format → Pi: Open (loop-mount) → Pi: write via afero → v.Close()
                                                                        ↓
Pi: Open (loop-mount) ← g.Disable() ← host done     g.Enable() + MassStorage(path)
        ↓                                                               ↓
Pi: reads host changes                               Host: reads/writes via USB
```

Never simultaneous. The Pi loop-mounts the image XOR the gadget serves it to the host.

## Public API

```go
package imgvol

type FSType string

const (
    FAT   FSType = "vfat"
    ExFAT FSType = "exfat"
    NTFS  FSType = "ntfs"
    Ext4  FSType = "ext4"
)

// Create creates a sparse .img file at path (os.Truncate) and formats it.
func Create(path string, size int64, fstype FSType) error

// Open loop-mounts an existing image and returns a ready-to-use Volume.
// Caller must call Close() when done.
func Open(path string) (*Volume, error)

// Volume represents a mounted disk image.
type Volume struct {
    Path   string
    FSType FSType
    FS     afero.Fs  // sandboxed via afero.NewBasePathFs
}

// Close unmounts the image and releases the loopback device.
func (v *Volume) Close() error
```

## Formatters (format.go)

Static binaries embedded via `go:embed`, extracted once to `/tmp/imgvol-bin/` on first use.

| FSType | Binary     | Source      |
|--------|------------|-------------|
| vfat   | mkfs.fat   | dosfstools  |
| exfat  | mkfs.exfat | exfatprogs  |
| ntfs   | mkfs.ntfs  | ntfsprogs   |
| ext4   | mkfs.ext4  | e2fsprogs   |

Build: `imgvol/build/Dockerfile` cross-compiles static ARM64 binaries, same pattern as `usbgadget/modules/build/`.

## Loopback Device (loop.go)

Syscall sequence for `Open`:
1. `open("/dev/loop-control")` + `ioctl(LOOP_CTL_GET_FREE)` → loop number N
2. `open("/dev/loopN", O_RDWR)`
3. `open(img path, O_RDWR)`
4. `ioctl(loopFd, LOOP_SET_FD, imgFd)`
5. `ioctl(loopFd, LOOP_SET_STATUS64)`
6. `os.MkdirAll("/tmp/imgvol/<sha>")` + `syscall.Mount("/dev/loopN", mountpoint, fstype, 0, "")`

Syscall sequence for `Close`:
1. `syscall.Unmount(mountpoint, 0)`
2. `ioctl(loopFd, LOOP_CLR_FD)`
3. Close all fds + `os.Remove(mountpoint)`

## hello/main.go — CLI Flags

All flags independent, combinable freely. EP budget enforced by existing `udc.go` error.

```
Gadget:
  --rndis          enable RNDIS (3 EP)
  --ecm            enable ECM (3 EP)
  --hid            enable HID keyboard (1 EP)
  --mass-storage   enable MassStorage using --img path (2 EP)

Image:
  --img            image path (default: /perm/data.img)
  --img-fs         fat|exfat|ntfs|ext4 (default: fat)
  --img-size       size in MiB (default: 64)
  --img-create     create + format image
  --img-write      open + write test files via afero + close
  --img-read       open + print contents via afero + close

Storage hotplug:
  --storage        enable USB hotplug storage manager
```

EP budget reference (Pi Zero 2W DWC2, max 7 usable):
- RNDIS + MassStorage = 5 EP ✓
- ECM + MassStorage = 5 EP ✓
- RNDIS + ECM + HID = 7 EP ✓ (no MassStorage)
- RNDIS + ECM + MassStorage = 8 EP ✗ → clear error from udc.go
