// Package canvas provides a 1-bit drawing surface for e-ink displays.
//
// A Canvas holds a packed byte buffer in physical layout:
// ((physW+7)/8) × physH bytes, MSB-first. Bit 0 represents black;
// bit 1 represents white — matching the EPD hardware convention.
//
// Canvases implement [draw.Image] so any standard-library drawing function
// (draw.Draw, golang.org/x/image/draw, etc.) works directly.
//
// # Rotation
//
// Logical coordinates can be rotated independently of the physical buffer via
// [Rot0], [Rot90], [Rot180], and [Rot270]. The backing buffer is always stored
// in physical layout; only coordinate mapping changes. This lets you mount the
// display in any orientation without touching draw call coordinates.
//
// # Color convention
//
// The package-level [Black] and [White] variables are the two valid colors.
// Any color with luminance below 50% is treated as black; at or above 50% is
// treated as white. Standard library colors (color.Black, color.White,
// color.RGBA, etc.) all work through the color model conversion.
//
// # Typical usage
//
//	// Create a 250×122 canvas (Waveshare 2.13" HAT, landscape via Rot90).
//	c := canvas.New(122, 250, canvas.Rot90)
//
//	// Draw with the standard library (logical coordinates).
//	draw.Draw(c, c.Bounds(), &image.Uniform{canvas.White}, image.Point{}, draw.Src)
//	c.DrawText(4, 4, "Hello!", canvas.EmbeddedFont(16), canvas.Black)
//	c.DrawRect(image.Rect(0, 0, 250, 122), canvas.Black, false)
//
//	// Hand the buffer to the EPD driver.
//	d.Init(epd.ModeFull)
//	d.DisplayBase(c.Bytes())
//
// # Partial updates
//
// [Canvas.SubRegion] extracts a sub-canvas aligned to 8-pixel column
// boundaries and returns the physical rectangle needed by the EPD driver's
// partial-refresh call:
//
//	sub, physRect := c.SubRegion(canvas.PhysicalRect(dirtyLogical))
//	// draw into sub …
//	d.DisplayPartial(sub.Bytes()) // fast update, no ghost flash
//
// # Testing / debugging
//
// [Canvas.ToImage] returns a standard *image.Gray for use with image/png or
// any image-based test assertion. It reads from the physical buffer, so it
// reflects the true bit layout regardless of rotation.
package canvas
