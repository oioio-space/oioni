// epaper/gui/layout.go — layout containers: VBox, HBox, Fixed, Overlay, WithPadding
package gui

import (
	"image"
	"slices"

	"awesomeProject/epaper/canvas"
)

// Alignment controls Overlay positioning.
type Alignment int

const (
	AlignCenter Alignment = iota
	AlignTop
	AlignBottom
	AlignLeft
	AlignRight
)

// layoutHint wraps a Widget with sizing constraints for HBox/VBox.
type layoutHint struct {
	widget Widget
	fixed  int  // > 0: fixed size in px on main axis; 0 = use PreferredSize
	expand bool // true: take remaining space equally with other expand widgets
}

// Expand makes w take all remaining space in HBox/VBox main axis.
func Expand(w Widget) layoutHint { return layoutHint{widget: w, expand: true} }

// FixedSize constrains w to px pixels in the main axis of HBox/VBox.
func FixedSize(w Widget, px int) layoutHint { return layoutHint{widget: w, fixed: px} }

func hintFor(v any) layoutHint {
	switch h := v.(type) {
	case layoutHint:
		return h
	case Widget:
		return layoutHint{widget: h}
	default:
		panic("gui: HBox/VBox child must be Widget or layoutHint")
	}
}

// touchableMin is the minimum size enforced for Touchable widgets.
const touchableMin = 20

// ── box (shared VBox/HBox implementation) ─────────────────────────────────────

// box is the shared layout engine for VBox (vertical=true) and HBox (vertical=false).
type box struct {
	BaseWidget
	vertical bool
	children []layoutHint
}

func newBox(vertical bool, children ...any) box {
	b := box{vertical: vertical}
	for _, c := range children {
		b.children = append(b.children, hintFor(c))
	}
	b.SetDirty()
	return b
}

// mainOf extracts the main-axis component of pt (Y if vertical, X otherwise).
func (b *box) mainOf(pt image.Point) int {
	if b.vertical {
		return pt.Y
	}
	return pt.X
}

// crossOf extracts the cross-axis component of pt (X if vertical, Y otherwise).
func (b *box) crossOf(pt image.Point) int {
	if b.vertical {
		return pt.X
	}
	return pt.Y
}

// mkPt builds a point with the given main and cross components.
func (b *box) mkPt(main, cross int) image.Point {
	if b.vertical {
		return image.Pt(cross, main)
	}
	return image.Pt(main, cross)
}

// childRect builds the child bounding rectangle at pos with the given main-axis size.
func (b *box) childRect(r image.Rectangle, pos, size int) image.Rectangle {
	if b.vertical {
		return image.Rect(r.Min.X, pos, r.Max.X, pos+size)
	}
	return image.Rect(pos, r.Min.Y, pos+size, r.Max.Y)
}

func (b *box) PreferredSize() image.Point {
	mainSum, crossMax := 0, 0
	for _, ch := range b.children {
		ps := ch.widget.PreferredSize()
		mainSum += b.mainOf(ps)
		crossMax = max(crossMax, b.crossOf(ps))
	}
	return b.mkPt(mainSum, crossMax)
}

func (b *box) MinSize() image.Point {
	mainSum, crossMax := 0, 0
	for _, ch := range b.children {
		ms := ch.widget.MinSize()
		mainSum += b.mainOf(ms)
		crossMax = max(crossMax, b.crossOf(ms))
	}
	return b.mkPt(mainSum, crossMax)
}

func (b *box) SetBounds(r image.Rectangle) {
	b.BaseWidget.SetBounds(r)
	b.doLayout(r)
}

// childMainSize returns the main-axis size for a non-expand child, applying
// the touchableMin floor for Touchable widgets.
func (b *box) childMainSize(ch layoutHint) int {
	var sz int
	if ch.fixed > 0 {
		sz = ch.fixed
	} else {
		ps := ch.widget.PreferredSize()
		ms := ch.widget.MinSize()
		sz = max(b.mainOf(ps), b.mainOf(ms))
	}
	if _, ok := ch.widget.(Touchable); ok {
		sz = max(sz, touchableMin)
	}
	return sz
}

func (b *box) doLayout(r image.Rectangle) {
	total := b.mainOf(image.Pt(r.Dx(), r.Dy()))
	startPos := b.mainOf(r.Min)

	// Compute child sizes once to avoid double-calling PreferredSize/MinSize.
	sizes := make([]int, len(b.children))
	used, expandCount := 0, 0
	for i, ch := range b.children {
		if ch.expand {
			expandCount++
		} else {
			sizes[i] = b.childMainSize(ch)
			used += sizes[i]
		}
	}
	expandSz := 0
	if expandCount > 0 && total > used {
		expandSz = (total - used) / expandCount
	}
	pos := startPos
	for i, ch := range b.children {
		sz := sizes[i]
		if ch.expand {
			sz = expandSz
		}
		ch.widget.SetBounds(b.childRect(r, pos, sz))
		pos += sz
	}
}

func (b *box) Draw(c *canvas.Canvas) {
	for _, ch := range b.children {
		ch.widget.Draw(c)
	}
}

func (b *box) IsDirty() bool {
	return b.BaseWidget.IsDirty() || slices.ContainsFunc(b.children, func(ch layoutHint) bool {
		return ch.widget.IsDirty()
	})
}

func (b *box) MarkClean() {
	b.BaseWidget.MarkClean()
	for _, ch := range b.children {
		ch.widget.MarkClean()
	}
}

// ── VBox ──────────────────────────────────────────────────────────────────────

