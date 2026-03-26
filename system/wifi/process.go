// system/wifi/process.go — subprocess management for wifi processes
package wifi

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
)

type realProcess struct{}

// Start runs a command to completion (used for daemon-mode -B processes).
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

// StartProcess launches a foreground process and returns its *os.Process for
// lifecycle management (Wait, Signal). The caller is responsible for Wait()ing.
func (r *realProcess) StartProcess(bin string, args []string) (*os.Process, error) {
	cmd := exec.Command(bin, args...)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start %s: %w", bin, err)
	}
	return cmd.Process, nil
}
