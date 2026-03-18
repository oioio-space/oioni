package gui

import (
	"image"
	"image/color"
	"testing"

	"github.com/oioio-space/oioni/ui/canvas"
	"github.com/oioio-space/oioni/drivers/epd"
)

func TestDrawRoundedRect_Outline_CornerCut(t *testing.T) {
	c := canvas.New(epd.Width, epd.Height, canvas.Rot90)
	r := image.Rect(10, 10, 30, 25)
	DrawRoundedRect(c, r, 4, false, canvas.Black)
	// Top-left corner pixel at (10,10) should be white (cut by radius)
	if c.At(10, 10) == (color.Gray{Y: 0}) {
		t.Error("corner pixel (10,10) should be white (cut by radius=4)")
	}
	// Middle top edge should be black (outline)
	if c.At(20, 10) != (color.Gray{Y: 0}) {
		t.Error("top edge pixel (20,10) should be black (outline)")
	}
}

func TestDrawRoundedRect_ZeroRadius_AllCorners(t *testing.T) {
	c := canvas.New(epd.Width, epd.Height, canvas.Rot90)
	r := image.Rect(5, 5, 20, 15)
	DrawRoundedRect(c, r, 0, false, canvas.Black)
	// With radius=0, all 4 corners must be black
	for _, pt := range []image.Point{{5, 5}, {19, 5}, {5, 14}, {19, 14}} {
		if c.At(pt.X, pt.Y) != (color.Gray{Y: 0}) {
			t.Errorf("corner %v should be black with radius=0", pt)
		}
	}
}

func TestDrawRoundedRect_Filled_Center(t *testing.T) {
	c := canvas.New(epd.Width, epd.Height, canvas.Rot90)
	r := image.Rect(10, 10, 30, 25)
	DrawRoundedRect(c, r, 0, true, canvas.Black)
	// Center pixel should be black (filled)
	if c.At(20, 17) != (color.Gray{Y: 0}) {
		t.Error("center pixel should be black when filled")
	}
}

func TestDrawRoundedRect_Filled_WithRadius(t *testing.T) {
	c := canvas.New(epd.Width, epd.Height, canvas.Rot90)
	r := image.Rect(10, 10, 40, 30)
	DrawRoundedRect(c, r, 4, true, canvas.Black)
	// Center of rect should be black (filled)
	if c.At(25, 20) != (color.Gray{Y: 0}) {
		t.Error("center pixel should be black (filled with radius=4)")
	}
	// Corner at (10,10) should be white — cut by radius=4
	// Top-left corner center is at (14,14); (10,10) is outside the circle
	if c.At(10, 10) == (color.Gray{Y: 0}) {
		t.Error("corner pixel (10,10) should be white (cut by radius=4)")
	}
}

func TestDrawRoundedRect_Empty_NoPanic(t *testing.T) {
	c := canvas.New(epd.Width, epd.Height, canvas.Rot90)
	// Empty rect must not panic
	DrawRoundedRect(c, image.Rectangle{}, 4, false, canvas.Black)
}
