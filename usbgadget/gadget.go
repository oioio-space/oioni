package usbgadget

import (
	"awesomeProject/usbgadget/functions"
	"awesomeProject/usbgadget/modules"
	"fmt"
	"os"
	"runtime"
)

type Gadget struct {
	name         string
	vendorID     uint16
	productID    uint16
	manufacturer string
	product      string
	serialNumber string
	langID       string
	usbMajor     uint8
	usbMinor     uint8
	funcs        []functions.Function
}

type Option func(*Gadget)

func New(opts ...Option) (*Gadget, error) {
	g := &Gadget{
		name:      "g1",
		vendorID:  0x1d6b,
		productID: 0x0104,
		langID:    "0x409",
		usbMajor:  2,
		usbMinor:  0,
	}
	for _, opt := range opts {
		opt(g)
	}
	return g, nil
}

func WithName(name string) Option {
	return func(g *Gadget) { g.name = name }
}

func WithVendorID(vendor, product uint16) Option {
	return func(g *Gadget) {
		g.vendorID = vendor
		g.productID = product
	}
}

func WithStrings(langID, manufacturer, product, serial string) Option {
	return func(g *Gadget) {
		g.langID = langID
		g.manufacturer = manufacturer
		g.product = product
		g.serialNumber = serial
	}
}

func WithUSBVersion(major, minor uint8) Option {
	return func(g *Gadget) {
		g.usbMajor = major
		g.usbMinor = minor
	}
}

// withFunction is the internal helper used by WithRNDIS, WithHID, etc.
func withFunction(f functions.Function) Option {
	return func(g *Gadget) { g.funcs = append(g.funcs, f) }
}

// — Network functions ——————————————————————————————————————————————

func WithRNDIS(opts ...functions.RNDISOption) Option   { return withFunction(functions.RNDIS(opts...)) }
func WithECM(opts ...functions.ECMOption) Option       { return withFunction(functions.ECM(opts...)) }
func WithNCM(opts ...functions.NCMOption) Option       { return withFunction(functions.NCM(opts...)) }
func WithEEM(opts ...functions.EEMOption) Option       { return withFunction(functions.EEM(opts...)) }
func WithSubset(opts ...functions.SubsetOption) Option { return withFunction(functions.Subset(opts...)) }

// — HID functions ——————————————————————————————————————————————————

// WithHID adds any HID function (use functions.Keyboard(), functions.Mouse(), etc.).
func WithHID(f functions.Function) Option { return withFunction(f) }

// — Storage ————————————————————————————————————————————————————————

func WithMassStorage(file string, opts ...functions.MassStorageOption) Option {
	return withFunction(functions.MassStorage(file, opts...))
}

// — Serial functions ————————————————————————————————————————————————

func WithACMSerial() Option { return withFunction(functions.ACMSerial()) }
func WithSerial() Option    { return withFunction(functions.Serial()) }
func WithOBEX() Option      { return withFunction(functions.OBEX()) }

// — Audio / MIDI ——————————————————————————————————————————————————

func WithMIDI(opts ...functions.MIDIOption) Option     { return withFunction(functions.MIDI(opts...)) }
func WithUAC1(opts ...functions.UAC1Option) Option     { return withFunction(functions.UAC1(opts...)) }
func WithUAC2(opts ...functions.UAC2Option) Option     { return withFunction(functions.UAC2(opts...)) }

// — Printer / Loopback ————————————————————————————————————————————

func WithPrinter(opts ...functions.PrinterOption) Option {
	return withFunction(functions.Printer(opts...))
}
func WithLoopback(opts ...functions.LoopbackOption) Option {
	return withFunction(functions.Loopback(opts...))
}

func (g *Gadget) Enable() error {
	if os.Getuid() != 0 {
		return fmt.Errorf("must run as root to manage USB gadgets")
	}
	if runtime.GOARCH != "arm64" {
		return fmt.Errorf("USB gadget only supported on arm64 (current: %s)", runtime.GOARCH)
	}
	kver, err := kernelVersion()
	if err != nil {
		return fmt.Errorf("kernelVersion: %w", err)
	}
	if err := modules.Load(kver); err != nil {
		return fmt.Errorf("modules.Load: %w", err)
	}
	if err := mountConfigfs(); err != nil {
		return fmt.Errorf("mountConfigfs: %w", err)
	}
	// Teardown any leftover state from a previous (crashed) run.
	_ = g.unbindUDC()
	_ = g.teardownConfigfs()
	if err := g.setupConfigfs(); err != nil {
		return fmt.Errorf("setupConfigfs: %w", err)
	}
	if err := g.bindUDC(); err != nil {
		return fmt.Errorf("bindUDC: %w", err)
	}
	return nil
}

func (g *Gadget) Disable() error {
	if err := g.unbindUDC(); err != nil {
		return fmt.Errorf("unbindUDC: %w", err)
	}
	return g.teardownConfigfs()
}

func (g *Gadget) Reload() error {
	if err := g.Disable(); err != nil {
		return err
	}
	return g.Enable()
}
