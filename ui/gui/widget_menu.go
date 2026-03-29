package gui

import (
	"image"
	"image/color"

	"github.com/oioio-space/oioni/ui/canvas"
)

const menuRowHeight = 20
const menuIconSize = 20

// MenuItem is one entry in a Menu. Icon is optional.
type MenuItem struct {
	Label    string
	Icon     *Icon
	OnSelect func()
}

// Menu is a scrollable list of items. Implements scrollable (for swipe gestures).
type Menu struct {
	BaseWidget
	Items    []MenuItem
	offset   int // index of first visible item
	selected int // index of last tapped item; -1 = none
}

func NewMenu(items []MenuItem) *Menu {
	m := &Menu{Items: items, selected: -1}
	m.SetDirty()
	return m
}

func (m *Menu) PreferredSize() image.Point {
	return image.Pt(80, len(m.Items)*menuRowHeight)
}
func (m *Menu) MinSize() image.Point { return image.Pt(40, menuRowHeight) }

func (m *Menu) visibleRows() int {
	r := m.Bounds()
	if r.Empty() {
		return 0
	}
	return r.Dy() / menuRowHeight
}

func (m *Menu) Scroll(dy int) {
	maxOff := len(m.Items) - m.visibleRows()
	if maxOff < 0 {
		maxOff = 0
	}
	m.offset += dy
	if m.offset < 0 {
		m.offset = 0
	}
	if m.offset > maxOff {
		m.offset = maxOff
	}
	m.SetDirty()
}

func (m *Menu) Draw(c *canvas.Canvas) {
	r := m.Bounds()
	if r.Empty() {
		return
	}
	c.DrawRect(r, canvas.White, true)
	f := canvas.EmbeddedFont(12)
	rows := m.visibleRows()

	for i := 0; i < rows; i++ {
		idx := m.offset + i
		if idx >= len(m.Items) {
			break
		}
		item := m.Items[idx]
		rowRect := image.Rect(r.Min.X, r.Min.Y+i*menuRowHeight,
			r.Max.X, r.Min.Y+(i+1)*menuRowHeight)

		if idx == m.selected {
			// Inverted background for selected row
			c.DrawRect(rowRect, canvas.Black, true)
			var fg color.Color = canvas.White
			m.drawItemContent(c, item, rowRect, f, fg)
		} else {
			c.DrawRect(rowRect, canvas.Black, false) // row border
			m.drawItemContent(c, item, rowRect, f, canvas.Black)
		}
	}

	// Scroll indicator (2px right bar) if content overflows
	if len(m.Items) > rows && rows > 0 {
		barH := r.Dy() * rows / len(m.Items)
		barY := r.Min.Y + r.Dy()*m.offset/len(m.Items)
		bar := image.Rect(r.Max.X-2, barY, r.Max.X, barY+barH)
		c.DrawRect(bar, canvas.Black, true)
	}
}

func (m *Menu) drawItemContent(c *canvas.Canvas, item MenuItem, row image.Rectangle, f canvas.Font, col color.Color) {
	x := row.Min.X + 4
	if item.Icon != nil {
		iconR := image.Rect(x, row.Min.Y+2, x+menuIconSize-2, row.Max.Y-2)
		item.Icon.Draw(c, iconR)
		x += menuIconSize + 2
	}
	if f != nil {
		ty := row.Min.Y + (row.Dy()-f.LineHeight())/2
		c.DrawText(x, ty, item.Label, f, col)
	}
}

func (m *Menu) HandleTouch(pt TouchPoint) bool {
	r := m.Bounds()
	if r.Empty() {
		return false
	}
	px, py := int(pt.X), int(pt.Y)
	if px < r.Min.X || px >= r.Max.X || py < r.Min.Y || py >= r.Max.Y {
		return false
	}
	row := (py - r.Min.Y) / menuRowHeight
	idx := m.offset + row
	if idx < 0 || idx >= len(m.Items) {
		return false
	}
	m.selected = idx
	m.SetDirty()
	if m.Items[idx].OnSelect != nil {
		m.Items[idx].OnSelect()
	}
	return true
}
