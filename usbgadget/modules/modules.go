// usbgadget/modules/modules.go
package modules

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
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

// MODULE_INIT_IGNORE_MODVERSIONS bypasse la vérification CRC des symboles exportés.
// Nécessaire car nous compilons sans Module.symvers du kernel gokrazy.
const moduleInitIgnoreModversions = 1

// insmod charge un module kernel depuis son contenu binaire.
// Utilise finit_module(fd, "", IGNORE_MODVERSIONS) pour contourner les CRC
// manquants (compilation sans Module.symvers du kernel gokrazy).
// Le .ko est écrit dans /tmp pour éviter les problèmes d'écriture partielle.
func insmod(data []byte) error {
	if len(data) == 0 {
		return nil
	}

	f, err := os.CreateTemp("/tmp", "*.ko")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpPath := f.Name()
	defer os.Remove(tmpPath)

	if _, err := f.Write(data); err != nil {
		f.Close()
		return fmt.Errorf("write module: %w", err)
	}
	// Fermer le fd d'écriture : finit_module refuse un fd ouvert en écriture (ETXTBSY).
	f.Close()

	// Rouvrir en lecture seule pour finit_module.
	ro, err := os.Open(tmpPath)
	if err != nil {
		return fmt.Errorf("open ro: %w", err)
	}
	defer ro.Close()

	paramsPtr, err := syscall.BytePtrFromString("")
	if err != nil {
		return err
	}
	_, _, errno := syscall.Syscall(
		syscall.SYS_FINIT_MODULE,
		ro.Fd(),
		uintptr(unsafe.Pointer(paramsPtr)),
		moduleInitIgnoreModversions,
	)
	if errno != 0 && errno != syscall.EEXIST {
		return fmt.Errorf("finit_module: %w", errno)
	}
	return nil
}
