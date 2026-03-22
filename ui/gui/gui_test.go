package gui

import (
	"context"
	"image"
	"testing"
	"time"

	"github.com/oioio-space/oioni/ui/canvas"
	"github.com/oioio-space/oioni/drivers/epd"
	"github.com/oioio-space/oioni/drivers/touch"
)

// mockHScrollable is a package-level test helper used by TestHScrollableInterface_CompileGuard.
type mockHScrollable struct {
	BaseWidget
	scrolled int
}

func (m *mockHScrollable) ScrollH(delta int)        { m.scrolled += delta }
func (m *mockHScrollable) Draw(c *canvas.Canvas)   {}

// TestHScrollableInterface_CompileGuard verifies the hScrollable interface
// can be satisfied by a concrete type within the package.
func TestHScrollableInterface_CompileGuard(t *testing.T) {
	m := &mockHScrollable{}
	// Verify mockHScrollable satisfies hScrollable at compile time
	var _ hScrollable = m
}

func TestBaseWidgetInitiallyClean(t *testing.T) {
	var b BaseWidget
	if b.IsDirty() {
		t.Error("new BaseWidget should not be dirty")
	}
}

func TestBaseWidgetSetDirty(t *testing.T) {
	var b BaseWidget
	b.SetDirty()
	if !b.IsDirty() {
		t.Error("expected dirty after SetDirty()")
	}
}

func TestBaseWidgetMarkClean(t *testing.T) {
	var b BaseWidget
	b.SetDirty()
	b.MarkClean()
	if b.IsDirty() {
		t.Error("expected clean after MarkClean()")
	}
}

func TestBaseWidgetSetBoundsMarksDirty(t *testing.T) {
	var b BaseWidget
	r := image.Rect(10, 20, 50, 40)
	b.SetBounds(r)
	if b.Bounds() != r {
		t.Errorf("Bounds = %v, want %v", b.Bounds(), r)
	}
	if !b.IsDirty() {
		t.Error("SetBounds should mark dirty")
	}
}

func TestBaseWidgetPreferredAndMinSizeZero(t *testing.T) {
	var b BaseWidget
	if b.PreferredSize() != (image.Point{}) {
		t.Errorf("PreferredSize should be zero, got %v", b.PreferredSize())
	}
	if b.MinSize() != (image.Point{}) {
		t.Errorf("MinSize should be zero, got %v", b.MinSize())
	}
}

func newTestCanvas() *canvas.Canvas {
	return canvas.New(epd.Width, epd.Height, canvas.Rot90)
}

// ── layout tests ──────────────────────────────────────────────────────────────

// fixedWidget is a test widget with fixed preferred and min sizes.
type fixedWidget struct {
	BaseWidget
	pref image.Point
	min  image.Point
	drew bool
}

func newFixedWidget(pw, ph, mw, mh int) *fixedWidget {
	w := &fixedWidget{pref: image.Pt(pw, ph), min: image.Pt(mw, mh)}
	w.SetDirty()
	return w
}
func (w *fixedWidget) PreferredSize() image.Point { return w.pref }
func (w *fixedWidget) MinSize() image.Point       { return w.min }
func (w *fixedWidget) Draw(c *canvas.Canvas)      { w.drew = true }

// touchWidget is a fixedWidget that also implements Touchable.
type touchWidget struct {
	fixedWidget
	touched bool
}

func newTouchWidget(pw, ph int) *touchWidget {
	tw := &touchWidget{}
	tw.pref = image.Pt(pw, ph)
	tw.min = image.Pt(pw, ph)
	tw.SetDirty()
	return tw
}
func (tw *touchWidget) HandleTouch(pt touch.TouchPoint) bool { tw.touched = true; return true }

func TestVBoxAllocatesChildren(t *testing.T) {
	a := newFixedWidget(100, 20, 0, 10)
	b := newFixedWidget(100, 30, 0, 10)
	box := NewVBox(a, b)
	box.SetBounds(image.Rect(0, 0, 100, 100))

	if a.Bounds().Dy() != 20 {
		t.Errorf("child a height = %d, want 20", a.Bounds().Dy())
	}
	if b.Bounds().Dy() != 30 {
		t.Errorf("child b height = %d, want 30", b.Bounds().Dy())
	}
	if a.Bounds().Min.Y != 0 {
		t.Errorf("child a y = %d, want 0", a.Bounds().Min.Y)
	}
	if b.Bounds().Min.Y != 20 {
		t.Errorf("child b y = %d, want 20", b.Bounds().Min.Y)
	}
}

