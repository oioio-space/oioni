// epaper/gui/navigator.go — scene stack, touch routing, refresh coordination
package gui

import (
	"context"
	"image"
	"sync"
	"time"

	"github.com/oioio-space/oioni/ui/canvas"
	"github.com/oioio-space/oioni/drivers/epd"
	"github.com/oioio-space/oioni/drivers/touch"
)

const debounce = 200 * time.Millisecond

// Scene is a screen's widget tree and optional lifecycle hooks.
type Scene struct {
	Title   string   // metadata for NavBar; Navigator does not read it
	Widgets []Widget
	OnEnter func() // called when scene becomes active
	OnLeave func() // called when scene is popped
}

// SwipeDir is the direction of a swipe gesture (reserved for future use).
type SwipeDir int

const (
	SwipeLeft SwipeDir = iota
	SwipeRight
	SwipeUp
	SwipeDown
)

// Navigator manages a stack of Scenes and coordinates touch routing + refresh.
//
// Concurrency: Push, Pop, and Render are NOT concurrent-safe with Run().
// In tests, call these methods directly; in production, they must be called
// from inside scene callbacks (OnEnter/OnLeave) or before Run().
type Navigator struct {
	display     Display
	rm          *refreshManager
	canvas      *canvas.Canvas
	stack       []*Scene
	mu          sync.Mutex
	lastFire    map[Widget]time.Time
	renderCh     chan struct{}  // buffered(1): non-blocking RequestRender
	wakeCh       chan struct{}  // buffered(1): non-blocking Wake()
	regenerateCh chan struct{}  // buffered(1): non-blocking RequestRegenerate (black→white purge)
	idleTimeout  time.Duration // 0 = no idle sleep
}

// NewNavigator creates a Navigator. The Display must outlive the Navigator.
func NewNavigator(d Display) *Navigator {
	c := canvas.New(epd.Width, epd.Height, canvas.Rot90)
	return &Navigator{
		display:  d,
		rm:       newRefreshManager(d),
		canvas:   c,
		lastFire: make(map[Widget]time.Time),
		renderCh:     make(chan struct{}, 1),
		wakeCh:       make(chan struct{}, 1),
		regenerateCh: make(chan struct{}, 1),
	}
}

// NewNavigatorWithIdle creates a Navigator with idle sleep after idleTimeout of inactivity.
// After idleTimeout, display.Sleep() is called. On next touch or Wake(), display is re-initialized.
func NewNavigatorWithIdle(d Display, idleTimeout time.Duration) *Navigator {
	nav := NewNavigator(d)
	nav.idleTimeout = idleTimeout
	return nav
}

// Depth returns the number of scenes on the stack (0=empty, 1=root only).
func (nav *Navigator) Depth() int { return len(nav.stack) }

// RequestRender triggers a re-render. Non-blocking: if a render is already queued, this is a no-op.
func (nav *Navigator) RequestRender() {
	select {
	case nav.renderCh <- struct{}{}:
	default:
	}
}

// Wake wakes the display if sleeping and triggers a full refresh. Non-blocking.
func (nav *Navigator) Wake() {
	select {
	case nav.wakeCh <- struct{}{}:
	default:
	}
}

// RequestRegenerate triggers a black→white purge cycle before re-rendering.
// Non-blocking: if already queued, this is a no-op. Slower than Wake() (~4s).
// Intended for 24h keep-alive calls to prevent display degradation.
func (nav *Navigator) RequestRegenerate() {
	select {
	case nav.regenerateCh <- struct{}{}:
	default:
	}
}

// FastRender renders dirty widgets using DisplayFast with automatic base-sync tracking.
// After maxFastBeforeBase consecutive calls, automatically falls back to full refresh.
func (nav *Navigator) FastRender() error {
	if len(nav.stack) == 0 {
		return nil
	}
	return nav.rm.FastRefresh(nav.canvas, nav.stack[len(nav.stack)-1].Widgets)
}

// Push adds a scene to the stack and triggers a forced full refresh.
func (nav *Navigator) Push(s *Scene) error {
	if len(nav.stack) > 0 {
		top := nav.stack[len(nav.stack)-1]
		if top.OnLeave != nil {
			top.OnLeave()
		}
	}
	nav.stack = append(nav.stack, s)
	if s.OnEnter != nil {
		s.OnEnter()
	}
	return nav.rm.RenderWith(nav.canvas, s.Widgets, true)
}

