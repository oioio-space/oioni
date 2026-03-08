package usbdetect

import (
	"context"
	"fmt"
)

// Event represents a USB block partition appearing or disappearing.
type Event struct {
	Action string // "add" or "remove"
	Device string // e.g. "/dev/sda1"
}

// Detector discovers USB mass storage partitions.
type Detector struct {
	sysBlock string // normally "/sys/block"; override in tests
}

// New returns a Detector using the real sysfs path.
func New() *Detector {
	return &Detector{sysBlock: "/sys/block"}
}

// Start performs an initial sysfs scan (emitting "add" events for existing
// drives), then listens for netlink hotplug events until ctx is cancelled.
// The returned channel is closed when ctx is cancelled.
func (d *Detector) Start(ctx context.Context) (<-chan Event, error) {
	// Start netlink first so we don't miss events that arrive during the scan.
	nl, err := listenNetlink(ctx)
	if err != nil {
		return nil, fmt.Errorf("usbdetect: %w", err)
	}

	out := make(chan Event, 32)

	go func() {
		defer close(out)

		// Emit synthetic "add" events for drives already present.
		devs, _ := scanSysfs(d.sysBlock)
		for _, dev := range devs {
			select {
			case out <- Event{Action: "add", Device: dev}:
			case <-ctx.Done():
				return
			}
		}

		// Forward netlink events.
		for ev := range nl {
			select {
			case out <- ev:
			case <-ctx.Done():
				return
			}
		}
	}()

	return out, nil
}
