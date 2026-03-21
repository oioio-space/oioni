// cmd/oioni/ui/home.go — HomeScene: operator-style menu home screen
package ui

import (
	"image"

	"github.com/oioio-space/oioni/drivers/epd"
	"github.com/oioio-space/oioni/ui/gui"
)

// NewHomeScene builds the home screen: 22px NetworkStatusBar header + 100px HomeMenuWidget.
// Returns the scene and the NetworkStatusBar so the caller can call SetInterfaces/SetTools.
func NewHomeScene(nav *gui.Navigator) (*gui.Scene, *gui.NetworkStatusBar) {
	nsb := gui.NewNetworkStatusBar(nav)

	menu := newHomeMenuWidget([]homeMenuItem{
		{
			name:  "Config",
			desc:  "reseau · interfaces · device",
			onTap: func() { nav.Dispatch(func() { nav.Push(NewConfigScene(nav)) }) }, //nolint:errcheck
		},
		{
			name:  "System",
			desc:  "services · logs · processus",
			onTap: func() { nav.Dispatch(func() { nav.Push(NewSystemScene(nav)) }) }, //nolint:errcheck
		},
		{
			name:  "Attack",
			desc:  "MITM · scan · deauth · spoof",
			onTap: func() { nav.Dispatch(func() { nav.Push(NewAttackScene(nav)) }) }, //nolint:errcheck
		},
		{
			name:  "DFIR",
			desc:  "capture · pcap · forensics",
			onTap: func() { nav.Dispatch(func() { nav.Push(NewDFIRScene(nav)) }) }, //nolint:errcheck
		},
		{
			name:  "Info",
			desc:  "aide · licences · a propos",
			onTap: func() { nav.Dispatch(func() { nav.Push(NewInfoScene(nav)) }) }, //nolint:errcheck
		},
	})

	content := gui.NewVBox(
		gui.FixedSize(nsb, 22),
		gui.Expand(menu),
	)
	content.SetBounds(image.Rect(0, 0, epd.Height, epd.Width))

	return &gui.Scene{
		Title: "Home",
		// nsb and menu at top level so Navigator finds HandleTouch for badge tap and row taps.
		Widgets: []gui.Widget{content, nsb, menu},
	}, nsb
}
