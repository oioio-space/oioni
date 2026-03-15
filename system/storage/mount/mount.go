package mount

import (
	"fmt"
	"os"
	"syscall"
)

const mountFlags = syscall.MS_NOATIME | syscall.MS_RELATIME

// Mount mounts device at mountpoint with the given fstype.
// Creates mountpoint directory if it does not exist.
func Mount(device, mountpoint, fstype string) error {
	if err := os.MkdirAll(mountpoint, 0755); err != nil {
		return fmt.Errorf("mount mkdir %s: %w", mountpoint, err)
	}
	if err := syscall.Mount(device, mountpoint, fstype, mountFlags, ""); err != nil {
		return fmt.Errorf("mount %s → %s (%s): %w", device, mountpoint, fstype, err)
	}
	return nil
}

// Unmount unmounts the given mountpoint using MNT_DETACH (lazy unmount).
// Safe to call even if the device was yanked out.
func Unmount(mountpoint string) error {
	if err := syscall.Unmount(mountpoint, syscall.MNT_DETACH); err != nil {
		return fmt.Errorf("unmount %s: %w", mountpoint, err)
	}
	return nil
}
