// epaper/gui/widgets.go — built-in widgets: Label, Button, ProgressBar, StatusBar, Spacer, Divider
package gui

import (
	"image"
	"time"

	"awesomeProject/epaper/canvas"
	"awesomeProject/epaper/epd"
	"awesomeProject/epaper/touch"
)

// textWidth returns the pixel width of text rendered in font f.
// Returns 0 if f is nil.
func textWidth(text string, f canvas.Font) int {
	if f == nil {
		return 0
	}
	w := 0
	for _, r := range text {
		_, gw, _ := f.Glyph(r)
		w += gw
	}
	return w
}

// ── Label ─────────────────────────────────────────────────────────────────────

// Label displays a single line of text.
type Label struct {
	BaseWidget
	text  string
	font  canvas.Font
	align Alignment
}

func NewLabel(text string) *Label {
	l := &Label{
		text: text,
		font: canvas.EmbeddedFont(12),
	}
	l.SetDirty()
	return l
}

func (l *Label) SetText(text string) {
	if l.text == text {
		return
	}
	l.text = text
	l.SetDirty()
}

func (l *Label) SetFont(f canvas.Font) { l.font = f; l.SetDirty() }
func (l *Label) SetAlign(a Alignment)  { l.align = a; l.SetDirty() }

func (l *Label) PreferredSize() image.Point {
	if l.font == nil {
		return image.Pt(0, 0)
	}
	return image.Pt(textWidth(l.text, l.font)+4, l.font.LineHeight()+4)
}

func (l *Label) MinSize() image.Point { return image.Pt(0, l.PreferredSize().Y) }

func (l *Label) Draw(c *canvas.Canvas) {
	r := l.Bounds()
	c.DrawRect(r, canvas.White, true)
	if l.font == nil || l.text == "" {
		return
	}
	lh := l.font.LineHeight()
	tw := textWidth(l.text, l.font)
	var x int
	switch l.align {
	case AlignCenter:
		x = r.Min.X + (r.Dx()-tw)/2
	case AlignRight:
		x = r.Max.X - tw - 2
	default: // AlignLeft
		x = r.Min.X + 2
	}
	y := r.Min.Y + (r.Dy()-lh)/2
	if y < r.Min.Y {
		y = r.Min.Y
	}
	c.DrawText(x, y, l.text, l.font, canvas.Black)
}

// ── Button ────────────────────────────────────────────────────────────────────

// Button is a pressable widget that fires OnClick on touch.
type Button struct {
	BaseWidget
	label   string
	font    canvas.Font
	onClick func()
	pressed bool
}

func NewButton(label string) *Button {
	b := &Button{
		label: label,
		font:  canvas.EmbeddedFont(12),
	}
	b.SetDirty()
	return b
}

func (b *Button) OnClick(fn func()) { b.onClick = fn }

func (b *Button) PreferredSize() image.Point {
	if b.font == nil {
		return image.Pt(60, 24)
	}
	return image.Pt(textWidth(b.label, b.font)+16, b.font.LineHeight()+8)
}

func (b *Button) MinSize() image.Point { return image.Pt(20, 20) }

// HandleTouch fires onClick and sets pressed state for visual feedback.
func (b *Button) HandleTouch(pt touch.TouchPoint) bool {
	b.pressed = true
	b.SetDirty()
	if b.onClick != nil {
		b.onClick()
	}
	return true
}

// Draw renders the button. If pressed, shows inverted colours and schedules
// a re-render by calling SetDirty() so the normal state is restored next frame.
func (b *Button) Draw(c *canvas.Canvas) {
	r := b.Bounds()
	bg, fg := canvas.White, canvas.Black
	if b.pressed {
		bg, fg = canvas.Black, canvas.White
		b.pressed = false
		b.SetDirty() // trigger one more render to restore normal state
	}
	c.DrawRect(r, bg, true)
	c.DrawRect(r, canvas.Black, false)
	if b.font == nil || b.label == "" {
		return
	}
	tw := textWidth(b.label, b.font)
	lh := b.font.LineHeight()
	x := r.Min.X + (r.Dx()-tw)/2
	y := r.Min.Y + (r.Dy()-lh)/2
	if y < r.Min.Y {
		y = r.Min.Y
	}
	c.DrawText(x, y, b.label, b.font, fg)
}

