// epaper/gui/refresh.go — smart partial/full refresh decision engine
package gui

import (
	"bytes"
	"slices"

	"github.com/oioio-space/oioni/ui/canvas"
)

// maxFastBeforeBase is the maximum consecutive DisplayFast() calls before forcing a
// DisplayBase() to re-sync the 0x26 reference frame. DisplayFast only writes 0x24;
// without periodic base sync, subsequent partial updates see a stale reference.
const maxFastBeforeBase = 3

// refreshManager tracks dirty state and decides refresh strategy.
// Full refreshes occur only on explicit scene transitions (Push/Pop) and when
// content changes exceed 60% of bytes (content-based anti-ghosting).
type refreshManager struct {
	display   Display
	fastCount int    // consecutive fast refreshes since last base sync
	hasBase   bool
	prevBuf   []byte // last buffer sent to the display (nil until first render)
}

func newRefreshManager(d Display) *refreshManager {
	return &refreshManager{display: d}
}

// Render draws dirty widgets and refreshes with the appropriate strategy.
// Noop if no widget is dirty. Always uses partial refresh; full refresh is
// triggered only by content-based threshold or explicit RenderWith(forced=true).
func (rm *refreshManager) Render(c *canvas.Canvas, widgets []Widget) error {
	if !slices.ContainsFunc(widgets, func(w Widget) bool { return w.IsDirty() }) {
		return nil
	}
	return rm.partialRefresh(c, widgets)
}

// RenderWith draws all widgets and forces a full (forced=true) or partial refresh.
// forced=true is used on Push/Pop (scene change) and on first render.
func (rm *refreshManager) RenderWith(c *canvas.Canvas, widgets []Widget, forced bool) error {
	if forced {
		return rm.fullRefresh(c, widgets)
	}
	return rm.partialRefresh(c, widgets)
}

func (rm *refreshManager) fullRefresh(c *canvas.Canvas, widgets []Widget) error {
	if err := rm.display.Init(ModeFull); err != nil {
		return err
	}
	drawAll(c, widgets)
	buf := c.Bytes()
	if err := rm.display.DisplayBase(buf); err != nil {
		return err
	}
	rm.prevBuf = buf
	markAllClean(widgets)
	rm.fastCount = 0
	rm.hasBase = true
	return nil
}

// FastRefresh renders dirty widgets using DisplayFast.
// After maxFastBeforeBase consecutive calls, automatically falls back to fullRefresh
// to re-sync the 0x26 reference frame (DisplayFast only writes 0x24).
func (rm *refreshManager) FastRefresh(c *canvas.Canvas, widgets []Widget) error {
	if !slices.ContainsFunc(widgets, func(w Widget) bool { return w.IsDirty() }) {
		return nil
	}
	if rm.fastCount >= maxFastBeforeBase {
		rm.fastCount = 0
		return rm.fullRefresh(c, widgets)
	}
	drawAll(c, widgets)
	buf := c.Bytes()
	if bytes.Equal(buf, rm.prevBuf) {
		markAllClean(widgets)
		return nil
	}
	if err := rm.display.DisplayFast(buf); err != nil {
		return err
	}
	rm.prevBuf = buf
	markAllClean(widgets)
	rm.fastCount++
	return nil
}

func (rm *refreshManager) partialRefresh(c *canvas.Canvas, widgets []Widget) error {
	if !rm.hasBase {
		// No base established yet — fall back to full.
		return rm.fullRefresh(c, widgets)
	}
	for _, w := range widgets {
		if w.IsDirty() {
			w.Draw(c)
		}
	}
	buf := c.Bytes()
	// Skip SPI transfer if framebuffer is identical to what's currently on screen.
	if bytes.Equal(buf, rm.prevBuf) {
		markAllClean(widgets)
		return nil
	}
	// Content-based full refresh: >60% bytes changed → ghosting risk too high for partial.
	if rm.prevBuf != nil && countDiffBytes(buf, rm.prevBuf)*100 > len(buf)*60 {
		return rm.fullRefresh(c, widgets)
	}
	if err := rm.display.DisplayPartial(buf); err != nil {
		return err
	}
	rm.prevBuf = buf
	markAllClean(widgets)
	return nil
}

// countDiffBytes returns the number of byte positions that differ between a and b.
func countDiffBytes(a, b []byte) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	diff := 0
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			diff++
		}
	}
	return diff
}

func drawAll(c *canvas.Canvas, widgets []Widget) {
	for _, w := range widgets {
		w.Draw(c)
	}
}

func markAllClean(widgets []Widget) {
	for _, w := range widgets {
		w.MarkClean()
	}
}
