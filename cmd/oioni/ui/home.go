// cmd/oioni/ui/home.go — HomeScene: operator-style menu home screen
package ui

import (
	"image"

	"github.com/oioio-space/oioni/drivers/epd"
	"github.com/oioio-space/oioni/ui/gui"
)

// NewHomeScene builds the home screen: 22px NetworkStatusBar + scrollable menu.
// Returns the scene and the NetworkStatusBar so the caller can call SetInterfaces/SetTools.
func NewHomeScene(nav *gui.Navigator) (*gui.Scene, *gui.NetworkStatusBar) {
	nsb := gui.NewNetworkStatusBar(nav)

	// NOTE: items are *homeListItem (pointer to struct implementing gui.ListItem).
	// Do NOT use struct literals on gui.ListItem — it is an interface, not a struct.
	items := []gui.ListItem{
		&homeListItem{name: "Config", icon: Icons.Config, onTap: func() { nav.Dispatch(func() { nav.Push(NewConfigScene(nav)) }) }}, //nolint:errcheck
		&homeListItem{name: "System", icon: Icons.System, onTap: func() { nav.Dispatch(func() { nav.Push(NewSystemScene(nav)) }) }}, //nolint:errcheck
		&homeListItem{name: "Attack", icon: Icons.Attack, onTap: func() { nav.Dispatch(func() { nav.Push(NewAttackScene(nav)) }) }}, //nolint:errcheck
		&homeListItem{name: "DFIR",   icon: Icons.DFIR,   onTap: func() { nav.Dispatch(func() { nav.Push(NewDFIRScene(nav)) }) }}, //nolint:errcheck
		&homeListItem{name: "Info",   icon: Icons.Info,   onTap: func() { nav.Dispatch(func() { nav.Push(NewInfoScene(nav)) }) }}, //nolint:errcheck
	}

	list := gui.NewScrollableList(items, homeRowH)
	sidebar := gui.NewActionSidebar(
		gui.SidebarButton{Icon: Icons.Oni, OnTap: func() {}},
		gui.SidebarButton{Icon: Icons.Up, OnTap: list.ScrollUp},
		gui.SidebarButton{Icon: Icons.Down, OnTap: list.ScrollDown},
	)

	menuRow := gui.NewHBox(gui.Expand(list), gui.FixedSize(sidebar, homeNavW))
	content := gui.NewVBox(gui.FixedSize(nsb, 22), gui.Expand(menuRow))
	content.SetBounds(image.Rect(0, 0, epd.Height, epd.Width))

	return &gui.Scene{
		Title:   "Home",
		Widgets: []gui.Widget{content},
	}, nsb
}
