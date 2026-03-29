package wifi

import "errors"

// Sentinel errors returned by Manager methods.
// Callers should use errors.Is to test for these values.
var (
	// ErrNotStarted is returned by any Manager method that requires an active
	// wpa_supplicant connection when Start has not yet been called.
	ErrNotStarted = errors.New("wifi: not started")

	// ErrWPATimeout is returned when polling for wpa_supplicant state
	// (connection, channel, control socket) exceeds the deadline.
	ErrWPATimeout = errors.New("wifi: timed out waiting for wpa_supplicant")
)
