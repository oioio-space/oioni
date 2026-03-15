package touch

import (
	"context"
	"testing"
	"time"
)

type fakeI2C struct {
	responses [][]byte
	idx       int
}

func (f *fakeI2C) Tx(w, r []byte) error {
	if r == nil {
		return nil
	}
	if f.idx < len(f.responses) {
		copy(r, f.responses[f.idx])
		f.idx++
	}
	return nil
}

type fakeOutputPin struct{ last bool }

func (f *fakeOutputPin) Out(high bool) error { f.last = high; return nil }

type fakeINT struct{ trigger chan struct{} }

func (f *fakeINT) WaitFalling(ctx context.Context) error {
	select {
	case <-f.trigger:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func TestNewDetector(t *testing.T) {
	d := newDetector(&fakeI2C{}, &fakeOutputPin{}, &fakeINT{trigger: make(chan struct{})})
	if d == nil {
		t.Fatal("newDetector returned nil")
	}
}

func TestStartReceivesTouchEvent(t *testing.T) {
	intCh := make(chan struct{}, 1)
	i2c := &fakeI2C{
		responses: [][]byte{
			{0x39, 0x35, 0x30, 0x31}, // product ID "9501" at 0x8140
			{0x81},                   // 0x814E: flag=1(bit7), count=1(bits 3:0)
			{0x01, 0x1E, 0x00, 0x3C, 0x00, 0x08, 0x00, 0x00}, // point: ID=1, X=30, Y=60, S=8
		},
	}
	d := newDetector(i2c, &fakeOutputPin{}, &fakeINT{trigger: intCh})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ch, err := d.Start(ctx)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	intCh <- struct{}{} // simulate falling edge

	select {
	case evt := <-ch:
		if len(evt.Points) != 1 {
			t.Fatalf("expected 1 point, got %d", len(evt.Points))
		}
		if evt.Points[0].X != 30 || evt.Points[0].Y != 60 {
			t.Errorf("expected X=30 Y=60, got X=%d Y=%d", evt.Points[0].X, evt.Points[0].Y)
		}
	case <-ctx.Done():
		t.Fatal("timeout waiting for touch event")
	}
}
