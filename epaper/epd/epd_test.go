// epaper/epd/epd_test.go
package epd

import (
	"testing"
)

// fakeSPI captures all bytes sent via Tx.
type fakeSPI struct{ log [][]byte }

func (f *fakeSPI) Tx(w []byte) error {
	f.log = append(f.log, append([]byte(nil), w...))
	return nil
}

// fakeOutputPin records the last value written.
type fakeOutputPin struct{ last bool }

func (f *fakeOutputPin) Out(high bool) error { f.last = high; return nil }

// fakeInputPin returns a configurable value.
type fakeInputPin struct{ val bool }

func (f *fakeInputPin) Read() bool { return f.val }

func TestNewDisplay(t *testing.T) {
	spi := &fakeSPI{}
	rst := &fakeOutputPin{}
	dc := &fakeOutputPin{}
	cs := &fakeOutputPin{}
	busy := &fakeInputPin{}

	d := newDisplay(spi, rst, dc, cs, busy)
	if d == nil {
		t.Fatal("newDisplay returned nil")
	}
}

func TestOpenSPIFailsOnMissingDevice(t *testing.T) {
	_, err := openSPI("/dev/spidev_nonexistent", 4_000_000)
	if err == nil {
		t.Fatal("expected error opening non-existent SPI device")
	}
}

func TestNewFailsOnMissingDevice(t *testing.T) {
	_, err := New(Config{
		SPIDevice: "/dev/spidev_nonexistent",
		SPISpeed:  4_000_000,
		PinRST:    17,
		PinDC:     25,
		PinCS:     8,
		PinBUSY:   24,
	})
	if err == nil {
		t.Fatal("expected error for missing SPI device")
	}
}

func TestInitFullSendsReset(t *testing.T) {
	spi := &fakeSPI{}
	busy := &fakeInputPin{val: false} // BUSY=low means ready
	d := newDisplay(spi, &fakeOutputPin{}, &fakeOutputPin{}, &fakeOutputPin{}, busy)

	if err := d.Init(ModeFull); err != nil {
		t.Fatalf("Init: %v", err)
	}
	// After init, at least one command must have been sent
	if len(spi.log) == 0 {
		t.Fatal("expected SPI commands, got none")
	}
	// First meaningful command should be software reset (0x12)
	found := false
	for _, pkt := range spi.log {
		if len(pkt) == 1 && pkt[0] == 0x12 {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected software reset command 0x12 in SPI log")
	}
}

