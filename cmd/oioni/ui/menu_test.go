// cmd/oioni/ui/menu_test.go
package ui

import (
	"image"
	"testing"

	"github.com/oioio-space/oioni/drivers/epd"
	"github.com/oioio-space/oioni/drivers/touch"
	"github.com/oioio-space/oioni/ui/canvas"
)

// Bounds en production : liste y=22..121, x=0..199 (menu area minus nav column).
func setListBounds(l *ScrollableMenuList) {
	l.SetBounds(image.Rect(0, 22, 200, 122))
}

// Bounds en production : upBtn y=22..71, x=200..249.
func setUpBtnBounds(b *NavButton) {
	b.SetBounds(image.Rect(200, 22, 250, 72))
}

// Bounds en production : downBtn y=72..121, x=200..249.
func setDownBtnBounds(b *NavButton) {
	b.SetBounds(image.Rect(200, 72, 250, 122))
}

func newTestList() *ScrollableMenuList {
	return newScrollableMenuList([]homeMenuItem{
		{name: "Config", desc: "reseau"},
		{name: "System", desc: "services"},
		{name: "Attack", desc: "MITM"},
		{name: "DFIR", desc: "capture"},
		{name: "Info", desc: "aide"},
	})
}

// ── ScrollableMenuList ────────────────────────────────────────────────────────

func TestScrollableMenuList_PreferredSize(t *testing.T) {
	l := newTestList()
	sz := l.PreferredSize()
	want := menuVisible * menuRowH
	if sz.Y != want {
		t.Errorf("PreferredSize().Y = %d, want %d", sz.Y, want)
	}
}

func TestScrollableMenuList_ScrollDown(t *testing.T) {
	l := newTestList()
	l.ScrollDown()
	if l.offset != 1 {
		t.Errorf("offset = %d, want 1", l.offset)
	}
}

func TestScrollableMenuList_ScrollUp(t *testing.T) {
	l := newTestList()
	l.offset = 1
	l.ScrollUp()
	if l.offset != 0 {
		t.Errorf("offset = %d, want 0", l.offset)
	}
}

func TestScrollableMenuList_ScrollUpAtTop(t *testing.T) {
	l := newTestList()
	l.ScrollUp() // no-op at top
	if l.offset != 0 {
		t.Errorf("offset = %d, want 0 (no-op)", l.offset)
	}
}

func TestScrollableMenuList_ScrollDownAtBottom(t *testing.T) {
	// 5 items, menuVisible=2 → max offset=3
	l := newTestList()
	l.offset = 3
	l.ScrollDown() // no-op at bottom
	if l.offset != 3 {
		t.Errorf("offset = %d, want 3 (no-op)", l.offset)
	}
}

func TestScrollableMenuList_CanScrollUp(t *testing.T) {
	l := newTestList()
	if l.CanScrollUp() {
		t.Error("CanScrollUp() = true at offset 0, want false")
	}
	l.offset = 1
	if !l.CanScrollUp() {
		t.Error("CanScrollUp() = false at offset 1, want true")
	}
}

func TestScrollableMenuList_CanScrollDown(t *testing.T) {
	l := newTestList()
	if !l.CanScrollDown() {
		t.Error("CanScrollDown() = false at offset 0, want true")
	}
	l.offset = 3 // max
	if l.CanScrollDown() {
		t.Error("CanScrollDown() = true at max offset, want false")
	}
}

func TestScrollableMenuList_CanScrollDown_ShortList(t *testing.T) {
	// Lists shorter than menuVisible must never report CanScrollDown=true.
	for _, n := range []int{0, 1} {
		items := make([]homeMenuItem, n)
		l := newScrollableMenuList(items)
		if l.CanScrollDown() {
			t.Errorf("CanScrollDown() = true for %d-item list, want false", n)
		}
	}
}

func TestScrollableMenuList_TapRow0(t *testing.T) {
	called := ""
	items := []homeMenuItem{
		{name: "A", desc: "a", onTap: func() { called = "A" }},
		{name: "B", desc: "b", onTap: func() { called = "B" }},
		{name: "C", desc: "c"},
		{name: "D", desc: "d"},
		{name: "E", desc: "e"},
	}
	l := newScrollableMenuList(items)
	setListBounds(l)
	// row 0: y = 22..71 → center y=47
	l.HandleTouch(touch.TouchPoint{X: 100, Y: 47})
	if called != "A" {
		t.Errorf("expected A, got %q", called)
	}
}

func TestScrollableMenuList_TapRow1(t *testing.T) {
	called := ""
	items := []homeMenuItem{
		{name: "A", desc: "a", onTap: func() { called = "A" }},
		{name: "B", desc: "b", onTap: func() { called = "B" }},
		{name: "C", desc: "c"},
		{name: "D", desc: "d"},
		{name: "E", desc: "e"},
	}
	l := newScrollableMenuList(items)
	setListBounds(l)
	// row 1: y = 72..121 → center y=97
	l.HandleTouch(touch.TouchPoint{X: 100, Y: 97})
	if called != "B" {
		t.Errorf("expected B, got %q", called)
	}
}

func TestScrollableMenuList_TapNilOnTapIsNoOp(t *testing.T) {
	items := []homeMenuItem{
		{name: "A", desc: "a"}, // onTap nil
		{name: "B", desc: "b"},
		{name: "C", desc: "c"},
		{name: "D", desc: "d"},
		{name: "E", desc: "e"},
	}
	l := newScrollableMenuList(items)
	setListBounds(l)
	l.HandleTouch(touch.TouchPoint{X: 100, Y: 47}) // ne doit pas paniquer
}

func TestScrollableMenuList_DrawDoesNotPanic(t *testing.T) {
	l := newTestList()
	setListBounds(l)
	c := canvas.New(epd.Width, epd.Height, canvas.Rot90)
	l.Draw(c)
}

// ── NavButton ─────────────────────────────────────────────────────────────────

func TestNavButton_TapCallsOnTap(t *testing.T) {
	called := false
	b := newNavButton("^", func() { called = true }, func() bool { return true })
	setUpBtnBounds(b)
	b.HandleTouch(touch.TouchPoint{X: 225, Y: 47})
	if !called {
		t.Error("onTap not called")
	}
}

func TestNavButton_TapWhenDisabledStillCallsOnTap(t *testing.T) {
	// Le NavButton appelle toujours onTap — c'est onTap qui décide si c'est no-op.
	called := false
	b := newNavButton("^", func() { called = true }, func() bool { return false })
	setUpBtnBounds(b)
	b.HandleTouch(touch.TouchPoint{X: 225, Y: 47})
	if !called {
		t.Error("onTap not called even when disabled")
	}
}

func TestNavButton_DrawActiveDoesNotPanic(t *testing.T) {
	b := newNavButton("^", func() {}, func() bool { return true })
	setUpBtnBounds(b)
	c := canvas.New(epd.Width, epd.Height, canvas.Rot90)
	b.Draw(c)
}

func TestNavButton_DrawDisabledDoesNotPanic(t *testing.T) {
	b := newNavButton("^", func() {}, func() bool { return false })
	setUpBtnBounds(b)
	c := canvas.New(epd.Width, epd.Height, canvas.Rot90)
	b.Draw(c)
}
