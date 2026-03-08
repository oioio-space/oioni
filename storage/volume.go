package storage

import "github.com/spf13/afero"

// Volume represents a mounted storage volume.
type Volume struct {
	// Name is the volume label or "perm" for the gokrazy persistent partition.
	Name string
	// Device is the block device path (e.g. "/dev/sda1"). Empty for perm.
	Device string
	// MountPath is where the volume is mounted (e.g. "/tmp/storage/sda1" or "/perm").
	MountPath string
	// FSType is the detected filesystem type: "vfat", "exfat", "ext4", or "perm".
	FSType string
	// Persistent is true only for the /perm volume.
	Persistent bool
	// FS is the afero filesystem ready to use. All reads/writes go through it.
	FS afero.Fs
}
