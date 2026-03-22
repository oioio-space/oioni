// cmd/oioni/ui/scene_network.go — network interface list scene
package ui

import (
	"image"

	"github.com/oioio-space/oioni/system/netconf"
	"github.com/oioio-space/oioni/ui/canvas"
	"github.com/oioio-space/oioni/ui/gui"
)

// NewNetworkScene lists physical interfaces; tapping one opens IP Config.
func NewNetworkScene(nav *gui.Navigator, mgr *netconf.Manager) *gui.Scene {
	var ifaces []string
	if mgr != nil {
		ifaces, _ = mgr.ListInterfaces()
	}

	items := make([]gui.ListItem, len(ifaces))
	for i, name := range ifaces {
		name := name
		var ip string
		if mgr != nil {
			if st, err := mgr.Status(name); err == nil && st.IP != "" {
				ip = st.IP
			}
		}
		items[i] = &ifaceListItem{
			name: name,
			ip:   ip,
			onTap: func() {
				nav.Push(newIPConfigScene(nav, mgr, name)) //nolint:errcheck
			},
		}
	}

	list := gui.NewScrollableList(items, 36)
	return newCategoryScene(nav, "Network", list)
}

type ifaceListItem struct {
	name  string
	ip    string
	onTap func()
}

func (f *ifaceListItem) Draw(cv *canvas.Canvas, b image.Rectangle) {
	text := f.name
	if f.ip != "" {
		text += " " + f.ip
	}
	if fn := canvas.EmbeddedFont(12); fn != nil {
		cv.DrawText(b.Min.X+6, b.Min.Y+(b.Dy()-fn.LineHeight())/2, text, fn, canvas.Black)
	}
}

func (f *ifaceListItem) OnTap() { f.onTap() }
