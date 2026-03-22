// cmd/oioni/ui/scene_helpers.go — shared category scene builder
package ui

import (
	"image"

	"github.com/oioio-space/oioni/drivers/epd"
	"github.com/oioio-space/oioni/ui/gui"
)

// SubSceneOption configures optional behavior of a category scene.
type SubSceneOption func(*subSceneConfig)

type subSceneConfig struct {
	extraSidebarButtons []gui.SidebarButton
}

// withExtraSidebarBtn adds an extra icon button above the default Oni/Back sidebar buttons.
func withExtraSidebarBtn(icon gui.Icon, onTap func()) SubSceneOption {
	return func(cfg *subSceneConfig) {
		cfg.extraSidebarButtons = append(cfg.extraSidebarButtons, gui.SidebarButton{
			Icon:  icon,
			OnTap: onTap,
		})
	}
}

// newCategoryScene builds a category scene: NavBar breadcrumb + content area + ActionSidebar.
//
// Layout (250×122px logical):
//
//	┌─────────────────────────┬────┐
//	│ Home > title   (18px)   │    │
//	├─────────────────────────┤Oni │ 44px wide
//	│      contentWidget      │────│
//	│      (expands)          │Back│
//	└─────────────────────────┴────┘
//
// Default sidebar: [Oni → home, Back → pop one level].
// Extra buttons prepended above Oni via SubSceneOption.
//
// Touch routing: root's Children() traversal reaches the sidebar recursively,
// so sidebar must NOT be listed separately in Scene.Widgets.
//
// Swipe-scroll: if contentWidget needs swipe-to-scroll, the caller must add it
// as an additional top-level Scene.Widget (Navigator's swipe handler is not recursive).
func newCategoryScene(nav *gui.Navigator, title string, contentWidget gui.Widget, opts ...SubSceneOption) *gui.Scene {
	cfg := &subSceneConfig{}
	for _, o := range opts {
		o(cfg)
	}

	navbar := gui.NewNavBar("Home", title)

	// Build sidebar: extra buttons first, then default Oni + Back.
	sidebarBtns := make([]gui.SidebarButton, 0, len(cfg.extraSidebarButtons)+2)
	sidebarBtns = append(sidebarBtns, cfg.extraSidebarButtons...)
	sidebarBtns = append(sidebarBtns,
		gui.SidebarButton{Icon: Icons.Oni, OnTap: func() { popToRoot(nav) }},
		gui.SidebarButton{Icon: Icons.Back, OnTap: func() { nav.Pop() }}, //nolint:errcheck
	)
	sidebar := gui.NewActionSidebar(sidebarBtns...)

	// NavBar.Draw() renders its 2px separator within its own 18px bounds —
	// no external spacer needed between navbar and content.
	content := gui.NewVBox(
		gui.FixedSize(navbar, 18),
		gui.Expand(contentWidget),
	)
	root := gui.NewHBox(gui.Expand(content), gui.FixedSize(sidebar, 44))
	root.SetBounds(image.Rect(0, 0, epd.Height, epd.Width))

	return &gui.Scene{
		Title:   title,
		Widgets: []gui.Widget{root},
	}
}

// popToRoot pops all scenes until only the root (home) scene remains.
func popToRoot(nav *gui.Navigator) {
	nav.PopTo(1) //nolint:errcheck
}
