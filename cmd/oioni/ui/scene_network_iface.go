// cmd/oioni/ui/scene_network_iface.go — IP configuration scene for one interface
package ui

import (
	"image"

	"github.com/oioio-space/oioni/system/netconf"
	"github.com/oioio-space/oioni/ui/canvas"
	"github.com/oioio-space/oioni/ui/gui"
)

// newIPConfigScene builds the IP Config scene for a single interface.
func newIPConfigScene(nav *gui.Navigator, mgr *netconf.Manager, iface string) *gui.Scene {
	var current netconf.IfaceCfg
	if mgr != nil {
		current, _ = mgr.Get(iface)
	}

	initialMode := modeDHCP
	if current.Mode == netconf.ModeStatic {
		initialMode = modeStatic
	}

	modeSel := newModeSelector(initialMode)

	ipVal := current.IP
	gwVal := current.Gateway
	dnsVal := ""
	if len(current.DNS) > 0 {
		dnsVal = current.DNS[0]
	}

	makeFieldItem := func(label string, valPtr *string) *fieldItem {
		return &fieldItem{
			label:  label,
			valPtr: valPtr,
			onTap: func() {
				gui.ShowTextInput(nav, label, 40, func(v string) {
					*valPtr = v
				})
			},
		}
	}

	ipItem := makeFieldItem("IP (CIDR)", &ipVal)
	gwItem := makeFieldItem("Passerelle", &gwVal)
	dnsItem := makeFieldItem("DNS", &dnsVal)

	list := gui.NewScrollableList([]gui.ListItem{ipItem, gwItem, dnsItem}, 34)

	onSave := func() {
		if mgr == nil {
			return
		}
		cfg := netconf.IfaceCfg{Mode: netconf.ModeDHCP}
		if modeSel.Mode() == modeStatic {
			cfg = netconf.IfaceCfg{
				Mode:    netconf.ModeStatic,
				IP:      ipVal,
				Gateway: gwVal,
			}
			if dnsVal != "" {
				cfg.DNS = []string{dnsVal}
			}
		}
		_ = mgr.Apply(iface, cfg)
		nav.Pop() //nolint:errcheck
	}

	s := newCategoryScene(nav, "Network", list,
		withExtraSidebarBtn(Icons.Back, onSave), // Icons.Back as placeholder for Save
	)

	modeSel.SetBounds(image.Rect(0, 18, 206, 62))
	s.Widgets = append(s.Widgets, modeSel)

	return s
}

type fieldItem struct {
	label  string
	valPtr *string
	onTap  func()
}

func (f *fieldItem) Draw(cv *canvas.Canvas, b image.Rectangle) {
	val := *f.valPtr
	if val == "" {
		val = "--"
	}
	text := f.label + ": " + val
	if fn := canvas.EmbeddedFont(12); fn != nil {
		cv.DrawText(b.Min.X+6, b.Min.Y+(b.Dy()-fn.LineHeight())/2, text, fn, canvas.Black)
	}
}

func (f *fieldItem) OnTap() { f.onTap() }
