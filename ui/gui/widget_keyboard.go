package gui

import (
	"image"
	"unicode/utf8"

	"github.com/oioio-space/oioni/drivers/touch"
	"github.com/oioio-space/oioni/ui/canvas"
)

// keyboardConfig configures the internal keyboard widget.
type keyboardConfig struct {
	Rows      []string
	MaxLen    int
	Current   func() int // returns current text length (for MaxLen check)
	OnKey     func(rune)
	OnBack    func()
	OnConfirm func()
}

// defaultKeyboardConfig returns a standard alphanumeric layout.
func defaultKeyboardConfig(maxLen int, current func() int, onKey func(rune), onBack, onConfirm func()) keyboardConfig {
	return keyboardConfig{
		Rows: []string{
			"ABCDEFGHIJ",
			"KLMNOPQRST",
			"UVWXYZ!@#$",
			"0123456789",
			"_-./:=?+( ",
		},
		MaxLen:    maxLen,
		Current:   current,
		OnKey:     onKey,
		OnBack:    onBack,
		OnConfirm: onConfirm,
	}
}

// keyboardWidget is a package-internal reusable keyboard grid widget.
type keyboardWidget struct {
	BaseWidget
	cfg keyboardConfig
}

func newKeyboard(cfg keyboardConfig) *keyboardWidget {
	kb := &keyboardWidget{cfg: cfg}
	kb.SetDirty()
	return kb
}

// Back calls OnBack directly.
func (kb *keyboardWidget) Back() {
	if kb.cfg.OnBack != nil {
		kb.cfg.OnBack()
	}
}

// Confirm calls OnConfirm directly.
func (kb *keyboardWidget) Confirm() {
	if kb.cfg.OnConfirm != nil {
		kb.cfg.OnConfirm()
	}
}

func (kb *keyboardWidget) PreferredSize() image.Point { return image.Pt(250, 100) }
func (kb *keyboardWidget) MinSize() image.Point       { return image.Pt(100, 40) }

func (kb *keyboardWidget) keySize() (keyW, keyH int) {
	r := kb.Bounds()
	if r.Empty() || len(kb.cfg.Rows) == 0 {
		return 1, 1
	}
	maxCols := 0
	for _, row := range kb.cfg.Rows {
		n := utf8.RuneCountInString(row)
		if n > maxCols {
			maxCols = n
		}
	}
	if maxCols == 0 {
		maxCols = 1
	}
	return r.Dx() / maxCols, r.Dy() / len(kb.cfg.Rows)
}

func (kb *keyboardWidget) Draw(c *canvas.Canvas) {
	r := kb.Bounds()
	if r.Empty() {
		return
	}
	c.DrawRect(r, canvas.White, true)
	keyW, keyH := kb.keySize()
	f := canvas.EmbeddedFont(12)

	atMax := kb.cfg.MaxLen > 0 && kb.cfg.Current != nil && kb.cfg.Current() >= kb.cfg.MaxLen

	for row, chars := range kb.cfg.Rows {
		for col, ch := range chars {
			x := r.Min.X + col*keyW
			y := r.Min.Y + row*keyH
			keyR := image.Rect(x, y, x+keyW, y+keyH)
			c.DrawRect(keyR, canvas.Black, false)
			if f != nil {
				label := string(ch)
				if ch == ' ' {
					label = "SP"
				}
				_, gw, _ := f.Glyph(ch)
				tx := x + (keyW-gw)/2
				ty := y + (keyH-f.LineHeight())/2
				if atMax && ch != ' ' {
					c.DrawText(tx, ty, label, f, canvas.White)
				} else {
					c.DrawText(tx, ty, label, f, canvas.Black)
				}
			}
		}
	}
	kb.MarkClean()
}

func (kb *keyboardWidget) HandleTouch(pt touch.TouchPoint) bool {
	r := kb.Bounds()
	if r.Empty() {
		return false
	}
	keyW, keyH := kb.keySize()
	if keyW == 0 || keyH == 0 {
		return false
	}
	col := (int(pt.X) - r.Min.X) / keyW
	row := (int(pt.Y) - r.Min.Y) / keyH
	if row < 0 || row >= len(kb.cfg.Rows) {
		return false
	}
	runes := []rune(kb.cfg.Rows[row])
	if col < 0 || col >= len(runes) {
		return false
	}
	ch := runes[col]
	if kb.cfg.MaxLen > 0 && kb.cfg.Current != nil && kb.cfg.Current() >= kb.cfg.MaxLen {
		return true // consumed but no-op
	}
	if kb.cfg.OnKey != nil {
		kb.cfg.OnKey(ch)
	}
	return true
}
