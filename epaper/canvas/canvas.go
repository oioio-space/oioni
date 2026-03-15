package canvas

import (
	"image"
	"image/color"
)

// Black and White are the two display colors.
// 0=black, 1=white in the 1-bit buffer (e-ink convention).
var (
	Black color.Color = color.Gray{Y: 0}
	White color.Color = color.Gray{Y: 255}
)

// Rotation is the logical coordinate orientation for draw calls.
type Rotation int

const (
	Rot0   Rotation = 0
	Rot90  Rotation = 90
	Rot180 Rotation = 180
	Rot270 Rotation = 270
)

// Canvas is a 1-bit drawing surface implementing draw.Image.
// The backing buffer is always in physical layout: ((physW+7)/8) × physH bytes.
// Convention: bit=0 → black, bit=1 → white.
type Canvas struct {
	buf          []byte
	physW, physH int
	rot          Rotation
	clip         image.Rectangle
}

// New creates a canvas with physical size physW×physH, initially white.
func New(physW, physH int, rot Rotation) *Canvas {
	stride := (physW + 7) / 8
	buf := make([]byte, stride*physH)
	for i := range buf {
		buf[i] = 0xFF // all white
	}
	c := &Canvas{buf: buf, physW: physW, physH: physH, rot: rot}
	lw, lh := c.logicalSize()
	c.clip = image.Rect(0, 0, lw, lh)
	return c
}

// logicalSize returns canvas dimensions in logical (rotated) coordinates.
func (c *Canvas) logicalSize() (w, h int) {
	if c.rot == Rot90 || c.rot == Rot270 {
		return c.physH, c.physW
	}
	return c.physW, c.physH
}

// toPhysical converts logical (x,y) to physical (px,py).
func (c *Canvas) toPhysical(x, y int) (px, py int) {
	switch c.rot {
	case Rot90:
		return c.physW - 1 - y, x
	case Rot180:
		return c.physW - 1 - x, c.physH - 1 - y
	case Rot270:
		return y, c.physH - 1 - x
	default: // Rot0
		return x, y
	}
}

// --- draw.Image interface ---

func (c *Canvas) ColorModel() color.Model { return color.GrayModel }

func (c *Canvas) Bounds() image.Rectangle {
	w, h := c.logicalSize()
	return image.Rect(0, 0, w, h)
}

func (c *Canvas) At(x, y int) color.Color {
	px, py := c.toPhysical(x, y)
	stride := (c.physW + 7) / 8
	idx := py*stride + px/8
	if idx < 0 || idx >= len(c.buf) {
		return White
	}
	if (c.buf[idx]>>(7-uint(px%8)))&1 == 1 {
		return White
	}
	return Black
}

func (c *Canvas) Set(x, y int, col color.Color) { c.SetPixel(x, y, col) }

// --- Public API ---

func isBlack(col color.Color) bool {
	gray, _, _, _ := col.RGBA()
	return gray < 0x8000
}

// SetPixel draws a single pixel at logical (x,y). Clipped silently.
func (c *Canvas) SetPixel(x, y int, col color.Color) {
	lw, lh := c.logicalSize()
	if x < 0 || y < 0 || x >= lw || y >= lh {
		return
	}
	if !image.Pt(x, y).In(c.clip) {
		return
	}
	px, py := c.toPhysical(x, y)
	stride := (c.physW + 7) / 8
	idx := py*stride + px/8
	bit := uint(7 - px%8)
	if isBlack(col) {
		c.buf[idx] &^= 1 << bit
	} else {
		c.buf[idx] |= 1 << bit
	}
}

// Fill sets all pixels to col.
func (c *Canvas) Fill(col color.Color) {
	fill := byte(0xFF)
	if isBlack(col) {
		fill = 0x00
	}
	for i := range c.buf {
		c.buf[i] = fill
	}
}

// Clear fills the canvas white.
func (c *Canvas) Clear() { c.Fill(White) }

// SetClip sets the clipping rectangle (logical coordinates).
func (c *Canvas) SetClip(r image.Rectangle) { c.clip = r }

// ClearClip removes the clipping rectangle.
func (c *Canvas) ClearClip() {
	lw, lh := c.logicalSize()
	c.clip = image.Rect(0, 0, lw, lh)
}

// SetRotation changes the logical coordinate mapping for future draw calls.
// The backing buffer is NOT transformed; only coordinate remapping changes.
func (c *Canvas) SetRotation(r Rotation) {
	c.rot = r
	c.ClearClip()
}

// Bytes returns a COPY of the physical backing buffer.
// Safe to pass to epd.DisplayFull concurrently with ongoing draw calls on a different canvas.
func (c *Canvas) Bytes() []byte {
	out := make([]byte, len(c.buf))
	copy(out, c.buf)
	return out
}

// PhysicalRect converts a logical rectangle to physical coordinates.
func (c *Canvas) PhysicalRect(r image.Rectangle) image.Rectangle {
	p1x, p1y := c.toPhysical(r.Min.X, r.Min.Y)
	p2x, p2y := c.toPhysical(r.Max.X-1, r.Max.Y-1)
	if p1x > p2x {
		p1x, p2x = p2x, p1x
	}
	if p1y > p2y {
		p1y, p2y = p2y, p1y
	}
	return image.Rect(p1x, p1y, p2x+1, p2y+1)
}

// SubRegion extracts a sub-canvas from physical coordinates r.
// Returns the sub-canvas and the physical rectangle for epd.DisplayPartial.
func (c *Canvas) SubRegion(r image.Rectangle) (*Canvas, image.Rectangle) {
	r = r.Intersect(image.Rect(0, 0, c.physW, c.physH))
	stride := (c.physW + 7) / 8
	subStride := (r.Dx() + 7) / 8
	subBuf := make([]byte, subStride*r.Dy())
	for row := 0; row < r.Dy(); row++ {
		for col := 0; col < r.Dx(); col++ {
			srcIdx := (r.Min.Y+row)*stride + (r.Min.X+col)/8
			srcBit := uint(7 - (r.Min.X+col)%8)
			bit := (c.buf[srcIdx] >> srcBit) & 1
			dstIdx := row*subStride + col/8
			dstBit := uint(7 - col%8)
			subBuf[dstIdx] = (subBuf[dstIdx] &^ (1 << dstBit)) | (bit << dstBit)
		}
	}
	sub := &Canvas{buf: subBuf, physW: r.Dx(), physH: r.Dy(), rot: Rot0}
	sub.clip = image.Rect(0, 0, r.Dx(), r.Dy())
	return sub, r
}

// ToImage converts the canvas to *image.Gray (8 bits/pixel).
// NOT concurrent-safe: caller must not draw concurrently.
func (c *Canvas) ToImage() *image.Gray {
	img := image.NewGray(image.Rect(0, 0, c.physW, c.physH))
	stride := (c.physW + 7) / 8
	for y := 0; y < c.physH; y++ {
		for x := 0; x < c.physW; x++ {
			idx := y*stride + x/8
			bit := uint(7 - x%8)
			if (c.buf[idx]>>bit)&1 == 1 {
				img.SetGray(x, y, color.Gray{Y: 255})
			} else {
				img.SetGray(x, y, color.Gray{Y: 0})
			}
		}
	}
	return img
}
