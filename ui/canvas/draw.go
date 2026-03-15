package canvas

import (
	"image"
	"image/color"
)

// DrawImage renders src at logical point pt, thresholding to 1-bit at 50% luminance.
// Pixels with luminance >= 128 are treated as white; below 128 as black.
// The image is clipped by SetPixel to the canvas bounds and active clip region.
func (c *Canvas) DrawImage(pt image.Point, img image.Image) {
	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			col := img.At(x, y)
			r16, g16, b16, _ := col.RGBA()
			luma := (3*r16 + 6*g16 + b16) / 10 / 256
			dx := pt.X + (x - bounds.Min.X)
			dy := pt.Y + (y - bounds.Min.Y)
			if luma < 128 {
				c.SetPixel(dx, dy, Black)
			} else {
				c.SetPixel(dx, dy, White)
			}
		}
	}
}

// DrawText renders text at logical (x, y) using font f and color col.
// Glyphs are drawn left-to-right; x advances by the glyph width after each character.
// Characters whose glyphs extend beyond the canvas are clipped by SetPixel.
func (c *Canvas) DrawText(x, y int, text string, f Font, col color.Color) {
	cx := x
	for _, r := range text {
		data, gw, gh := f.Glyph(r)
		if data == nil {
			// Unknown rune: advance by half the line height.
			cx += f.LineHeight() / 2
			continue
		}
		stride := (gw + 7) / 8
		for row := 0; row < gh; row++ {
			for bit := 0; bit < gw; bit++ {
				byteIdx := row*stride + bit/8
				bitIdx := uint(7 - bit%8)
				if (data[byteIdx]>>bitIdx)&1 == 1 {
					c.SetPixel(cx+bit, y+row, col)
				}
			}
		}
		cx += gw
	}
}

func (c *Canvas) DrawRect(r image.Rectangle, col color.Color, filled bool) {
	if filled {
		for y := r.Min.Y; y < r.Max.Y; y++ {
			for x := r.Min.X; x < r.Max.X; x++ {
				c.SetPixel(x, y, col)
			}
		}
		return
	}
	// Top and bottom edges
	for x := r.Min.X; x < r.Max.X; x++ {
		c.SetPixel(x, r.Min.Y, col)
		c.SetPixel(x, r.Max.Y-1, col)
	}
	// Left and right edges
	for y := r.Min.Y; y < r.Max.Y; y++ {
		c.SetPixel(r.Min.X, y, col)
		c.SetPixel(r.Max.X-1, y, col)
	}
}

func (c *Canvas) DrawLine(x0, y0, x1, y1 int, col color.Color) {
	// Bresenham's line algorithm
	dx := x1 - x0
	if dx < 0 {
		dx = -dx
	}
	dy := y1 - y0
	if dy < 0 {
		dy = -dy
	}
	sx, sy := -1, -1
	if x0 < x1 {
		sx = 1
	}
	if y0 < y1 {
		sy = 1
	}
	err := dx - dy
	for {
		c.SetPixel(x0, y0, col)
		if x0 == x1 && y0 == y1 {
			break
		}
		e2 := 2 * err
		if e2 > -dy {
			err -= dy
			x0 += sx
		}
		if e2 < dx {
			err += dx
			y0 += sy
		}
	}
}

func (c *Canvas) DrawCircle(cx, cy, radius int, col color.Color, filled bool) {
	// Midpoint circle algorithm
	x, y := 0, radius
	d := 1 - radius
	for x <= y {
		if filled {
			for i := cx - x; i <= cx+x; i++ {
				c.SetPixel(i, cy+y, col)
				c.SetPixel(i, cy-y, col)
			}
			for i := cx - y; i <= cx+y; i++ {
				c.SetPixel(i, cy+x, col)
				c.SetPixel(i, cy-x, col)
			}
		} else {
			for _, p := range [][2]int{
				{cx + x, cy + y}, {cx - x, cy + y},
				{cx + x, cy - y}, {cx - x, cy - y},
				{cx + y, cy + x}, {cx - y, cy + x},
				{cx + y, cy - x}, {cx - y, cy - x},
			} {
				c.SetPixel(p[0], p[1], col)
			}
		}
		if d < 0 {
			d += 2*x + 3
		} else {
			d += 2*(x-y) + 5
			y--
		}
		x++
	}
}
