package gui

import (
	"image"
	"testing"
	"time"

)

func TestIconCarousel_PreferredSize(t *testing.T) {
	c := NewIconCarousel(nil)
	ps := c.PreferredSize()
	if ps.Y != 88 {
		t.Errorf("carousel height = %d, want 88", ps.Y)
	}
	if ps.X != 206 {
		t.Errorf("carousel width = %d, want 206", ps.X)
	}
}

func TestIconCarousel_ScrollH_Left(t *testing.T) {
	items := make([]CarouselItem, 5)
	for i := range items {
		items[i] = CarouselItem{Label: "item"}
	}
	c := NewIconCarousel(items)
	c.SetIndex(0)
	c.ScrollH(-1) // swipe left → index++
	if c.Index() != 1 {
		t.Errorf("after ScrollH(-1), Index = %d, want 1", c.Index())
	}
}

func TestIconCarousel_ScrollH_Right(t *testing.T) {
	items := make([]CarouselItem, 5)
	for i := range items {
		items[i] = CarouselItem{Label: "item"}
	}
	c := NewIconCarousel(items)
	c.SetIndex(2)
	c.ScrollH(+1) // swipe right → index--
	if c.Index() != 1 {
		t.Errorf("after ScrollH(+1), Index = %d, want 1", c.Index())
	}
}

func TestIconCarousel_ScrollH_ClampMin(t *testing.T) {
	items := make([]CarouselItem, 3)
	for i := range items {
		items[i] = CarouselItem{Label: "item"}
	}
	c := NewIconCarousel(items)
	c.ScrollH(+1) // already at 0
	if c.Index() != 0 {
		t.Errorf("clamped at min: Index = %d, want 0", c.Index())
	}
}

func TestIconCarousel_ScrollH_ClampMax(t *testing.T) {
	items := make([]CarouselItem, 3)
	for i := range items {
		items[i] = CarouselItem{Label: "item"}
	}
	c := NewIconCarousel(items)
	c.SetIndex(2)
	c.ScrollH(-1) // already at last
	if c.Index() != 2 {
		t.Errorf("clamped at max: Index = %d, want 2", c.Index())
	}
}

func TestIconCarousel_ScrollH_MarksDirty(t *testing.T) {
	items := make([]CarouselItem, 3)
	for i := range items {
		items[i] = CarouselItem{Label: "item"}
	}
	c := NewIconCarousel(items)
	c.MarkClean()
	c.ScrollH(-1)
	if !c.IsDirty() {
		t.Error("ScrollH should mark dirty")
	}
}

func TestIconCarousel_TapCallsOnTap(t *testing.T) {
	called := make(chan struct{}, 1)
	items := []CarouselItem{
		{Label: "item", OnTap: func() { called <- struct{}{} }},
		{Label: "item2"},
	}
	c := NewIconCarousel(items)
	c.SetBounds(image.Rect(0, 0, 206, 88))
	c.tapDelay = 10 * time.Millisecond

	// Touch inside first button region (leading=13, button=80px wide)
	c.HandleTouch(TouchPoint{X: uint16(13 + 20), Y: 44})

	select {
	case <-called:
		// pass
	case <-time.After(500 * time.Millisecond):
		t.Error("OnTap not called within 500ms")
	}
}

func TestIconCarousel_Draw_NoPanic(t *testing.T) {
	items := []CarouselItem{
		{Label: "Config"},
		{Label: "System"},
		{Label: "Attack"},
	}
	c := NewIconCarousel(items)
	c.SetBounds(image.Rect(0, 0, 206, 88))
	cv := newTestCanvas()
	c.Draw(cv) // must not panic
}

func TestIconCarousel_Draw_Empty_NoPanic(t *testing.T) {
	c := NewIconCarousel(nil)
	c.SetBounds(image.Rect(0, 0, 206, 88))
	cv := newTestCanvas()
	c.Draw(cv) // must not panic on empty items
}
