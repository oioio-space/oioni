# Storage Package Design

**Date:** 2026-03-08
**Status:** Approved

## Context

gokrazy programs need simple, reliable access to both persistent storage (`/perm` partition) and external USB drives. This package provides a unified Go API over both, using [afero](https://github.com/spf13/afero) as the filesystem abstraction layer.

## Goals

- Detect USB mass storage devices via kernel netlink events (no polling)
- Mount USB drives automatically with auto-detected filesystem type
- Expose every volume (perm + USB) as an `afero.Fs`
- Fire `OnMount` / `OnUnmount` callbacks so callers react to changes
- Work within gokrazy constraints (read-only root, no udev daemon)

## Non-Goals

- Write-back caching or overlay unions across volumes
- LUKS/encrypted volumes
- Network filesystems (NFS, SMB)

## Package Structure

```
awesomeProject/
└── storage/
    ├── doc.go
    ├── manager.go       # Manager, options, main loop
    ├── volume.go        # Volume struct
    ├── perm.go          # /perm gokrazy integration
    ├── mount/
    │   ├── doc.go
    │   ├── detect.go    # FS type detection via magic bytes
    │   └── mount.go     # syscall.Mount / Unmount wrappers
    └── usbdetect/
        ├── doc.go
        ├── detector.go  # Detector, Start(ctx) → <-chan Event
        ├── netlink.go   # AF_NETLINK KOBJECT_UEVENT socket
        └── sysfs.go     # initial scan of /sys/block/sd*/
```

Same pattern as `usbgadget/functions/` — each sub-package is standalone and independently testable.

## Public API — `storage/`

```go
type Volume struct {
    Name       string    // USB label or "perm"
    Device     string    // "/dev/sda1" — empty for perm
    MountPath  string    // "/tmp/storage/sda1" or "/perm"
    FSType     string    // "vfat", "exfat", "ext4", "perm"
    Persistent bool      // true only for /perm
    FS         afero.Fs  // ready-to-use filesystem
}

type Manager struct {
    OnMount   func(v *Volume)
    OnUnmount func(v *Volume)
}

func New(opts ...Option) *Manager
func (m *Manager) Start(ctx context.Context) error
func (m *Manager) Volumes() []*Volume
func (m *Manager) Volume(name string) (*Volume, bool)

// Options
WithPermPath(path string)       // default: "/perm"
WithMountBase(path string)      // default: "/tmp/storage"
WithOnMount(fn func(*Volume))
WithOnUnmount(fn func(*Volume))
```

`Start()` sequence:
1. Mount `/perm` → fire `OnMount`
2. Initial sysfs scan (USB drives already connected at boot)
3. Netlink loop until `ctx.Done()`

## Sub-package — `storage/usbdetect/`

```go
type Event struct {
    Action string  // "add" | "remove"
    Device string  // "/dev/sda1"
}

type Detector struct{}

func New() *Detector
func (d *Detector) Start(ctx context.Context) (<-chan Event, error)
```

`Start()` opens an `AF_NETLINK / NETLINK_KOBJECT_UEVENT` socket, fires an initial sysfs scan goroutine for already-connected drives, then reads kernel uevents. Filters on `SUBSYSTEM=block` + `DEVTYPE=partition`.

## Sub-package — `storage/mount/`

```go
func DetectFSType(device string) (string, error)
// Reads magic bytes: FAT signature @ 0x1FE, exFAT @ 0x3, ext4 @ 0x438

func Mount(device, mountpoint, fstype string) error
// syscall.Mount with MS_NOATIME | MS_RELATIME

func Unmount(mountpoint string) error
// syscall.Unmount with MNT_DETACH
```

No external dependencies — only `syscall` and binary device reads.

## Kernel Modules

The `Manager` loads these modules at startup (same mechanism as `usbgadget`):

| Module        | Purpose                   | Already in gokrazy kernel? |
|---------------|---------------------------|---------------------------|
| `usb_storage` | USB mass storage driver   | Likely yes                |
| `fat`         | FAT base module           | Likely yes                |
| `vfat`        | FAT32/FAT16               | Likely yes                |
| `exfat`       | exFAT (kernel ≥ 5.7)      | Check                     |
| `ext4`        | ext4                      | Likely yes                |

Modules already in-kernel (built-in) are silently skipped — same behavior as `usbgadget/modules`.

## Approach

**Netlink + initial sysfs scan (Approach B)**

- At startup: scan `/sys/block/sd*` to discover drives already connected
- Then: kernel netlink events for hot-plug add/remove
- No periodic polling — event-driven after initial scan

## Usage Example

```go
m := storage.New(
    storage.WithOnMount(func(v *storage.Volume) {
        log.Printf("volume monté: %s @ %s (%s)", v.Name, v.MountPath, v.FSType)
        // v.FS est un afero.Fs prêt à l'emploi
        afero.WriteFile(v.FS, "hello.txt", []byte("bonjour"), 0644)
    }),
    storage.WithOnUnmount(func(v *storage.Volume) {
        log.Printf("volume retiré: %s", v.Name)
    }),
)

if err := m.Start(ctx); err != nil {
    log.Printf("storage: %v", err)
}
```
