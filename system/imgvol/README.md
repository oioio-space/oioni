# imgvol — disk image manager for gokrazy

[![Go Reference](https://pkg.go.dev/badge/github.com/oioio-space/oioni/system/imgvol.svg)](https://pkg.go.dev/github.com/oioio-space/oioni/system/imgvol)

Creates, formats, and loop-mounts disk image files on Linux ARM64 (gokrazy).
Static `mkfs` binaries are embedded via `go:embed` — no host tools required on
the target system.

**Supported filesystems:** FAT (`vfat`), exFAT, ext4.

## Install

```sh
go get github.com/oioio-space/oioni/system/imgvol
```

## Quick start

```go
import (
    "github.com/oioio-space/oioni/system/imgvol"
    "github.com/spf13/afero"
)

// Create a 64 MiB FAT image on the persistent partition
if err := imgvol.Create("/perm/data.img", 64<<20, imgvol.FAT); err != nil {
    log.Fatal(err)
}

// Open loop-mounts the image and exposes it as afero.Fs
vol, err := imgvol.Open("/perm/data.img")
if err != nil {
    log.Fatal(err)
}
defer vol.Close()

// Standard afero operations
afero.WriteFile(vol.FS, "notes.txt", []byte("hello"), 0644)
data, _ := afero.ReadFile(vol.FS, "notes.txt")
fmt.Println(string(data)) // hello
```

## Filesystems

| Constant | Type | Notes |
|----------|------|-------|
| `imgvol.FAT` | vfat | Best host compatibility (Windows/macOS/Linux) |
| `imgvol.ExFAT` | exfat | Large files, good host compatibility |
| `imgvol.Ext4` | ext4 | Linux-only, journaled |

> NTFS is not supported — the ntfs3 kernel module causes panics on the gokrazy kernel.

## Persistence

| Path | Survives reboot? |
|------|-----------------|
| `/perm/data.img` | Yes — gokrazy persistent partition |
| `/tmp/data.img` | No — tmpfs, cleared on boot |

Store images under `/perm` for data that must survive reboots.

## Constraints

- Requires Linux root (loop device ioctls).
- ARM64 only (embedded static binaries are ARM64).
- One `Open` per image at a time (second `Open` fails with `EBUSY`).

## License

MIT — see [LICENSE](../../LICENSE).
