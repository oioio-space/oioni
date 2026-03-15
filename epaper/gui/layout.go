// epaper/gui/layout.go — layout containers: VBox, HBox, Fixed, Overlay, WithPadding
package gui

import (
	"image"

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

// ── VBox ──────────────────────────────────────────────────────────────────────

// VBox stacks children vertically.
// Children may be Widget or layoutHint (created by Expand/FixedSize).
type VBox struct {
	BaseWidget
	children []layoutHint
}

func NewVBox(children ...any) *VBox {
	v := &VBox{}
	for _, c := range children {
		v.children = append(v.children, hintFor(c))
	}
	v.SetDirty()
	return v
}

func (v *VBox) PreferredSize() image.Point {
	w, h := 0, 0
	for _, ch := range v.children {
		ps := ch.widget.PreferredSize()
		if ps.X > w {
			w = ps.X
		}
		h += ps.Y
	}
	return image.Pt(w, h)
}

func (v *VBox) MinSize() image.Point {
	w, h := 0, 0
	for _, ch := range v.children {
		ms := ch.widget.MinSize()
		if ms.X > w {
			w = ms.X
		}
		h += ms.Y
	}
	return image.Pt(w, h)
}

func (v *VBox) SetBounds(r image.Rectangle) {
	v.BaseWidget.SetBounds(r)
	v.doLayout(r)
}

func (v *VBox) doLayout(r image.Rectangle) {
	totalH := r.Dy()
	used, expandCount := 0, 0
	for _, ch := range v.children {
		if ch.expand {
			expandCount++
		} else if ch.fixed > 0 {
			used += ch.fixed
		} else {
			ps := ch.widget.PreferredSize()
			ms := ch.widget.MinSize()
			h := ps.Y
			if h < ms.Y {
				h = ms.Y
			}
			if _, ok := ch.widget.(Touchable); ok && h < touchableMin {
				h = touchableMin
			}
			used += h
		}
	}
	expandH := 0
	if expandCount > 0 && totalH > used {
		expandH = (totalH - used) / expandCount
	}
	y := r.Min.Y
	for _, ch := range v.children {
		var h int
		if ch.expand {
			h = expandH
		} else if ch.fixed > 0 {
			h = ch.fixed
		} else {
			ps := ch.widget.PreferredSize()
			ms := ch.widget.MinSize()
			h = ps.Y
			if h < ms.Y {
				h = ms.Y
			}
		}
		if _, ok := ch.widget.(Touchable); ok && h < touchableMin {
			h = touchableMin
		}
		ch.widget.SetBounds(image.Rect(r.Min.X, y, r.Max.X, y+h))
		y += h
	}
}

func (v *VBox) Draw(c *canvas.Canvas) {
	for _, ch := range v.children {
		ch.widget.Draw(c)
	}
}

func (v *VBox) IsDirty() bool {
	if v.BaseWidget.IsDirty() {
		return true
	}
	for _, ch := range v.children {
		if ch.widget.IsDirty() {
			return true
		}
	}
	return false
}

func (v *VBox) MarkClean() {
	v.BaseWidget.MarkClean()
	for _, ch := range v.children {
		ch.widget.MarkClean()
	}
}

// ── HBox ──────────────────────────────────────────────────────────────────────

// HBox distributes children horizontally.
type HBox struct {
	BaseWidget
	children []layoutHint
}

func NewHBox(children ...any) *HBox {
	h := &HBox{}
	for _, c := range children {
		h.children = append(h.children, hintFor(c))
	}
	h.SetDirty()
	return h
}

func (h *HBox) PreferredSize() image.Point {
	w, ht := 0, 0
	for _, ch := range h.children {
		ps := ch.widget.PreferredSize()
		w += ps.X
		if ps.Y > ht {
			ht = ps.Y
		}
	}
	return image.Pt(w, ht)
}

func (h *HBox) MinSize() image.Point {
	w, ht := 0, 0
	for _, ch := range h.children {
		ms := ch.widget.MinSize()
		w += ms.X
		if ms.Y > ht {
			ht = ms.Y
		}
	}
	return image.Pt(w, ht)
}

func (h *HBox) SetBounds(r image.Rectangle) {
	h.BaseWidget.SetBounds(r)
	h.doLayout(r)
}

func (h *HBox) doLayout(r image.Rectangle) {
	totalW := r.Dx()
	used, expandCount := 0, 0
	for _, ch := range h.children {
		if ch.expand {
			expandCount++
		} else if ch.fixed > 0 {
			used += ch.fixed
		} else {
			ps := ch.widget.PreferredSize()
			ms := ch.widget.MinSize()
			w := ps.X
			if w < ms.X {
				w = ms.X
			}
			if _, ok := ch.widget.(Touchable); ok && w < touchableMin {
				w = touchableMin
			}
			used += w
		}
	}
	expandW := 0
	if expandCount > 0 && totalW > used {
		expandW = (totalW - used) / expandCount
	}
	x := r.Min.X
	for _, ch := range h.children {
		var w int
		if ch.expand {
			w = expandW
		} else if ch.fixed > 0 {
			w = ch.fixed
		} else {
			ps := ch.widget.PreferredSize()
			ms := ch.widget.MinSize()
			w = ps.X
			if w < ms.X {
				w = ms.X
			}
		}
		if _, ok := ch.widget.(Touchable); ok && w < touchableMin {
			w = touchableMin
		}
		ch.widget.SetBounds(image.Rect(x, r.Min.Y, x+w, r.Max.Y))
		x += w
	}
}

func (h *HBox) Draw(c *canvas.Canvas) {
	for _, ch := range h.children {
		ch.widget.Draw(c)
	}
}

func (h *HBox) IsDirty() bool {
	if h.BaseWidget.IsDirty() {
		return true
	}
	for _, ch := range h.children {
		if ch.widget.IsDirty() {
			return true
		}
	}
	return false
}

func (h *HBox) MarkClean() {
	h.BaseWidget.MarkClean()
	for _, ch := range h.children {
		ch.widget.MarkClean()
	}
}

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
		cw, ch2 := ps.X, ps.Y
		if cw < ms.X {
			cw = ms.X
		}
		if ch2 < ms.Y {
			ch2 = ms.Y
		}
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
	if f.BaseWidget.IsDirty() {
		return true
	}
	for _, ch := range f.children {
		if ch.widget.IsDirty() {
			return true
		}
	}
	return false
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
	cw, ch := ps.X, ps.Y
	if cw < ms.X {
		cw = ms.X
	}
	if ch < ms.Y {
		ch = ms.Y
	}
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
