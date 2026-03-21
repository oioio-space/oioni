// cmd/oioni/ui/menu.go — HomeMenuWidget: 5-row operator-style menu
package ui

import (
	"image"

	"github.com/oioio-space/oioni/drivers/touch"
	"github.com/oioio-space/oioni/ui/canvas"
	"github.com/oioio-space/oioni/ui/gui"
)

const (
	menuRowH   = 20
	menuRows   = 5
	menuIconX  = 11  // x center of icon circle
	menuIconR  = 7   // radius of icon circle
	menuTextX  = 24  // x start of name/desc text
	menuChevX  = 246 // chevron right edge x
	menuSepEndX = 250 // separator right edge (full display width)
)

type homeMenuItem struct {
	name  string
	desc  string
	onTap func()
}

// HomeMenuWidget renders 5 menu rows (5×20px = 100px total).
type HomeMenuWidget struct {
	gui.BaseWidget
	items    []homeMenuItem
	selected int // index of last tapped row, -1 = none
}

func newHomeMenuWidget(items []homeMenuItem) *HomeMenuWidget {
	m := &HomeMenuWidget{items: items, selected: -1}
	m.SetDirty()
	return m
}

func (m *HomeMenuWidget) PreferredSize() image.Point { return image.Pt(0, menuRows*menuRowH) }
func (m *HomeMenuWidget) MinSize() image.Point       { return image.Pt(0, menuRows*menuRowH) }

func (m *HomeMenuWidget) HandleTouch(pt touch.TouchPoint) bool {
	r := m.Bounds()
	if r.Empty() {
		return false
	}
	py := int(pt.Y)
	if py < r.Min.Y || py >= r.Max.Y {
		return false
	}
	row := (py - r.Min.Y) / menuRowH
	if row < 0 || row >= len(m.items) {
		return false
	}
	m.selected = row
	m.SetDirty()
	if m.items[row].onTap != nil {
		m.items[row].onTap()
	}
	return true
}

// menuTextWidth returns pixel width of text in the given canvas.Font.
func menuTextWidth(text string, f canvas.Font) int {
	if f == nil {
		return 0
	}
	w := 0
	for _, r := range text {
		_, gw, _ := f.Glyph(r)
		w += gw
	}
	return w
}

func (m *HomeMenuWidget) Draw(c *canvas.Canvas) {
	r := m.Bounds()
	if r.Empty() {
		return
	}
	c.DrawRect(r, canvas.White, true)

	f12 := canvas.EmbeddedFont(12)
	f8 := canvas.EmbeddedFont(8)

	for i, item := range m.items {
		rowTop := r.Min.Y + i*menuRowH
		rowBot := rowTop + menuRowH
		rowCenter := rowTop + menuRowH/2
		rowR := image.Rect(r.Min.X, rowTop, r.Max.X, rowBot)

		active := i == m.selected
		fg := canvas.Black
		if active {
			fg = canvas.White
			c.DrawRect(rowR, canvas.Black, true)
		}

		// Icon: filled circle at (menuIconX, rowCenter)
		ix := r.Min.X + menuIconX
		for dy := -menuIconR; dy <= menuIconR; dy++ {
			for dx := -menuIconR; dx <= menuIconR; dx++ {
				if dx*dx+dy*dy <= menuIconR*menuIconR {
					c.SetPixel(ix+dx, rowCenter+dy, fg)
				}
			}
		}

		// Name: 12pt
		if f12 != nil {
			c.DrawText(r.Min.X+menuTextX, rowTop+2, item.name, f12, fg)
		}

		// Description: 8pt at y=rowBot-9
		if f8 != nil {
			c.DrawText(r.Min.X+menuTextX, rowBot-9, item.desc, f8, fg)
		}

		// Chevron: ">" right-aligned so text ends at menuChevX
		if f12 != nil {
			cw := menuTextWidth(">", f12)
			c.DrawText(r.Min.X+menuChevX-cw, rowTop+2, ">", f12, fg)
		}

		// 1px separator (not on active row)
		if !active {
			c.DrawLine(r.Min.X+16, rowBot-1, r.Min.X+menuSepEndX, rowBot-1, canvas.Black)
		}
	}
}
