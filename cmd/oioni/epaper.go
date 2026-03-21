// cmd/oioni/epaper.go — e-ink display + touch integration for gokrazy/Pi Zero 2W
package main

import (
	"context"
	"log"
	"time"

	"github.com/oioio-space/oioni/drivers/epd"
	"github.com/oioio-space/oioni/drivers/touch"
	"github.com/oioio-space/oioni/ui/gui"
	oioniui "github.com/oioio-space/oioni/cmd/oioni/ui"
)

const (
	epdSPIDevice = "/dev/spidev0.0"
	epdSPISpeed  = uint32(4_000_000)
	epdPinRST    = 17
	epdPinDC     = 25
	epdPinCS     = 8
	epdPinBUSY   = 24
	touchDevice  = "/dev/i2c-1"
	touchAddr    = 0x14
	touchPinTRST = 22
	touchPinINT  = 27

	idleTimeout = 60 * time.Second
)

type epaperState struct {
	nav    *gui.Navigator
	nsb    *gui.NetworkStatusBar
	cancel context.CancelFunc
}

func startEPaper(ctx context.Context) *epaperState {
	d, err := epd.New(epd.Config{
		SPIDevice: epdSPIDevice,
		SPISpeed:  epdSPISpeed,
		PinRST:    epdPinRST,
		PinDC:     epdPinDC,
		PinCS:     epdPinCS,
		PinBUSY:   epdPinBUSY,
	})
	if err != nil {
		log.Printf("epaper: display unavailable: %v", err)
		return nil
	}

	td, err := touch.New(touch.Config{
		I2CDevice: touchDevice,
		I2CAddr:   uint16(touchAddr),
		PinTRST:   touchPinTRST,
		PinINT:    touchPinINT,
	})
	if err != nil {
		log.Printf("epaper: touch unavailable: %v", err)
		_ = d.Close()
		return nil
	}

	guiCtx, cancel := context.WithCancel(ctx)
	tc, err := td.Start(guiCtx)
	if err != nil {
		log.Printf("epaper: touch start failed: %v", err)
		cancel()
		_ = d.Close()
		return nil
	}

	nav := gui.NewNavigatorWithIdle(d, idleTimeout)

	home, nsb := oioniui.NewHomeScene(nav)
	if err := nav.Push(home); err != nil {
		log.Printf("epaper: initial render failed: %v", err)
		cancel()
		_ = d.Sleep()
		_ = d.Close()
		return nil
	}

	oioniui.StartKeepAlive(guiCtx, nav)

	go func() {
		defer func() {
			_ = td.Close()
			_ = d.Sleep()
			_ = d.Close()
		}()
		nav.Run(guiCtx, tc)
	}()

	return &epaperState{nav: nav, nsb: nsb, cancel: cancel}
}

// UpdateStatus is a no-op stub — wire real iface/tool data via nsb.SetInterfaces/SetTools.
func (e *epaperState) UpdateStatus(_, _ string) {
	if e == nil {
		return
	}
}

// Close cancels the GUI goroutine and releases hardware resources.
func (e *epaperState) Close() {
	if e == nil {
		return
	}
	e.cancel()
}
