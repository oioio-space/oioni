// epaper/gui/refresh.go — smart partial/full refresh decision engine
package gui

import (
	"slices"

	"github.com/oioio-space/oioni/ui/canvas"
	"github.com/oioio-space/oioni/drivers/epd"
)

const defaultAntiGhostN = 50 // full refresh every N partial updates

// refreshManager tracks dirty state and decides refresh strategy.
type refreshManager struct {
	display    Display
	antiGhostN int // full refresh every N partial updates
	counter    int // partial updates since last full refresh
	hasBase    bool
}

func newRefreshManager(d Display) *refreshManager {
	return &refreshManager{display: d, antiGhostN: defaultAntiGhostN}
}

// Render draws dirty widgets and refreshes with the appropriate strategy.
// Noop if no widget is dirty.
func (rm *refreshManager) Render(c *canvas.Canvas, widgets []Widget) error {
	if !slices.ContainsFunc(widgets, func(w Widget) bool { return w.IsDirty() }) {
		return nil
	}
	// Anti-ghosting: full refresh every antiGhostN partial updates.
	// counter tracks partials since last full; trigger when it would reach antiGhostN.
	if rm.counter >= rm.antiGhostN && rm.hasBase {
		return rm.fullRefresh(c, widgets)
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
	if err := rm.display.Init(epd.ModeFull); err != nil {
		return err
	}
	drawAll(c, widgets)
	if err := rm.display.DisplayBase(c.Bytes()); err != nil {
		return err
	}
	markAllClean(widgets)
	rm.counter = 0
	rm.hasBase = true
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
	if err := rm.display.DisplayPartial(c.Bytes()); err != nil {
		return err
	}
	markAllClean(widgets)
	rm.counter++
	return nil
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
