// ui/gui/widget_scrolllist.go — generic composable scrollable list widget
package gui

import (
	"image"

	"github.com/oioio-space/oioni/ui/canvas"
)

// ListItem is implemented by each row in a ScrollableList.
// The caller defines rendering; ScrollableList handles scroll state and touch routing.
type ListItem interface {
	Draw(c *canvas.Canvas, bounds image.Rectangle)
	OnTap()
}

// ScrollableList is a responsive composable scrollable list widget.
// The number of visible rows is computed dynamically: visible = Bounds().Dy() / RowH.
// Pair with NavButton for ∧/∨ scroll controls.
//
// Usage:
//
//	list    := gui.NewScrollableList(items, 25)
//	upBtn   := gui.NewNavButton("^", list.ScrollUp, list.CanScrollUp)
//	downBtn := gui.NewNavButton("v", list.ScrollDown, list.CanScrollDown)
type ScrollableList struct {
	BaseWidget
	items  []ListItem
	offset int
	RowH   int // row height in pixels; set by caller
}

// NewScrollableList creates a ScrollableList with the given items and row height.
func NewScrollableList(items []ListItem, rowH int) *ScrollableList {
	l := &ScrollableList{items: items, RowH: rowH}
	l.SetDirty()
	return l
}

// visible returns the number of rows that fit in current bounds.
func (l *ScrollableList) visible() int {
	if l.RowH <= 0 {
		return 0
	}
	return l.Bounds().Dy() / l.RowH
}

// CanScrollUp returns true when the list is not at the top.
func (l *ScrollableList) CanScrollUp() bool { return l.offset > 0 }

// CanScrollDown returns true when items exist beyond the visible window.
// Uses addition to avoid underflow when len(items) < visible().
func (l *ScrollableList) CanScrollDown() bool {
	return l.offset+l.visible() < len(l.items)
}

// ScrollUp decrements the offset by one (no-op at top).
func (l *ScrollableList) ScrollUp() {
	if l.CanScrollUp() {
		l.offset--
		l.SetDirty()
	}
}

// ScrollDown increments the offset by one (no-op at bottom).
func (l *ScrollableList) ScrollDown() {
	if l.CanScrollDown() {
		l.offset++
		l.SetDirty()
	}
}

// SetItems replaces the list contents and resets scroll to top.
func (l *ScrollableList) SetItems(items []ListItem) {
	l.items = items
	l.offset = 0
	l.SetDirty()
}

// HandleTouch routes the touch to the correct item by row index.
func (l *ScrollableList) HandleTouch(pt TouchPoint) bool {
	wb := l.Bounds()
	if l.RowH <= 0 {
		return false // Bug 3 fix: return false, not true
	}
	// Explicit bounds check — Go's integer division truncates toward zero,
	// so (negative) / RowH = 0, which would pass the row >= 0 guard incorrectly.
	if int(pt.Y) < wb.Min.Y || int(pt.Y) >= wb.Max.Y {
		return true
	}
	row := (int(pt.Y) - wb.Min.Y) / l.RowH
	vis := l.visible()
	if row >= 0 && row < vis {
		actual := l.offset + row
		if actual < len(l.items) {
			l.items[actual].OnTap()
		}
	}
	return true
}

// Draw renders the visible rows. Each item draws itself inside a rounded-rect
// card inset from the row cell, matching the ActionSidebar button style.
func (l *ScrollableList) Draw(c *canvas.Canvas) {
	wb := l.Bounds()
	if wb.Empty() || l.RowH <= 0 {
		return
	}
	c.DrawRect(wb, canvas.White, true)
	vis := l.visible()
	const pad = 2
	for i := 0; i < vis; i++ {
		idx := l.offset + i
		if idx >= len(l.items) {
			break
		}
		cellY := wb.Min.Y + i*l.RowH
		topPad := pad
		if i > 0 {
			topPad = pad + pad
		}
		cardRect := image.Rect(wb.Min.X+pad, cellY+topPad, wb.Max.X-pad, cellY+l.RowH-pad)
		if !cardRect.Empty() {
			DrawRoundedRect(c, cardRect, 4, true, canvas.White)
			DrawRoundedRect(c, cardRect, 4, false, canvas.Black)
		}
		l.items[idx].Draw(c, cardRect)
	}
}
