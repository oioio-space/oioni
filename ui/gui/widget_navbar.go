// ui/gui/widget_navbar.go — NavBar: breadcrumb title bar
package gui

import (
	"image"
	"strings"

	"github.com/oioio-space/oioni/ui/canvas"
)

// NavBar displays a breadcrumb path (e.g. "Home › Config") above a separator line.
// Fixed height: 16px. 8pt font, 2px top padding.
type NavBar struct {
	BaseWidget
	path []string
}

// NewNavBar creates a NavBar with the given breadcrumb segments.
func NewNavBar(path ...string) *NavBar {
	nb := &NavBar{path: append([]string(nil), path...)}
	nb.SetDirty()
	return nb
}

// SetPath updates the breadcrumb path and marks the widget dirty.
func (nb *NavBar) SetPath(path ...string) {
	nb.path = append([]string(nil), path...)
	nb.SetDirty()
}

func (nb *NavBar) PreferredSize() image.Point { return image.Pt(206, 16) }
func (nb *NavBar) MinSize() image.Point       { return image.Pt(60, 16) }

// Draw renders the breadcrumb text and a separator line at the bottom.
func (nb *NavBar) Draw(c *canvas.Canvas) {
	b := nb.Bounds()
	if b.Empty() {
		return
	}
	f := canvas.EmbeddedFont(8)
	text := strings.Join(nb.path, " › ")
	maxW := b.Dx() - 4
	if f != nil && textWidth(text, f) > maxW {
		if len(nb.path) > 0 {
			text = "… › " + nb.path[len(nb.path)-1]
			// Further truncate if still too wide
			for f != nil && len(text) > 5 && textWidth(text, f) > maxW {
				text = text[1:]
			}
		}
	}
	if f != nil {
		c.DrawText(b.Min.X+2, b.Min.Y+2, text, f, canvas.Black)
	}
	// Separator line at bottom
	c.DrawLine(b.Min.X, b.Max.Y-1, b.Max.X-1, b.Max.Y-1, canvas.Black)
}
