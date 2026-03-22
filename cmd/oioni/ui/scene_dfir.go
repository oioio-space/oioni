// cmd/oioni/ui/scene_dfir.go — DFIR category scene
package ui

import "github.com/oioio-space/oioni/ui/gui"

// NewDFIRScene builds the DFIR category scene.
func NewDFIRScene(nav *gui.Navigator) *gui.Scene {
	return newCategoryScene(nav, "DFIR", gui.NewLabel("(coming soon)"))
}
