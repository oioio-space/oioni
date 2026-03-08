// usbgadget/configfs.go
package usbgadget

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

const configfsRoot = "/sys/kernel/config/usb_gadget"

func (g *Gadget) gadgetDir() string {
	return filepath.Join(configfsRoot, g.name)
}

func (g *Gadget) setupConfigfs() error {
	dir := g.gadgetDir()

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("mkdir gadget: %w", err)
	}

	if err := writeHex(filepath.Join(dir, "idVendor"), uint64(g.vendorID)); err != nil {
		return err
	}
	if err := writeHex(filepath.Join(dir, "idProduct"), uint64(g.productID)); err != nil {
		return err
	}

	bcd := uint64(g.usbMajor)<<8 | uint64(g.usbMinor)
	if err := writeHex(filepath.Join(dir, "bcdUSB"), bcd); err != nil {
		return err
	}

	if g.manufacturer != "" || g.product != "" || g.serialNumber != "" {
		strDir := filepath.Join(dir, "strings", g.langID)
		if err := os.MkdirAll(strDir, 0755); err != nil {
			return fmt.Errorf("mkdir strings: %w", err)
		}
		if g.manufacturer != "" {
			if err := writeString(filepath.Join(strDir, "manufacturer"), g.manufacturer); err != nil {
				return err
			}
		}
		if g.product != "" {
			if err := writeString(filepath.Join(strDir, "product"), g.product); err != nil {
				return err
			}
		}
		if g.serialNumber != "" {
			if err := writeString(filepath.Join(strDir, "serialnumber"), g.serialNumber); err != nil {
				return err
			}
		}
	}

	cfgDir := filepath.Join(dir, "configs", "c.1")
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		return fmt.Errorf("mkdir config: %w", err)
	}
	if err := writeString(filepath.Join(cfgDir, "MaxPower"), "250"); err != nil {
		return err
	}

	orderedFuncs := sortFunctions(g.funcs)
	for _, f := range orderedFuncs {
		funcPath := fmt.Sprintf("%s.%s", f.TypeName(), f.InstanceName())
		funcDir := filepath.Join(dir, "functions", funcPath)
		if err := os.MkdirAll(funcDir, 0755); err != nil {
			return fmt.Errorf("mkdir function %s: %w", funcPath, err)
		}
		if err := f.Configure(funcDir); err != nil {
			return fmt.Errorf("configure %s: %w", funcPath, err)
		}
		linkDst := filepath.Join(dir, "functions", funcPath)
		linkSrc := filepath.Join(cfgDir, funcPath)
		if err := os.Symlink(linkDst, linkSrc); err != nil && !os.IsExist(err) {
			return fmt.Errorf("symlink %s: %w", funcPath, err)
		}
	}

	return nil
}

func (g *Gadget) teardownConfigfs() error {
	dir := g.gadgetDir()
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil
	}

	cfgDir := filepath.Join(dir, "configs", "c.1")
	entries, _ := os.ReadDir(cfgDir)
	for _, e := range entries {
		if e.Type()&os.ModeSymlink != 0 {
			os.Remove(filepath.Join(cfgDir, e.Name()))
		}
	}

	os.Remove(filepath.Join(dir, "configs", "c.1", "strings", g.langID))
	os.Remove(filepath.Join(dir, "configs", "c.1"))
	os.Remove(filepath.Join(dir, "configs"))

	funcsDir := filepath.Join(dir, "functions")
	entries, _ = os.ReadDir(funcsDir)
	for _, e := range entries {
		os.Remove(filepath.Join(funcsDir, e.Name()))
	}
	os.Remove(funcsDir)

	os.Remove(filepath.Join(dir, "strings", g.langID))
	os.Remove(filepath.Join(dir, "strings"))
	return os.Remove(dir)
}

func writeHex(path string, value uint64) error {
	s := "0x" + strconv.FormatUint(value, 16)
	return os.WriteFile(path, []byte(s), 0644)
}

func writeString(path, value string) error {
	return os.WriteFile(path, []byte(value), 0644)
}
