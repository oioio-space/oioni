// cmd/oioni/ui/scene_system.go — System category scene
package ui

import "github.com/oioio-space/oioni/ui/gui"

// NewSystemScene builds the System category scene.
func NewSystemScene(nav *gui.Navigator) *gui.Scene {
	return newCategoryScene(nav, "System", gui.NewLabel("(coming soon)"))
}
