// ui/gui/widget_scrolllist_test.go
package gui

import (
	"image"
	"testing"

	"github.com/oioio-space/oioni/ui/canvas"
)

// stubItem est un ListItem de test qui enregistre les taps.
type stubItem struct{ tapped bool }

func (s *stubItem) Draw(_ *canvas.Canvas, _ image.Rectangle) {}
func (s *stubItem) OnTap()                                    { s.tapped = true }

// newTestList5 crée une ScrollableList avec 5 stubItems et rowH=25.
func newTestList5() (*ScrollableList, []*stubItem) {
	stubs := make([]*stubItem, 5)
	items := make([]ListItem, 5)
	for i := range stubs {
		stubs[i] = &stubItem{}
		items[i] = stubs[i]
	}
	return NewScrollableList(items, 25), stubs
}

// setBoundsMenu simule les bounds de production : liste y=22..122, x=0..200 (100px height)
func setBoundsMenu(l *ScrollableList) {
	l.SetBounds(image.Rect(0, 22, 200, 122))
}

func TestScrollableList_Visible_100px(t *testing.T) {
	l, _ := newTestList5()
	setBoundsMenu(l) // 100px height / rowH=25 → 4
	if got := l.visible(); got != 4 {
		t.Errorf("visible() = %d, want 4", got)
	}
}

func TestScrollableList_Visible_50px(t *testing.T) {
	l, _ := newTestList5()
	l.SetBounds(image.Rect(0, 0, 200, 50)) // 50px height / rowH=25 → 2
	if got := l.visible(); got != 2 {
		t.Errorf("visible() = %d, want 2", got)
	}
}

func TestScrollableList_Visible_NoBounds(t *testing.T) {
	l, _ := newTestList5()
	// bounds zero → visible() must return 0, not divide by zero
	if got := l.visible(); got != 0 {
		t.Errorf("visible() = %d, want 0 with empty bounds", got)
	}
}

func TestScrollableList_CanScrollDown_5items_4visible(t *testing.T) {
	l, _ := newTestList5()
	setBoundsMenu(l) // visible=4, items=5 → can scroll
	if !l.CanScrollDown() {
		t.Error("CanScrollDown() = false with 5 items and 4 visible")
	}
}

func TestScrollableList_CannotScrollDown_WhenAllFit(t *testing.T) {
	items := []ListItem{&stubItem{}, &stubItem{}, &stubItem{}, &stubItem{}}
	l := NewScrollableList(items, 25)
	setBoundsMenu(l) // 4 items, visible=4 → no scroll
	if l.CanScrollDown() {
		t.Error("CanScrollDown() = true when all items fit")
	}
}

func TestScrollableList_CannotScrollDown_ShortList(t *testing.T) {
	for _, n := range []int{0, 1} {
		items := make([]ListItem, n)
		l := NewScrollableList(items, 25)
		setBoundsMenu(l)
		if l.CanScrollDown() {
			t.Errorf("CanScrollDown() = true for %d-item list", n)
		}
	}
}

func TestScrollableList_CanScrollUp_AtTop(t *testing.T) {
	l, _ := newTestList5()
	if l.CanScrollUp() {
		t.Error("CanScrollUp() = true at offset 0")
	}
}

func TestScrollableList_CanScrollUp_WithOffset(t *testing.T) {
	l, _ := newTestList5()
	l.offset = 1
	if !l.CanScrollUp() {
		t.Error("CanScrollUp() = false with offset=1")
	}
}

func TestScrollableList_ScrollDown(t *testing.T) {
	l, _ := newTestList5()
	setBoundsMenu(l)
	l.ScrollDown()
	if l.offset != 1 {
		t.Errorf("offset = %d, want 1", l.offset)
	}
}

func TestScrollableList_ScrollUp(t *testing.T) {
	l, _ := newTestList5()
	setBoundsMenu(l)
	l.offset = 1
	l.ScrollUp()
	if l.offset != 0 {
		t.Errorf("offset = %d, want 0", l.offset)
	}
}

func TestScrollableList_ScrollUpAtTop_Noop(t *testing.T) {
	l, _ := newTestList5()
	l.ScrollUp() // no-op
	if l.offset != 0 {
		t.Errorf("offset = %d, want 0 (no-op)", l.offset)
	}
}

func TestScrollableList_ScrollDownAtBottom_Noop(t *testing.T) {
	l, _ := newTestList5()
	setBoundsMenu(l)  // visible=4, max offset = 5-4 = 1
	l.offset = 1
	l.ScrollDown() // no-op
	if l.offset != 1 {
		t.Errorf("offset = %d, want 1 (no-op)", l.offset)
	}
}

func TestScrollableList_TapRow0(t *testing.T) {
	l, stubs := newTestList5()
	setBoundsMenu(l) // row 0: y=22..46
	l.HandleTouch(TouchPoint{X: 100, Y: 34}) // y=34 → row 0
	if !stubs[0].tapped {
		t.Error("row 0 not tapped")
	}
}

func TestScrollableList_TapRow3(t *testing.T) {
	l, stubs := newTestList5()
	setBoundsMenu(l) // row 3: y=97..121
	l.HandleTouch(TouchPoint{X: 100, Y: 109}) // y=109 → row 3
	if !stubs[3].tapped {
		t.Error("row 3 not tapped")
	}
}

func TestScrollableList_TapWithOffset(t *testing.T) {
	l, stubs := newTestList5()
	setBoundsMenu(l)
	l.offset = 1 // showing items 1..4
	l.HandleTouch(TouchPoint{X: 100, Y: 34}) // row 0 → item index 1
	if !stubs[1].tapped {
		t.Error("item 1 not tapped with offset=1")
	}
}

func TestScrollableList_DrawDoesNotPanic(t *testing.T) {
	l, _ := newTestList5()
	setBoundsMenu(l)
	c := canvas.New(ScreenWidth, ScreenHeight, canvas.Rot90)
	l.Draw(c)
}

func TestScrollableList_DrawEmptyList(t *testing.T) {
	l := NewScrollableList(nil, 25)
	setBoundsMenu(l)
	c := canvas.New(ScreenWidth, ScreenHeight, canvas.Rot90)
	l.Draw(c) // must not panic
}

func TestScrollableList_DrawEmptyBounds(t *testing.T) {
	l, _ := newTestList5()
	// bounds zero → early return, no panic
	c := canvas.New(ScreenWidth, ScreenHeight, canvas.Rot90)
	l.Draw(c)
}

func TestScrollableList_TapAboveWidget_DoesNotFireItem0(t *testing.T) {
	l, stubs := newTestList5()
	setBoundsMenu(l) // wb.Min.Y = 22
	// pt.Y = 21 → just above the widget. In Go: (21-22)/25 = -1/25 = 0,
	// which would pass row >= 0 without the explicit bounds check.
	l.HandleTouch(TouchPoint{X: 100, Y: 21})
	if stubs[0].tapped {
		t.Error("item 0 was tapped for a touch above the widget bounds")
	}
}
