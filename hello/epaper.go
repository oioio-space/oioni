// hello/epaper.go — e-ink display + touch integration for gokrazy/Pi Zero 2W
package main

import (
	"context"
	"fmt"
	"image"
	"log"
	"os"
	"time"

	"awesomeProject/epaper/canvas"
	"awesomeProject/epaper/epd"
	"awesomeProject/epaper/touch"
)

// epaperCfg matches the Waveshare 2.13" Touch e-Paper HAT wiring.
// BCM pin numbers from DEV_Config.c in the Waveshare reference library.
var epaperCfg = struct {
	epd   epd.Config
	touch touch.Config
}{
	epd: epd.Config{
		SPIDevice: "/dev/spidev0.0",
		SPISpeed:  4_000_000,
		PinRST:    17,
		PinDC:     25,
		PinCS:     8,
		PinBUSY:   24,
	},
	touch: touch.Config{
		I2CDevice: "/dev/i2c-1",
		I2CAddr:   0x14,
		PinTRST:   22,
		PinINT:    27,
	},
}

// epaperState holds the running display and canvas.
type epaperState struct {
	display *epd.Display
	c       *canvas.Canvas
}

// startEPaper initialises the display and touch controller, draws a boot screen,
// and launches a goroutine that logs touch events until ctx is cancelled.
// Returns nil if the display hardware could not be opened (error is logged;
// the caller continues without the display).
func startEPaper(ctx context.Context) *epaperState {
	disp, err := epd.New(epaperCfg.epd)
	if err != nil {
		log.Printf("epaper: display: %v", err)
		return nil
	}
	if err := disp.Init(epd.ModeFull); err != nil {
		log.Printf("epaper: Init: %v", err)
		disp.Close()
		return nil
	}

	// Rot90 → logical 250 wide × 122 tall (landscape).
	c := canvas.New(epd.Width, epd.Height, canvas.Rot90)
	drawBootScreen(c)
	if err := disp.DisplayFull(c.Bytes()); err != nil {
		log.Printf("epaper: DisplayFull: %v", err)
	}
	log.Println("epaper: boot screen displayed")

	det, err := touch.New(epaperCfg.touch)
	if err != nil {
		log.Printf("epaper: touch: %v (display still active)", err)
	} else {
		events, err := det.Start(ctx)
		if err != nil {
			log.Printf("epaper: touch.Start: %v", err)
			det.Close()
		} else {
			go logTouchEvents(ctx, events, det)
		}
	}

	return &epaperState{display: disp, c: c}
}

// UpdateStatus redraws the status lines below the header and does a fast refresh.
func (s *epaperState) UpdateStatus(lines []string) {
	if s == nil {
		return
	}
	if err := s.display.Init(epd.ModeFast); err != nil {
		log.Printf("epaper: Init(Fast): %v", err)
		return
	}
	drawStatusLines(s.c, lines)
	if err := s.display.DisplayFast(s.c.Bytes()); err != nil {
		log.Printf("epaper: DisplayFast: %v", err)
	}
}

// Close puts the display to sleep and releases all resources.
func (s *epaperState) Close() {
	if s == nil {
		return
	}
	if err := s.display.Sleep(); err != nil {
		log.Printf("epaper: Sleep: %v", err)
	}
	s.display.Close()
}

// ── drawing ──────────────────────────────────────────────────────────────────

// Logical canvas dimensions after Rot90: 250 wide × 122 tall.
const (
	epdLW = 250
	epdLH = 122
)

func drawBootScreen(c *canvas.Canvas) {
	c.Clear()
	f16 := canvas.EmbeddedFont(16)
	f12 := canvas.EmbeddedFont(12)

	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "oioio"
	}

	// Header: black bar with hostname in white.
	c.DrawRect(image.Rect(0, 0, epdLW, 20), canvas.Black, true)
	c.DrawText(4, 2, hostname, f16, canvas.White)

	// Separator line.
	c.DrawLine(0, 21, epdLW-1, 21, canvas.Black)

	// Status area: boot message + time.
	c.DrawText(4, 26, "demarrage...", f12, canvas.Black)
	c.DrawText(4, 40, time.Now().Format("15:04:05"), f12, canvas.Black)
}

func drawStatusLines(c *canvas.Canvas, lines []string) {
	f12 := canvas.EmbeddedFont(12)

	// Clear status area only (preserves header).
	c.SetClip(image.Rect(0, 22, epdLW, epdLH))
	c.Fill(canvas.White)
	c.ClearClip()

	y := 26
	for _, l := range lines {
		if y+12 > epdLH {
			break
		}
		c.DrawText(4, y, l, f12, canvas.Black)
		y += 14
	}
}

// ── touch logging ─────────────────────────────────────────────────────────────

func logTouchEvents(ctx context.Context, events <-chan touch.TouchEvent, det *touch.Detector) {
	for ev := range events {
		for _, pt := range ev.Points {
			log.Printf("touch: id=%d x=%d y=%d size=%d", pt.ID, pt.X, pt.Y, pt.Size)
		}
	}
	if err := det.Err(); err != nil {
		log.Printf("epaper: touch goroutine: %v", err)
	}
}

// statusLines builds the display status text from current gadget state.
func statusLines(gadgetActive bool, functions []string) []string {
	lines := make([]string, 0, 4)
	if gadgetActive {
		lines = append(lines, "USB: "+fmt.Sprintf("%v", functions))
	} else {
		lines = append(lines, "USB: inactif")
	}
	lines = append(lines, time.Now().Format("15:04:05"))
	return lines
}