func TestVBoxExpandTakesRemainingHeight(t *testing.T) {
	fixed := newFixedWidget(100, 20, 0, 10)
	expanded := newFixedWidget(100, 10, 0, 5)
	box := NewVBox(fixed, Expand(expanded))
	box.SetBounds(image.Rect(0, 0, 100, 100))

	if fixed.Bounds().Dy() != 20 {
		t.Errorf("fixed child height = %d, want 20", fixed.Bounds().Dy())
	}
	if expanded.Bounds().Dy() != 80 {
		t.Errorf("expand child height = %d, want 80", expanded.Bounds().Dy())
	}
}

func TestVBoxEnforces20pxForTouchable(t *testing.T) {
	small := newTouchWidget(100, 5)
	box := NewVBox(small)
	box.SetBounds(image.Rect(0, 0, 100, 100))
	if small.Bounds().Dy() < 20 {
		t.Errorf("Touchable child height = %d, want >= 20", small.Bounds().Dy())
	}
}

func TestVBoxIsDirtyIfChildDirty(t *testing.T) {
	a := newFixedWidget(100, 20, 0, 10)
	box := NewVBox(a)
	box.SetBounds(image.Rect(0, 0, 100, 50))
	box.MarkClean()
	a.SetDirty()
	if !box.IsDirty() {
		t.Error("VBox should be dirty when child is dirty")
	}
}

func TestVBoxMarkCleanClearsChildren(t *testing.T) {
	a := newFixedWidget(100, 20, 0, 10)
	box := NewVBox(a)
	box.SetBounds(image.Rect(0, 0, 100, 50))
	box.MarkClean()
	if a.IsDirty() {
		t.Error("MarkClean should clear children")
	}
}

func TestHBoxAllocatesChildren(t *testing.T) {
	a := newFixedWidget(40, 20, 0, 0)
	b := newFixedWidget(60, 20, 0, 0)
	box := NewHBox(a, b)
	box.SetBounds(image.Rect(0, 0, 200, 20))

	if a.Bounds().Dx() != 40 {
		t.Errorf("child a width = %d, want 40", a.Bounds().Dx())
	}
	if b.Bounds().Dx() != 60 {
		t.Errorf("child b width = %d, want 60", b.Bounds().Dx())
	}
	if a.Bounds().Min.X != 0 {
		t.Errorf("child a x = %d, want 0", a.Bounds().Min.X)
	}
	if b.Bounds().Min.X != 40 {
		t.Errorf("child b x = %d, want 40", b.Bounds().Min.X)
	}
}

func TestHBoxExpandTakesRemainingWidth(t *testing.T) {
	fixed := newFixedWidget(40, 20, 0, 0)
	expanded := newFixedWidget(10, 20, 0, 0)
	box := NewHBox(fixed, Expand(expanded))
	box.SetBounds(image.Rect(0, 0, 200, 20))

	if fixed.Bounds().Dx() != 40 {
		t.Errorf("fixed width = %d, want 40", fixed.Bounds().Dx())
	}
	if expanded.Bounds().Dx() != 160 {
		t.Errorf("expand width = %d, want 160", expanded.Bounds().Dx())
	}
}

func TestFixedPutsWidgetAtAbsolutePosition(t *testing.T) {
	w := newFixedWidget(30, 15, 0, 0)
	f := NewFixed(200, 100)
	f.Put(w, 10, 5)
	f.SetBounds(image.Rect(0, 0, 200, 100))

	if w.Bounds().Min.X != 10 {
		t.Errorf("widget x = %d, want 10", w.Bounds().Min.X)
	}
	if w.Bounds().Min.Y != 5 {
		t.Errorf("widget y = %d, want 5", w.Bounds().Min.Y)
	}
}

func TestOverlayCentersContent(t *testing.T) {
	content := newFixedWidget(60, 30, 60, 30)
	o := NewOverlay(content, AlignCenter)
	o.setScreen(250, 122)

	wantX := (250 - 60) / 2
	wantY := (122 - 30) / 2
	if content.Bounds().Min.X != wantX {
		t.Errorf("overlay x = %d, want %d", content.Bounds().Min.X, wantX)
	}
	if content.Bounds().Min.Y != wantY {
		t.Errorf("overlay y = %d, want %d", content.Bounds().Min.Y, wantY)
	}
}

func TestWithPaddingAddsPadding(t *testing.T) {
	inner := newFixedWidget(40, 20, 40, 20)
	padded := WithPadding(4, inner)
	padded.SetBounds(image.Rect(0, 0, 100, 50))

	if inner.Bounds().Min.X != 4 {
		t.Errorf("inner x = %d, want 4", inner.Bounds().Min.X)
	}
	if inner.Bounds().Min.Y != 4 {
		t.Errorf("inner y = %d, want 4", inner.Bounds().Min.Y)
	}
	if inner.Bounds().Max.X != 96 {
		t.Errorf("inner max x = %d, want 96", inner.Bounds().Max.X)
	}
}

