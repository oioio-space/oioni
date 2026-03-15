# storage — USB hotplug + persistent storage manager

[![Go Reference](https://pkg.go.dev/badge/github.com/oioio-space/oioni/system/storage.svg)](https://pkg.go.dev/github.com/oioio-space/oioni/system/storage)

Manages storage on gokrazy: the `/perm` persistent partition and USB mass
storage drives detected via kernel netlink events. Each volume is exposed as an
[afero.Fs](https://github.com/spf13/afero) sandboxed to its mount point.

## Install

```sh
go get github.com/oioio-space/oioni/system/storage
```

## Quick start

```go
m := storage.New(
    storage.WithOnMount(func(v *storage.Volume) {
        log.Printf("mounted %s (%s) at %s", v.Name, v.FSType, v.MountPath)
        // v.FS is ready: afero.WriteFile(v.FS, "hello.txt", ...)
    }),
    storage.WithOnUnmount(func(v *storage.Volume) {
        log.Printf("unmounted %s", v.Name)
    }),
)

ctx, cancel := context.WithCancel(context.Background())
defer cancel()

if err := m.Start(ctx); err != nil {
    log.Fatal(err)
}

// /perm is always available immediately after Start
perm, ok := m.Volume("perm")
if ok {
    afero.WriteFile(perm.FS, "config.json", data, 0644)
}

// USB drives appear via OnMount callback as they are plugged in
```

## How it works

1. `Start` opens a kernel netlink socket for `KOBJECT_UEVENT` before scanning
   `/sys/block/sd*` — this avoids a race where a drive plugged during boot
   would be missed.
2. Each block device is checked via `/sys/block/sdX/device/subsystem` to
   confirm it is USB (not SATA or eMMC).
3. The filesystem type is auto-detected from magic bytes:
   - exFAT first (shares the 0x55AA boot signature with FAT)
   - ext4 (`0xEF53` at offset 0x438)
   - FAT/vfat (0x55AA at offset 510)
4. Each volume is loop-mounted under `/tmp/storage/<device>` and wrapped in
   `afero.NewBasePathFs` — callers cannot escape the mount point.
5. `/perm` is never unmounted by the Manager (gokrazy owns its lifecycle).

## Volume fields

```go
type Volume struct {
    Name       string   // "perm" or partition label
    Device     string   // "/dev/sda1"
    MountPath  string   // "/tmp/storage/sda1" or "/perm"
    FSType     string   // "vfat", "exfat", "ext4", "perm"
    Persistent bool     // true only for /perm
    FS         afero.Fs // sandboxed filesystem ready to use
}
```

## Testability

`New()` injects real kernel/mount implementations. For unit tests without root
or hardware, the internal `newManager(det, mnt, ...)` constructor accepts fake
implementations of the `detector` and `mounter` interfaces.

## License

MIT — see [LICENSE](../../LICENSE).
