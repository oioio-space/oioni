// usbgadget/udc.go
package usbgadget

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	return os.WriteFile(udcFile, []byte(udc), 0644)
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