func TestVBoxFixedSizeAllocatesExactHeight(t *testing.T) {
	a := newFixedWidget(100, 20, 0, 0)
	b := newFixedWidget(100, 20, 0, 0)
	// b is pinned to 40px regardless of preferred size
	box := NewVBox(a, FixedSize(b, 40))
	box.SetBounds(image.Rect(0, 0, 100, 100))

	if a.Bounds().Dy() != 20 {
		t.Errorf("a height = %d, want 20", a.Bounds().Dy())
	}
	if b.Bounds().Dy() != 40 {
		t.Errorf("b height = %d, want 40 (FixedSize)", b.Bounds().Dy())
	}
}

// ── widget tests ──────────────────────────────────────────────────────────────

func TestLabelSetTextMarksDirty(t *testing.T) {
	l := NewLabel("hello")
	l.MarkClean()
	l.SetText("world")
	if !l.IsDirty() {
		t.Error("SetText should mark dirty")
	}
}

func TestLabelPreferredSizeUsesFont(t *testing.T) {
	l := NewLabel("A")
	ps := l.PreferredSize()
	if ps.Y == 0 {
		t.Error("PreferredSize height should be > 0 (font line height)")
	}
}

func TestLabelDrawDoesNotPanic(t *testing.T) {
	c := canvas.New(epd.Width, epd.Height, canvas.Rot90)
	l := NewLabel("hello")
	l.SetBounds(image.Rect(0, 0, 100, 20))
	l.Draw(c) // must not panic, even with nil font
}

func TestButtonHandleTouchFiresOnClick(t *testing.T) {
	clicked := false
	btn := NewButton("OK")
	btn.OnClick(func() { clicked = true })
	btn.SetBounds(image.Rect(0, 0, 60, 20))
	btn.HandleTouch(touch.TouchPoint{X: 30, Y: 10})
	if !clicked {
		t.Error("OnClick should fire on HandleTouch")
	}
}

func TestButtonDrawDoesNotPanic(t *testing.T) {
	c := canvas.New(epd.Width, epd.Height, canvas.Rot90)
	btn := NewButton("OK")
	btn.SetBounds(image.Rect(0, 0, 60, 20))
	btn.Draw(c)
}

func TestProgressBarClamps(t *testing.T) {
	bar := NewProgressBar()
	bar.SetValue(1.5)
	if bar.value != 1.0 {
		t.Errorf("value clamped to %v, want 1.0", bar.value)
	}
	bar.SetValue(-0.5)
	if bar.value != 0.0 {
		t.Errorf("value clamped to %v, want 0.0", bar.value)
	}
}

func TestProgressBarPreferredSize(t *testing.T) {
	bar := NewProgressBar()
	ps := bar.PreferredSize()
	if ps.Y != 12 {
		t.Errorf("ProgressBar height = %d, want 12", ps.Y)
	}
	if ps.X != 0 {
		t.Errorf("ProgressBar width = %d, want 0 (use Expand)", ps.X)
	}
}

func TestStatusBarDrawDoesNotPanic(t *testing.T) {
	c := canvas.New(epd.Width, epd.Height, canvas.Rot90)
	s := NewStatusBar("left", "right")
	s.SetBounds(image.Rect(0, 0, 250, 16))
	s.Draw(c)
}

func TestSpacerPreferredSizeZero(t *testing.T) {
	s := NewSpacer()
	if s.PreferredSize() != (image.Point{}) {
		t.Errorf("Spacer PreferredSize = %v, want (0,0)", s.PreferredSize())
	}
}

func TestDividerPreferredHeight(t *testing.T) {
	d := NewDivider()
	if d.PreferredSize().Y != 2 {
		t.Errorf("Divider PreferredSize.Y = %d, want 2", d.PreferredSize().Y)
	}
}

func TestButtonPressedStateCycle(t *testing.T) {
	c := canvas.New(epd.Width, epd.Height, canvas.Rot90)
	btn := NewButton("OK")
	btn.SetBounds(image.Rect(0, 0, 60, 20))
	btn.MarkClean()

	// Touch → pressed=true, dirty=true
	btn.HandleTouch(touch.TouchPoint{X: 30, Y: 10})
	if !btn.IsDirty() {
		t.Error("after HandleTouch, button should be dirty (pressed state)")
	}

	// First Draw → shows pressed (inverted), clears pressed, sets dirty again
	btn.MarkClean()
	btn.Draw(c)
	if !btn.IsDirty() {
		t.Error("after first Draw of pressed button, should be dirty again (restore normal state)")
	}

	// Second Draw → shows normal, no more dirty
	btn.MarkClean()
	btn.Draw(c)
	if btn.IsDirty() {
		t.Error("after second Draw, button should not be dirty")
	}
}

