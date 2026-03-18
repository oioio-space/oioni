package gui

import (
	"image"

	"github.com/oioio-space/oioni/drivers/epd"
	"github.com/oioio-space/oioni/ui/canvas"
)

// ShowAlert pushes a modal alert scene with a title, message, and buttons.
// If no buttons are provided, a single "OK" button that pops the scene is added.
func ShowAlert(nav *Navigator, title, message string, buttons ...AlertButton) {
	if len(buttons) == 0 {
		buttons = []AlertButton{{Label: "OK"}}
	}

	// Ensure each button pops after its callback.
	btns := make([]AlertButton, len(buttons))
	copy(btns, buttons)
	for i := range btns {
		orig := btns[i].OnPress
		btns[i].OnPress = func() {
			if orig != nil {
				orig()
			}
			nav.Pop() //nolint
		}
	}

	titleLbl := NewLabel(title)
	titleLbl.SetFont(canvas.EmbeddedFont(12))
	titleLbl.SetAlign(AlignCenter)

	msgLbl := NewLabel(message)
	msgLbl.SetFont(canvas.EmbeddedFont(8))
	msgLbl.SetAlign(AlignCenter)

	btnWidgets := make([]any, 0, len(btns))
	for _, ab := range btns {
		ab := ab
		btn := NewButton(ab.Label)
		btn.OnClick(ab.OnPress)
		btnWidgets = append(btnWidgets, btn)
	}

	content := NewVBox(
		FixedSize(titleLbl, 20),
		Expand(msgLbl),
		FixedSize(NewHBox(btnWidgets...), 24),
	)

	ov := NewOverlay(content, AlignCenter)
	ov.setScreen(epd.Height, epd.Width) // logical screen after Rot90: 250×122

	_ = nav.Push(&Scene{Widgets: []Widget{ov}})
}

// ShowMenu pushes a scrollable menu scene with an optional title.
func ShowMenu(nav *Navigator, title string, items []MenuItem) {
	menu := NewMenu(items)

	var top Widget
	if title != "" {
		top = NewVBox(FixedSize(NewStatusBar(title, ""), 16), Expand(menu))
	} else {
		top = NewVBox(Expand(menu))
	}
	// SetBounds required: Navigator does not set bounds automatically.
	top.SetBounds(image.Rect(0, 0, epd.Height, epd.Width))

	scene := &Scene{
		// top renders everything; menu listed at top level so Navigator can route
		// vertical swipes to it via the scrollable interface.
		Widgets: []Widget{top, menu},
		OnEnter: func() { menu.SetDirty() },
	}
	_ = nav.Push(scene)
}

// ShowTextInput pushes a keyboard scene. onConfirm is called with the entered
// string when the user taps [OK]. Swipe left cancels without calling onConfirm.
func ShowTextInput(nav *Navigator, placeholder string, maxLen int, onConfirm func(string)) {
	state := &textInputState{maxLen: maxLen}

	// Header label showing current text.
	header := newTextInputHeader(state, placeholder)

	kb := newKeyboard(defaultKeyboardConfig(
		maxLen,
		state.Len,
		func(r rune) {
			state.append(r)
			header.refresh(state, placeholder)
		},
		func() {
			state.backspace()
			header.refresh(state, placeholder)
		},
		func() {
			onConfirm(state.String())
			nav.Pop() //nolint
		},
	))

	// Header row: text display (Expand) + [⌫] + [OK]
	backBtn := NewButton("⌫")
	backBtn.OnClick(func() { kb.Back() })
	okBtn := NewButton("OK")
	okBtn.OnClick(func() { kb.Confirm() })

	headerRow := NewHBox(
		Expand(header),
		FixedSize(backBtn, 30),
		FixedSize(okBtn, 30),
	)

	vbox := NewVBox(
		FixedSize(headerRow, 22),
		Expand(kb),
	)
	// SetBounds required: Navigator does not set bounds automatically.
	vbox.SetBounds(image.Rect(0, 0, epd.Height, epd.Width))

	scene := &Scene{Widgets: []Widget{vbox}}
	_ = nav.Push(scene)
}

// textInputHeader is a Label that shows the current text + cursor.
type textInputHeader struct {
	*Label
}

func newTextInputHeader(state *textInputState, placeholder string) *textInputHeader {
	text := placeholder
	if state.Len() > 0 {
		text = state.String() + "│"
	}
	h := &textInputHeader{Label: NewLabel(text)}
	h.SetFont(canvas.EmbeddedFont(12))
	return h
}

func (h *textInputHeader) refresh(state *textInputState, placeholder string) {
	if state.Len() == 0 {
		h.SetText(placeholder)
		return
	}
	text := state.String() + "│"
	// Truncate from left if overflowing (keep last N chars).
	maxRunes := 18
	runes := []rune(text)
	if len(runes) > maxRunes {
		runes = runes[len(runes)-maxRunes:]
	}
	h.SetText(string(runes))
}