// stopWidgets recursively calls Stop() on any widget implementing Stoppable,
// walking into layout containers via Children() []Widget.
func stopWidgets(widgets []Widget) {
	type hasChildren interface{ Children() []Widget }
	for _, w := range widgets {
		if s, ok := w.(Stoppable); ok {
			s.Stop()
		}
		if c, ok := w.(hasChildren); ok {
			stopWidgets(c.Children())
		}
	}
}

// teardownScene calls OnLeave, stops all widgets, and prunes debounce state.
func (nav *Navigator) teardownScene(s *Scene) {
	if s.OnLeave != nil {
		s.OnLeave()
	}
	stopWidgets(s.Widgets)
	nav.mu.Lock()
	for _, w := range s.Widgets {
		delete(nav.lastFire, w)
	}
	nav.mu.Unlock()
}

// Pop removes the top scene and restores the previous one.
// If only one scene is on the stack, Pop is a noop.
func (nav *Navigator) Pop() error {
	if len(nav.stack) <= 1 {
		return nil
	}
	top := nav.stack[len(nav.stack)-1]
	nav.teardownScene(top)
	nav.stack = nav.stack[:len(nav.stack)-1]
	prev := nav.stack[len(nav.stack)-1]
	if prev.OnEnter != nil {
		prev.OnEnter()
	}
	return nav.rm.RenderWith(nav.canvas, prev.Widgets, true)
}

// PopTo pops scenes until len(stack) == depth, calling OnLeave for each removed
// scene (top-first) and rendering exactly once for the new top.
// If depth >= current depth, it is a noop. depth is clamped to at least 1.
func (nav *Navigator) PopTo(depth int) error {
	if depth < 1 {
		depth = 1
	}
	if len(nav.stack) <= depth {
		return nil
	}
	for i := len(nav.stack) - 1; i >= depth; i-- {
		nav.teardownScene(nav.stack[i])
	}
	nav.stack = nav.stack[:depth]
	top := nav.stack[depth-1]
	if top.OnEnter != nil {
		top.OnEnter()
	}
	return nav.rm.RenderWith(nav.canvas, top.Widgets, true)
}

// Render redraws the current scene's dirty widgets (partial or noop).
func (nav *Navigator) Render() error {
	if len(nav.stack) == 0 {
		return nil
	}
	return nav.rm.Render(nav.canvas, nav.stack[len(nav.stack)-1].Widgets)
}

// handleTouch maps physical touch coords → logical coords, then routes to widgets.
func (nav *Navigator) handleTouch(pt touch.TouchPoint) {
	logX := clamp((epd.Height-1)-int(pt.Y), 0, epd.Height-1)
	logY := clamp((epd.Width-1)-int(pt.X), 0, epd.Width-1)
	logPt := image.Pt(logX, logY)

	if len(nav.stack) == 0 {
		return
	}
	scene := nav.stack[len(nav.stack)-1]
	for i := len(scene.Widgets) - 1; i >= 0; i-- {
		w := scene.Widgets[i]
		if !logPt.In(w.Bounds()) {
			continue
		}
		t, ok := w.(Touchable)
		if !ok {
			continue
		}
		nav.mu.Lock()
		last := nav.lastFire[w]
		now := time.Now()
		if now.Sub(last) < debounce {
			nav.mu.Unlock()
			continue
		}
		nav.lastFire[w] = now
		nav.mu.Unlock()

		logTp := touch.TouchPoint{ID: pt.ID, X: uint16(logPt.X), Y: uint16(logPt.Y), Size: pt.Size}
		if t.HandleTouch(logTp) {
			break
		}
	}
}