// ── ProgressBar ───────────────────────────────────────────────────────────────

// ProgressBar renders a horizontal fill bar (0.0 = empty, 1.0 = full).
// PreferredSize.X = 0 — use Expand() to fill available width.
type ProgressBar struct {
	BaseWidget
	value float64
}

func NewProgressBar() *ProgressBar {
	p := &ProgressBar{}
	p.SetDirty()
	return p
}

func (p *ProgressBar) SetValue(v float64) {
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}
	p.value = v
	p.SetDirty()
}

func (p *ProgressBar) PreferredSize() image.Point { return image.Pt(0, 12) }
func (p *ProgressBar) MinSize() image.Point       { return image.Pt(20, 8) }

func (p *ProgressBar) Draw(c *canvas.Canvas) {
	r := p.Bounds()
	c.DrawRect(r, canvas.White, true)
	c.DrawRect(r, canvas.Black, false)
	fillW := int(float64(r.Dx()) * p.value)
	if fillW > 0 {
		fill := image.Rect(r.Min.X, r.Min.Y, r.Min.X+fillW, r.Max.Y)
		c.DrawRect(fill, canvas.Black, true)
	}
}

// ── StatusBar ─────────────────────────────────────────────────────────────────

// StatusBar renders a full-width black bar with white left and right text.
type StatusBar struct {
	BaseWidget
	left  string
	right string
	font  canvas.Font
}

func NewStatusBar(left, right string) *StatusBar {
	s := &StatusBar{
		left:  left,
		right: right,
		font:  canvas.EmbeddedFont(12),
	}
	s.SetDirty()
	return s
}

func (s *StatusBar) SetLeft(text string)  { s.left = text; s.SetDirty() }
func (s *StatusBar) SetRight(text string) { s.right = text; s.SetDirty() }

func (s *StatusBar) PreferredSize() image.Point { return image.Pt(0, 18) }
func (s *StatusBar) MinSize() image.Point       { return image.Pt(0, 18) }

func (s *StatusBar) Draw(c *canvas.Canvas) {
	r := s.Bounds()
	c.DrawRect(r, canvas.Black, true)
	if s.font == nil {
		return
	}
	f := s.font
	left := s.left
	if left == "" {
		left = time.Now().Format("15:04")
	}
	c.DrawText(r.Min.X+2, r.Min.Y+3, left, f, canvas.White)
	if s.right != "" {
		rw := textWidth(s.right, f)
		c.DrawText(r.Max.X-rw-2, r.Min.Y+3, s.right, f, canvas.White)
	}
}

// ── Spacer ────────────────────────────────────────────────────────────────────

// Spacer is a zero-size invisible gap. Use with Expand() for flexible space:
//
//	gui.Expand(gui.NewSpacer())
type Spacer struct{ BaseWidget }

func NewSpacer() *Spacer { return &Spacer{} }
func (s *Spacer) Draw(_ *canvas.Canvas) {}

// ── Divider ───────────────────────────────────────────────────────────────────

// Divider draws a 1 px black line — horizontal in VBox, vertical in HBox.
type Divider struct{ BaseWidget }

func NewDivider() *Divider {
	d := &Divider{}
	d.SetDirty()
	return d
}

func (d *Divider) PreferredSize() image.Point { return image.Pt(0, 1) }
func (d *Divider) MinSize() image.Point       { return image.Pt(0, 1) }

func (d *Divider) Draw(c *canvas.Canvas) {
	r := d.Bounds()
	if r.Dy() >= r.Dx() {
		c.DrawLine(r.Min.X, r.Min.Y, r.Min.X, r.Max.Y, canvas.Black) // vertical
	} else {
		c.DrawLine(r.Min.X, r.Min.Y, r.Max.X, r.Min.Y, canvas.Black) // horizontal
	}
}

// Ensure widgets package compiles even without direct epd/touch usage in some paths.
var _ = epd.ModeFull
var _ = touch.TouchEvent{}
