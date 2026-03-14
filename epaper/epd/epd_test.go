// epaper/epd/epd_test.go
package epd

import (
	"fmt"
	"image"
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

// failingSPI returns an error on every Tx call.
type failingSPI struct{}

func (f *failingSPI) Tx(w []byte) error { return fmt.Errorf("spi: bus error") }

func TestInitReturnsErrorOnSPIFailure(t *testing.T) {
	d := newDisplay(&failingSPI{}, &fakeOutputPin{}, &fakeOutputPin{}, &fakeOutputPin{}, &fakeInputPin{val: false})
	err := d.Init(ModeFull)
	if err == nil {
		t.Fatal("expected error when SPI fails, got nil")
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

func TestDisplayFullSendsBuffer(t *testing.T) {
	spi := &fakeSPI{}
	busy := &fakeInputPin{val: false}
	d := newDisplay(spi, &fakeOutputPin{}, &fakeOutputPin{}, &fakeOutputPin{}, busy)
	d.Init(ModeFull)
	spi.log = nil // reset log after init

	buf := make([]byte, BufferSize)
	buf[0] = 0xAB
	if err := d.DisplayFull(buf); err != nil {
		t.Fatal(err)
	}
	// The data byte 0xAB must appear in the SPI log
	found := false
	for _, pkt := range spi.log {
		for _, b := range pkt {
			if b == 0xAB {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected buffer content 0xAB in SPI log")
	}
}

func TestSleepSendsCommand(t *testing.T) {
	spi := &fakeSPI{}
	busy := &fakeInputPin{val: false}
	d := newDisplay(spi, &fakeOutputPin{}, &fakeOutputPin{}, &fakeOutputPin{}, busy)
	spi.log = nil
	d.Sleep()
	found := false
	for _, pkt := range spi.log {
		if len(pkt) == 1 && pkt[0] == 0x10 {
			found = true
		}
	}
	if !found {
		t.Error("expected deep sleep command 0x10")
	}
}

func TestCloseCallsClosers(t *testing.T) {
	closed := 0
	closer := func() error { closed++; return nil }
	d := newDisplay(&fakeSPI{}, &fakeOutputPin{}, &fakeOutputPin{}, &fakeOutputPin{}, &fakeInputPin{})
	d.closers = []func() error{closer, closer, closer}
	if err := d.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if closed != 3 {
		t.Errorf("expected 3 closers called, got %d", closed)
	}
}

func TestDisplayPartialSendsRegionBuffer(t *testing.T) {
	spi := &fakeSPI{}
	busy := &fakeInputPin{val: false}
	d := newDisplay(spi, &fakeOutputPin{}, &fakeOutputPin{}, &fakeOutputPin{}, busy)
	d.Init(ModePartial)
	spi.log = nil

	// 10×10 pixel region at (0,0): ceil(10/8)=2 bytes/row × 10 rows = 20 bytes
	r := image.Rect(0, 0, 10, 10)
	buf := make([]byte, 2*10)
	buf[0] = 0xCD
	if err := d.DisplayPartial(r, buf); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, pkt := range spi.log {
		for _, b := range pkt {
			if b == 0xCD {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected partial buffer content 0xCD in SPI log")
	}
}
