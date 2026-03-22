// cmd/oioni/ui/scene_wifi.go — WiFi management scene
package ui

import (
	"image"

	"github.com/oioio-space/oioni/system/wifi"
	"github.com/oioio-space/oioni/ui/canvas"
	"github.com/oioio-space/oioni/ui/gui"
)

// NewWifiScene builds the WiFi scene: toggle + network list + scan button.
func NewWifiScene(nav *gui.Navigator, mgr *wifi.Manager) *gui.Scene {
	var items []gui.ListItem

	toggle := gui.NewToggle(true)
	items = append(items, &wifiToggleItem{
		toggle: toggle,
		onTap: func() {
			if mgr == nil {
				return
			}
			_ = mgr.SetEnabled(!toggle.On)
			toggle.On = !toggle.On
			toggle.SetDirty()
		},
	})

	list := gui.NewScrollableList(items, 36)

	onScan := func() {
		if mgr == nil {
			return
		}
		go func() {
			nets, err := mgr.Scan()
			nav.Dispatch(func() {
				if err != nil {
					return
				}
				newItems := []gui.ListItem{items[0]}
				for _, n := range nets {
					net := n
					newItems = append(newItems, &wifiNetItem{
						net: net,
						onTap: func() {
							if net.Saved {
								go func() {
									_ = mgr.Connect(net.SSID, "", false)
									nav.Dispatch(func() {
										nav.Push(newConnectingScene(nav, mgr, net.SSID)) //nolint:errcheck
									})
								}()
							} else {
								nav.Push(newPasswordScene(nav, mgr, net.SSID)) //nolint:errcheck
							}
						},
					})
				}
				list.SetItems(newItems)
				nav.RequestRender()
			})
		}()
	}

	return newCategoryScene(nav, "WiFi", list,
		withExtraSidebarBtn(Icons.Back, onScan), // Icons.Back as placeholder for Scan
	)
}

type wifiToggleItem struct {
	toggle *gui.Toggle
	onTap  func()
}

func (w *wifiToggleItem) Draw(cv *canvas.Canvas, bounds image.Rectangle) {
	f := canvas.EmbeddedFont(12)
	if f != nil {
		ty := bounds.Min.Y + (bounds.Dy()-f.LineHeight())/2
		cv.DrawText(bounds.Min.X+6, ty, "WiFi", f, canvas.Black)
	}
	toggleBounds := image.Rect(bounds.Max.X-46, bounds.Min.Y+4, bounds.Max.X-4, bounds.Max.Y-4)
	w.toggle.SetBounds(toggleBounds)
	w.toggle.Draw(cv)
}

func (w *wifiToggleItem) OnTap() { w.onTap() }

type wifiNetItem struct {
	net   wifi.Network
	onTap func()
}

func (w *wifiNetItem) Draw(cv *canvas.Canvas, bounds image.Rectangle) {
	name := w.net.SSID
	if w.net.Saved {
		name = "* " + name
	}
	if f := canvas.EmbeddedFont(12); f != nil {
		ty := bounds.Min.Y + (bounds.Dy()-f.LineHeight())/2
		cv.DrawText(bounds.Min.X+6, ty, name, f, canvas.Black)
	}
	drawSignalBars(cv, bounds, w.net.Signal)
	if w.net.Security != "Open" {
		drawLockIcon(cv, bounds)
	}
}

func (w *wifiNetItem) OnTap() { w.onTap() }

func drawSignalBars(cv *canvas.Canvas, bounds image.Rectangle, signal int) {
	x := bounds.Max.X - 20
	y := bounds.Max.Y - 4
	bars := 1
	if signal > -75 {
		bars = 2
	}
	if signal > -60 {
		bars = 3
	}
	for i := 0; i < 3; i++ {
		h := (i + 1) * 4
		r := image.Rect(x+i*5, y-h, x+i*5+3, y)
		if i < bars {
			cv.DrawRect(r, canvas.Black, true)
		} else {
			cv.DrawRect(r, canvas.Black, false)
		}
	}
}

func drawLockIcon(cv *canvas.Canvas, bounds image.Rectangle) {
	x := bounds.Max.X - 36
	y := bounds.Min.Y + (bounds.Dy()-8)/2
	cv.DrawRect(image.Rect(x, y, x+6, y+8), canvas.Black, true)
}
