// cmd/oioni/ui/scene_config.go — Config category scene
package ui

import "github.com/oioio-space/oioni/ui/gui"

// NewConfigScene builds the Config category scene.
func NewConfigScene(nav *gui.Navigator) *gui.Scene {
	return newCategoryScene(nav, "Config", gui.NewLabel("(coming soon)"))
}
