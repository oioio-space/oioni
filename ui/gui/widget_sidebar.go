// ui/gui/widget_sidebar.go — ActionSidebar: adaptive right-side action buttons
package gui

import (
	"image"

	"github.com/oioio-space/oioni/drivers/touch"
	"github.com/oioio-space/oioni/ui/canvas"
)

// SidebarButton is one action button in the ActionSidebar.
// If Icon is zero-value, Label is drawn centered instead.
// Height sets a fixed pixel height; 0 means the button auto-sizes to share remaining space.
type SidebarButton struct {
	Icon   Icon
	Label  string
	Height int // px; 0 = auto (equal share of remaining space)
	OnTap  func()
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

// cellHeights returns the pixel height for each button.
// Fixed-height buttons (Height > 0) use their declared height; the remaining space is
// divided equally among auto-height buttons (Height == 0).
func (s *ActionSidebar) cellHeights(totalH int) []int {
	heights := make([]int, len(s.buttons))
	fixed, autoCount := 0, 0
	for _, btn := range s.buttons {
		if btn.Height > 0 {
			fixed += btn.Height
		} else {
			autoCount++
		}
	}
	autoH := 0
	if autoCount > 0 {
		autoH = (totalH - fixed) / autoCount
		if autoH < 0 {
			autoH = 0
		}
	}
	for i, btn := range s.buttons {
		if btn.Height > 0 {
			heights[i] = btn.Height
		} else {
			heights[i] = autoH
		}
	}
	return heights
}

// HandleTouch routes a tap to the button at the touch's Y position.
func (s *ActionSidebar) HandleTouch(pt touch.TouchPoint) bool {
	b := s.Bounds()
	if b.Empty() || len(s.buttons) == 0 {
		return false
	}
	heights := s.cellHeights(b.Dy())
	y := b.Min.Y
	for i, btn := range s.buttons {
		if int(pt.Y) >= y && int(pt.Y) < y+heights[i] {
			if btn.OnTap != nil {
				btn.OnTap()
			}
			return true
		}
		y += heights[i]
	}
	return false
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

	heights := s.cellHeights(b.Dy())
	cellY := b.Min.Y
	for i, btn := range s.buttons {
		h := heights[i]
		cellRect := image.Rect(b.Min.X+2, cellY, b.Max.X, cellY+h)

		// 2px separator between buttons (not before first).
		if i > 0 {
			c.DrawLine(b.Min.X+2, cellY, b.Max.X-1, cellY, canvas.Black)
			c.DrawLine(b.Min.X+2, cellY+1, b.Max.X-1, cellY+1, canvas.Black)
		}

		if _, ih := btn.Icon.Size(); ih > 0 {
			// Icon centered in cell
			iconSize := 24
			iconX := cellRect.Min.X + (cellRect.Dx()-iconSize)/2
			iconY := cellRect.Min.Y + (cellRect.Dy()-iconSize)/2
			iconRect := image.Rect(iconX, iconY, iconX+iconSize, iconY+iconSize)
			if !iconRect.Empty() {
				btn.Icon.Draw(c, iconRect)
			}
		} else if btn.Label != "" {
			// Text label centered in cell
			f := canvas.EmbeddedFont(12)
			if f != nil {
				tw := textWidth(btn.Label, f)
				lx := cellRect.Min.X + (cellRect.Dx()-tw)/2
				ly := cellRect.Min.Y + (cellRect.Dy()-f.LineHeight())/2
				c.DrawText(lx, ly, btn.Label, f, canvas.Black)
			}
		}
		cellY += h
	}
}
