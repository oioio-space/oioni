// Package imgvol creates, formats, and loop-mounts disk image files on Linux.
//
// Static mkfs binaries for ARM64 are embedded via go:embed, so no host tools
// are required on the target system (gokrazy). Supported filesystems: FAT
// (vfat), exFAT, and ext4.
//
// # Basic usage
//
//	// Create a 64 MiB FAT image (fails if path already exists)
//	if err := imgvol.Create("/perm/data.img", 64<<20, imgvol.FAT); err != nil {
//	    log.Fatal(err)
//	}
//
//	// Open loop-mounts the image and returns an afero.Fs
//	vol, err := imgvol.Open("/perm/data.img")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer vol.Close()
//
//	if err := afero.WriteFile(vol.FS, "hello.txt", []byte("hi"), 0644); err != nil {
//	    log.Fatal(err)
//	}
//
// # Persistence on gokrazy
//
// Store images under /perm (the gokrazy persistent partition) to survive
// reboots. /tmp is a tmpfs and is cleared on every boot.
//
// # Constraints
//
//   - Requires Linux root and ARM64 (uses loop device ioctls and embedded ARM64 binaries).
//   - Only one [Volume] per image path may be open at a time; a second Open on
//     the same path while the first is still open will fail with EBUSY.
//   - NTFS is not supported (kernel module issues on gokrazy).
package imgvol
