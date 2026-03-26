// system/wifi/hal.go — injectable interfaces for wpa_supplicant HAL
package wifi

import "os"

// wpaConn is the interface over a wpa_supplicant control socket connection.
// Real implementation: realWpaConn (wpa.go). Test implementation: fakeWpaConn.
type wpaConn interface {
	// SendCommand sends a single-line command and returns the full response.
	SendCommand(cmd string) (string, error)
	Close() error
}

// processRunner starts external processes used by wifi management.
type processRunner interface {
	// Start runs a command to completion (used for daemon-mode -B processes).
	Start(bin string, args []string) error
	// StartProcess launches a foreground process and returns it for lifecycle management.
	StartProcess(bin string, args []string) (*os.Process, error)
}