// VBox stacks children vertically.
// Children may be Widget or layoutHint (created by Expand/FixedSize).
type VBox struct{ box }

func NewVBox(children ...any) *VBox { return &VBox{newBox(true, children...)} }

// ── HBox ──────────────────────────────────────────────────────────────────────

// HBox distributes children horizontally.
// Children may be Widget or layoutHint (created by Expand/FixedSize).
type HBox struct{ box }

func NewHBox(children ...any) *HBox { return &HBox{newBox(false, children...)} }

// ── Fixed ─────────────────────────────────────────────────────────────────────

// Fixed places children at absolute pixel positions within a fixed-size area.
type Fixed struct {
	BaseWidget
	w, h     int
	children []fixedChild
}

type fixedChild struct {
	widget Widget
	x, y   int
}

func NewFixed(w, h int) *Fixed {
	f := &Fixed{w: w, h: h}
	f.SetDirty()
	return f
}

// Put registers widget at position (x, y) relative to the Fixed container's origin.
func (f *Fixed) Put(w Widget, x, y int) {
	f.children = append(f.children, fixedChild{w, x, y})
	f.SetDirty()
}

func (f *Fixed) PreferredSize() image.Point { return image.Pt(f.w, f.h) }
func (f *Fixed) MinSize() image.Point       { return image.Pt(f.w, f.h) }

func (f *Fixed) SetBounds(r image.Rectangle) {
	f.BaseWidget.SetBounds(r)
	for _, ch := range f.children {
		ps := ch.widget.PreferredSize()
		ms := ch.widget.MinSize()
		cw, ch2 := max(ps.X, ms.X), max(ps.Y, ms.Y)
		ch.widget.SetBounds(image.Rect(r.Min.X+ch.x, r.Min.Y+ch.y,
			r.Min.X+ch.x+cw, r.Min.Y+ch.y+ch2))
	}
}

func (f *Fixed) Draw(c *canvas.Canvas) {
	for _, ch := range f.children {
		ch.widget.Draw(c)
	}
}

func (f *Fixed) IsDirty() bool {
	return f.BaseWidget.IsDirty() || slices.ContainsFunc(f.children, func(ch fixedChild) bool {
		return ch.widget.IsDirty()
	})
}

func (f *Fixed) MarkClean() {
	f.BaseWidget.MarkClean()
	for _, ch := range f.children {
		ch.widget.MarkClean()
	}
}

// ── Overlay ───────────────────────────────────────────────────────────────────

// Overlay positions content on top of the current scene.
type Overlay struct {
	BaseWidget
	content          Widget
	align            Alignment
	screenW, screenH int
}

func NewOverlay(content Widget, align Alignment) *Overlay {
	o := &Overlay{content: content, align: align}
	o.SetDirty()
	return o
}

// setScreen is called by Navigator.PushOverlay to set the screen dimensions.
func (o *Overlay) setScreen(w, h int) {
	o.screenW, o.screenH = w, h
	o.doLayout()
}

func (o *Overlay) doLayout() {
	ps := o.content.PreferredSize()
	ms := o.content.MinSize()
	cw, ch := max(ps.X, ms.X), max(ps.Y, ms.Y)
	var x, y int
	switch o.align {
	case AlignCenter:
		x = (o.screenW - cw) / 2
		y = (o.screenH - ch) / 2
	case AlignTop:
		x = (o.screenW - cw) / 2
		y = 0
	case AlignBottom:
		x = (o.screenW - cw) / 2
		y = o.screenH - ch
	case AlignLeft:
		x = 0
		y = (o.screenH - ch) / 2
	case AlignRight:
		x = o.screenW - cw
		y = (o.screenH - ch) / 2
	}
	o.content.SetBounds(image.Rect(x, y, x+cw, y+ch))
	o.BaseWidget.SetBounds(image.Rect(0, 0, o.screenW, o.screenH))
}

func (o *Overlay) Draw(c *canvas.Canvas) { o.content.Draw(c) }

func (o *Overlay) IsDirty() bool {
	return o.BaseWidget.IsDirty() || o.content.IsDirty()
}

func (o *Overlay) MarkClean() {
	o.BaseWidget.MarkClean()
	o.content.MarkClean()
}

// ── WithPadding ───────────────────────────────────────────────────────────────

type paddingWidget struct {
	BaseWidget
	inner Widget
	px    int
}

// WithPadding wraps w with uniform px padding on all 4 sides.
func WithPadding(px int, w Widget) Widget {
	p := &paddingWidget{inner: w, px: px}
	p.SetDirty()
	return p
}

func (p *paddingWidget) PreferredSize() image.Point {
	ps := p.inner.PreferredSize()
	return image.Pt(ps.X+2*p.px, ps.Y+2*p.px)
}

func (p *paddingWidget) MinSize() image.Point {
	ms := p.inner.MinSize()
	return image.Pt(ms.X+2*p.px, ms.Y+2*p.px)
}

func (p *paddingWidget) SetBounds(r image.Rectangle) {
	p.BaseWidget.SetBounds(r)
	inner := image.Rect(r.Min.X+p.px, r.Min.Y+p.px, r.Max.X-p.px, r.Max.Y-p.px)
	p.inner.SetBounds(inner)
}

func (p *paddingWidget) Draw(c *canvas.Canvas) { p.inner.Draw(c) }

func (p *paddingWidget) IsDirty() bool {
	return p.BaseWidget.IsDirty() || p.inner.IsDirty()
}

func (p *paddingWidget) MarkClean() {
	p.BaseWidget.MarkClean()
	p.inner.MarkClean()
}
