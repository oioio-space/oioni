// Package usbgadget configures Linux USB composite gadgets via configfs.
//
// It loads the required kernel modules, mounts configfs, creates the gadget
// directory tree, and binds the UDC (USB Device Controller) — all from a
// single Enable() call using functional options.
//
// # Architecture
//
// The Linux USB gadget stack works as follows:
//
//	Host PC ──USB──► Pi (dwc2 UDC) ──► configfs ──► function drivers
//	                                                  (rndis, hid, ...)
//
// Each function appears as a distinct USB interface to the host.
// Multiple functions combined form a composite gadget.
//
// # Quick Start
//
//	g, err := usbgadget.New(
//	    usbgadget.WithName("my-gadget"),
//	    usbgadget.WithVendorID(0x1d6b, 0x0104),
//	    usbgadget.WithStrings("0x409", "ACME", "My Device", "001"),
//	    usbgadget.WithRNDIS(),
//	    usbgadget.WithHID(functions.Keyboard()),
//	)
//	if err := g.Enable(); err != nil {
//	    log.Fatal(err)
//	}
//	defer g.Disable()
//
// # Network functions
//
// Three USB network protocols are supported, each creating a real network
// interface on both the gadget (Pi) and the host:
//
//   - RNDIS: Windows-compatible (must be listed FIRST in composite)
//   - ECM:   Linux/macOS (CDC Ethernet Control Model)
//   - NCM:   High-throughput (CDC Network Control Model, Linux 3.10+)
//   - EEM:   Simple bulk-only Ethernet (Linux-to-Linux)
//   - Subset: CDC Subset, no union descriptor, broadest compatibility
//
// After Enable(), use IfName() to get the Pi-side interface name and
// configure an IP address with ip(8) or netlink.
//
// # HID functions
//
// HID functions create /dev/hidgN devices. Use WriteReport() to send
// input events to the host and ReadLEDs() to receive LED state changes
// (NumLock, CapsLock, ScrollLock) from the host.
//
// # Serial functions
//
// ACM, GSER, and OBEX all create /dev/ttyGSN devices on the Pi.
// Port numbers are assigned in creation order across all serial-type functions.
//
// # Audio functions
//
// UAC1 and UAC2 create standard USB audio devices. No host driver needed on
// Linux, macOS, or Windows 10+. Audio streams appear under ALSA on the host.
//
// # Kernel modules
//
// Modules are loaded from the embedded .ko files in usbgadget/modules/.
// Missing modules are silently skipped (EEXIST is also ignored — the module
// may be built into the gokrazy kernel).
package usbgadget
