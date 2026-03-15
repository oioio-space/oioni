# gui — retained-mode GUI framework for the Waveshare 2.13" Touch e-Paper HAT

[![Go Reference](https://pkg.go.dev/badge/github.com/oioio-space/oioni/ui/gui.svg)](https://pkg.go.dev/github.com/oioio-space/oioni/ui/gui)

`gui` is a minimal, retained-mode widget toolkit built for the
**Waveshare 2.13" Touch e-Paper HAT** (250×122 px, black/white) running on a
**Raspberry Pi Zero 2W** under [gokrazy](https://gokrazy.org).

It handles scene navigation, touch routing with debounce, and automatic
anti-ghosting refreshes — letting application code focus on layout and logic.

> **Requires** the `drivers/epd` and `drivers/touch` modules from this
> repository (or any type that satisfies the `gui.Display` interface).

## Install

```sh
go get github.com/oioio-space/oioni/ui/gui
```

## Quick start

```go
package main

import (
    "context"

    "github.com/oioio-space/oioni/drivers/epd"
    "github.com/oioio-space/oioni/drivers/touch"
    "github.com/oioio-space/oioni/ui/gui"
)

func main() {
    // Open hardware.
    d, err := epd.New(epd.Config{
        SPIDevice: "/dev/spidev0.0", SPISpeed: 4_000_000,
        PinRST: 17, PinDC: 25, PinCS: 8, PinBUSY: 24,
    })
    if err != nil {
        panic(err)
    }
    defer d.Close()

    tc, err := touch.Open(touch.Config{I2CDevice: "/dev/i2c-1", PinINT: 4})
    if err != nil {
        panic(err)
    }
    defer tc.Close()

    // Build the navigator (manages scene stack + refresh strategy).
    nav := gui.NewNavigator(d)

    // Build a screen from widgets and layout containers.
    lbl := gui.NewLabel("Hello, e-ink!")
    lbl.SetAlign(gui.AlignCenter)

    count := 0
    btn := gui.NewButton("Tap me")
    btn.OnClick(func() {
        count++
        lbl.SetText(fmt.Sprintf("Tapped %d times", count))
    })

    screen := &gui.Scene{
        Widgets: []gui.Widget{
            gui.NewVBox(
                gui.NewStatusBar("oioni", ""),
                gui.NewDivider(),
                gui.Expand(lbl),
                gui.FixedSize(btn, 32),
            ),
        },
        OnEnter: func() { lbl.SetText("Hello, e-ink!") },
    }

    // Push the first screen (triggers a full refresh).
    if err := nav.Push(screen); err != nil {
        panic(err)
    }

    // Run blocks until the context is cancelled, routing touch events.
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    nav.Run(ctx, tc.Events())

    d.Sleep()
}
```

## Widgets

| Widget | Constructor | Notes |
|--------|-------------|-------|
| Label | `NewLabel(text)` | Single line; `SetAlign` for left/center/right |
| Button | `NewButton(label)` | `OnClick(fn)` callback; touch feedback via inversion |
| ProgressBar | `NewProgressBar()` | `SetValue(0.0–1.0)`; use `Expand()` for full width |
| StatusBar | `NewStatusBar(left, right)` | Black bar, white text; left defaults to current time |
| Spacer | `NewSpacer()` | Invisible; use with `Expand()` for flexible gaps |
| Divider | `NewDivider()` | 1 px separator; horizontal in VBox, vertical in HBox |

## Layout

| Container | Constructor | Notes |
|-----------|-------------|-------|
| VBox | `NewVBox(children...)` | Vertical stack |
| HBox | `NewHBox(children...)` | Horizontal row |
| Fixed | `NewFixed(w, h)` + `Put(w, x, y)` | Absolute pixel positioning |
| Overlay | `NewOverlay(content, align)` | Float content over a scene |
| WithPadding | `WithPadding(px, w)` | Uniform padding on all 4 sides |

Wrap a child with `Expand(w)` to fill remaining space, or `FixedSize(w, px)`
to pin its main-axis size.

## Refresh behavior

Partial refreshes (~0.3 s) are used for routine widget updates. A full
refresh (~2 s) is forced on `Push`/`Pop` and automatically every 50 partial
updates to clear ghosting. The anti-ghost interval is controlled internally
by `refreshManager`.

## Custom widgets

Embed `gui.BaseWidget` and implement `Draw`, `PreferredSize`, and `MinSize`:

```go
type MyWidget struct {
    gui.BaseWidget
    text string
}

func (w *MyWidget) Draw(c *canvas.Canvas) {
    c.DrawText(w.Bounds().Min.X, w.Bounds().Min.Y, w.text,
               canvas.EmbeddedFont(12), canvas.Black)
}
func (w *MyWidget) PreferredSize() image.Point { return image.Pt(80, 20) }
func (w *MyWidget) MinSize() image.Point       { return image.Pt(20, 20) }
```

Implement `gui.Touchable` to receive touch events:

```go
func (w *MyWidget) HandleTouch(pt touch.TouchPoint) bool {
    // handle touch; return true to consume the event
    return true
}
```

## License

MIT — see [LICENSE](../../LICENSE) at the repository root.
