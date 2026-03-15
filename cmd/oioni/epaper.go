// hello/epaper.go — e-ink display + touch integration for gokrazy/Pi Zero 2W
package main

import (
	"context"
	"image"
	"log"
	"sync"

	"github.com/oioio-space/oioni/drivers/epd"
	"github.com/oioio-space/oioni/ui/gui"
	"github.com/oioio-space/oioni/drivers/touch"
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
)

type epaperState struct {
	nav       *gui.Navigator
	status    *gui.StatusBar
	td        *touch.Detector
	mu        sync.Mutex
	cancelFn  context.CancelFunc
	renderReq chan struct{}
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

	nav := gui.NewNavigator(d)

	header := gui.NewLabel("oioio")
	header.SetBounds(image.Rect(0, 0, 250, 16))

	divider := gui.NewDivider()
	divider.SetBounds(image.Rect(0, 16, 250, 17))

	status := gui.NewStatusBar("", "")
	status.SetBounds(image.Rect(0, 17, 250, 122))

	scene := &gui.Scene{
		Widgets: []gui.Widget{header, divider, status},
	}
	if err := nav.Push(scene); err != nil {
		log.Printf("epaper: initial render failed: %v", err)
		cancel()
		_ = d.Sleep()
		_ = d.Close()
		return nil
	}

	renderReq := make(chan struct{}, 1)

	go func() {
		defer func() {
			_ = td.Close()
			_ = d.Sleep()
			_ = d.Close()
		}()
		for {
			select {
			case <-guiCtx.Done():
				return
			case _, ok := <-tc:
				if !ok {
					return
				}
				// Touch events are handled by nav.Run internally,
				// but since we need a custom loop, just call Render on touch too.
				if err := nav.Render(); err != nil {
					log.Printf("epaper: touch render: %v", err)
				}
			case <-renderReq:
				if err := nav.Render(); err != nil {
					log.Printf("epaper: render: %v", err)
				}
			}
		}
	}()

	return &epaperState{nav: nav, status: status, td: td, cancelFn: cancel, renderReq: renderReq}
}

// UpdateStatus updates the status bar text and triggers a re-render.
func (e *epaperState) UpdateStatus(left, right string) {
	if e == nil {
		return
	}
	e.mu.Lock()
	e.status.SetLeft(left)
	e.status.SetRight(right)
	e.mu.Unlock()
	// Non-blocking send — if channel is full, a render is already queued.
	select {
	case e.renderReq <- struct{}{}:
	default:
	}
}

// Close cancels the GUI goroutine and releases hardware resources.
func (e *epaperState) Close() {
	if e == nil {
		return
	}
	e.cancelFn()
}
