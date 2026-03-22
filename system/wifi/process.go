// system/wifi/process.go — wpa_supplicant subprocess management
package wifi

import "os/exec"

type realProcess struct{}

func (r *realProcess) Start(bin string, args []string) error {
	cmd := exec.Command(bin, args...)
	return cmd.Run() // -B causes immediate exit after daemonising
}
