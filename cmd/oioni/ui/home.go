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
		{Icon: Icons.Config, Label: "Config", OnTap: func() { nav.Push(NewConfigScene(nav)) }},  //nolint:errcheck
		{Icon: Icons.System, Label: "System", OnTap: func() { nav.Push(NewSystemScene(nav)) }},  //nolint:errcheck
		{Icon: Icons.Attack, Label: "Attack", OnTap: func() { nav.Push(NewAttackScene(nav)) }},  //nolint:errcheck
		{Icon: Icons.DFIR, Label: "DFIR", OnTap: func() { nav.Push(NewDFIRScene(nav)) }},        //nolint:errcheck
		{Icon: Icons.Info, Label: "Info", OnTap: func() { nav.Push(NewInfoScene(nav)) }},         //nolint:errcheck
	})

	navbar := gui.NewNavBar("Home")
	sidebar := gui.NewActionSidebar(
		gui.SidebarButton{Icon: Icons.Oni, OnTap: func() { /* already at home */ }},
	)

	content := gui.NewVBox(
		gui.FixedSize(navbar, 16),
		gui.FixedSize(carousel, 88),
		gui.FixedSize(status, 18),
	)
	root := gui.NewHBox(gui.Expand(content), gui.FixedSize(sidebar, 44))
	// epd.Height=250 = logical width (after Rot90), epd.Width=122 = logical height.
	root.SetBounds(image.Rect(0, 0, epd.Height, epd.Width))

	var savedIdx int
	return &gui.Scene{
		Title: "Home",
		// carousel listed twice: root renders it; direct entry enables hScrollable routing.
		Widgets: []gui.Widget{root, carousel},
		OnLeave: func() { savedIdx = carousel.Index() },
		OnEnter: func() { carousel.SetIndex(savedIdx) },
	}
}
