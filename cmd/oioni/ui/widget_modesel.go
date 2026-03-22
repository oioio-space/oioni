// cmd/oioni/ui/widget_modesel.go — two-button DHCP/Static mode selector
package ui

import (
	"image"
	"image/color"

	"github.com/oioio-space/oioni/drivers/touch"
	"github.com/oioio-space/oioni/ui/canvas"
	"github.com/oioio-space/oioni/ui/gui"
)

type ipMode int

const (
	modeDHCP   ipMode = iota
	modeStatic ipMode = iota
)

// modeSelector is a 44px-tall two-button widget for selecting DHCP or Static.
type modeSelector struct {
	gui.BaseWidget
	mode     ipMode
	onChange func(ipMode)
}

func newModeSelector(initial ipMode) *modeSelector {
	return &modeSelector{mode: initial}
}

func (m *modeSelector) Mode() ipMode { return m.mode }

func (m *modeSelector) SetOnChange(fn func(ipMode)) { m.onChange = fn }

func (m *modeSelector) HandleTouch(pt touch.TouchPoint) bool {
	b := m.Bounds()
	mid := b.Min.X + b.Dx()/2
	if int(pt.X) < mid {
		m.mode = modeDHCP
	} else {
		m.mode = modeStatic
	}
	m.SetDirty()
	if m.onChange != nil {
		m.onChange(m.mode)
	}
	return true
}

func (m *modeSelector) Draw(cv *canvas.Canvas) {
	b := m.Bounds()
	mid := b.Min.X + b.Dx()/2

	left := image.Rect(b.Min.X, b.Min.Y, mid, b.Max.Y)
	right := image.Rect(mid, b.Min.Y, b.Max.X, b.Max.Y)

	if m.mode == modeDHCP {
		cv.DrawRect(left, canvas.Black, true)
		cv.DrawRect(right, canvas.White, true)
		cv.DrawRect(right, canvas.Black, false)
	} else {
		cv.DrawRect(left, canvas.White, true)
		cv.DrawRect(left, canvas.Black, false)
		cv.DrawRect(right, canvas.Black, true)
	}

	f := canvas.EmbeddedFont(12)
	if f == nil {
		return
	}
	drawLabel := func(rect image.Rectangle, text string, fg color.Color) {
		x := rect.Min.X + 6
		y := rect.Min.Y + (rect.Dy()-f.LineHeight())/2
		cv.DrawText(x, y, text, f, fg)
	}
	if m.mode == modeDHCP {
		drawLabel(left, "DHCP", canvas.White)
		drawLabel(right, "Static", canvas.Black)
	} else {
		drawLabel(left, "DHCP", canvas.Black)
		drawLabel(right, "Static", canvas.White)
	}
}
