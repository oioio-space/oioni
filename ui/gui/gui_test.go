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
	if d.PreferredSize().Y != 1 {
		t.Errorf("Divider PreferredSize.Y = %d, want 1", d.PreferredSize().Y)
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
	initCalled    int
	baseCalled    int
	partialCalled int
	fastCalled    int
	lastMode      epd.Mode
}

func (f *fakeDisplay) Init(m epd.Mode) error         { f.initCalled++; f.lastMode = m; return nil }
func (f *fakeDisplay) DisplayBase(b []byte) error    { f.baseCalled++; return nil }
func (f *fakeDisplay) DisplayPartial(b []byte) error { f.partialCalled++; return nil }
func (f *fakeDisplay) DisplayFast(b []byte) error    { f.fastCalled++; return nil }
func (f *fakeDisplay) Sleep() error                  { return nil }
func (f *fakeDisplay) Close() error                  { return nil }

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
	w := NewLabel("test")
	w.SetBounds(image.Rect(0, 0, 100, 20))
	// Establish base first
	rm.RenderWith(c, []Widget{w}, true)
	d.partialCalled = 0
	// Dirty widget → partial update
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

func TestRefreshManagerAntiGhostCounter(t *testing.T) {
	d := &fakeDisplay{}
	rm := newRefreshManager(d)
	rm.antiGhostN = 3 // low threshold for test
	c := canvas.New(epd.Width, epd.Height, canvas.Rot90)
	w := NewLabel("test")
	w.SetBounds(image.Rect(0, 0, 100, 20))
	// First forced to establish base
	rm.RenderWith(c, []Widget{w}, true)
	initBefore := d.initCalled
	// Run N partial updates — on the Nth, a full refresh must occur
	for i := 0; i <= rm.antiGhostN; i++ {
		w.SetDirty()
		rm.Render(c, []Widget{w})
	}
	if d.initCalled <= initBefore {
		t.Errorf("expected anti-ghost full refresh after %d partial updates", rm.antiGhostN)
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
	// logX = clamp(pt.Y, 0, 249) = 20 → want inside [10,60)
	// logY = clamp((122-1)-pt.X, 0, 121) = 121-106 = 15 → want inside [10,30)
	// So pt.Y=20, pt.X=106
	nav.handleTouch(touch.TouchPoint{X: 106, Y: 20})
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
	d := &fakeDisplay{}
	nav := NewNavigator(d)

	s1 := &Scene{Widgets: []Widget{NewLabel("one")}}
	s2 := &Scene{Widgets: []Widget{NewLabel("two")}}
	nav.Push(s1) //nolint
	nav.Push(s2) //nolint

	if len(nav.stack) != 2 {
		t.Fatalf("expected 2 scenes, got %d", len(nav.stack))
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events := make(chan touch.TouchEvent, 2)
	events <- touch.TouchEvent{Points: []touch.TouchPoint{{X: 100, Y: 60}}}
	events <- touch.TouchEvent{Points: []touch.TouchPoint{{X: 60, Y: 60}}} // ΔX=-40

	done := make(chan struct{})
	go func() {
		nav.Run(ctx, events)
		close(done)
	}()

	// Wait for swipe to be processed (stack shrinks to 1)
	deadline := time.After(1 * time.Second)
	for {
		if len(nav.stack) == 1 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timeout waiting for swipe to process")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
	cancel()
	<-done
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
