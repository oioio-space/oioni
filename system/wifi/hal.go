// system/wifi/hal.go — injectable interfaces for wpa_supplicant HAL
package wifi

// wpaConn is the interface over a wpa_supplicant control socket connection.
// Real implementation: realWpaConn (wpa.go). Test implementation: fakeWpaConn.
type wpaConn interface {
	// SendCommand sends a single-line command and returns the full response.
	SendCommand(cmd string) (string, error)
	Close() error
}

// processRunner starts and terminates wpa_supplicant.
type processRunner interface {
	// Start launches wpa_supplicant with the given args. Returns an error if
	// the process fails to start. The process runs detached (daemon mode -B).
	Start(bin string, args []string) error
}
