package touch

import (
	"context"
	"fmt"
	"time"
)

// TouchPoint is a single contact point from the GT1151.
type TouchPoint struct {
	ID   uint8
	X, Y uint16
	Size uint8
}

// TouchEvent carries all active touch points at a moment in time.
type TouchEvent struct {
	Points []TouchPoint
	Time   time.Time
}

// Config holds device paths and BCM pin numbers.
type Config struct {
	I2CDevice string // e.g. "/dev/i2c-1"
	I2CAddr   uint16 // 0x14
	PinTRST   int    // BCM 22
	PinINT    int    // BCM 27
}

// Detector reads touch events from the GT1151.
type Detector struct {
	i2c     I2CConn
	trst    OutputPin
	intPin  InterruptPin
	err     error
	closers []func() error
}

func newDetector(i2c I2CConn, trst OutputPin, intPin InterruptPin) *Detector {
	return &Detector{i2c: i2c, trst: trst, intPin: intPin}
}

// New opens hardware resources for the GT1151 touch controller.
func New(cfg Config) (*Detector, error) {
	i2c, err := openI2C(cfg.I2CDevice, cfg.I2CAddr)
	if err != nil {
		return nil, fmt.Errorf("touch New: %w", err)
	}
	trst, err := openGPIOOutput(cfg.PinTRST)
	if err != nil {
		i2c.Close()
		return nil, fmt.Errorf("touch New TRST: %w", err)
	}
	intPin, err := openGPIOInterrupt(cfg.PinINT)
	if err != nil {
		i2c.Close()
		trst.Close()
		return nil, fmt.Errorf("touch New INT: %w", err)
	}
	d := newDetector(i2c, trst, intPin)
	d.closers = []func() error{i2c.Close, trst.Close, intPin.Close}
	return d, nil
}

// Start initialises the GT1151 and launches the event goroutine (Task 8).
func (d *Detector) Start(ctx context.Context) (<-chan TouchEvent, error) { return nil, nil }

// Err returns the first runtime error from the goroutine, or nil.
// Only valid after the channel returned by Start is closed.
func (d *Detector) Err() error { return d.err }

// Close releases all hardware resources.
func (d *Detector) Close() error {
	var first error
	for _, fn := range d.closers {
		if err := fn(); err != nil && first == nil {
			first = err
		}
	}
	return first
}
