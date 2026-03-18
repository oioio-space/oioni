// cmd/oioni/ui/icons.go — pre-loaded icon assets for the oioni UI
package ui

import (
	"embed"
	"image/png"

	"github.com/oioio-space/oioni/ui/gui"
)

//go:embed icons/*.png
var iconFS embed.FS

// Icons holds all pre-loaded icon assets. Populated by init().
var Icons struct {
	Config, System, Attack, DFIR, Info gui.Icon
	Oni, Back                          gui.Icon
}

func init() {
	Icons.Config = mustLoadIcon("config")
	Icons.System = mustLoadIcon("system")
	Icons.Attack = mustLoadIcon("attack")
	Icons.DFIR = mustLoadIcon("dfir")
	Icons.Info = mustLoadIcon("info")
	Icons.Oni = mustLoadIcon("oni")
	Icons.Back = mustLoadIcon("back")
}

func mustLoadIcon(name string) gui.Icon {
	f, err := iconFS.Open("icons/" + name + ".png")
	if err != nil {
		panic("icon not found: " + name)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		panic("icon decode failed: " + name + ": " + err.Error())
	}
	return gui.NewImageIcon(img)
}
