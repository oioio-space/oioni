// cmd/oioni/ui/menu.go — homeListItem: gui.ListItem implementation for the home menu
package ui

import (
	"image"

	"github.com/oioio-space/oioni/ui/canvas"
	"github.com/oioio-space/oioni/ui/gui"
)

const (
	homeRowH     = 33 // 3 rows × 33px = 99px (122px total - 22px NSB, scroll at 4th item)
	homeNavW     = 50 // nav column width (∧/∨ buttons)
	homeIconSize = 16 // icon size in px (scaled from 32×32 source)
	homeIconX    = 4  // left margin for icon
	homeIconYOff = 8  // (homeRowH - homeIconSize) / 2 — vertical center
	homeTextX    = 24 // homeIconX + homeIconSize + 4 — text start
)

// homeListItem implements gui.ListItem for the home menu.
// Renders a 16×16 icon on the left and the item name vertically centered.
type homeListItem struct {
	name  string
	icon  gui.Icon
	onTap func()
}

// Draw renders the item within its row bounds.
func (h *homeListItem) Draw(c *canvas.Canvas, r image.Rectangle) {
	// Icon (16×16, vertically centered in row)
	h.icon.Draw(c, image.Rect(
		r.Min.X+homeIconX,
		r.Min.Y+homeIconYOff,
		r.Min.X+homeIconX+homeIconSize,
		r.Min.Y+homeIconYOff+homeIconSize,
	))

	// Name — vertically centered
	f := canvas.EmbeddedFont(12)
	if f != nil {
		ty := r.Min.Y + (r.Dy()-f.LineHeight())/2
		c.DrawText(r.Min.X+homeTextX, ty, h.name, f, canvas.Black)
	}
}

// OnTap fires the item's action (no-op if nil).
func (h *homeListItem) OnTap() {
	if h.onTap != nil {
		h.onTap()
	}
}