// ── refresh tests ─────────────────────────────────────────────────────────────

// fakeDisplay implements the Display interface for tests — no hardware needed.
type fakeDisplay struct {
	initCalled       int
	baseCalled       int
	partialCalled    int
	fastCalled       int
	regenerateCalled int
	sleepCalled      int
	lastMode         epd.Mode
}

func (f *fakeDisplay) Init(m epd.Mode) error          { f.initCalled++; f.lastMode = m; return nil }
func (f *fakeDisplay) DisplayBase(b []byte) error     { f.baseCalled++; return nil }
func (f *fakeDisplay) DisplayPartial(b []byte) error  { f.partialCalled++; return nil }
func (f *fakeDisplay) DisplayFast(b []byte) error     { f.fastCalled++; return nil }
func (f *fakeDisplay) DisplayRegenerate() error       { f.regenerateCalled++; return nil }
func (f *fakeDisplay) Sleep() error                   { f.sleepCalled++; return nil }
func (f *fakeDisplay) Close() error                   { return nil }

func TestRefreshManagerNoop(t *testing.T) {
	d := &fakeDisplay{}
	rm := newRefreshManager(d)
	c := canvas.New(epd.Width, epd.Height, canvas.Rot90)
	// No dirty widgets — render should be a noop
	if err := rm.Render(c, nil); err != nil {
		t.Fatalf("Render noop: %v", err)
	}
	if d.partialCalled != 0 || d.baseCalled != 0 {
		t.Error("expected noop, got display calls")
	}
}

func TestRefreshManagerPartialOnDirtyWidget(t *testing.T) {
	d := &fakeDisplay{}
	rm := newRefreshManager(d)
	c := canvas.New(epd.Width, epd.Height, canvas.Rot90)
	w := NewLabel("before")
	w.SetBounds(image.Rect(0, 0, 100, 20))
	// Establish base first
	rm.RenderWith(c, []Widget{w}, true)
	d.partialCalled = 0
	// Change content so buffer differs from prevBuf → partial update expected.
	w.SetText("after")
	w.SetDirty()
	if err := rm.Render(c, []Widget{w}); err != nil {
		t.Fatalf("Render partial: %v", err)
	}
	if d.partialCalled != 1 {
		t.Errorf("expected 1 partial call, got %d", d.partialCalled)
	}
}

func TestRefreshManagerFullOnForced(t *testing.T) {
	d := &fakeDisplay{}
	rm := newRefreshManager(d)
	c := canvas.New(epd.Width, epd.Height, canvas.Rot90)
	w := NewLabel("test")
	w.SetBounds(image.Rect(0, 0, 100, 20))
	if err := rm.RenderWith(c, []Widget{w}, true); err != nil {
		t.Fatalf("RenderWith forced: %v", err)
	}
	// forced → Init(ModeFull) + DisplayBase
	if d.initCalled == 0 || d.baseCalled == 0 {
		t.Error("forced render must call Init(ModeFull)+DisplayBase")
	}
}

func TestRefreshManagerSkipsPartialWhenBufferUnchanged(t *testing.T) {
	d := &fakeDisplay{}
	rm := newRefreshManager(d)
	c := canvas.New(epd.Width, epd.Height, canvas.Rot90)
	w := NewLabel("hello")
	w.SetBounds(image.Rect(0, 0, 100, 20))
	rm.RenderWith(c, []Widget{w}, true) // establishes base + prevBuf

	// Re-render identical content: buffer unchanged → no SPI call.
	beforePartial := d.partialCalled
	w.SetDirty()
	rm.Render(c, []Widget{w})
	if d.partialCalled != beforePartial {
		t.Errorf("partial called %d times on unchanged buffer, want 0", d.partialCalled-beforePartial)
	}

	// Change content: buffer differs → SPI call expected.
	w.SetText("world")
	w.SetDirty()
	rm.Render(c, []Widget{w})
	if d.partialCalled != beforePartial+1 {
		t.Errorf("partial called %d times after buffer change, want 1", d.partialCalled-beforePartial)
	}
}


// ── navigator tests ───────────────────────────────────────────────────────────

func TestNavigatorPushRendersScene(t *testing.T) {
	d := &fakeDisplay{}
	nav := NewNavigator(d)
	l := NewLabel("hello")
	l.SetBounds(image.Rect(0, 0, 100, 20))
	s := &Scene{Widgets: []Widget{l}}
	if err := nav.Push(s); err != nil {
		t.Fatalf("Push: %v", err)
	}
	if d.initCalled == 0 {
		t.Error("Push must trigger full refresh (Init called)")
	}
}

