// cmd/oioni/ui/icons.go — pre-loaded icon assets for the oioni UI
package ui

import (
	"embed"
	"image/png"
	"log"

	"github.com/oioio-space/oioni/ui/gui"
)

//go:embed icons/*.png
var iconFS embed.FS

// Icons holds all pre-loaded icon assets. Populated by init().
var Icons struct {
	Config, System, Attack, DFIR, Info gui.Icon
	Oni, Back, Up, Down                gui.Icon
}

func init() {
	Icons.Config = mustLoadIcon("config")
	Icons.System = mustLoadIcon("system")
	Icons.Attack = mustLoadIcon("attack")
	Icons.DFIR = mustLoadIcon("dfir")
	Icons.Info = mustLoadIcon("info")
	Icons.Oni = mustLoadIcon("oni")
	Icons.Back = mustLoadIcon("back")
	Icons.Up = mustLoadIcon("up")
	Icons.Down = mustLoadIcon("down")
}

func mustLoadIcon(name string) gui.Icon {
	f, err := iconFS.Open("icons/" + name + ".png")
	if err != nil {
		log.Printf("ui: icon not found: %s: %v", name, err)
		return gui.Icon{}
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		log.Printf("ui: icon decode failed: %s: %v", name, err)
		return gui.Icon{}
	}
	return gui.NewImageIcon(img)
}
