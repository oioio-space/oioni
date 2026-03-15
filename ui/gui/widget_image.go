package gui

import (
	"image"

	"github.com/oioio-space/oioni/ui/canvas"
)

// ImageWidget renders an image.Image scaled to its bounds.
// No intrinsic size — must be given bounds via Expand or FixedSize in a layout.
type ImageWidget struct {
	BaseWidget
	img image.Image
}

func NewImageWidget(img image.Image) *ImageWidget {
	w := &ImageWidget{img: img}
	w.SetDirty()
	return w
}

func (w *ImageWidget) SetImage(img image.Image) {
	w.img = img
	w.SetDirty()
}

func (w *ImageWidget) PreferredSize() image.Point { return image.Point{} }
func (w *ImageWidget) MinSize() image.Point       { return image.Point{} }

func (w *ImageWidget) Draw(c *canvas.Canvas) {
	if w.img == nil || w.Bounds().Empty() {
		return
	}
	c.DrawImageScaled(w.Bounds(), w.img)
	w.MarkClean()
}
