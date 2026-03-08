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
