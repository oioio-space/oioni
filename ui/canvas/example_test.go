package canvas_test

import (
	"fmt"
	"image"

	"github.com/oioio-space/oioni/ui/canvas"
)

// ExampleNew demonstrates creating a canvas, filling it, drawing primitives,
// reading back a pixel, and extracting a sub-region for partial EPD updates.
func ExampleNew() {
	// Create a canvas with physical size 122×250 (Waveshare 2.13" HAT).
	// Rot90 maps logical (0,0) to the top-left corner in landscape orientation.
	c := canvas.New(122, 250, canvas.Rot90)

	// The logical bounds are 250×122 after the 90-degree rotation.
	bounds := c.Bounds()
	fmt.Println(bounds.Dx(), bounds.Dy())

	// Fill the canvas white, then draw a black border and a label.
	c.Fill(canvas.White)
	c.DrawRect(image.Rect(0, 0, 250, 122), canvas.Black, false)
	c.DrawText(4, 4, "hello", canvas.EmbeddedFont(12), canvas.Black)

	// Bytes() returns a copy of the 4000-byte physical buffer (safe to pass
	// to epd.DisplayBase concurrently with ongoing draws on a different canvas).
	buf := c.Bytes()
	fmt.Println(len(buf))

	// SubRegion extracts a sub-canvas aligned to 8-pixel column boundaries.
	// The returned physical rectangle is what epd.DisplayPartial needs.
	sub, physRect := c.SubRegion(image.Rect(0, 0, 122, 32))
	fmt.Println(physRect.Empty())   // false: there are pixels to send
	fmt.Println(len(sub.Bytes()) > 0) // true: sub-canvas has content

	// Output:
	// 250 122
	// 4000
	// false
	// true
}
