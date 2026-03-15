// Package touch drives the Waveshare GT1151 capacitive touch controller over I2C.
//
// The GT1151 supports up to 5 simultaneous touch points. It is used on the
// Waveshare 2.13inch Touch e-Paper HAT alongside the EPD_2in13_V4 display.
//
// Hardware: Waveshare 2.13inch Touch e-Paper HAT
// https://www.waveshare.com/wiki/2.13inch_Touch_e-Paper_HAT
//
// Usage:
//
//	td, err := touch.New(touch.Config{
//	    I2CDevice: "/dev/i2c-1", I2CAddr: 0x14,
//	    PinTRST: 22, PinINT: 27,
//	})
//	events, err := td.Start(ctx)
//	for ev := range events {
//	    for _, pt := range ev.Points {
//	        log.Printf("touch at (%d,%d)", pt.X, pt.Y)
//	    }
//	}
package touch
