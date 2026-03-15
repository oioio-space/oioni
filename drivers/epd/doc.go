// Package epd drives the Waveshare EPD 2.13" V4 e-ink display over SPI.
//
// The display is 122×250 pixels, 1 bit per pixel (black/white).
// It supports three refresh modes: full (~2 s, best quality), fast (~0.5 s),
// and partial (~0.3 s, no full-screen flash — requires a prior DisplayBase call).
//
// Hardware: Waveshare 2.13inch Touch e-Paper HAT
// https://www.waveshare.com/wiki/2.13inch_Touch_e-Paper_HAT
//
// Typical usage:
//
//	d, err := epd.New(epd.Config{
//	    SPIDevice: "/dev/spidev0.0", SPISpeed: 4_000_000,
//	    PinRST: 17, PinDC: 25, PinCS: 8, PinBUSY: 24,
//	})
//	d.Init(epd.ModeFull)
//	d.DisplayBase(buf)          // sets reference frame
//	d.DisplayPartial(buf)       // fast update, no ghost flash
//	d.Sleep()
//	d.Close()
package epd
