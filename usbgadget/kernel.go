// usbgadget/kernel.go
package usbgadget

import (
	"fmt"
	"os"
	"strings"
	"syscall"
)

func kernelVersion() (string, error) {
	var uts syscall.Utsname
	if err := syscall.Uname(&uts); err != nil {
		return "", fmt.Errorf("uname: %w", err)
	}
	buf := make([]byte, 0, len(uts.Release))
	for _, c := range uts.Release {
		if c == 0 {
			break
		}
		buf = append(buf, byte(c))
	}
	return strings.TrimSpace(string(buf)), nil
}

func mountConfigfs() error {
	const target = "/sys/kernel/config"
	if _, err := os.Stat(target + "/usb_gadget"); err == nil {
		return nil
	}
	err := syscall.Mount("configfs", target, "configfs", 0, "")
	if err != nil && err != syscall.EBUSY {
		return fmt.Errorf("mount configfs: %w", err)
	}
	return nil
}
