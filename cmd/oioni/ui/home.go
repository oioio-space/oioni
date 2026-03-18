// cmd/oioni/ui/home.go — HomeScene: main navigation screen
package ui

import (
	"image"

	"github.com/oioio-space/oioni/drivers/epd"
	"github.com/oioio-space/oioni/ui/gui"
)

// NewHomeScene builds the home screen with NavBar + IconCarousel + StatusBar + ActionSidebar.
// The carousel is listed both in the layout (for rendering) and at the top level of
// Scene.Widgets (for Navigator hScrollable routing).
func NewHomeScene(nav *gui.Navigator, status *gui.StatusBar) *gui.Scene {
	carousel := gui.NewIconCarousel([]gui.CarouselItem{
		{Icon: Icons.Config, Label: "Config", OnTap: func() { nav.Dispatch(func() { nav.Push(NewConfigScene(nav)) }) }},  //nolint:errcheck
		{Icon: Icons.System, Label: "System", OnTap: func() { nav.Dispatch(func() { nav.Push(NewSystemScene(nav)) }) }},  //nolint:errcheck
		{Icon: Icons.Attack, Label: "Attack", OnTap: func() { nav.Dispatch(func() { nav.Push(NewAttackScene(nav)) }) }},  //nolint:errcheck
		{Icon: Icons.DFIR, Label: "DFIR", OnTap: func() { nav.Dispatch(func() { nav.Push(NewDFIRScene(nav)) }) }},        //nolint:errcheck
		{Icon: Icons.Info, Label: "Info", OnTap: func() { nav.Dispatch(func() { nav.Push(NewInfoScene(nav)) }) }},         //nolint:errcheck
	})

	navbar := gui.NewNavBar("Home")
	sidebar := gui.NewActionSidebar(
		gui.SidebarButton{Label: "<", Height: 44, OnTap: func() { carousel.ScrollH(-1) }},
		gui.SidebarButton{Icon: Icons.Oni, OnTap: func() { /* already at home */ }},
		gui.SidebarButton{Label: ">", Height: 44, OnTap: func() { carousel.ScrollH(+1) }},
	)

	content := gui.NewVBox(
		gui.FixedSize(navbar, 18),           // 18px: 12pt text + 2px separator + padding
		gui.FixedSize(gui.NewSpacer(), 2),   // 2px gap below separator
		gui.FixedSize(carousel, 82),         // 18+2+82+2+18=122 = epd.Width
		gui.FixedSize(gui.NewSpacer(), 2),   // 2px gap above status bar
		gui.FixedSize(status, 18),
	)
	root := gui.NewHBox(gui.Expand(content), gui.FixedSize(sidebar, 44))
	// epd.Height=250 = logical width (after Rot90), epd.Width=122 = logical height.
	// 2px horizontal inset gives padding and ensures carousel fits all 5 items.
	root.SetBounds(image.Rect(2, 0, epd.Height-2, epd.Width))

	var savedIdx int
	return &gui.Scene{
		Title: "Home",
		// root renders everything; carousel at top level enables hScrollable routing;
		// sidebar at top level ensures Navigator touch routing reaches it.
		Widgets: []gui.Widget{root, carousel, sidebar},
		OnLeave: func() { savedIdx = carousel.Index() },
		OnEnter: func() { carousel.SetIndex(savedIdx) },
	}
}
