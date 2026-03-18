package canvas

import (
	"image"
	"image/color"
)

// --- dithering helpers ---

// imageLuma returns BT.601 luma for the pixel at (x,y) in [0, 255].
func imageLuma(img image.Image, x, y int) float32 {
	r, g, b, _ := img.At(x, y).RGBA() // 0..65535
	return float32(r>>8)*0.299 + float32(g>>8)*0.587 + float32(b>>8)*0.114
}

// floydSteinberg dithers the luma float32 buffer (w×h) to the canvas at offset (ox,oy).
// buf is consumed (modified in place) so callers must pass a mutable copy.
func floydSteinberg(cv *Canvas, ox, oy, w, h int, buf []float32) {
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			old := buf[y*w+x]
			var newVal float32
			if old >= 128.0 {
				newVal = 255.0
				cv.SetPixel(ox+x, oy+y, White)
			} else {
				cv.SetPixel(ox+x, oy+y, Black)
			}
			quant := old - newVal
			if x+1 < w {
				buf[y*w+x+1] += quant * 7 / 16
			}
			if y+1 < h {
				if x > 0 {
					buf[(y+1)*w+x-1] += quant * 3 / 16
				}
				buf[(y+1)*w+x] += quant * 5 / 16
				if x+1 < w {
					buf[(y+1)*w+x+1] += quant * 1 / 16
				}
			}
		}
	}
}

// boxBlur applies a box blur with the given radius to src and returns a new buffer.
func boxBlur(src []float32, w, h, radius int) []float32 {
	dst := make([]float32, len(src))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var sum float32
			count := 0
			for dy := -radius; dy <= radius; dy++ {
				for dx := -radius; dx <= radius; dx++ {
					nx, ny := x+dx, y+dy
					if nx >= 0 && nx < w && ny >= 0 && ny < h {
						sum += src[ny*w+nx]
						count++
					}
				}
			}
			dst[y*w+x] = sum / float32(count)
		}
	}
	return dst
}

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

// DrawImageScaled renders img scaled to fit r using nearest-neighbour resampling,
// then thresholds to 1-bit (luma < 128 → black). Letterboxed and centered.
// No-op if img bounds or r are empty.
func (c *Canvas) DrawImageScaled(r image.Rectangle, img image.Image) {
	sb := img.Bounds()
	if sb.Empty() || r.Empty() {
		return
	}
	// float64 scale so downscaling works correctly (s < 1)
	s := min(float64(r.Dx())/float64(sb.Dx()), float64(r.Dy())/float64(sb.Dy()))
	dw := int(float64(sb.Dx()) * s)
	dh := int(float64(sb.Dy()) * s)
	// center in r
	offX := r.Min.X + (r.Dx()-dw)/2
	offY := r.Min.Y + (r.Dy()-dh)/2

	for dy := 0; dy < dh; dy++ {
		for dx := 0; dx < dw; dx++ {
			srcX := sb.Min.X + clamp(int(float64(dx)/s), 0, sb.Dx()-1)
			srcY := sb.Min.Y + clamp(int(float64(dy)/s), 0, sb.Dy()-1)
			r32, g32, b32, _ := img.At(srcX, srcY).RGBA()
			luma := (19595*r32 + 38470*g32 + 7471*b32) >> 24
			if luma < 128 {
				c.SetPixel(offX+dx, offY+dy, Black)
			} else {
				c.SetPixel(offX+dx, offY+dy, White)
			}
		}
	}
}

// DrawImageScaledDithered renders img scaled to fit r using Floyd-Steinberg dithering.
// Letterboxed and centered. Produces smoother edges on anti-aliased icons vs simple threshold.
// No-op if img bounds or r are empty.
func (c *Canvas) DrawImageScaledDithered(r image.Rectangle, img image.Image) {
	sb := img.Bounds()
	if sb.Empty() || r.Empty() {
		return
	}
	s := min(float64(r.Dx())/float64(sb.Dx()), float64(r.Dy())/float64(sb.Dy()))
	dw := int(float64(sb.Dx()) * s)
	dh := int(float64(sb.Dy()) * s)
	offX := r.Min.X + (r.Dx()-dw)/2
	offY := r.Min.Y + (r.Dy()-dh)/2

	luma := make([]float32, dw*dh)
	for dy := 0; dy < dh; dy++ {
		for dx := 0; dx < dw; dx++ {
			srcX := sb.Min.X + clamp(int(float64(dx)/s), 0, sb.Dx()-1)
			srcY := sb.Min.Y + clamp(int(float64(dy)/s), 0, sb.Dy()-1)
			luma[dy*dw+dx] = imageLuma(img, srcX, srcY)
		}
	}
	floydSteinberg(c, offX, offY, dw, dh, luma)
}

// DrawImageScaledFill renders img filling r entirely with a blurred background and a
// centered, aspect-ratio-preserved overlay — both dithered to 1-bit.
//
// Technique (from InkyPi pad_image_blur):
//  1. Zoom-crop the image to fill r (background), then box-blur it (radius 6).
//  2. Render the image contained within r (letterboxed) as the sharp overlay.
//
// Useful for photos or screenshots where the source aspect ratio differs from r.
func (c *Canvas) DrawImageScaledFill(r image.Rectangle, img image.Image) {
	sb := img.Bounds()
	if sb.Empty() || r.Empty() {
		return
	}
	rw, rh := r.Dx(), r.Dy()

	// 1. Zoom-fill: scale so the image covers r entirely, then center-crop.
	fillScale := max(float64(rw)/float64(sb.Dx()), float64(rh)/float64(sb.Dy()))
	cropOffX := int((float64(sb.Dx())*fillScale - float64(rw)) / 2)
	cropOffY := int((float64(sb.Dy())*fillScale - float64(rh)) / 2)

	bgLuma := make([]float32, rw*rh)
	for dy := 0; dy < rh; dy++ {
		for dx := 0; dx < rw; dx++ {
			srcX := sb.Min.X + clamp(int(float64(dx+cropOffX)/fillScale), 0, sb.Dx()-1)
			srcY := sb.Min.Y + clamp(int(float64(dy+cropOffY)/fillScale), 0, sb.Dy()-1)
			bgLuma[dy*rw+dx] = imageLuma(img, srcX, srcY)
		}
	}
	floydSteinberg(c, r.Min.X, r.Min.Y, rw, rh, boxBlur(bgLuma, rw, rh, 6))

	// 2. Contained overlay: aspect-ratio-preserved, centered over the blurred background.
	s := min(float64(rw)/float64(sb.Dx()), float64(rh)/float64(sb.Dy()))
	ow := int(float64(sb.Dx()) * s)
	oh := int(float64(sb.Dy()) * s)
	offX := r.Min.X + (rw-ow)/2
	offY := r.Min.Y + (rh-oh)/2

	ovLuma := make([]float32, ow*oh)
	for dy := 0; dy < oh; dy++ {
		for dx := 0; dx < ow; dx++ {
			srcX := sb.Min.X + clamp(int(float64(dx)/s), 0, sb.Dx()-1)
			srcY := sb.Min.Y + clamp(int(float64(dy)/s), 0, sb.Dy()-1)
			ovLuma[dy*ow+dx] = imageLuma(img, srcX, srcY)
		}
	}
	floydSteinberg(c, offX, offY, ow, oh, ovLuma)
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
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
