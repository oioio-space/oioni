// cmd/oioni/ui/keepalive.go — 24h e-paper keep-alive (Waveshare hardware requirement)
//
// Per Waveshare 2.13" Touch e-Paper HAT hardware manual:
// "If the screen is not refreshed for a long time, it will cause permanent damage to the screen."
// The maximum idle time between refreshes is approximately 24 hours.
//
// StartKeepAlive runs a goroutine that wakes the Navigator every 24 hours,
// triggering a full refresh even without user interaction.
package ui

import (
	"context"
	"time"

	"github.com/oioio-space/oioni/ui/gui"
)

const keepAliveInterval = 24 * time.Hour

// StartKeepAlive starts a background goroutine that calls nav.Wake() every 24 hours.
// The goroutine stops when ctx is cancelled.
func StartKeepAlive(ctx context.Context, nav *gui.Navigator) {
	go func() {
		t := time.NewTicker(keepAliveInterval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				nav.Wake()
			}
		}
	}()
}
