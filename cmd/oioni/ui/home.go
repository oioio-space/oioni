// cmd/oioni/ui/home.go — HomeScene: operator-style menu home screen
package ui

import (
	"image"

	"github.com/oioio-space/oioni/drivers/epd"
	"github.com/oioio-space/oioni/ui/gui"
)

// NewHomeScene builds the home screen: 22px NetworkStatusBar + 100px scrollable menu.
// Returns the scene and the NetworkStatusBar so the caller can call SetInterfaces/SetTools.
func NewHomeScene(nav *gui.Navigator) (*gui.Scene, *gui.NetworkStatusBar) {
	nsb := gui.NewNetworkStatusBar(nav)

	list := newScrollableMenuList([]homeMenuItem{
		{
			name:  "Config",
			desc:  "reseau - interfaces",
			icon:  Icons.Config,
			onTap: func() { nav.Dispatch(func() { nav.Push(NewConfigScene(nav)) }) }, //nolint:errcheck
		},
		{
			name:  "System",
			desc:  "services - logs",
			icon:  Icons.System,
			onTap: func() { nav.Dispatch(func() { nav.Push(NewSystemScene(nav)) }) }, //nolint:errcheck
		},
		{
			name:  "Attack",
			desc:  "MITM - scan - deauth",
			icon:  Icons.Attack,
			onTap: func() { nav.Dispatch(func() { nav.Push(NewAttackScene(nav)) }) }, //nolint:errcheck
		},
		{
			name:  "DFIR",
			desc:  "capture - forensics",
			icon:  Icons.DFIR,
			onTap: func() { nav.Dispatch(func() { nav.Push(NewDFIRScene(nav)) }) }, //nolint:errcheck
		},
		{
			name:  "Info",
			desc:  "aide - a propos",
			icon:  Icons.Info,
			onTap: func() { nav.Dispatch(func() { nav.Push(NewInfoScene(nav)) }) }, //nolint:errcheck
		},
	})

	upBtn   := newNavButton("^", list.ScrollUp, list.CanScrollUp)
	downBtn := newNavButton("v", list.ScrollDown, list.CanScrollDown)

	navCol := gui.NewVBox(
		gui.Expand(upBtn),
		gui.Expand(downBtn),
	)
	menuRow := gui.NewHBox(
		gui.Expand(list),
		gui.FixedSize(navCol, menuNavW),
	)
	content := gui.NewVBox(
		gui.FixedSize(nsb, 22),
		gui.Expand(menuRow),
	)
	content.SetBounds(image.Rect(0, 0, epd.Height, epd.Width))

	return &gui.Scene{
		Title: "Home",
		// nsb, list, upBtn, downBtn at top level for Navigator automatic touch routing by bounds.
		Widgets: []gui.Widget{content, nsb, list, upBtn, downBtn},
	}, nsb
}
