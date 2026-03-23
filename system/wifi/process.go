// system/wifi/process.go — wpa_supplicant subprocess management
package wifi

import (
	"bytes"
	"fmt"
	"os/exec"
)

type realProcess struct{}

func (r *realProcess) Start(bin string, args []string) error {
	cmd := exec.Command(bin, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil { // -B causes immediate exit after daemonising
		out := bytes.TrimSpace(stderr.Bytes())
		if len(out) > 0 {
			return fmt.Errorf("%w; stderr: %s", err, out)
		}
		return err
	}
	return nil
}
