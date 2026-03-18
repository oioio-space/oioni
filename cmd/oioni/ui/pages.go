// cmd/oioni/ui/pages.go — Category scene stubs for future implementation
package ui

import (
	"image"

	"github.com/oioio-space/oioni/drivers/epd"
	"github.com/oioio-space/oioni/ui/gui"
)

// NewConfigScene builds the Config category scene stub.
func NewConfigScene(nav *gui.Navigator) *gui.Scene {
	return newCategoryScene(nav, "Config")
}

// NewSystemScene builds the System category scene stub.
func NewSystemScene(nav *gui.Navigator) *gui.Scene {
	return newCategoryScene(nav, "System")
}

// NewAttackScene builds the Attack category scene stub.
func NewAttackScene(nav *gui.Navigator) *gui.Scene {
	return newCategoryScene(nav, "Attack")
}

// NewDFIRScene builds the DFIR category scene stub.
func NewDFIRScene(nav *gui.Navigator) *gui.Scene {
	return newCategoryScene(nav, "DFIR")
}

// NewInfoScene builds the Info category scene stub.
func NewInfoScene(nav *gui.Navigator) *gui.Scene {
	return newCategoryScene(nav, "Info")
}

// newCategoryScene builds a category page: NavBar with breadcrumb, placeholder content,
// sidebar with [oni → home, back → pop one level].
func newCategoryScene(nav *gui.Navigator, title string) *gui.Scene {
	navbar := gui.NewNavBar("Home", title)
	sidebar := gui.NewActionSidebar(
		gui.SidebarButton{Icon: Icons.Oni, OnTap: func() { popToRoot(nav) }},
		gui.SidebarButton{Icon: Icons.Back, OnTap: func() { nav.Pop() }}, //nolint:errcheck
	)
	placeholder := gui.NewLabel("(coming soon)")

	content := gui.NewVBox(
		gui.FixedSize(navbar, 16),
		gui.Expand(placeholder),
	)
	root := gui.NewHBox(gui.Expand(content), gui.FixedSize(sidebar, 44))
	// 2px horizontal inset for padding; sidebar also listed directly for touch routing.
	root.SetBounds(image.Rect(2, 0, epd.Height-2, epd.Width))

	return &gui.Scene{
		Title: title,
		// root renders everything; sidebar at top level ensures Navigator touch routing reaches it.
		Widgets: []gui.Widget{root, sidebar},
	}
}

// popToRoot pops all scenes until only the root (home) scene remains,
// rendering exactly once via nav.PopTo(1).
func popToRoot(nav *gui.Navigator) {
	nav.PopTo(1) //nolint:errcheck
}
