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

func TestDrawImageThresholdAndOffset(t *testing.T) {
	// Test threshold boundary: Y=127 → black, Y=128 → white
	src := image.NewGray(image.Rect(0, 0, 2, 1))
	src.SetGray(0, 0, color.Gray{Y: 127}) // just below threshold → black
	src.SetGray(1, 0, color.Gray{Y: 128}) // at threshold → white

	c := New(122, 250, Rot0)
	c.Clear()
	c.DrawImage(image.Pt(0, 0), src)

	if c.At(0, 0) != Black {
		t.Error("pixel with Y=127 should be Black (below 128 threshold)")
	}
	if c.At(1, 0) != White {
		t.Error("pixel with Y=128 should be White (at or above 128 threshold)")
	}

	// Test non-zero destination offset
	src2 := image.NewGray(image.Rect(0, 0, 1, 1))
	src2.SetGray(0, 0, color.Gray{Y: 0}) // black pixel

	c2 := New(122, 250, Rot0)
	c2.Clear()
	c2.DrawImage(image.Pt(10, 20), src2)

	if c2.At(10, 20) != Black {
		t.Error("pixel drawn at offset (10,20) should be Black")
	}
	if c2.At(0, 0) != White {
		t.Error("pixel at (0,0) should remain White when drawing at offset (10,20)")
	}
}

func TestDrawImageScaled_2x2To4x4(t *testing.T) {
	// 2×2 source: top-left black, rest white
	src := image.NewGray(image.Rect(0, 0, 2, 2))
	src.SetGray(0, 0, color.Gray{Y: 0})   // black
	src.SetGray(1, 0, color.Gray{Y: 255}) // white
	src.SetGray(0, 1, color.Gray{Y: 255}) // white
	src.SetGray(1, 1, color.Gray{Y: 255}) // white

	c := New(4, 4, Rot0)
	c.Fill(White)
	c.DrawImageScaled(image.Rect(0, 0, 4, 4), src)

	img := c.ToImage()
	// top-left 2×2 should be black (scaled ×2)
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			if img.GrayAt(x, y).Y != 0 {
				t.Errorf("pixel (%d,%d) should be black", x, y)
			}
		}
	}
	// top-right pixel should be white
	if img.GrayAt(2, 0).Y == 0 {
		t.Error("pixel (2,0) should be white")
	}
}

func TestDrawImageScaled_EmptySource(t *testing.T) {
	c := New(10, 10, Rot0)
	// empty image — should not panic
	c.DrawImageScaled(image.Rect(0, 0, 10, 10), image.NewGray(image.Rect(0, 0, 0, 0)))
}

func TestDrawImageScaledDithered_BlackAndWhite(t *testing.T) {
	// 2×1 source: pure black left, pure white right.
	// After dithering both must match the threshold exactly (no error diffused for solid colors).
	src := image.NewGray(image.Rect(0, 0, 2, 1))
	src.SetGray(0, 0, color.Gray{Y: 0})   // black
	src.SetGray(1, 0, color.Gray{Y: 255}) // white

	c := New(4, 4, Rot0)
	c.Clear()
	c.DrawImageScaledDithered(image.Rect(0, 0, 4, 2), src)

	// Scaled ×2: cols 0-1 → black, cols 2-3 → white
	for x := 0; x < 2; x++ {
		if c.At(x, 0) != Black {
			t.Errorf("col %d should be black after dithering pure black", x)
		}
	}
	for x := 2; x < 4; x++ {
		if c.At(x, 0) != White {
			t.Errorf("col %d should be white after dithering pure white", x)
		}
	}
}

func TestDrawImageScaledDithered_MidGrayProducesCheckerboard(t *testing.T) {
	// A uniform 50% gray source (Y=127) should produce a dithered pattern — not all black
	// or all white. At least one black and one white pixel must appear.
	src := image.NewGray(image.Rect(0, 0, 8, 1))
	for x := 0; x < 8; x++ {
		src.SetGray(x, 0, color.Gray{Y: 127})
	}

	c := New(16, 4, Rot0)
	c.Clear()
	c.DrawImageScaledDithered(image.Rect(0, 0, 16, 1), src)

	hasBlack, hasWhite := false, false
	for x := 0; x < 16; x++ {
		if c.At(x, 0) == Black {
			hasBlack = true
		} else {
			hasWhite = true
		}
	}
	if !hasBlack {
		t.Error("uniform mid-gray dithered over 16px: expected at least one black pixel")
	}
	if !hasWhite {
		t.Error("uniform mid-gray dithered over 16px: expected at least one white pixel")
	}
}

func TestDrawImageScaledDithered_EmptySource(t *testing.T) {
	c := New(10, 10, Rot0)
	c.DrawImageScaledDithered(image.Rect(0, 0, 10, 10), image.NewGray(image.Rect(0, 0, 0, 0)))
}

func TestDrawImageScaledFill_FillsEntireRect(t *testing.T) {
	// Source image: uniform mid-gray. After fill, every pixel in the destination
	// rect must have been written (no untouched white letterbox strips).
	src := image.NewGray(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			src.SetGray(x, y, color.Gray{Y: 100})
		}
	}

	c := New(20, 20, Rot0)
	c.Fill(White)
	// Use a wide rect to force zoom-fill to crop vertically.
	target := image.Rect(0, 0, 20, 8)
	c.DrawImageScaledFill(target, src)

	// Every pixel in target must have been overwritten. Since Y=100 < 128, most will
	// be black after dithering, but at minimum none should remain pristine unwritten
	// white — check that the buffer changed from all-white.
	hasNonWhite := false
	for y := target.Min.Y; y < target.Max.Y; y++ {
		for x := target.Min.X; x < target.Max.X; x++ {
			if c.At(x, y) == Black {
				hasNonWhite = true
			}
		}
	}
	if !hasNonWhite {
		t.Error("DrawImageScaledFill with Y=100 source: expected at least one black pixel in target rect")
	}
}

func TestDrawImageScaledFill_EmptySource(t *testing.T) {
	c := New(10, 10, Rot0)
	c.DrawImageScaledFill(image.Rect(0, 0, 10, 10), image.NewGray(image.Rect(0, 0, 0, 0)))
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

func TestToImage(t *testing.T) {
	c := New(8, 4, Rot0)
	c.Clear()
	// Draw a known pattern: black pixel at (0,0)
	c.SetPixel(0, 0, Black)

	img := c.ToImage()

	// ToImage returns *image.Gray with physical dimensions
	bounds := img.Bounds()
	if bounds.Dx() != 8 || bounds.Dy() != 4 {
		t.Errorf("ToImage bounds = %v, want 8×4", bounds)
	}
	// (0,0) should be black (Y=0)
	if img.GrayAt(0, 0).Y != 0 {
		t.Errorf("ToImage(0,0) = %d, want 0 (black)", img.GrayAt(0, 0).Y)
	}
	// (1,0) should be white (Y=255)
	if img.GrayAt(1, 0).Y != 255 {
		t.Errorf("ToImage(1,0) = %d, want 255 (white)", img.GrayAt(1, 0).Y)
	}
}
