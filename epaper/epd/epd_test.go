// epaper/epd/epd_test.go
package epd

import (
	"fmt"
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

func TestDisplayPartialSendsFullBuffer(t *testing.T) {
	spi := &fakeSPI{}
	busy := &fakeInputPin{val: false}
	d := newDisplay(spi, &fakeOutputPin{}, &fakeOutputPin{}, &fakeOutputPin{}, busy)
	spi.log = nil

	buf := make([]byte, BufferSize)
	buf[0] = 0xCD
	if err := d.DisplayPartial(buf); err != nil {
		t.Fatal(err)
	}
	// 0xCD must appear in the SPI log
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
	// partial update sequence byte 0xFF must be sent
	foundFF := false
	for _, pkt := range spi.log {
		if len(pkt) == 1 && pkt[0] == 0xFF {
			foundFF = true
		}
	}
	if !foundFF {
		t.Error("expected partial update sequence 0xFF in SPI log")
	}
}

func TestDisplayBaseSendsToRAMBanks(t *testing.T) {
	spi := &fakeSPI{}
	busy := &fakeInputPin{val: false}
	d := newDisplay(spi, &fakeOutputPin{}, &fakeOutputPin{}, &fakeOutputPin{}, busy)
	spi.log = nil

	buf := make([]byte, BufferSize)
	buf[0] = 0xEF
	if err := d.DisplayBase(buf); err != nil {
		t.Fatal(err)
	}
	// Commands 0x24 (new RAM) and 0x26 (old RAM reference) must both appear
	var cmds []byte
	for _, pkt := range spi.log {
		if len(pkt) == 1 {
			cmds = append(cmds, pkt[0])
		}
	}
	has24, has26 := false, false
	for _, c := range cmds {
		if c == 0x24 {
			has24 = true
		}
		if c == 0x26 {
			has26 = true
		}
	}
	if !has24 {
		t.Error("expected write RAM command 0x24 for new frame")
	}
	if !has26 {
		t.Error("expected write RAM command 0x26 for reference frame")
	}
	// 0xEF must appear in the SPI log (buffer content)
	found := false
	for _, pkt := range spi.log {
		for _, b := range pkt {
			if b == 0xEF {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected buffer content 0xEF in SPI log")
	}
}
