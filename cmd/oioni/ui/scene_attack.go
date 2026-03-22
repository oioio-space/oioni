// cmd/oioni/ui/scene_attack.go — Attack category scene
package ui

import "github.com/oioio-space/oioni/ui/gui"

// NewAttackScene builds the Attack category scene.
func NewAttackScene(nav *gui.Navigator) *gui.Scene {
	return newCategoryScene(nav, "Attack", gui.NewLabel("(coming soon)"))
}
