package gui

import (
	"image"

	"github.com/oioio-space/oioni/ui/canvas"
	"rsc.io/qr"
)

// QRCode renders a QR code from a string. Matrix is cached and regenerated
// only when SetData is called.
type QRCode struct {
	BaseWidget
	data   string
	matrix [][]bool // true = black module
}

func NewQRCode(data string) *QRCode {
	q := &QRCode{}
	q.SetData(data)
	return q
}

func (q *QRCode) SetData(data string) {
	q.data = data
	q.matrix = nil
	if data != "" {
		if code, err := qr.Encode(data, qr.M); err == nil {
			n := code.Size
			q.matrix = make([][]bool, n)
			for y := 0; y < n; y++ {
				q.matrix[y] = make([]bool, n)
				for x := 0; x < n; x++ {
					q.matrix[y][x] = code.Black(x, y)
				}
			}
		}
	}
	q.SetDirty()
}

func (q *QRCode) PreferredSize() image.Point { return image.Pt(80, 80) }
func (q *QRCode) MinSize() image.Point       { return image.Pt(40, 40) }

func (q *QRCode) Draw(c *canvas.Canvas) {
	r := q.Bounds()
	if r.Empty() || len(q.matrix) == 0 {
		return
	}
	n := len(q.matrix)
	// scale: largest integer that fits
	scale := min(r.Dx(), r.Dy()) / n
	if scale < 1 {
		scale = 1
	}
	dw := n * scale
	dh := n * scale
	ox := r.Min.X + (r.Dx()-dw)/2
	oy := r.Min.Y + (r.Dy()-dh)/2

	// White background
	c.DrawRect(r, canvas.White, true)

	for y, row := range q.matrix {
		for x, black := range row {
			if black {
				px := image.Rect(ox+x*scale, oy+y*scale, ox+x*scale+scale, oy+y*scale+scale)
				c.DrawRect(px, canvas.Black, true)
			}
		}
	}
	q.MarkClean()
}