func TestNavigatorPopRestoresPreviousScene(t *testing.T) {
	d := &fakeDisplay{}
	nav := NewNavigator(d)
	s1 := &Scene{Widgets: []Widget{NewLabel("s1")}}
	s2 := &Scene{Widgets: []Widget{NewLabel("s2")}}
	nav.Push(s1)
	nav.Push(s2)
	if err := nav.Pop(); err != nil {
		t.Fatalf("Pop: %v", err)
	}
	if d.initCalled < 2 {
		t.Error("Pop must trigger full refresh")
	}
}

func TestNavigatorPopOnSingleSceneIsNoop(t *testing.T) {
	d := &fakeDisplay{}
	nav := NewNavigator(d)
	s := &Scene{Widgets: []Widget{NewLabel("root")}}
	nav.Push(s)
	if err := nav.Pop(); err != nil {
		t.Fatalf("Pop on single scene: %v", err)
	}
}

func TestNavigatorTouchRoutingCallsHandler(t *testing.T) {
	d := &fakeDisplay{}
	nav := NewNavigator(d)
	clicked := false
	btn := NewButton("OK")
	btn.OnClick(func() { clicked = true })
	// Logical coords: 250 wide x 122 tall. Place button at logical (10,10)-(60,30).
	btn.SetBounds(image.Rect(10, 10, 60, 30))
	s := &Scene{Widgets: []Widget{btn}}
	nav.Push(s)
	// logX = pt.Y → want inside [10,60), so pt.Y=35
	// logY = pt.X → want inside [10,30), so pt.X=20
	nav.handleTouch(touch.TouchPoint{X: 20, Y: 35})
	if !clicked {
		t.Error("touch should route to button and fire OnClick")
	}
}

func TestNavigatorTouchDebounce(t *testing.T) {
	d := &fakeDisplay{}
	nav := NewNavigator(d)
	count := 0
	btn := NewButton("OK")
	btn.OnClick(func() { count++ })
	btn.SetBounds(image.Rect(0, 0, 250, 122)) // full screen
	s := &Scene{Widgets: []Widget{btn}}
	nav.Push(s)
	// Two rapid touches — second should be debounced
	nav.handleTouch(touch.TouchPoint{X: 50, Y: 50})
	nav.handleTouch(touch.TouchPoint{X: 50, Y: 50})
	if count > 1 {
		t.Errorf("rapid touches should be debounced, got %d clicks", count)
	}
}

func TestNavigatorPushCallsHooksInOrder(t *testing.T) {
	d := &fakeDisplay{}
	nav := NewNavigator(d)
	var calls []string
	s1 := &Scene{
		Widgets: []Widget{NewLabel("s1")},
		OnLeave: func() { calls = append(calls, "s1.Leave") },
	}
	s2 := &Scene{
		Widgets: []Widget{NewLabel("s2")},
		OnEnter: func() { calls = append(calls, "s2.Enter") },
	}
	nav.Push(s1)
	calls = nil // reset after first push
	nav.Push(s2)
	if len(calls) != 2 || calls[0] != "s1.Leave" || calls[1] != "s2.Enter" {
		t.Errorf("hook order = %v, want [s1.Leave s2.Enter]", calls)
	}
}

func TestNavigatorPopCallsHooksInOrder(t *testing.T) {
	d := &fakeDisplay{}
	nav := NewNavigator(d)
	var calls []string
	s1 := &Scene{
		Widgets: []Widget{NewLabel("s1")},
		OnEnter: func() { calls = append(calls, "s1.Enter") },
	}
	s2 := &Scene{
		Widgets: []Widget{NewLabel("s2")},
		OnLeave: func() { calls = append(calls, "s2.Leave") },
	}
	nav.Push(s1)
	nav.Push(s2)
	calls = nil // reset
	nav.Pop()
	if len(calls) != 2 || calls[0] != "s2.Leave" || calls[1] != "s1.Enter" {
		t.Errorf("hook order = %v, want [s2.Leave s1.Enter]", calls)
	}
}

