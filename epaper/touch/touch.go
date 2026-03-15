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

const (
	regProductID = 0x8140
	regTouchFlag = 0x814E
	regTouchData = 0x814F
)

func (d *Detector) readReg(reg uint16, n int) ([]byte, error) {
	w := []byte{byte(reg >> 8), byte(reg)}
	r := make([]byte, n)
	if err := d.i2c.Tx(w, r); err != nil {
		return nil, err
	}
	return r, nil
}

func (d *Detector) writeReg(reg uint16, val byte) error {
	return d.i2c.Tx([]byte{byte(reg >> 8), byte(reg), val}, nil)
}

func (d *Detector) gt1151Reset() error {
	if err := d.trst.Out(true); err != nil {
		return err
	}
	time.Sleep(100 * time.Millisecond)
	if err := d.trst.Out(false); err != nil {
		return err
	}
	time.Sleep(100 * time.Millisecond)
	if err := d.trst.Out(true); err != nil {
		return err
	}
	time.Sleep(200 * time.Millisecond)
	return nil
}

// Start initialises the GT1151 and launches the event goroutine (Task 8).
func (d *Detector) Start(ctx context.Context) (<-chan TouchEvent, error) {
	// Hardware init: reset + verify product ID
	if err := d.gt1151Reset(); err != nil {
		return nil, fmt.Errorf("touch reset: %w", err)
	}
	pid, err := d.readReg(regProductID, 4)
	if err != nil {
		return nil, fmt.Errorf("GT1151 init: read product ID: %w", err)
	}
	// All-zeros means I2C is not responding
	if pid[0] == 0 && pid[1] == 0 && pid[2] == 0 && pid[3] == 0 {
		return nil, fmt.Errorf("GT1151 init: no response (product ID all zeros)")
	}

	ch := make(chan TouchEvent, 8)
	go func() {
		defer close(ch)
		for {
			if err := d.intPin.WaitFalling(ctx); err != nil {
				return // ctx cancelled or interrupt error
			}

			flagBuf, err := d.readReg(regTouchFlag, 1)
			if err != nil {
				d.err = err
				return
			}
			flag := flagBuf[0]
			if flag&0x80 == 0 {
				// flag bit not set — not a valid touch event
				continue
			}
			count := int(flag & 0x0F)
			if count < 1 || count > 5 {
				d.writeReg(regTouchFlag, 0x00) //nolint:errcheck
				continue
			}

			data, err := d.readReg(regTouchData, count*8)
			if err != nil {
				d.err = err
				return
			}
			if err := d.writeReg(regTouchFlag, 0x00); err != nil {
				d.err = err
				return
			}

			evt := TouchEvent{Time: time.Now(), Points: make([]TouchPoint, count)}
			for i := 0; i < count; i++ {
				o := i * 8
				evt.Points[i] = TouchPoint{
					ID:   data[o],
					X:    uint16(data[o+1]) | uint16(data[o+2])<<8,
					Y:    uint16(data[o+3]) | uint16(data[o+4])<<8,
					Size: data[o+5],
				}
			}

			select {
			case ch <- evt:
			default: // drop if consumer is slow
			}
		}
	}()
	return ch, nil
}

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
