// ui/gui/widget_networkstatus.go — NetworkStatusBar: 22px header with iface status + tool tray
package gui

import (
	"fmt"
	"image"
	"sync"

	"github.com/oioio-space/oioni/drivers/touch"
	"github.com/oioio-space/oioni/ui/canvas"
)

// IfaceInfo describes one network interface.
type IfaceInfo struct {
	Name string // "eth0", "wlan0", "usb0"
	IP   string // "192.168.0.33", or "" if not assigned
	Up   bool   // link state — controls filled/empty circle in popup
}

// ToolStatus describes one running tool for the header tray.
type ToolStatus struct {
	Label    string  // max 5 chars: "MITM", "SCAN", "HID"
	Progress float64 // 0.0–1.0
}

// NetworkStatusBar renders the 22px operator-style header bar.
// Left: primary interface + IP + optional [+N] badge.
// Right: up to 2 tool-progress chips (label + progress bar outline+fill).
// SetInterfaces and SetTools are goroutine-safe.
type NetworkStatusBar struct {
	BaseWidget
	mu         sync.Mutex
	nav        *Navigator // may be nil in tests
	interfaces []IfaceInfo
	tools      []ToolStatus
	// badgeBounds is updated during Draw; used for touch routing.
	badgeBounds image.Rectangle
}

// NewNetworkStatusBar creates a NetworkStatusBar. nav may be nil (badge tap is no-op).
func NewNetworkStatusBar(nav *Navigator) *NetworkStatusBar {
	nsb := &NetworkStatusBar{nav: nav}
	nsb.SetDirty()
	return nsb
}

// SetInterfaces updates the displayed interfaces. Goroutine-safe.
func (nsb *NetworkStatusBar) SetInterfaces(ifaces []IfaceInfo) {
	nsb.mu.Lock()
	nsb.interfaces = ifaces
	nsb.mu.Unlock()
	nsb.SetDirty()
}

// SetTools updates the tool tray. Goroutine-safe.
func (nsb *NetworkStatusBar) SetTools(tools []ToolStatus) {
	nsb.mu.Lock()
	nsb.tools = tools
	nsb.mu.Unlock()
	nsb.SetDirty()
}

func (nsb *NetworkStatusBar) PreferredSize() image.Point { return image.Pt(0, 22) }
func (nsb *NetworkStatusBar) MinSize() image.Point       { return image.Pt(0, 22) }

// HandleTouch implements Touchable. Badge tap pushes the interface detail popup.
func (nsb *NetworkStatusBar) HandleTouch(pt touch.TouchPoint) bool {
	p := image.Pt(int(pt.X), int(pt.Y))
	nsb.mu.Lock()
	bb := nsb.badgeBounds
	ifaces := nsb.interfaces
	nsb.mu.Unlock()
	if nsb.nav != nil && !bb.Empty() && p.In(bb) {
		nav := nsb.nav
		nav.Dispatch(func() { //nolint:errcheck
			nav.Push(newInterfaceDetailScene(nav, ifaces)) //nolint:errcheck
		})
		return true
	}
	return false
}

// Draw renders the header. Called from Navigator's render loop (single goroutine).
func (nsb *NetworkStatusBar) Draw(c *canvas.Canvas) {
	r := nsb.Bounds()
	if r.Empty() {
		return
	}

	nsb.mu.Lock()
	ifaces := nsb.interfaces
	tools := nsb.tools
	nsb.mu.Unlock()

	// Black background
	c.DrawRect(r, canvas.Black, true)

	f8 := canvas.EmbeddedFont(8)
	f12 := canvas.EmbeddedFont(12)

	// ── Left zone: interface status ────────────────────────────────────────
	upIfaces := make([]IfaceInfo, 0, len(ifaces))
	for _, iface := range ifaces {
		if iface.Up {
			upIfaces = append(upIfaces, iface)
		}
	}

	var newBadge image.Rectangle // empty by default

	if len(upIfaces) == 0 {
		// OFFLINE state
		c.DrawText(r.Min.X+3, r.Min.Y+2, "OFFLINE", f8, canvas.White)
		c.DrawText(r.Min.X+3, r.Min.Y+12, "no link", f8, canvas.White)
	} else {
		primary := upIfaces[0]
		// Line 1: interface name in 12pt bold
		c.DrawText(r.Min.X+3, r.Min.Y+2, primary.Name, f12, canvas.White)
		// IP in 8pt after the name
		nameW := textWidth(primary.Name, f12)
		if primary.IP != "" {
			c.DrawText(r.Min.X+3+nameW+3, r.Min.Y+4, primary.IP, f8, canvas.White)
		}
		// Badge [+N] if more than 1 up interface
		if len(upIfaces) > 1 {
			extra := len(upIfaces) - 1
			badgeText := fmt.Sprintf("+%d", extra)
			bw := textWidth(badgeText, f8) + 4
			bx := r.Min.X + 3 + nameW + 3 + textWidth(primary.IP, f8) + 4
			badgeR := image.Rect(bx, r.Min.Y+2, bx+bw, r.Min.Y+11)
			c.DrawRect(badgeR, canvas.White, true)
			c.DrawText(bx+2, r.Min.Y+2, badgeText, f8, canvas.Black)
			newBadge = badgeR // capture for touch routing
		}
	}

	// Single write — replaces both the reset and the old conditional write
	nsb.mu.Lock()
	nsb.badgeBounds = newBadge
	nsb.mu.Unlock()

	// ── Right zone: tool tray ──────────────────────────────────────────────
	// Show at most 2 chips; if more, show "+N" badge to their left.
	const chipW = 28
	const chipGap = 5
	const barH = 6
	const barY = 13 // y offset within header for progress bar

	visible := tools
	overflow := 0
	if len(tools) > 2 {
		overflow = len(tools) - 2
		visible = tools[len(tools)-2:]
	}

	rx := r.Max.X - 2
	for i := len(visible) - 1; i >= 0; i-- {
		rx -= chipW
		tool := visible[i]
		// Label
		c.DrawText(rx, r.Min.Y+2, tool.Label, f8, canvas.White)
		// Progress bar outline
		barR := image.Rect(rx, r.Min.Y+barY, rx+chipW, r.Min.Y+barY+barH)
		c.DrawRect(barR, canvas.White, false)
		// Fill — clamp progress to [0, 1] to prevent overflow
		progress := tool.Progress
		if progress < 0 {
			progress = 0
		}
		if progress > 1 {
			progress = 1
		}
		fillW := int(float64(chipW-2) * progress)
		if fillW > 0 {
			fillR := image.Rect(rx+1, r.Min.Y+barY+1, rx+1+fillW, r.Min.Y+barY+barH-1)
			c.DrawRect(fillR, canvas.White, true)
		}
		if i > 0 {
			rx -= chipGap
		}
	}

	if overflow > 0 {
		rx -= chipGap
		overText := fmt.Sprintf("+%d", overflow)
		rx -= textWidth(overText, f8)
		c.DrawText(rx, r.Min.Y+2, overText, f8, canvas.White)
	}

}

// newInterfaceDetailScene wraps InterfaceDetailPopup in a Scene and pushes it.
// Defined here (same package) so NetworkStatusBar can reference it.
func newInterfaceDetailScene(nav *Navigator, ifaces []IfaceInfo) *Scene {
	popup := newInterfaceDetailPopup(nav, ifaces)
	return &Scene{
		Title:   "Interfaces",
		Widgets: []Widget{popup},
	}
}

