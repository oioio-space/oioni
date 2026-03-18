// ui/gui/widget_sidebar.go — ActionSidebar: adaptive right-side action buttons
package gui

import (
	"image"

	"github.com/oioio-space/oioni/drivers/touch"
	"github.com/oioio-space/oioni/ui/canvas"
)

// SidebarButton is one action button in the ActionSidebar.
type SidebarButton struct {
	Icon  Icon
	OnTap func()
}

// ActionSidebar displays a vertical column of icon buttons on the right side.
// Buttons are equally distributed vertically. A separator line is drawn on the left edge.
type ActionSidebar struct {
	BaseWidget
	buttons []SidebarButton
}

// NewActionSidebar creates an ActionSidebar with the given buttons.
func NewActionSidebar(buttons ...SidebarButton) *ActionSidebar {
	s := &ActionSidebar{buttons: append([]SidebarButton(nil), buttons...)}
	s.SetDirty()
	return s
}

// SetButtons replaces the button set and marks the widget dirty.
func (s *ActionSidebar) SetButtons(buttons ...SidebarButton) {
	s.buttons = append([]SidebarButton(nil), buttons...)
	s.SetDirty()
}

func (s *ActionSidebar) PreferredSize() image.Point { return image.Pt(44, 122) }
func (s *ActionSidebar) MinSize() image.Point       { return image.Pt(44, 40) }

// HandleTouch routes a tap to the button at the touch's Y position.
func (s *ActionSidebar) HandleTouch(pt touch.TouchPoint) bool {
	b := s.Bounds()
	if b.Empty() || len(s.buttons) == 0 {
		return false
	}
	y := int(pt.Y) - b.Min.Y
	cellH := b.Dy() / len(s.buttons)
	if cellH <= 0 {
		return false
	}
	idx := y / cellH
	if idx < 0 {
		idx = 0
	}
	if idx >= len(s.buttons) {
		idx = len(s.buttons) - 1
	}
	if s.buttons[idx].OnTap != nil {
		s.buttons[idx].OnTap()
	}
	return true
}

// Draw renders the sidebar: white background, left separator line, and equally distributed icon buttons.
func (s *ActionSidebar) Draw(c *canvas.Canvas) {
	b := s.Bounds()
	if b.Empty() {
		return
	}

	// Clear background
	c.DrawRect(b, canvas.White, true)

	// 2px left separator — 1px lines can vanish during partial refresh.
	c.DrawLine(b.Min.X, b.Min.Y, b.Min.X, b.Max.Y-1, canvas.Black)
	c.DrawLine(b.Min.X+1, b.Min.Y, b.Min.X+1, b.Max.Y-1, canvas.Black)

	if len(s.buttons) == 0 {
		return
	}

	cellH := b.Dy() / len(s.buttons)
	for i, btn := range s.buttons {
		cellY := b.Min.Y + i*cellH
		cellRect := image.Rect(b.Min.X+1, cellY, b.Max.X, cellY+cellH)

		// 2px separator between buttons (not before first).
		if i > 0 {
			c.DrawLine(b.Min.X+2, cellY, b.Max.X-1, cellY, canvas.Black)
			c.DrawLine(b.Min.X+2, cellY+1, b.Max.X-1, cellY+1, canvas.Black)
		}

		// Icon centered in cell
		iconSize := 24
		iconX := cellRect.Min.X + (cellRect.Dx()-iconSize)/2
		iconY := cellRect.Min.Y + (cellRect.Dy()-iconSize)/2
		iconRect := image.Rect(iconX, iconY, iconX+iconSize, iconY+iconSize)
		if !iconRect.Empty() {
			btn.Icon.Draw(c, iconRect)
		}
	}
}
