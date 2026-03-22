// cmd/oioni/ui/scene_wifi_password.go — PSK entry scene for WiFi
package ui

import (
	"image"

	"github.com/oioio-space/oioni/system/wifi"
	"github.com/oioio-space/oioni/ui/canvas"
	"github.com/oioio-space/oioni/ui/gui"
)

// newPasswordScene builds the password entry scene for a new (unsaved) network.
func newPasswordScene(nav *gui.Navigator, mgr *wifi.Manager, ssid string) *gui.Scene {
	var psk string
	save := gui.NewCheckbox("Memoriser", false)

	pskItem := &pskFieldItem{
		label: "Mot de passe",
		onTap: func() {
			gui.ShowTextInput(nav, "Mot de passe WiFi", 64, func(entered string) {
				psk = entered
			})
		},
	}

	list := gui.NewScrollableList([]gui.ListItem{
		&ssidLabelItem{ssid: ssid},
		pskItem,
		&checkboxItem{cb: save},
	}, 36)

	onConnect := func() {
		if mgr == nil {
			return
		}
		capturedPsk := psk
		capturedSave := save.Checked
		go func() {
			_ = mgr.Connect(ssid, capturedPsk, capturedSave)
			nav.Dispatch(func() {
				nav.Push(newConnectingScene(nav, mgr, ssid)) //nolint:errcheck
			})
		}()
	}

	return newCategoryScene(nav, "WiFi", list,
		withExtraSidebarBtn(Icons.Back, onConnect), // Icons.Back as placeholder for Connect
	)
}

type ssidLabelItem struct{ ssid string }

func (s *ssidLabelItem) Draw(cv *canvas.Canvas, b image.Rectangle) {
	if f := canvas.EmbeddedFont(12); f != nil {
		cv.DrawText(b.Min.X+6, b.Min.Y+(b.Dy()-f.LineHeight())/2, s.ssid, f, canvas.Black)
	}
}
func (s *ssidLabelItem) OnTap() {}

type pskFieldItem struct {
	label string
	onTap func()
}

func (p *pskFieldItem) Draw(cv *canvas.Canvas, b image.Rectangle) {
	if f := canvas.EmbeddedFont(12); f != nil {
		cv.DrawText(b.Min.X+6, b.Min.Y+(b.Dy()-f.LineHeight())/2, p.label+" >", f, canvas.Black)
	}
}
func (p *pskFieldItem) OnTap() { p.onTap() }

type checkboxItem struct{ cb *gui.Checkbox }

func (c *checkboxItem) Draw(cv *canvas.Canvas, b image.Rectangle) {
	c.cb.SetBounds(b)
	c.cb.Draw(cv)
}
func (c *checkboxItem) OnTap() { c.cb.Checked = !c.cb.Checked; c.cb.SetDirty() }
