package gui

import (
	"context"
	"image"
	"time"

	"github.com/oioio-space/oioni/ui/canvas"
)

// ClockWidget displays the current time and auto-refreshes each minute (NewClock)
// or each second (NewClockFull). Implements Stoppable — Navigator.Pop() calls Stop().
type ClockWidget struct {
	BaseWidget
	format string
	cancel context.CancelFunc
}

func NewClock() *ClockWidget     { return newClock("15:04", time.Minute) }
func NewClockFull() *ClockWidget { return newClock("15:04:05", time.Second) }

func newClock(format string, interval time.Duration) *ClockWidget {
	ctx, cancel := context.WithCancel(context.Background())
	w := &ClockWidget{format: format, cancel: cancel}
	w.SetDirty()
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				w.SetDirty()
			}
		}
	}()
	return w
}

func (w *ClockWidget) Stop() { w.cancel() }

func (w *ClockWidget) PreferredSize() image.Point { return image.Pt(60, 24) }
func (w *ClockWidget) MinSize() image.Point       { return image.Pt(40, 16) }

func (w *ClockWidget) Draw(c *canvas.Canvas) {
	r := w.Bounds()
	if r.Empty() {
		return
	}
	text := time.Now().Format(w.format)
	f := canvas.EmbeddedFont(20)
	if f == nil {
		return
	}
	tw := 0
	for _, ch := range text {
		_, gw, _ := f.Glyph(ch)
		tw += gw
	}
	x := r.Min.X + (r.Dx()-tw)/2
	y := r.Min.Y + (r.Dy()-f.LineHeight())/2
	c.DrawRect(r, canvas.White, true) // clear background
	c.DrawText(x, y, text, f, canvas.Black)
	w.MarkClean()
}
