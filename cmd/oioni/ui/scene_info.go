// cmd/oioni/ui/scene_info.go — Info category scene
package ui

import "github.com/oioio-space/oioni/ui/gui"

// NewInfoScene builds the Info category scene.
func NewInfoScene(nav *gui.Navigator) *gui.Scene {
	return newCategoryScene(nav, "Info", gui.NewLabel("(coming soon)"))
}
