package gui

import (
	"image"
	"image/color"
	"testing"

	"github.com/oioio-space/oioni/ui/canvas"
)

func TestImageWidget_DrawDoesNotPanic(t *testing.T) {
	src := image.NewGray(image.Rect(0, 0, 10, 10))
	w := NewImageWidget(src)
	w.SetBounds(image.Rect(0, 0, 20, 20))
	c := newTestCanvas()
	w.Draw(c) // must not panic
}

func TestImageWidget_NilSafe(t *testing.T) {
	w := NewImageWidget(nil)
	w.SetBounds(image.Rect(0, 0, 20, 20))
	c := newTestCanvas()
	w.Draw(c) // must not panic
}

func TestImageWidget_SetImageRendersBlack(t *testing.T) {
	src := image.NewGray(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			src.SetGray(x, y, color.Gray{Y: 0}) // all black
		}
	}
	w := NewImageWidget(src)
	w.SetBounds(image.Rect(0, 0, 4, 4))
	// Use Rot0 canvas to avoid physical↔logical coordinate mapping confusion in test
	c := canvas.New(10, 10, canvas.Rot0)
	c.Fill(canvas.White)
	w.Draw(c)
	// At least one pixel in the 4×4 region should be black
	img := c.ToImage()
	found := false
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			if img.GrayAt(x, y).Y == 0 {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected at least one black pixel from all-black source image")
	}
}
