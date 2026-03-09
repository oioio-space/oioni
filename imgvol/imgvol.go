// Package imgvol creates, formats, and loop-mounts disk image files.
// Supported formats: FAT (vfat), exFAT, NTFS, ext4.
// Static mkfs binaries for ARM64 are embedded via go:embed.
//
// Usage:
//
//	if err := imgvol.Create("/perm/data.img", 64<<20, imgvol.FAT); err != nil { ... }
//	vol, err := imgvol.Open("/perm/data.img")
//	defer vol.Close()
//	afero.WriteFile(vol.FS, "hello.txt", []byte("hi"), 0644)
package imgvol

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/afero"
)

// FSType identifies the filesystem to create or mount.
type FSType string

const (
	FAT   FSType = "vfat"
	ExFAT FSType = "exfat"
	NTFS  FSType = "ntfs"
	Ext4  FSType = "ext4"
)

// Volume represents a loop-mounted disk image ready for afero access.
// Call Close when done — it unmounts the image and releases the loop device.
type Volume struct {
	Path   string
	FSType FSType
	// FS is an afero filesystem rooted at the image's mount point.
	// All reads and writes go through this interface.
	FS afero.Fs

	loopDev    string
	mountpoint string
}

// Create creates a sparse image file at path with the given size (bytes)
// and formats it with fstype. Fails if path already exists.
func Create(path string, size int64, fstype FSType) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("imgvol.Create: %s already exists", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("imgvol.Create mkdir: %w", err)
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("imgvol.Create: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(path)
		return fmt.Errorf("imgvol.Create close: %w", err)
	}
	if err := os.Truncate(path, size); err != nil {
		os.Remove(path)
		return fmt.Errorf("imgvol.Create truncate: %w", err)
	}
	if err := format(path, fstype); err != nil {
		os.Remove(path)
		return fmt.Errorf("imgvol.Create format: %w", err)
	}
	return nil
}

// Open detects the filesystem type, loop-mounts the image, and returns a Volume.
// The caller must call vol.Close() when done.
// Only one Volume per image path may be open at a time; a second Open on the
// same path while the first is still open will fail with EBUSY at mount time.
func Open(path string) (*Volume, error) {
	fstype, err := detectFSType(path)
	if err != nil {
		return nil, fmt.Errorf("imgvol.Open detect: %w", err)
	}
	mp := filepath.Join("/tmp/imgvol", filepath.Base(path))
	if err := os.MkdirAll(mp, 0755); err != nil {
		return nil, fmt.Errorf("imgvol.Open mkdir: %w", err)
	}
	loopDev, err := attach(path, mp, string(fstype))
	if err != nil {
		os.Remove(mp)
		return nil, fmt.Errorf("imgvol.Open attach: %w", err)
	}
	return &Volume{
		Path:       path,
		FSType:     fstype,
		FS:         afero.NewBasePathFs(afero.NewOsFs(), mp),
		loopDev:    loopDev,
		mountpoint: mp,
	}, nil
}

// Close unmounts the image and releases the loopback device.
func (v *Volume) Close() error {
	return detach(v.mountpoint, v.loopDev)
}
