// cmd/oioni/ui/menu.go — ScrollableMenuList + NavButton for the home screen
package ui

import (
	"image"

	"github.com/oioio-space/oioni/drivers/touch"
	"github.com/oioio-space/oioni/ui/canvas"
	"github.com/oioio-space/oioni/ui/gui"
)

const (
	menuRowH     = 50
	menuVisible  = 2
	menuNavW     = 50
	menuIconX    = 8
	menuIconSize = 32
	menuIconYOff = 9  // = (menuRowH - menuIconSize) / 2
	menuTextX    = 48 // = menuIconX + menuIconSize + 8
)

type homeMenuItem struct {
	name  string
	desc  string
	icon  gui.Icon
	onTap func()
}

// ── ScrollableMenuList ────────────────────────────────────────────────────────

// ScrollableMenuList renders 2 rows at a time from a list of items.
// Scroll state is managed via ScrollUp/ScrollDown called by NavButton closures.
type ScrollableMenuList struct {
	gui.BaseWidget
	items  []homeMenuItem
	offset int
}

func newScrollableMenuList(items []homeMenuItem) *ScrollableMenuList {
	l := &ScrollableMenuList{items: items}
	l.SetDirty()
	return l
}

func (l *ScrollableMenuList) PreferredSize() image.Point { return image.Pt(0, menuVisible*menuRowH) }
func (l *ScrollableMenuList) MinSize() image.Point       { return image.Pt(0, menuVisible*menuRowH) }

func (l *ScrollableMenuList) CanScrollUp() bool { return l.offset > 0 }
func (l *ScrollableMenuList) CanScrollDown() bool {
	return l.offset < len(l.items)-menuVisible
}

func (l *ScrollableMenuList) ScrollUp() {
	if l.CanScrollUp() {
		l.offset--
		l.SetDirty()
	}
}

func (l *ScrollableMenuList) ScrollDown() {
	if l.CanScrollDown() {
		l.offset++
		l.SetDirty()
	}
}

func (l *ScrollableMenuList) HandleTouch(pt touch.TouchPoint) bool {
	wb := l.Bounds()
	row := (int(pt.Y) - wb.Min.Y) / menuRowH
	if row >= 0 && row < menuVisible {
		actual := l.offset + row
		if actual < len(l.items) && l.items[actual].onTap != nil {
			l.items[actual].onTap()
		}
	}
	return true
}

func (l *ScrollableMenuList) Draw(c *canvas.Canvas) {
	wb := l.Bounds()
	if wb.Empty() {
		return
	}
	c.DrawRect(wb, canvas.White, true)

	f12 := canvas.EmbeddedFont(12)
	f8 := canvas.EmbeddedFont(8)

	for i := 0; i < menuVisible; i++ {
		idx := l.offset + i
		if idx >= len(l.items) {
			break
		}
		item := l.items[idx]
		rowTop := wb.Min.Y + i*menuRowH

		// Icon (32×32, centered vertically in row)
		item.icon.Draw(c, image.Rect(
			wb.Min.X+menuIconX,
			rowTop+menuIconYOff,
			wb.Min.X+menuIconX+menuIconSize,
			rowTop+menuIconYOff+menuIconSize,
		))

		// Name
		if f12 != nil {
			c.DrawText(wb.Min.X+menuTextX, rowTop+6, item.name, f12, canvas.Black)
		}

		// Description
		if f8 != nil {
			c.DrawText(wb.Min.X+menuTextX, rowTop+28, item.desc, f8, canvas.Black)
		}

		// Row separator (between rows only)
		if i < menuVisible-1 {
			sep := rowTop + menuRowH - 1
			c.DrawLine(wb.Min.X, sep, wb.Max.X, sep, canvas.Black)
		}
	}
}

// ── NavButton ─────────────────────────────────────────────────────────────────

// NavButton is a 50×50px tap button. isActive controls the rendered state.
// onTap is always called on touch — the caller (ScrollUp/ScrollDown) handles no-op logic.
type NavButton struct {
	gui.BaseWidget
	sym      string
	onTap    func()
	isActive func() bool
}

func newNavButton(sym string, onTap func(), isActive func() bool) *NavButton {
	b := &NavButton{sym: sym, onTap: onTap, isActive: isActive}
	b.SetDirty()
	return b
}

func (b *NavButton) PreferredSize() image.Point { return image.Pt(menuNavW, menuNavW) }
func (b *NavButton) MinSize() image.Point       { return image.Pt(menuNavW, menuNavW) }

func (b *NavButton) HandleTouch(pt touch.TouchPoint) bool {
	if b.onTap != nil {
		b.onTap()
	}
	return true
}

func (b *NavButton) Draw(c *canvas.Canvas) {
	r := b.Bounds()
	if r.Empty() {
		return
	}
	c.DrawRect(r, canvas.White, true)
	c.DrawRect(r, canvas.Black, false)

	f12 := canvas.EmbeddedFont(12)

	if !b.isActive() {
		// Disabled: horizontal bar
		cx := r.Min.X + r.Dx()/2
		cy := r.Min.Y + r.Dy()/2
		c.DrawLine(cx-4, cy, cx+4, cy, canvas.Black)
		return
	}
	if f12 == nil {
		return
	}
	// Center symbol
	tw := 0
	for _, ch := range b.sym {
		_, w, _ := f12.Glyph(ch)
		tw += w
	}
	tx := r.Min.X + (r.Dx()-tw)/2
	ty := r.Min.Y + (r.Dy()-f12.LineHeight())/2
	c.DrawText(tx, ty, b.sym, f12, canvas.Black)
}
