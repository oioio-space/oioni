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

func TestClipInitWithRotation(t *testing.T) {
	// When created with Rot90, logical size is 250×122, not 122×250
	c := New(122, 250, Rot90)
	expectedClip := image.Rect(0, 0, 250, 122) // logical size
	if c.clip != expectedClip {
		t.Errorf("BUG: clip initialized with physical dims instead of logical. Got %v, expected %v", c.clip, expectedClip)
	}
}

func TestDrawRect(t *testing.T) {
	c := New(122, 250, Rot0)
	c.Clear()
	c.DrawRect(image.Rect(10, 10, 20, 20), Black, false)
	// Top-left corner pixel must be black
	if c.At(10, 10) != Black {
		t.Error("expected pixel (10,10) to be black")
	}
	// Interior pixel must be white (outline only)
	if c.At(15, 15) != White {
		t.Error("expected interior pixel (15,15) to be white")
	}
}

func TestDrawLine(t *testing.T) {
	c := New(122, 250, Rot0)
	c.Clear()
	c.DrawLine(0, 0, 10, 0, Black)
	for x := 0; x <= 10; x++ {
		if c.At(x, 0) != Black {
			t.Errorf("expected pixel (%d,0) to be black", x)
		}
	}
}

func TestDrawText(t *testing.T) {
	c := New(122, 250, Rot0)
	c.Clear()
	f := EmbeddedFont(16)
	if f == nil {
		t.Fatal("EmbeddedFont(16) returned nil")
	}
	// Font metrics
	if f.LineHeight() <= 0 {
		t.Errorf("LineHeight() = %d, want > 0", f.LineHeight())
	}
	// Glyph for 'A' must have pixels
	data, w, h := f.Glyph('A')
	if data == nil {
		t.Fatal("Glyph('A') returned nil data")
	}
	if w <= 0 || h <= 0 {
		t.Errorf("Glyph('A') dimensions = %d×%d, want > 0", w, h)
	}
	// Drawing 'A' must produce black pixels
	c.DrawText(0, 0, "A", f, Black)
	buf := c.Bytes()
	hasBlack := false
	for _, b := range buf {
		if b != 0xFF {
			hasBlack = true
			break
		}
	}
	if !hasBlack {
		t.Error("expected at least one black pixel after DrawText('A')")
	}
	// Unknown rune should not panic
	c.DrawText(0, 0, "\x01", f, Black)
}

func TestEmbeddedFontSizes(t *testing.T) {
	// All 5 sizes must return non-nil fonts with valid metrics and glyph data.
	for _, size := range []int{8, 12, 16, 20, 24} {
		f := EmbeddedFont(size)
		if f == nil {
			t.Errorf("EmbeddedFont(%d) returned nil", size)
			continue
		}
		if f.LineHeight() <= 0 {
			t.Errorf("EmbeddedFont(%d).LineHeight() = %d, want > 0", size, f.LineHeight())
		}
		data, w, h := f.Glyph('A')
		if data == nil {
			t.Errorf("EmbeddedFont(%d).Glyph('A') returned nil data", size)
			continue
		}
		if w <= 0 || h <= 0 {
			t.Errorf("EmbeddedFont(%d).Glyph('A') dimensions = %dx%d, want > 0", size, w, h)
		}
	}
}