func TestNavigator_SwipeLeft_Pops(t *testing.T) {
	fd := &fakeDisplay{}
	nav := NewNavigator(fd)
	root := &Scene{Widgets: []Widget{}}
	sub := &Scene{Widgets: []Widget{}}
	nav.Push(root) //nolint:errcheck
	nav.Push(sub)  //nolint:errcheck

	events := make(chan touch.TouchEvent, 4)
	// Swipe left: physical Y increases (logX = 249-pt.Y → increasing Y = decreasing logX = moving left).
	events <- touch.TouchEvent{Points: []touch.TouchPoint{{X: 50, Y: 150}}}
	events <- touch.TouchEvent{Points: []touch.TouchPoint{{X: 50, Y: 200}}}
	close(events) // Run exits naturally when channel is closed and drained

	ctx := context.Background()
	nav.Run(ctx, events) // blocks until events closed and drained

	if nav.Depth() != 1 {
		t.Errorf("expected depth 1 after swipe-left pop, got %d", nav.Depth())
	}
}

func TestNavigator_SlowTap_NotLost(t *testing.T) {
	d := &fakeDisplay{}
	nav := NewNavigator(d)
	tapped := false
	btn := NewButton("ok")
	btn.OnClick(func() { tapped = true })
	s := &Scene{Widgets: []Widget{btn}}
	nav.Push(s) //nolint
	btn.SetBounds(image.Rect(0, 0, 250, 122))

	ctx, cancel := context.WithCancel(context.Background())
	events := make(chan touch.TouchEvent, 1)
	// Single tap with no second event — timer should flush it
	events <- touch.TouchEvent{Points: []touch.TouchPoint{{X: 60, Y: 10}}}
	// Give timer (300ms) time to fire, then cancel
	go func() {
		time.Sleep(400 * time.Millisecond)
		cancel()
	}()
	nav.Run(ctx, events)

	if !tapped {
		t.Error("slow single tap should not be lost")
	}
}

func TestNavigatorDepth_Empty(t *testing.T) {
	fd := &fakeDisplay{}
	nav := NewNavigator(fd)
	if got := nav.Depth(); got != 0 {
		t.Errorf("Depth() = %d, want 0", got)
	}
}

func TestNavigatorDepth_AfterPush(t *testing.T) {
	fd := &fakeDisplay{}
	nav := NewNavigator(fd)
	nav.Push(&Scene{Widgets: []Widget{}}) //nolint:errcheck
	if got := nav.Depth(); got != 1 {
		t.Errorf("Depth() after 1 push = %d, want 1", got)
	}
	nav.Push(&Scene{Widgets: []Widget{}}) //nolint:errcheck
	if got := nav.Depth(); got != 2 {
		t.Errorf("Depth() after 2 pushes = %d, want 2", got)
	}
	nav.Pop() //nolint:errcheck
	if got := nav.Depth(); got != 1 {
		t.Errorf("Depth() after pop = %d, want 1", got)
	}
}

func TestSceneTitle(t *testing.T) {
	s := &Scene{Title: "Config", Widgets: []Widget{}}
	if s.Title != "Config" {
		t.Errorf("Scene.Title = %q, want %q", s.Title, "Config")
	}
}

func TestNavigatorIdleSleep_CallsSleep(t *testing.T) {
	fd := &fakeDisplay{}
	nav := NewNavigatorWithIdle(fd, 20*time.Millisecond)
	nav.Push(&Scene{Widgets: []Widget{}}) //nolint:errcheck

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	nav.Run(ctx, nil)

	if fd.sleepCalled == 0 {
		t.Error("expected Sleep() to be called after idle timeout")
	}
}

func TestNavigatorIdleReset_OnTouch(t *testing.T) {
	fd := &fakeDisplay{}
	nav := NewNavigatorWithIdle(fd, 500*time.Millisecond) // long enough to never fire in test
	nav.Push(&Scene{Widgets: []Widget{}}) //nolint:errcheck

	tc := make(chan touch.TouchEvent, 5)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	go func() {
		time.Sleep(30 * time.Millisecond)
		tc <- touch.TouchEvent{Points: []touch.TouchPoint{{X: 10, Y: 10}}}
		time.Sleep(30 * time.Millisecond)
		tc <- touch.TouchEvent{Points: []touch.TouchPoint{{X: 10, Y: 10}}}
	}()

	nav.Run(ctx, tc)

	// idleTimeout=500ms, last touch at ~60ms → timer would fire at ~560ms.
	// Context expires at 200ms → Run exits before timer ever fires.
	if fd.sleepCalled > 0 {
		t.Error("Sleep should not be called when touch events reset the timer")
	}
}

