package gui

import (
	"image"
	"testing"
)

func TestQRCode_DrawDoesNotPanic(t *testing.T) {
	q := NewQRCode("https://oioni.local")
	q.SetBounds(image.Rect(0, 0, 80, 80))
	c := newTestCanvas()
	q.Draw(c)
}

func TestQRCode_EmptyData(t *testing.T) {
	q := NewQRCode("")
	q.SetBounds(image.Rect(0, 0, 40, 40))
	c := newTestCanvas()
	q.Draw(c) // must not panic
}

func TestQRCode_SetDataRegenerates(t *testing.T) {
	q := NewQRCode("hello")
	q.SetBounds(image.Rect(0, 0, 60, 60))
	c1 := newTestCanvas()
	q.Draw(c1)
	b1 := make([]byte, len(c1.Bytes()))
	copy(b1, c1.Bytes())

	q.SetData("different content that produces a different QR code")
	c2 := newTestCanvas()
	q.Draw(c2)

	same := true
	for i := range b1 {
		if b1[i] != c2.Bytes()[i] {
			same = false
			break
		}
	}
	if same {
		t.Error("expected different QR code after SetData")
	}
}
