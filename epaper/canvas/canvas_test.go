package canvas

import (
	"image"
	"image/color"
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

	// Verify DrawText places pixels in the correct column range for font16 (8×16).
	c2 := New(122, 250, Rot0)
	c2.DrawText(0, 0, "A", EmbeddedFont(16), Black)
	// At least one black pixel must be within the glyph bounding box (cols 0..7, rows 0..15).
	hasPixelInBounds := false
	for row := 0; row < 16; row++ {
		for col := 0; col < 8; col++ {
			if c2.At(col, row) == Black {
				hasPixelInBounds = true
			}
		}
	}
	if !hasPixelInBounds {
		t.Error("expected black pixel within 'A' glyph bounding box (cols 0-7, rows 0-15)")
	}
	// No black pixels should appear at column 8 (one past the 8-wide glyph).
	for row := 0; row < 16; row++ {
		if c2.At(8, row) == Black {
			t.Errorf("unexpected black pixel outside 'A' glyph at col=8, row=%d", row)
		}
	}
}

func TestDrawImage(t *testing.T) {
	// Create a simple 4×4 source image with known pixel values
	src := image.NewGray(image.Rect(0, 0, 4, 4))
	// Top-left 2×2: dark (luminance < 128) → black on canvas
	src.SetGray(0, 0, color.Gray{Y: 50})
	src.SetGray(1, 0, color.Gray{Y: 60})
	src.SetGray(0, 1, color.Gray{Y: 70})
	src.SetGray(1, 1, color.Gray{Y: 80})
	// Bottom-right 2×2: bright (luminance >= 128) → white on canvas
	src.SetGray(2, 2, color.Gray{Y: 200})
	src.SetGray(3, 2, color.Gray{Y: 210})
	src.SetGray(2, 3, color.Gray{Y: 220})
	src.SetGray(3, 3, color.Gray{Y: 230})

	c := New(122, 250, Rot0)
	c.Clear()
	c.DrawImage(image.Pt(0, 0), src)

	// Top-left pixels should be black
	if c.At(0, 0) != Black {
		t.Error("expected (0,0) to be black")
	}
	if c.At(1, 1) != Black {
		t.Error("expected (1,1) to be black")
	}
	// Bottom-right bright pixels should be white (canvas was already white there)
	if c.At(2, 2) != White {
		t.Error("expected (2,2) to be white")
	}
	// Default (untouched) pixels should be white
	if c.At(10, 10) != White {
		t.Error("expected (10,10) to be white (untouched)")
	}
}

func TestLoadTTFInvalidData(t *testing.T) {
	_, err := LoadTTF([]byte("not a font"), 16, 72)
	if err == nil {
		t.Error("LoadTTF with invalid data should return an error")
	}
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