func TestNavigator_HScrollable_SwipeLeft(t *testing.T) {
	fd := &fakeDisplay{}
	nav := NewNavigator(fd)
	hs := &mockHScrollable{}
	hs.SetBounds(image.Rect(0, 0, 200, 100))
	nav.Push(&Scene{Widgets: []Widget{hs}}) //nolint:errcheck

	tc := make(chan touch.TouchEvent, 10)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	go func() {
		// Swipe left: physical Y increases (logX = 249-pt.Y → increasing Y = decreasing logX = moving left).
		tc <- touch.TouchEvent{Points: []touch.TouchPoint{{X: 50, Y: 150}}}
		time.Sleep(50 * time.Millisecond)
		tc <- touch.TouchEvent{Points: []touch.TouchPoint{{X: 50, Y: 200}}} // ΔY=+50
	}()
	nav.Run(ctx, tc)

	if hs.scrolled >= 0 {
		t.Errorf("expected negative cumulative scroll (ScrollH(-1) called), got %d", hs.scrolled)
	}
}

// TestSwipe_NoDoubleScroll verifies that a slow swipe (multiple events, each consecutive
// pair exceeding the 30px threshold) triggers ScrollH exactly once, not multiple times.
// Regression test for the "swipeConsumed" fix.
func TestSwipe_NoDoubleScroll(t *testing.T) {
	fd := &fakeDisplay{}
	nav := NewNavigator(fd)
	hs := &mockHScrollable{}
	hs.SetBounds(image.Rect(0, 0, 200, 100))
	nav.Push(&Scene{Widgets: []Widget{hs}}) //nolint:errcheck

	tc := make(chan touch.TouchEvent, 10)
	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()

	go func() {
		// Four consecutive events: pairs [1→2] and [3→4] each exceed the 30px threshold.
		// Without swipeConsumed guard, this would fire ScrollH(-1) twice.
		tc <- touch.TouchEvent{Points: []touch.TouchPoint{{X: 50, Y: 100}}}
		time.Sleep(20 * time.Millisecond)
		tc <- touch.TouchEvent{Points: []touch.TouchPoint{{X: 50, Y: 140}}} // ΔY=+40 → swipe
		time.Sleep(20 * time.Millisecond)
		tc <- touch.TouchEvent{Points: []touch.TouchPoint{{X: 50, Y: 170}}} // still moving
		time.Sleep(20 * time.Millisecond)
		tc <- touch.TouchEvent{Points: []touch.TouchPoint{{X: 50, Y: 210}}} // ΔY=+40 from prev
	}()
	nav.Run(ctx, tc)

	if hs.scrolled != -1 {
		t.Errorf("expected exactly one left scroll (scrolled=-1), got %d", hs.scrolled)
	}
}

func TestNavigator_NoHScrollable_SwipeLeft_Pops(t *testing.T) {
	fd := &fakeDisplay{}
	nav := NewNavigator(fd)
	root := &Scene{Widgets: []Widget{}}
	sub := &Scene{Widgets: []Widget{}}
	nav.Push(root) //nolint:errcheck
	nav.Push(sub)  //nolint:errcheck

	if nav.Depth() != 2 {
		t.Fatalf("expected depth 2, got %d", nav.Depth())
	}

	tc := make(chan touch.TouchEvent, 10)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	go func() {
		// Swipe left: physical Y increases (logX = 249-pt.Y → increasing Y = decreasing logX = moving left).
		tc <- touch.TouchEvent{Points: []touch.TouchPoint{{X: 50, Y: 150}}}
		time.Sleep(50 * time.Millisecond)
		tc <- touch.TouchEvent{Points: []touch.TouchPoint{{X: 50, Y: 200}}} // ΔY=+50
	}()
	nav.Run(ctx, tc)

	if nav.Depth() != 1 {
		t.Errorf("expected depth 1 after swipe-left pop, got %d", nav.Depth())
	}
}

// TestNavigatorHandleTouchPassesLogicalCoordinates verifies that HandleTouch
// receives logical (rotated) coordinates, not raw GT1151 touch coordinates.
//
// GT1151→logical transform (Rot90, epd.Width=122, epd.Height=250):
//
//	logX = pt.Y  (GT1151 Y → logical horizontal axis)
//	logY = pt.X  (GT1151 X → logical vertical axis, no inversion)
//
// ActionSidebar decomposes pt.Y to route a tap to the correct button cell.
// If it receives raw GT1151 X instead of logical Y, the wrong cell is selected.
func TestNavigatorHandleTouchPassesLogicalCoordinates(t *testing.T) {
	d := &fakeDisplay{}
	nav := NewNavigator(d)

	tapped := -1
	sidebar := NewActionSidebar(
		SidebarButton{OnTap: func() { tapped = 0 }},
		SidebarButton{OnTap: func() { tapped = 1 }},
	)
	// Sidebar at full logical height: 122px split into two 61px cells.
	// Cell 0: logical Y 0..60, Cell 1: logical Y 61..121.
	sidebar.SetBounds(image.Rect(0, 0, 44, 122))

	nav.Push(&Scene{Widgets: []Widget{sidebar}})

	// We want to hit logical (22, 90) — inside cell 1 (Y=90 > 61).
	// GT1151 coords: logX=pt.Y → pt.Y=22; logY=pt.X → pt.X=90.
	nav.handleTouch(touch.TouchPoint{X: 90, Y: 22})

	// With the bug (pt.X=22 used as logY): idx = 22/61 = 0 → wrong cell.
	// With the fix (logY=pt.X=90 used): idx = 90/61 = 1 → correct.
	if tapped != 1 {
		t.Errorf("touch at logical Y=90 should tap cell 1, got %d (coordinate mismatch?)", tapped)
	}
}

