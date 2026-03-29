// Package functions provides USB gadget function drivers for use with the
// usbgadget package.
//
// Each driver implements the Function interface so it can be registered with
// usbgadget.WithFunc. Network drivers (RNDIS, ECM, EEM, Subset, NCM) also
// expose IfName and ReadStats; HID exposes TypeKey and ReleaseKeys.
//
// Usage:
//
//	rndis := functions.RNDIS(
//	    functions.WithRNDISHostAddr("02:00:00:aa:bb:01"),
//	    functions.WithRNDISDevAddr("02:00:00:aa:bb:02"),
//	)
//	g, _ := usbgadget.New(usbgadget.WithFunc(rndis))
//	g.Enable()
//	iface, _ := rndis.IfName()   // e.g. "usb0"
package functions

// Function représente un USB function driver (HID, RNDIS, ECM, etc.)
type Function interface {
	// TypeName retourne le nom du driver (ex: "hid", "rndis", "ecm")
	TypeName() string
	// InstanceName retourne le nom d'instance (ex: "usb0", "usb1")
	InstanceName() string
	// Configure écrit les attributs spécifiques dans le répertoire configfs du function
	Configure(dir string) error
}
