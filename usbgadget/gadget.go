package usbgadget

import (
	"awesomeProject/usbgadget/functions"
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

func (g *Gadget) Enable() error { return nil } // implémenté task 4

func (g *Gadget) Disable() error { return nil } // implémenté task 4

func (g *Gadget) Reload() error {
	if err := g.Disable(); err != nil {
		return err
	}
	return g.Enable()
}
