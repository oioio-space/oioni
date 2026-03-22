// cmd/oioni/ui/scene_config.go — Config category scene
package ui

import (
	"image"

	"github.com/oioio-space/oioni/system/netconf"
	"github.com/oioio-space/oioni/system/wifi"
	"github.com/oioio-space/oioni/ui/canvas"
	"github.com/oioio-space/oioni/ui/gui"
)

// NewConfigScene builds the Config category scene: WiFi + Network items.
func NewConfigScene(nav *gui.Navigator, wifiMgr *wifi.Manager, netconfMgr *netconf.Manager) *gui.Scene {
	items := []gui.ListItem{
		&configListItem{name: "WiFi", onTap: func() {
			nav.Dispatch(func() { nav.Push(NewWifiScene(nav, wifiMgr)) }) //nolint:errcheck
		}},
		&configListItem{name: "Network", onTap: func() {
			nav.Dispatch(func() { nav.Push(NewNetworkScene(nav, netconfMgr)) }) //nolint:errcheck
		}},
	}
	list := gui.NewScrollableList(items, 40)
	return newCategoryScene(nav, "Config", list)
}

// configListItem renders a single-line text menu entry.
type configListItem struct {
	name  string
	onTap func()
}

func (c *configListItem) Draw(cv *canvas.Canvas, bounds image.Rectangle) {
	f := canvas.EmbeddedFont(12)
	if f != nil {
		ty := bounds.Min.Y + (bounds.Dy()-f.LineHeight())/2
		cv.DrawText(bounds.Min.X+6, ty, c.name, f, canvas.Black)
	}
}

func (c *configListItem) OnTap() { c.onTap() }

