// ui/gui/widget_carousel.go — IconCarousel: horizontal icon category picker
package gui

import (
	"image"
	"sync/atomic"
	"time"

	"github.com/oioio-space/oioni/drivers/touch"
	"github.com/oioio-space/oioni/ui/canvas"
)

const (
	carouselButtonSize = 80 // px per button square
	carouselGap        = 6  // px gap between buttons
	carouselLeading    = 13 // px leading indent
	carouselPagHeight  = 8  // px for pagination dots row at bottom
)

// CarouselItem describes one button in the IconCarousel.
type CarouselItem struct {
	Icon  Icon
	Label string
	OnTap func() // called after tapDelay; may be nil
}

// IconCarousel is a horizontally scrollable row of icon+label category buttons.
// Implements hScrollable for Navigator swipe routing.
// Implements Touchable for tap handling.
//
// hScrollable constraint: must be a direct member of Scene.Widgets (not inside a
// layout container) for Navigator.Run() horizontal swipe routing to find it.
type IconCarousel struct {
	BaseWidget
	items    []CarouselItem
	index    int           // leftmost visible item index (snap position)
	pressed  atomic.Int32  // index of pressed item (-1 = none, ≥0 = pressed index)
	tapDelay time.Duration // override for tests; 0 = default 100ms
}

// NewIconCarousel creates an IconCarousel with the given items.
func NewIconCarousel(items []CarouselItem) *IconCarousel {
	c := &IconCarousel{
		items: items,
	}
	c.pressed.Store(-1)
	c.SetDirty()
	return c
}

func (c *IconCarousel) PreferredSize() image.Point { return image.Pt(206, 88) }
func (c *IconCarousel) MinSize() image.Point       { return image.Pt(100, 60) }

// Index returns the current snap position (leftmost visible item index).
func (c *IconCarousel) Index() int { return c.index }

// SetIndex restores the scroll position.
func (c *IconCarousel) SetIndex(i int) {
	n := len(c.items)
	if n == 0 {
		c.index = 0
	} else {
		c.index = clamp(i, 0, n-1)
	}
	c.SetDirty()
}

// ScrollH implements hScrollable.
// delta=-1: scroll right (swipe-left gesture → next item enters from right).
// delta=+1: scroll left (swipe-right gesture → previous item).
func (c *IconCarousel) ScrollH(delta int) {
	n := len(c.items)
	if n == 0 {
		return
	}
	c.index = clamp(c.index-delta, 0, n-1)
	c.SetDirty()
}

// HandleTouch implements Touchable. Non-blocking: tap feedback via time.AfterFunc.
func (c *IconCarousel) HandleTouch(pt touch.TouchPoint) bool {
	b := c.Bounds()
	if b.Empty() || len(c.items) == 0 {
		return false
	}
	x := int(pt.X) - b.Min.X
	// Find which button was tapped
	bx := carouselLeading
	buttonIdx := -1
	for i := c.index; i < len(c.items); i++ {
		if x >= bx && x < bx+carouselButtonSize {
			buttonIdx = i
			break
		}
		bx += carouselButtonSize + carouselGap
		if bx >= b.Dx() {
			break
		}
	}
	if buttonIdx < 0 || buttonIdx >= len(c.items) {
		return false
	}
	delay := c.tapDelay
	if delay <= 0 {
		delay = 100 * time.Millisecond
	}
	item := c.items[buttonIdx]
	c.pressed.Store(int32(buttonIdx))
	c.SetDirty()
	time.AfterFunc(delay, func() {
		c.pressed.Store(-1)
		c.SetDirty()
		if item.OnTap != nil {
			item.OnTap()
		}
	})
	return true
}

// Draw renders carousel buttons, pagination dots, and scroll arrows.
func (c *IconCarousel) Draw(cv *canvas.Canvas) {
	b := c.Bounds()
	if b.Empty() {
		return
	}
	cv.DrawRect(b, canvas.White, true)
	f8 := canvas.EmbeddedFont(8)
	buttonH := b.Dy() - carouselPagHeight

	bx := b.Min.X + carouselLeading
	for i := c.index; i < len(c.items); i++ {
		btnRect := image.Rect(bx, b.Min.Y, bx+carouselButtonSize, b.Min.Y+buttonH)
		if btnRect.Min.X >= b.Max.X {
			break
		}
		if btnRect.Max.X > b.Max.X {
			btnRect.Max.X = b.Max.X
		}

		inverted := c.pressed.Load() == int32(i)
		bg := canvas.White
		fg := canvas.Black
		if inverted {
			bg, fg = canvas.Black, canvas.White
		}

		// Background fill + border
		DrawRoundedRect(cv, btnRect, 4, true, bg)
		DrawRoundedRect(cv, btnRect, 4, false, fg)

		// Icon: centered in top portion of button
		iconSize := 32
		iconX := btnRect.Min.X + (carouselButtonSize-iconSize)/2
		iconY := btnRect.Min.Y + (buttonH-iconSize)/2 - 6 // shift up to leave room for label
		iconRect := image.Rect(iconX, iconY, iconX+iconSize, iconY+iconSize)
		if !iconRect.Intersect(b).Empty() {
			c.items[i].Icon.Draw(cv, iconRect)
		}

		// Label: centered below icon
		if f8 != nil && c.items[i].Label != "" {
			lw := textWidth(c.items[i].Label, f8)
			lx := btnRect.Min.X + (btnRect.Dx()-lw)/2
			ly := btnRect.Max.Y - f8.LineHeight() - 2
			if lx < b.Max.X {
				cv.DrawText(lx, ly, c.items[i].Label, f8, fg)
			}
		}

		bx += carouselButtonSize + carouselGap
	}

	// Pagination dots
	if len(c.items) > 1 {
		dotY := b.Max.Y - carouselPagHeight/2
		dotSpacing := 6
		totalW := (len(c.items)-1)*dotSpacing + 4
		startX := b.Min.X + (b.Dx()-totalW)/2
		for i := range c.items {
			dx := startX + i*dotSpacing
			if i == c.index {
				cv.DrawCircle(dx, dotY, 2, canvas.Black, true)
			} else {
				cv.DrawCircle(dx, dotY, 2, canvas.Black, false)
			}
		}
	}

	// Scroll arrows
	if c.index > 0 && f8 != nil {
		cv.DrawText(b.Min.X, b.Min.Y+(b.Dy()-carouselPagHeight)/2-4, "‹", f8, canvas.Black)
	}
	if c.index < len(c.items)-1 && f8 != nil {
		cv.DrawText(b.Max.X-8, b.Min.Y+(b.Dy()-carouselPagHeight)/2-4, "›", f8, canvas.Black)
	}
}
