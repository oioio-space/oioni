// ui/gui/draw.go — shared drawing primitives for gui widgets
package gui

import (
	"image"
	"image/color"
	"math"

	"github.com/oioio-space/oioni/ui/canvas"
)

// DrawRoundedRect draws a rounded rectangle outline (fill=false) or filled shape (fill=true).
// radius is the corner radius in pixels. col is thresholded to 1-bit by the canvas (any non-black color renders as white).
// Uses Bresenham midpoint circle for corners, straight segments between corners.
func DrawRoundedRect(c *canvas.Canvas, r image.Rectangle, radius int, fill bool, col color.Color) {
	if r.Empty() {
		return
	}
	if radius < 0 {
		radius = 0
	}
	// Clamp radius so corners don't overlap
	maxR := min(r.Dx(), r.Dy()) / 2
	if radius > maxR {
		radius = maxR
	}

	x0, y0 := r.Min.X, r.Min.Y
	x1, y1 := r.Max.X-1, r.Max.Y-1

	// Corner centers
	tlX, tlY := x0+radius, y0+radius
	trX, trY := x1-radius, y0+radius
	blX, blY := x0+radius, y1-radius
	brX, brY := x1-radius, y1-radius

	if fill {
		for y := y0; y <= y1; y++ {
			var xLeft, xRight int
			switch {
			case y < tlY: // top corner band
				dy := float64(tlY - y)
				dx := int(math.Sqrt(float64(radius*radius) - dy*dy))
				xLeft = tlX - dx
				xRight = trX + dx
			case y > blY: // bottom corner band
				dy := float64(y - blY)
				dx := int(math.Sqrt(float64(radius*radius) - dy*dy))
				xLeft = blX - dx
				xRight = brX + dx
			default: // middle band
				xLeft = x0
				xRight = x1
			}
			for x := xLeft; x <= xRight; x++ {
				c.SetPixel(x, y, col)
			}
		}
	} else {
		// Top and bottom straight edges
		for x := tlX; x <= trX; x++ {
			c.SetPixel(x, y0, col)
			c.SetPixel(x, y1, col)
		}
		// Left and right straight edges
		for y := tlY; y <= blY; y++ {
			c.SetPixel(x0, y, col)
			c.SetPixel(x1, y, col)
		}
		// Corner arcs
		if radius > 0 {
			drawCornerArc(c, tlX, tlY, radius, 2, col) // top-left
			drawCornerArc(c, trX, trY, radius, 1, col) // top-right
			drawCornerArc(c, blX, blY, radius, 3, col) // bottom-left
			drawCornerArc(c, brX, brY, radius, 4, col) // bottom-right
		}
	}
}

// drawCornerArc draws one quadrant of a circle arc using Bresenham's midpoint algorithm.
// quad: 1=top-right (+x,-y), 2=top-left (-x,-y), 3=bottom-left (-x,+y), 4=bottom-right (+x,+y).
func drawCornerArc(c *canvas.Canvas, cx, cy, r, quad int, col color.Color) {
	x, y := 0, r
	d := 1 - r
	for x <= y {
		switch quad {
		case 1:
			c.SetPixel(cx+x, cy-y, col)
			c.SetPixel(cx+y, cy-x, col)
		case 2:
			c.SetPixel(cx-x, cy-y, col)
			c.SetPixel(cx-y, cy-x, col)
		case 3:
			c.SetPixel(cx-x, cy+y, col)
			c.SetPixel(cx-y, cy+x, col)
		case 4:
			c.SetPixel(cx+x, cy+y, col)
			c.SetPixel(cx+y, cy+x, col)
		}
		x++
		if d < 0 {
			d += 2*x + 1
		} else {
			y--
			d += 2*(x-y) + 1
		}
	}
}
