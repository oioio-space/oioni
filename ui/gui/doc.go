// Package gui is a retained-mode GUI framework for the Waveshare 2.13"
// Touch e-Paper HAT (250×122 px, black/white).
//
// The package targets gokrazy on a Raspberry Pi Zero 2W but has no
// hard dependency on the runtime; the [Display] interface can be satisfied
// by any compatible EPD driver, and touch events are delivered over a plain
// Go channel.
//
// # Architecture
//
// The three main concepts are Widgets, Scenes, and the Navigator.
//
//   - A [Widget] is anything that can draw itself onto a [canvas.Canvas] and
//     report a preferred and minimum size. Embed [BaseWidget] in custom types
//     to get the dirty-flag and bounds bookkeeping for free.
//
//   - A [Scene] is a flat slice of Widgets that form one logical screen, plus
//     optional OnEnter/OnLeave lifecycle hooks.
//
//   - The [Navigator] owns a stack of Scenes. [Navigator.Push] and
//     [Navigator.Pop] transition between screens; [Navigator.Run] drives the
//     touch event loop until the context is cancelled.
//
// # Refresh strategy
//
// The navigator uses a smart partial/full refresh engine. Touch events trigger
// a partial refresh (~0.3 s, no ghost flash) for dirty widgets. Every 50
// partial updates the engine performs a full refresh automatically to clear
// accumulated ghosting — no application code required.
//
// # Built-in widgets
//
//   - [NewLabel] — single line of text with optional alignment
//   - [NewButton] — pressable widget with touch feedback and an OnClick callback
//   - [NewProgressBar] — horizontal fill bar (0.0 – 1.0)
//   - [NewStatusBar] — full-width black bar with left/right text (defaults to clock)
//   - [NewSpacer] — invisible flexible gap; use with [Expand] in a box layout
//   - [NewDivider] — 1 px separator line
//
// # Layout containers
//
//   - [NewVBox] — stacks children vertically
//   - [NewHBox] — arranges children horizontally
//   - [NewFixed] — places children at absolute pixel positions
//   - [NewOverlay] — positions content over the current scene with alignment
//   - [WithPadding] — wraps any widget with uniform padding
//
// Use [Expand] to let a child take all remaining space in a box, and
// [FixedSize] to pin a child to a specific pixel size.
//
// # Typical usage
//
//	d, _ := epd.New(epd.Config{ /* SPI pins */ })
//	tc, _ := touch.Open(touch.Config{ /* I2C */ })
//	events := tc.Events()
//
//	nav := gui.NewNavigator(d)
//
//	lbl  := gui.NewLabel("Hello, e-ink!")
//	btn  := gui.NewButton("Refresh")
//	btn.OnClick(func() { lbl.SetText("Refreshed!") })
//
//	screen := &gui.Scene{
//	    Widgets: []gui.Widget{
//	        gui.NewVBox(
//	            gui.NewStatusBar("oioni", ""),
//	            gui.Expand(lbl),
//	            gui.FixedSize(btn, 32),
//	        ),
//	    },
//	}
//	nav.Push(screen)
//
//	ctx, cancel := context.WithCancel(context.Background())
//	defer cancel()
//	nav.Run(ctx, events)
//	d.Sleep()
//	d.Close()
package gui
