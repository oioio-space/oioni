package gui

// textInputState holds the mutable state of a ShowTextInput scene.
type textInputState struct {
	text   []rune
	maxLen int
}

func (s *textInputState) append(r rune) {
	if s.maxLen > 0 && len(s.text) >= s.maxLen {
		return
	}
	s.text = append(s.text, r)
}

func (s *textInputState) backspace() {
	if len(s.text) > 0 {
		s.text = s.text[:len(s.text)-1]
	}
}

func (s *textInputState) String() string { return string(s.text) }
func (s *textInputState) Len() int       { return len(s.text) }
