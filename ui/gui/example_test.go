//go:build ignore

// This file shows how to wire up the gui package on real hardware
// (Waveshare 2.13" Touch e-Paper HAT, Raspberry Pi Zero 2W / gokrazy).
// It is excluded from normal builds and tests with the "ignore" build tag.

package gui_test

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/oioio-space/oioni/drivers/epd"
	"github.com/oioio-space/oioni/drivers/touch"
	"github.com/oioio-space/oioni/ui/gui"
)

// ExampleNavigator demonstrates the full Navigator lifecycle:
// open hardware → build a scene → push → run the touch loop → shutdown.
func ExampleNavigator() {
	// ── Hardware setup ────────────────────────────────────────────────────────

	d, err := epd.New(epd.Config{
		SPIDevice: "/dev/spidev0.0",
		SPISpeed:  4_000_000,
		PinRST:    17, PinDC: 25, PinCS: 8, PinBUSY: 24,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "epd:", err)
		os.Exit(1)
	}
	defer d.Close()

	tc, err := touch.Open(touch.Config{I2CDevice: "/dev/i2c-1", PinINT: 4})
	if err != nil {
		fmt.Fprintln(os.Stderr, "touch:", err)
		os.Exit(1)
	}
	defer tc.Close()

	// ── Widget tree ───────────────────────────────────────────────────────────

	// A label that the button will update.
	lbl := gui.NewLabel("Tap the button")
	lbl.SetAlign(gui.AlignCenter)

	taps := 0
	btn := gui.NewButton("Tap me")
	btn.OnClick(func() {
		taps++
		lbl.SetText(fmt.Sprintf("Tapped %d time(s)", taps))
	})

	// A second scene pushed when the user taps a "Details" button.
	detailLbl := gui.NewLabel("You are on screen 2")
	detailLbl.SetAlign(gui.AlignCenter)

	// ── Scenes ────────────────────────────────────────────────────────────────

	nav := gui.NewNavigator(d)

	var detailScene *gui.Scene // forward declaration for the back button

	backBtn := gui.NewButton("Back")

	mainScene := &gui.Scene{
		Widgets: []gui.Widget{
			gui.NewVBox(
				gui.NewStatusBar("oioni", "v1"),
				gui.NewDivider(),
				gui.Expand(lbl),
				gui.NewHBox(
					gui.Expand(btn),
					gui.FixedSize(gui.NewButton("Details"), 60),
				),
			),
		},
		OnEnter: func() { lbl.SetText("Tap the button") },
	}

	_ = detailScene // referenced below
	detailScene = &gui.Scene{
		Widgets: []gui.Widget{
			gui.NewVBox(
				gui.NewStatusBar("oioni", "detail"),
				gui.NewDivider(),
				gui.Expand(detailLbl),
				gui.FixedSize(backBtn, 32),
			),
		},
	}

	backBtn.OnClick(func() {
		if err := nav.Pop(); err != nil {
			fmt.Fprintln(os.Stderr, "pop:", err)
		}
	})

	// Wire the Details button to push the second scene.
	// (In real code you would keep a reference to the Details button above;
	// simplified here for clarity.)

	// ── Run ──────────────────────────────────────────────────────────────────

	if err := nav.Push(mainScene); err != nil {
		fmt.Fprintln(os.Stderr, "push:", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	nav.Run(ctx, tc.Events())

	d.Sleep()
}
