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
			break
		}
		time.Sleep(time.Duration(i+1) * 500 * time.Millisecond)
	}
	if err != nil {
		return err
	}
	// The write may succeed at the syscall level while the in-kernel bind
	// fails silently (e.g. not enough USB endpoints for the requested
	// functions). When that happens the UDC file resets to empty.
	// Read it back to detect this case and return a clear error.
	content, err := os.ReadFile(udcFile)
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(content)) == "" {
		return fmt.Errorf("UDC bind failed: gadget did not attach to %s "+
			"(too many functions for the controller's endpoint budget?)", udc)
	}
	return nil
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
