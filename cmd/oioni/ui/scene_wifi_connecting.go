// cmd/oioni/ui/scene_wifi_connecting.go — connection status polling scene
package ui

import (
	"fmt"
	"time"

	"github.com/oioio-space/oioni/system/wifi"
	"github.com/oioio-space/oioni/ui/gui"
)

// newConnectingScene builds the WiFi connecting/status scene.
// It polls wifi.Status() every second in a goroutine.
// The goroutine is cancelled when Scene.OnLeave fires.
func newConnectingScene(nav *gui.Navigator, mgr *wifi.Manager, ssid string) *gui.Scene {
	statusLabel := gui.NewLabel(fmt.Sprintf("Connexion a %s...", ssid))
	cancel := make(chan struct{})

	var s *gui.Scene
	s = newCategoryScene(nav, "WiFi", statusLabel)
	s.OnLeave = func() {
		close(cancel)
	}
	s.OnEnter = func() {
		if mgr == nil {
			return
		}
		go func() {
			for {
				st, err := mgr.Status()
				nav.Dispatch(func() {
					if err != nil {
						statusLabel.SetText("Erreur de connexion")
					} else {
						switch st.State {
						case "COMPLETED":
							statusLabel.SetText(fmt.Sprintf("Connecte -- %s", st.SSID))
						case "ASSOCIATING", "AUTHENTICATING":
							statusLabel.SetText(fmt.Sprintf("Connexion a %s...", ssid))
						case "DISCONNECTED", "INACTIVE":
							statusLabel.SetText("Echec de connexion")
						default:
							statusLabel.SetText(st.State)
						}
					}
					nav.RequestRender()
				})
				select {
				case <-cancel:
					return
				case <-time.After(time.Second):
				}
			}
		}()
	}
	return s
}
