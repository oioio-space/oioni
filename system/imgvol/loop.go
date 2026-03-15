package imgvol

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	loopControlPath = "/dev/loop-control"
	loopDevFmt      = "/dev/loop%d"
)

// withLoopDev associates imgPath with a free loop device, calls fn with the
// loop device path (e.g. "/dev/loop3"), then releases it.
func withLoopDev(imgPath string, fn func(loopDevPath string) error) error {
	ctlFd, err := os.OpenFile(loopControlPath, os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("open loop-control: %w", err)
	}
	defer ctlFd.Close()

	n, err := unix.IoctlRetInt(int(ctlFd.Fd()), unix.LOOP_CTL_GET_FREE)
	if err != nil {
		return fmt.Errorf("LOOP_CTL_GET_FREE: %w", err)
	}
	loopDev := fmt.Sprintf(loopDevFmt, n)

	loopFd, err := os.OpenFile(loopDev, os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("open %s: %w", loopDev, err)
	}
	defer loopFd.Close()

	imgFd, err := os.OpenFile(imgPath, os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("open image %s: %w", imgPath, err)
	}
	defer imgFd.Close()

	if _, _, errno := unix.Syscall(unix.SYS_IOCTL, loopFd.Fd(), unix.LOOP_SET_FD, imgFd.Fd()); errno != 0 {
		return fmt.Errorf("LOOP_SET_FD: %w", errno)
	}
	defer unix.Syscall(unix.SYS_IOCTL, loopFd.Fd(), unix.LOOP_CLR_FD, 0) //nolint:errcheck

	return fn(loopDev)
}

// attach associates path with a free loopback device, mounts it at mountpoint,
// and returns the loop device path (e.g. "/dev/loop3").
func attach(path, mountpoint, fstype string) (loopDev string, err error) {
	// 1. Get a free loop device number.
	ctlFd, err := os.OpenFile(loopControlPath, os.O_RDWR, 0)
	if err != nil {
		return "", fmt.Errorf("open loop-control: %w", err)
	}
	defer ctlFd.Close()

	n, err := unix.IoctlRetInt(int(ctlFd.Fd()), unix.LOOP_CTL_GET_FREE)
	if err != nil {
		return "", fmt.Errorf("LOOP_CTL_GET_FREE: %w", err)
	}

	loopDev = fmt.Sprintf(loopDevFmt, n)

	// 2. Open the loop device.
	loopFd, err := os.OpenFile(loopDev, os.O_RDWR, 0)
	if err != nil {
		return "", fmt.Errorf("open %s: %w", loopDev, err)
	}

	// 3. Open the image file.
	imgFd, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		loopFd.Close()
		return "", fmt.Errorf("open image %s: %w", path, err)
	}
	defer imgFd.Close()

	// 4. Associate image with loop device.
	if _, _, errno := unix.Syscall(unix.SYS_IOCTL, loopFd.Fd(), unix.LOOP_SET_FD, imgFd.Fd()); errno != 0 {
		loopFd.Close()
		return "", fmt.Errorf("LOOP_SET_FD: %w", errno)
	}
	// Defer registered after LOOP_SET_FD: LOOP_CLR_FD must run before Close to undo
	// the association. Closing loopFd first would set fd=-1, making CLR_FD a no-op.
	defer func() {
		if err != nil {
			unix.Syscall(unix.SYS_IOCTL, loopFd.Fd(), unix.LOOP_CLR_FD, 0) //nolint:errcheck
		}
		loopFd.Close()
	}()

	// 5. Set loop info (filename for /proc/mounts readability).
	var info unix.LoopInfo64
	copy(info.File_name[:], []byte(filepath.Base(path)))
	if _, _, errno := unix.Syscall(unix.SYS_IOCTL,
		loopFd.Fd(),
		unix.LOOP_SET_STATUS64,
		uintptr(unsafe.Pointer(&info))); errno != 0 {
		return "", fmt.Errorf("LOOP_SET_STATUS64: %w", errno)
	}

	// 6. Mount the loop device.
	if err := unix.Mount(loopDev, mountpoint, fstype, unix.MS_NOATIME, ""); err != nil {
		return "", fmt.Errorf("mount %s → %s: %w", loopDev, mountpoint, err)
	}

	return loopDev, nil
}

// detach unmounts mountpoint and releases the loop device.
func detach(mountpoint, loopDev string) error {
	// Lazy unmount: safe even if files are still open.
	if err := unix.Unmount(mountpoint, unix.MNT_DETACH); err != nil {
		return fmt.Errorf("unmount %s: %w", mountpoint, err)
	}

	loopFd, err := os.OpenFile(loopDev, os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("open %s for detach: %w", loopDev, err)
	}
	defer loopFd.Close()

	if _, _, errno := unix.Syscall(unix.SYS_IOCTL, loopFd.Fd(), unix.LOOP_CLR_FD, 0); errno != 0 {
		return fmt.Errorf("LOOP_CLR_FD: %w", errno)
	}
	if err := os.Remove(mountpoint); err != nil {
		return fmt.Errorf("remove mountpoint %s: %w", mountpoint, err)
	}
	return nil
}

// detectFSType reads magic bytes from the image file to identify its filesystem.
func detectFSType(path string) (FSType, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	buf := make([]byte, 1082)
	if _, err := io.ReadFull(f, buf); err != nil {
		return "", fmt.Errorf("detectFSType read: %w", err)
	}

	if len(buf) >= 11 {
		oem := string(buf[3:11])
		switch oem {
		case "EXFAT   ":
			return ExFAT, nil
		}
	}
	if len(buf) >= 0x43A {
		if buf[0x438] == 0x53 && buf[0x439] == 0xEF {
			return Ext4, nil
		}
	}
	if len(buf) >= 512 && buf[510] == 0x55 && buf[511] == 0xAA {
		return FAT, nil
	}
	return "", fmt.Errorf("imgvol: unrecognized filesystem in %s", path)
}
