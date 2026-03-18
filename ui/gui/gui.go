// epaper/gui/gui.go — core interfaces and BaseWidget
package gui

import (
	"image"
	"sync/atomic"

	"github.com/oioio-space/oioni/ui/canvas"
	"github.com/oioio-space/oioni/drivers/epd"
	"github.com/oioio-space/oioni/drivers/touch"
)

// Display is the subset of *epd.Display used by Navigator.
// *epd.Display satisfies this interface.
// Note: DisplayFull is intentionally excluded — it only writes the 0x24 RAM bank,
// not the 0x26 reference frame, so subsequent DisplayPartial calls would ghost.
type Display interface {
	Init(m epd.Mode) error
	DisplayBase(buf []byte) error    // full refresh: writes 0x24 + 0x26 RAM banks
	DisplayPartial(buf []byte) error // partial refresh: full 4000-byte buffer, self-contained
	DisplayFast(buf []byte) error    // fast full refresh
	Sleep() error
	Close() error
}

// Widget is the core interface every GUI element must implement.
type Widget interface {
	Draw(c *canvas.Canvas)
	Bounds() image.Rectangle
	SetBounds(r image.Rectangle)
	PreferredSize() image.Point // intrinsic preferred size; (0,0) = no preference
	MinSize() image.Point       // minimum allocation; layout enforces this floor
	IsDirty() bool
	SetDirty()
	MarkClean()
}

// Touchable is implemented by interactive widgets.
// Navigator calls HandleTouch after hit-testing and debounce.
type Touchable interface {
	HandleTouch(pt touch.TouchPoint) bool // true = event consumed
}

// Stoppable is implemented by widgets that own background goroutines.
// Navigator.Pop() calls Stop() recursively on all widgets in a popped scene.
type Stoppable interface {
	Stop()
}

// scrollable is package-internal. Navigator.Run() calls Scroll on widgets
// that implement it when a SwipeUp or SwipeDown gesture is detected.
type scrollable interface {
	Scroll(dy int)
}

// hScrollable is package-internal. Navigator.Run() routes horizontal swipes to
// the first widget at the TOP LEVEL of Scene.Widgets that implements this interface.
// Widgets nested inside layout containers (VBox, HBox…) are NOT found — hScrollable
// widgets must be direct members of Scene.Widgets.
type hScrollable interface {
	ScrollH(delta int)
}

// ContextMenuProvider is an optional exported interface any widget can implement.
// Navigator checks for it on long-press (>500ms). Currently a no-op hook point:
// Navigator will type-assert silently if the feature is not yet wired.
// Exported so widgets in cmd/oioni can implement it for future use.
type ContextMenuProvider interface {
	ContextMenu() []ContextMenuItem
}

// ContextMenuItem describes one entry in a future context menu.
type ContextMenuItem struct {
	Label  string
	Icon   *Icon // optional, nil = text-only
	Action func()
}

// BaseWidget provides dirty-flag and bounds bookkeeping.
// Embed in custom widgets and override Draw, PreferredSize, MinSize.
//
//	type MyWidget struct {
//	    gui.BaseWidget
//	    // your fields
//	}
//
//	func (w *MyWidget) Draw(c *canvas.Canvas) { /* draw using w.Bounds() */ }
//	func (w *MyWidget) PreferredSize() image.Point { return image.Pt(60, 20) }
//	func (w *MyWidget) MinSize() image.Point       { return image.Pt(20, 20) }
type BaseWidget struct {
	bounds image.Rectangle
	dirty  atomic.Bool
}

func (b *BaseWidget) Bounds() image.Rectangle     { return b.bounds }
func (b *BaseWidget) SetBounds(r image.Rectangle) { b.bounds = r; b.dirty.Store(true) }
func (b *BaseWidget) IsDirty() bool               { return b.dirty.Load() }
func (b *BaseWidget) SetDirty()                   { b.dirty.Store(true) }
func (b *BaseWidget) MarkClean()                  { b.dirty.Store(false) }
func (b *BaseWidget) PreferredSize() image.Point  { return image.Point{} }
func (b *BaseWidget) MinSize() image.Point        { return image.Point{} }
