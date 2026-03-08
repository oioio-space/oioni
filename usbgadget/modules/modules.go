// usbgadget/modules/modules.go
package modules

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"syscall"
	"unsafe"
)

// Load charge les modules USB gadget nécessaires pour la version kernel kver.
// Les .ko sont extraits de l'embed FS et chargés via insmod (init_module syscall).
func Load(kver string) error {
	deps := []string{
		"libcomposite",
		"u_ether",
		"usb_f_rndis",
		"usb_f_ecm",
		"usb_f_ncm",
		"usb_f_hid",
		"usb_f_mass_storage",
		"u_serial",
		"usb_f_acm",
	}
	for _, name := range deps {
		if err := loadModule(kver, name); err != nil {
			return fmt.Errorf("loading %s: %w", name, err)
		}
	}
	return nil
}

func loadModule(kver, name string) error {
	src := path.Join(kver, name+".ko")
	data, err := koFS.ReadFile(src)
	if errors.Is(err, fs.ErrNotExist) {
		// .ko absent pour cette version kernel — skip silencieux
		return nil
	}
	if err != nil {
		return err
	}
	return insmod(data)
}

// insmod charge un module kernel depuis son contenu binaire via init_module syscall.
func insmod(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	params := ""
	paramsPtr, err := syscall.BytePtrFromString(params)
	if err != nil {
		return err
	}
	_, _, errno := syscall.Syscall(
		syscall.SYS_INIT_MODULE,
		uintptr(unsafe.Pointer(&data[0])),
		uintptr(len(data)),
		uintptr(unsafe.Pointer(paramsPtr)),
	)
	if errno != 0 && errno != syscall.EEXIST {
		return fmt.Errorf("init_module: %w", errno)
	}
	return nil
}