// --- PopTo tests ---

func TestPopTo_NopWhenAlreadyAtDepth(t *testing.T) {
	d := &fakeDisplay{}
	nav := NewNavigator(d)
	nav.Push(&Scene{Widgets: []Widget{NewLabel("root")}})
	nav.Push(&Scene{Widgets: []Widget{NewLabel("a")}})
	before := d.baseCalled
	nav.PopTo(2) // already at depth 2 — noop
	if nav.Depth() != 2 {
		t.Errorf("depth = %d, want 2", nav.Depth())
	}
	if d.baseCalled != before {
		t.Errorf("PopTo at current depth triggered render (%d extra calls)", d.baseCalled-before)
	}
}

func TestPopTo_PopsToRequestedDepth(t *testing.T) {
	d := &fakeDisplay{}
	nav := NewNavigator(d)
	nav.Push(&Scene{Widgets: []Widget{NewLabel("root")}})
	nav.Push(&Scene{Widgets: []Widget{NewLabel("a")}})
	nav.Push(&Scene{Widgets: []Widget{NewLabel("b")}})
	nav.Push(&Scene{Widgets: []Widget{NewLabel("c")}})
	// depth is 4; pop to 1 (root only)
	nav.PopTo(1)
	if got := nav.Depth(); got != 1 {
		t.Errorf("depth after PopTo(1) = %d, want 1", got)
	}
}

func TestPopTo_RendersExactlyOnce(t *testing.T) {
	d := &fakeDisplay{}
	nav := NewNavigator(d)
	nav.Push(&Scene{Widgets: []Widget{NewLabel("root")}})
	nav.Push(&Scene{Widgets: []Widget{NewLabel("a")}})
	nav.Push(&Scene{Widgets: []Widget{NewLabel("b")}})
	nav.Push(&Scene{Widgets: []Widget{NewLabel("c")}})
	// 4 pushes = 4 baseCalled so far
	before := d.baseCalled
	nav.PopTo(1) // pops 3 scenes — must produce exactly 1 render
	if got := d.baseCalled - before; got != 1 {
		t.Errorf("PopTo rendered %d times, want exactly 1", got)
	}
}

func TestPopTo_CallsOnLeaveForEachPoppedScene(t *testing.T) {
	d := &fakeDisplay{}
	nav := NewNavigator(d)
	var left []string
	nav.Push(&Scene{Widgets: []Widget{NewLabel("root")}})
	nav.Push(&Scene{Widgets: []Widget{NewLabel("a")}, OnLeave: func() { left = append(left, "a") }})
	nav.Push(&Scene{Widgets: []Widget{NewLabel("b")}, OnLeave: func() { left = append(left, "b") }})
	nav.Push(&Scene{Widgets: []Widget{NewLabel("c")}, OnLeave: func() { left = append(left, "c") }})
	// Push calls OnLeave on the previous top; reset to count only PopTo's calls.
	left = nil
	nav.PopTo(1)
	if len(left) != 3 {
		t.Errorf("OnLeave called %d times, want 3; left=%v", len(left), left)
	}
}

func TestPopTo_CallsOnEnterForNewTop(t *testing.T) {
	d := &fakeDisplay{}
	nav := NewNavigator(d)
	entered := false
	nav.Push(&Scene{
		Widgets: []Widget{NewLabel("root")},
		OnEnter: func() { entered = true },
	})
	nav.Push(&Scene{Widgets: []Widget{NewLabel("a")}})
	nav.Push(&Scene{Widgets: []Widget{NewLabel("b")}})
	entered = false // reset after initial pushes
	nav.PopTo(1)
	if !entered {
		t.Error("OnEnter not called for root scene after PopTo(1)")
	}
}

// drainDispatch executes one pending Dispatch function synchronously.
// For use in tests only — simulates the Run() event loop draining dispatchFn.
func (nav *Navigator) drainDispatch() {
	select {
	case fn := <-nav.dispatchFn:
		fn()
	default:
	}
}
