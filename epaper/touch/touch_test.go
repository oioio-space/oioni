package touch

import (
	"context"
	"testing"
)

type fakeI2C struct {
	responses [][]byte
	idx       int
}

func (f *fakeI2C) Tx(w, r []byte) error {
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
