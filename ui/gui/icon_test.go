package gui

import (
	"image"
	"image/color"
	"testing"

	"github.com/oioio-space/oioni/ui/canvas"
)

func TestNewBitmapIcon_DrawsBlack(t *testing.T) {
	// 8×8 bitmap: all black (0x00 = all bits 0 = all black in e-ink convention)
	data := make([]byte, 8) // 8 rows × 1 byte/row for 8px wide
	ic := NewBitmapIcon(data, 8, 8)
	w, h := ic.Size()
	if w != 8 || h != 8 {
		t.Fatalf("Size() = (%d,%d), want (8,8)", w, h)
	}
	c := canvas.New(8, 8, canvas.Rot0)
	c.Fill(canvas.White)
	ic.Draw(c, image.Rect(0, 0, 8, 8))
	img := c.ToImage()
	if img.GrayAt(0, 0).Y != 0 {
		t.Error("expected black pixel at (0,0)")
	}
}

func TestNewImageIcon_DrawsThresholded(t *testing.T) {
	src := image.NewGray(image.Rect(0, 0, 4, 4))
	src.SetGray(0, 0, color.Gray{Y: 0}) // black
	ic := NewImageIcon(src)
	c := canvas.New(4, 4, canvas.Rot0)
	c.Fill(canvas.White)
	ic.Draw(c, image.Rect(0, 0, 4, 4))
	img := c.ToImage()
	if img.GrayAt(0, 0).Y != 0 {
		t.Error("expected black pixel from image icon")
	}
}
