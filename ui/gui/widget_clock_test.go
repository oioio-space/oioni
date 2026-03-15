package gui

import (
	"image"
	"testing"
	"time"
)

func TestClock_DrawDoesNotPanic(t *testing.T) {
	clk := NewClock()
	clk.SetBounds(image.Rect(0, 0, 60, 24))
	c := newTestCanvas()
	clk.Draw(c)
	clk.Stop()
}

func TestClock_StopPreventsSetDirty(t *testing.T) {
	clk := NewClockFull()
	clk.SetBounds(image.Rect(0, 0, 80, 24))
	clk.MarkClean()
	clk.Stop()
	// After Stop, no more ticks should fire.
	time.Sleep(1200 * time.Millisecond)
	// IsDirty should remain false (no tick fired after Stop)
	if clk.IsDirty() {
		t.Error("ClockWidget set dirty after Stop() — goroutine not cancelled")
	}
}

func TestClock_ImplementsStoppable(t *testing.T) {
	clk := NewClock()
	var _ Stoppable = clk // compile-time check
	clk.Stop()
}