// Run starts the touch event loop and blocks until ctx is cancelled.
// After Run returns, call display.Sleep() then display.Close().
func (nav *Navigator) Run(ctx context.Context, events <-chan touch.TouchEvent) {
	var swipePt *touch.TouchPoint
	var swipeTimer *time.Timer
	timerCh := func() <-chan time.Time {
		if swipeTimer != nil {
			return swipeTimer.C
		}
		return nil
	}
	flush := func() {
		if swipePt != nil {
			nav.handleTouch(*swipePt)
			swipePt = nil
		}
		if swipeTimer != nil {
			swipeTimer.Stop()
			swipeTimer = nil
		}
	}

	// Idle sleep state
	sleeping := false
	var idleTimer *time.Timer
	var idleTimerCh <-chan time.Time

	drainTimer := func(t *time.Timer) {
		if !t.Stop() {
			select {
			case <-t.C:
			default:
			}
		}
	}

	resetIdle := func() {
		if nav.idleTimeout <= 0 {
			return
		}
		if idleTimer != nil {
			drainTimer(idleTimer)
		}
		idleTimer = time.NewTimer(nav.idleTimeout)
		idleTimerCh = idleTimer.C
	}
	if nav.idleTimeout > 0 {
		resetIdle()
	}

	// nil events channel would block forever in select; replace with never-firing channel
	touchEvents := events
	if touchEvents == nil {
		touchEvents = make(chan touch.TouchEvent)
	}

	for {
		select {
		case <-ctx.Done():
			flush()
			return

		case <-idleTimerCh:
			if !sleeping {
				sleeping = true
				_ = nav.display.Sleep()
			}
			idleTimerCh = nil // prevent re-firing until reset

		case <-nav.regenerateCh:
			// Black→white purge cycle, then full re-render. Used by 24h keep-alive.
			sleeping = false
			_ = nav.display.DisplayRegenerate()
			if len(nav.stack) > 0 {
				_ = nav.rm.RenderWith(nav.canvas, nav.stack[len(nav.stack)-1].Widgets, true)
			}
			resetIdle()

		case <-nav.wakeCh:
			if sleeping {
				sleeping = false
				_ = nav.display.Init(epd.ModeFull)
				if len(nav.stack) > 0 {
					_ = nav.rm.RenderWith(nav.canvas, nav.stack[len(nav.stack)-1].Widgets, true)
				}
			}
			resetIdle()

		case <-nav.renderCh:
			nav.Render() //nolint:errcheck

		case <-timerCh():
			if swipePt != nil {
				nav.handleTouch(*swipePt)
				swipePt = nil
			}
			swipeTimer = nil

		case ev, ok := <-touchEvents:
			if !ok {
				flush()
				return
			}
			// Wake display if sleeping before routing any touch
			if sleeping {
				sleeping = false
				_ = nav.display.Init(epd.ModeFull)
				if len(nav.stack) > 0 {
					_ = nav.rm.RenderWith(nav.canvas, nav.stack[len(nav.stack)-1].Widgets, true)
				}
			}
			// Reset idle timer on every touch
			resetIdle()

			for _, pt := range ev.Points {
				if swipePt == nil {
					cp := pt
					swipePt = &cp
					swipeTimer = time.NewTimer(300 * time.Millisecond)
					continue
				}
				swipeTimer.Stop()
				swipeTimer = nil
				firstPt := *swipePt
				swipePt = nil
				// Physical→logical: logX = pt.Y, logY = 121 - pt.X
				// Horizontal (left/right) corresponds to physical Y; vertical to physical X.
				dx := int(firstPt.Y) - int(pt.Y)
				dy := int(pt.X) - int(firstPt.X)
				adx, ady := dx, dy
				if adx < 0 {
					adx = -adx
				}
				if ady < 0 {
					ady = -ady
				}
				const threshold = 30
				if adx >= ady && adx > threshold {
					if dx < 0 {
						// Swipe left: route to hScrollable or Pop
						routed := false
						if len(nav.stack) > 0 {
							for _, w := range nav.stack[len(nav.stack)-1].Widgets {
								if hs, ok := w.(hScrollable); ok {
									hs.ScrollH(-1)
									routed = true
									break
								}
							}
						}
						if !routed {
							nav.Pop() //nolint:errcheck
						}
					} else {
						// Swipe right: route to hScrollable
						if len(nav.stack) > 0 {
							for _, w := range nav.stack[len(nav.stack)-1].Widgets {
								if hs, ok := w.(hScrollable); ok {
									hs.ScrollH(+1)
									break
								}
							}
						}
					}
				} else if ady > adx && ady > threshold {
					if len(nav.stack) > 0 {
						for _, w := range nav.stack[len(nav.stack)-1].Widgets {
							if s, ok := w.(scrollable); ok {
								if dy < 0 {
									s.Scroll(-1)
								} else {
									s.Scroll(1)
								}
								break
							}
						}
					}
				} else {
					nav.handleTouch(firstPt)
					nav.handleTouch(pt)
				}
			}
			nav.Render() //nolint:errcheck
		}
	}
}

func clamp(v, lo, hi int) int { return max(lo, min(v, hi)) }
