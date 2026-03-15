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
	display  Display
	rm       *refreshManager
	canvas   *canvas.Canvas
	stack    []*Scene
	mu       sync.Mutex
	lastFire map[Widget]time.Time
}

// NewNavigator creates a Navigator. The Display must outlive the Navigator.
func NewNavigator(d Display) *Navigator {
	c := canvas.New(epd.Width, epd.Height, canvas.Rot90)
	return &Navigator{
		display:  d,
		rm:       newRefreshManager(d),
		canvas:   c,
		lastFire: make(map[Widget]time.Time),
	}
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

// Pop removes the top scene and restores the previous one.
// If only one scene is on the stack, Pop is a noop.
func (nav *Navigator) Pop() error {
	if len(nav.stack) <= 1 {
		return nil
	}
	top := nav.stack[len(nav.stack)-1]
	if top.OnLeave != nil {
		top.OnLeave()
	}
	stopWidgets(top.Widgets)
	// Prune debounce state for widgets in the popped scene.
	nav.mu.Lock()
	for _, w := range top.Widgets {
		delete(nav.lastFire, w)
	}
	nav.mu.Unlock()
	nav.stack = nav.stack[:len(nav.stack)-1]
	prev := nav.stack[len(nav.stack)-1]
	if prev.OnEnter != nil {
		prev.OnEnter()
	}
	return nav.rm.RenderWith(nav.canvas, prev.Widgets, true)
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
	logX := clamp(int(pt.Y), 0, epd.Height-1)
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

		if t.HandleTouch(pt) {
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

	for {
		select {
		case <-ctx.Done():
			flush()
			return

		case <-timerCh():
			// No second touch arrived within 300ms — treat buffered event as tap.
			if swipePt != nil {
				nav.handleTouch(*swipePt)
				swipePt = nil
			}
			swipeTimer = nil

		case ev, ok := <-events:
			if !ok {
				flush()
				return
			}
			for _, pt := range ev.Points {
				if swipePt == nil {
					// First touch: buffer, start timer.
					cp := pt
					swipePt = &cp
					swipeTimer = time.NewTimer(300 * time.Millisecond)
					continue
				}
				// Second touch within 300ms: classify.
				swipeTimer.Stop()
				swipeTimer = nil
				firstPt := *swipePt // save before clearing
				swipePt = nil
				// Cast to int — touch coords may be uint16, negative delta would wrap
				dx := int(pt.X) - int(firstPt.X)
				dy := int(pt.Y) - int(firstPt.Y)

				adx := dx
				if adx < 0 {
					adx = -adx
				}
				ady := dy
				if ady < 0 {
					ady = -ady
				}

				const threshold = 30
				if adx >= ady && adx > threshold {
					// Horizontal swipe
					if dx < 0 {
						nav.Pop() //nolint
					}
					// SwipeRight: reserved, no-op
				} else if ady > adx && ady > threshold {
					// Vertical swipe — route to scrollable top widget
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
					// Not a swipe — deliver both touches as taps
					nav.handleTouch(firstPt)
					nav.handleTouch(pt)
				}
			}
			nav.Render() //nolint:errcheck
		}
	}
}

func clamp(v, lo, hi int) int { return max(lo, min(v, hi)) }
