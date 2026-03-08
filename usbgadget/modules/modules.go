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

// MODULE_INIT_IGNORE_MODVERSIONS bypasse la vérification CRC des symboles exportés.
// Nécessaire car nous compilons sans Module.symvers du kernel gokrazy.
const moduleInitIgnoreModversions = 1

// insmod charge un module kernel depuis son contenu binaire.
// Utilise finit_module(memfd, "", IGNORE_MODVERSIONS) pour contourner les CRC
// manquants (compilation sans Module.symvers du kernel gokrazy).
func insmod(data []byte) error {
	if len(data) == 0 {
		return nil
	}

	// Crée un fichier anonyme en mémoire (pas d'accès disque requis).
	namePtr, err := syscall.BytePtrFromString("ko")
	if err != nil {
		return err
	}
	fd, _, errno := syscall.Syscall(syscall.SYS_MEMFD_CREATE, uintptr(unsafe.Pointer(namePtr)), 0, 0)
	if errno != 0 {
		return fmt.Errorf("memfd_create: %w", errno)
	}
	defer syscall.Close(int(fd))

	if _, err := syscall.Write(int(fd), data); err != nil {
		return fmt.Errorf("write module data: %w", err)
	}
	// Rewind: finit_module lit depuis la position courante du fd.
	if _, err := syscall.Seek(int(fd), 0, 0); err != nil {
		return fmt.Errorf("seek: %w", err)
	}

	paramsPtr, err := syscall.BytePtrFromString("")
	if err != nil {
		return err
	}
	_, _, errno = syscall.Syscall(
		syscall.SYS_FINIT_MODULE,
		fd,
		uintptr(unsafe.Pointer(paramsPtr)),
		moduleInitIgnoreModversions,
	)
	if errno != 0 && errno != syscall.EEXIST {
		return fmt.Errorf("finit_module: %w", errno)
	}
	return nil
}
