// Package usbdetect detects USB mass storage devices via kernel events and sysfs.
package usbdetect

import (
	"os"
	"path/filepath"
	"strings"
)

// scanSysfs reads sysRoot (normally "/sys/block") and returns a list of
// partition device paths (e.g. "/dev/sda1") that belong to USB drives.
func scanSysfs(sysRoot string) ([]string, error) {
	entries, err := os.ReadDir(sysRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var devs []string
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, "sd") {
			continue
		}
		diskPath := filepath.Join(sysRoot, name)
		if !isUSBDisk(diskPath) {
			continue
		}
		parts, err := os.ReadDir(diskPath)
		if err != nil {
			continue
		}
		for _, p := range parts {
			pname := p.Name()
			if strings.HasPrefix(pname, name) && pname != name {
				devs = append(devs, "/dev/"+pname)
			}
		}
	}
	return devs, nil
}

// isUSBDisk reports whether the sysfs disk directory belongs to a USB device
// by checking if device/subsystem symlink target contains "usb".
func isUSBDisk(diskPath string) bool {
	subsystem := filepath.Join(diskPath, "device", "subsystem")
	target, err := os.Readlink(subsystem)
	if err != nil {
		return false
	}
	return strings.Contains(target, "usb")
}
