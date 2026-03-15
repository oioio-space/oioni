package gui

import (
	"image"

	"github.com/oioio-space/oioni/ui/canvas"
)

// Icon is a renderable 1-bit image. Value type — copy-safe.
// Use NewBitmapIcon for compile-time assets, NewImageIcon for runtime images.
type Icon struct {
	w, h   int
	render func(c *canvas.Canvas, r image.Rectangle)
}

// NewBitmapIcon creates an icon from a packed 1-bit bitmap.
// Layout: MSB-first, ((w+7)/8) bytes per row × h rows.
// Bit=0 → black, bit=1 → white (e-ink convention, same as canvas buffer).
func NewBitmapIcon(data []byte, w, h int) Icon {
	// copy data to avoid aliasing
	d := make([]byte, len(data))
	copy(d, data)
	stride := (w + 7) / 8
	return Icon{w: w, h: h, render: func(c *canvas.Canvas, r image.Rectangle) {
		s := min(float64(r.Dx())/float64(w), float64(r.Dy())/float64(h))
		dw := int(float64(w) * s)
		dh := int(float64(h) * s)
		ox := r.Min.X + (r.Dx()-dw)/2
		oy := r.Min.Y + (r.Dy()-dh)/2
		for dy := 0; dy < dh; dy++ {
			srcY := int(float64(dy) / s)
			for dx := 0; dx < dw; dx++ {
				srcX := int(float64(dx) / s)
				byteIdx := srcY*stride + srcX/8
				bit := uint(7 - srcX%8)
				if byteIdx < len(d) && (d[byteIdx]>>bit)&1 == 0 {
					c.SetPixel(ox+dx, oy+dy, canvas.Black)
				} else {
					c.SetPixel(ox+dx, oy+dy, canvas.White)
				}
			}
		}
	}}
}

// NewImageIcon creates an icon from any image.Image.
// Thresholded to 1-bit via DrawImageScaled at draw time.
func NewImageIcon(img image.Image) Icon {
	b := img.Bounds()
	return Icon{w: b.Dx(), h: b.Dy(), render: func(c *canvas.Canvas, r image.Rectangle) {
		c.DrawImageScaled(r, img)
	}}
}

// Draw renders the icon scaled to fit r (letterboxed, centered).
func (ic Icon) Draw(c *canvas.Canvas, r image.Rectangle) {
	if ic.render == nil {
		return
	}
	ic.render(c, r)
}

// Size returns the icon's natural size in pixels.
func (ic Icon) Size() (w, h int) { return ic.w, ic.h }
