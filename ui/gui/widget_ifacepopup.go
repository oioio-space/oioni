// ui/gui/widget_ifacepopup.go — InterfaceDetailPopup: full-screen dithered overlay
package gui

import (
	"image"

	"github.com/oioio-space/oioni/ui/canvas"
)

// InterfaceDetailPopup renders a full-screen dithered overlay with a centred
// popup box listing all network interfaces. Swipe-down dismisses it.
// Implements scrollable (package-internal interface).
type InterfaceDetailPopup struct {
	BaseWidget
	nav        *Navigator // may be nil in tests
	interfaces []IfaceInfo
}

func newInterfaceDetailPopup(nav *Navigator, ifaces []IfaceInfo) *InterfaceDetailPopup {
	p := &InterfaceDetailPopup{nav: nav, interfaces: ifaces}
	p.SetDirty()
	return p
}

// Scroll implements scrollable. Swipe-down (dy > 0) pops this scene.
func (p *InterfaceDetailPopup) Scroll(dy int) {
	if dy > 0 && p.nav != nil {
		nav := p.nav
		nav.Dispatch(func() { //nolint:errcheck
			nav.Pop() //nolint:errcheck
		})
	}
}

// Draw renders the popup overlay.
func (p *InterfaceDetailPopup) Draw(c *canvas.Canvas) {
	r := p.Bounds()
	if r.Empty() {
		return
	}

	// 1. Checkerboard dither fill (visual "dimming")
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			if (x+y)%2 == 0 {
				c.SetPixel(x, y, canvas.Black)
			} else {
				c.SetPixel(x, y, canvas.White)
			}
		}
	}

	// 2. Popup box: white fill + 2px black border
	// Centered: x=45..205, y=4..78 (160×74px)
	const boxX0, boxY0, boxX1, boxY1 = 45, 4, 205, 78
	boxR := image.Rect(boxX0, boxY0, boxX1, boxY1)
	c.DrawRect(boxR, canvas.White, true)
	// 2px border
	c.DrawRect(boxR, canvas.Black, false)
	borderInner := image.Rect(boxX0+1, boxY0+1, boxX1-1, boxY1-1)
	c.DrawRect(borderInner, canvas.Black, false)

	// 3. Title bar (16px): black fill, "Interfaces" centered in 12pt white
	const titleH = 16
	titleR := image.Rect(boxX0, boxY0, boxX1, boxY0+titleH)
	c.DrawRect(titleR, canvas.Black, true)
	f12 := canvas.EmbeddedFont(12)
	f8 := canvas.EmbeddedFont(8)
	title := "Interfaces"
	tw := textWidth(title, f12)
	c.DrawText(boxX0+(boxX1-boxX0-tw)/2, boxY0+2, title, f12, canvas.White)

	// 4. Interface rows (18px each)
	const rowH = 18
	const hintH = 11 // 8pt line height (~8px) + 3px margin
	y := boxY0 + titleH
	for _, iface := range p.interfaces {
		if y+rowH > boxY1-hintH { // leave room for hint
			break
		}
		cx := boxX0 + 6
		cy := y + rowH/2

		// Circle indicator: filled=Up, empty=down
		if iface.Up {
			// Filled circle r=2
			for dy := -2; dy <= 2; dy++ {
				for dx := -2; dx <= 2; dx++ {
					if dx*dx+dy*dy <= 4 {
						c.SetPixel(cx+dx, cy+dy, canvas.Black)
					}
				}
			}
		} else {
			// Empty circle r=2 outline: d in [2,4] gives pixels at
			// (±1,±1), (±2,0), (0,±2) — 8 pixels forming a circle.
			for dx := -2; dx <= 2; dx++ {
				for dy := -2; dy <= 2; dy++ {
					d := dx*dx + dy*dy
					if d >= 2 && d <= 4 {
						c.SetPixel(cx+dx, cy+dy, canvas.Black)
					}
				}
			}
		}

		// Name: 12pt if Up, 8pt if down
		tx := cx + 6
		if iface.Up {
			c.DrawText(tx, y+2, iface.Name, f12, canvas.Black)
			tx += textWidth(iface.Name, f12) + 4
		} else {
			c.DrawText(tx, y+5, iface.Name, f8, canvas.Black)
			tx += textWidth(iface.Name, f8) + 4
		}

		// IP (8pt)
		if iface.IP != "" {
			c.DrawText(tx, y+5, iface.IP, f8, canvas.Black)
		}

		// 1px separator
		c.DrawLine(boxX0+2, y+rowH-1, boxX1-2, y+rowH-1, canvas.Black)

		y += rowH
	}

	// 5. Hint row: "swipe down to close" centered in 8pt
	hintText := "swipe down to close"
	hw := textWidth(hintText, f8)
	hintY := boxY1 - hintH + 1
	c.DrawText(boxX0+(boxX1-boxX0-hw)/2, hintY, hintText, f8, canvas.Black)
}
