package canvas

import (
	"image"
	"testing"
)

func TestNewCanvas(t *testing.T) {
	c := New(122, 250, Rot0)
	if c == nil {
		t.Fatal("New returned nil")
	}
	if c.Bounds() != (image.Rectangle{Max: image.Point{X: 122, Y: 250}}) {
		t.Errorf("unexpected bounds: %v", c.Bounds())
	}
}

func TestSetPixelAndBytes(t *testing.T) {
	c := New(122, 250, Rot0)
	c.Clear()

	c.SetPixel(0, 0, Black)
	buf := c.Bytes()
	if len(buf) != 4000 {
		t.Fatalf("expected 4000 bytes, got %d", len(buf))
	}
	// Pixel (0,0) is bit 7 of byte 0. Black=0, after Clear all bits=1 (white).
	if buf[0]&0x80 != 0 {
		t.Errorf("expected bit7 of byte0 = 0 (black), got 0x%02X", buf[0])
	}
}

func TestFill(t *testing.T) {
	c := New(122, 250, Rot0)
	c.Fill(Black)
	buf := c.Bytes()
	for i, b := range buf {
		if b != 0x00 {
			t.Fatalf("byte %d: expected 0x00 (black), got 0x%02X", i, b)
		}
	}
}
