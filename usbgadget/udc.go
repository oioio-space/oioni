// usbgadget/udc.go
package usbgadget

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const udcPath = "/sys/class/udc"

func detectUDC() (string, error) {
	entries, err := os.ReadDir(udcPath)
	if err != nil {
		return "", fmt.Errorf("no UDC found at %s: %w", udcPath, err)
	}
	for _, e := range entries {
		return e.Name(), nil
	}
	return "", fmt.Errorf("no UDC devices found in %s", udcPath)
}

func (g *Gadget) bindUDC() error {
	udc, err := detectUDC()
	if err != nil {
		return err
	}
	udcFile := filepath.Join(g.gadgetDir(), "UDC")
	// DWC2 can return EBUSY for a brief window after boot while the UDC
	// finishes initialising in peripheral mode. Retry up to 5 times.
	for i := range 5 {
		err = os.WriteFile(udcFile, []byte(udc), 0644)
		if err == nil || !errors.Is(err, syscall.EBUSY) {
			return err
		}
		time.Sleep(time.Duration(i+1) * 500 * time.Millisecond)
	}
	return err
}

func (g *Gadget) unbindUDC() error {
	udcFile := filepath.Join(g.gadgetDir(), "UDC")
	content, err := os.ReadFile(udcFile)
	if err != nil {
		return nil // already unbound or gadget not set up
	}
	if strings.TrimSpace(string(content)) == "" {
		return nil
	}
	return os.WriteFile(udcFile, []byte(""), 0644)
}
