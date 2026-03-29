// ui/gui/widget_navbutton.go — NavButton: tap button with active/disabled state
package gui

import (
	"image"

	"github.com/oioio-space/oioni/ui/canvas"
)

// NavButton is a tap button with an active/disabled visual state.
// Typically used for scroll controls alongside a ScrollableList.
//
// The button always fires onTap on touch — the disabled state is visual only.
// The caller's onTap function (e.g. ScrollableList.ScrollUp) handles no-op logic.
// Rendering is responsive: icon/symbol and disabled bar are centered in actual Bounds().
type NavButton struct {
	BaseWidget
	sym      string
	icon     Icon
	onTap    func()
	isActive func() bool
}

// NewNavButton creates a NavButton with a text symbol.
//   - sym: ASCII symbol displayed when active (e.g. "^" or "v")
//   - onTap: called on every touch, regardless of active state; nil = no-op
//   - isActive: returns true when button appears enabled; nil → always disabled
func NewNavButton(sym string, onTap func(), isActive func() bool) *NavButton {
	if isActive == nil {
		isActive = func() bool { return false }
	}
	b := &NavButton{sym: sym, onTap: onTap, isActive: isActive}
	b.SetDirty()
	return b
}

// NewIconNavButton creates a NavButton with an icon instead of a text symbol.
//   - icon: icon rendered when active
//   - onTap: called on every touch, regardless of active state; nil = no-op
//   - isActive: returns true when button appears enabled; nil → always disabled
func NewIconNavButton(icon Icon, onTap func(), isActive func() bool) *NavButton {
	if isActive == nil {
		isActive = func() bool { return false }
	}
	b := &NavButton{icon: icon, onTap: onTap, isActive: isActive}
	b.SetDirty()
	return b
}

// HandleTouch fires onTap. Touch routing is handled by the Navigator.
func (b *NavButton) HandleTouch(_ TouchPoint) bool {
	if b.onTap != nil {
		b.onTap()
	}
	return true
}

// Draw renders the button: border + symbol (active) or 8px bar (disabled).
// All positioning is relative to Bounds(), so the widget works in any container.
func (b *NavButton) Draw(c *canvas.Canvas) {
	r := b.Bounds()
	if r.Empty() {
		return
	}
	DrawRoundedRect(c, r, 4, true, canvas.White)
	DrawRoundedRect(c, r, 4, false, canvas.Black)

	cx := r.Min.X + r.Dx()/2
	cy := r.Min.Y + r.Dy()/2

	if !b.isActive() {
		// Disabled: 8px-wide horizontal bar (e-ink disabled convention)
		c.DrawLine(cx-4, cy, cx+4, cy, canvas.Black)
		return
	}
	// Icon mode: draw icon centered in bounds
	if _, ih := b.icon.Size(); ih > 0 {
		iconSize := 16
		ix := r.Min.X + (r.Dx()-iconSize)/2
		iy := r.Min.Y + (r.Dy()-iconSize)/2
		b.icon.Draw(c, image.Rect(ix, iy, ix+iconSize, iy+iconSize))
		return
	}
	// Text mode: draw ASCII symbol centered
	f := canvas.EmbeddedFont(12)
	if f == nil {
		return
	}
	tw := textWidth(b.sym, f)
	tx := r.Min.X + (r.Dx()-tw)/2
	ty := r.Min.Y + (r.Dy()-f.LineHeight())/2
	c.DrawText(tx, ty, b.sym, f, canvas.Black)
}
