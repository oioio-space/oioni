// cmd/oioni/epaper.go — e-ink display + touch integration for gokrazy/Pi Zero 2W
package main

import (
	"context"
	"log"
	"time"

	oioniui "github.com/oioio-space/oioni/cmd/oioni/ui"
	"github.com/oioio-space/oioni/drivers/epd"
	"github.com/oioio-space/oioni/drivers/touch"
	"github.com/oioio-space/oioni/system/netconf"
	"github.com/oioio-space/oioni/system/wifi"
	"github.com/oioio-space/oioni/ui/gui"
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

// epdAdapter wraps *epd.Display so it satisfies gui.Display.
// gui.DisplayMode values are identical to epd.Mode, so the cast is lossless.
type epdAdapter struct{ *epd.Display }

func (a epdAdapter) Init(m gui.DisplayMode) error { return a.Display.Init(epd.Mode(m)) }

// adaptTouchEvents converts a drivers/touch event channel into a gui.TouchEvent
// channel. Runs in a goroutine until src is closed or ctx is cancelled.
func adaptTouchEvents(ctx context.Context, src <-chan touch.TouchEvent) <-chan gui.TouchEvent {
	dst := make(chan gui.TouchEvent)
	go func() {
		defer close(dst)
		for {
			select {
			case <-ctx.Done():
				return
			case e, ok := <-src:
				if !ok {
					return
				}
				pts := make([]gui.TouchPoint, len(e.Points))
				for i, p := range e.Points {
					pts[i] = gui.TouchPoint{ID: p.ID, X: p.X, Y: p.Y, Size: p.Size}
				}
				select {
				case dst <- gui.TouchEvent{Points: pts, Time: e.Time}:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return dst
}

type epaperState struct {
	nav    *gui.Navigator
	nsb    *gui.NetworkStatusBar
	cancel context.CancelFunc
}

func startEPaper(ctx context.Context, wifiMgr *wifi.Manager, netconfMgr *netconf.Manager) *epaperState {
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

	nav := gui.NewNavigatorWithIdle(epdAdapter{d}, idleTimeout)

	home, nsb := oioniui.NewHomeScene(nav, wifiMgr, netconfMgr)
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
		nav.Run(guiCtx, adaptTouchEvents(guiCtx, tc))
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
