# usbgadget — Linux USB composite gadget driver

[![Go Reference](https://pkg.go.dev/badge/github.com/oioio-space/oioni/drivers/usbgadget.svg)](https://pkg.go.dev/github.com/oioio-space/oioni/drivers/usbgadget)

Configures Linux USB composite gadgets via configfs on a Raspberry Pi Zero 2W
running [gokrazy](https://gokrazy.org). A single `Enable()` call loads the
required kernel modules, mounts configfs, builds the gadget directory tree, and
binds the DWC2 UDC — no shell scripts required.

## Install

```sh
go get github.com/oioio-space/oioni/drivers/usbgadget
```

## Quick start

RNDIS + ECM dual-stack (Windows and Linux/macOS hosts over USB):

```go
rndis := functions.RNDIS(
    functions.WithRNDISHostAddr("02:00:00:aa:bb:01"),
    functions.WithRNDISDevAddr("02:00:00:aa:bb:02"),
)
ecm := functions.ECM(
    functions.WithECMHostAddr("02:00:00:cc:dd:01"),
    functions.WithECMDevAddr("02:00:00:cc:dd:02"),
)

g, err := usbgadget.New(
    usbgadget.WithName("netgadget"),
    usbgadget.WithVendorID(0x1d6b, 0x0104),
    usbgadget.WithStrings("0x409", "ACME Corp", "USB Network", "net001"),
    usbgadget.WithFunc(rndis), // WithFunc gives access to rndis.IfName()
    usbgadget.WithFunc(ecm),
)
if err := g.Enable(); err != nil {
    log.Fatal(err)
}
defer g.Disable()

// Configure IP on the Pi-side network interface
ifName, _ := rndis.IfName()
log.Println("interface:", ifName)
```

> **RNDIS must be listed first** so Windows correctly identifies the composite device.

## EP budget (DWC2, Pi Zero 2W)

The BCM2835 DWC2 controller provides **7 usable endpoints** beyond EP0.
Plan your function selection accordingly:

| Function | EPs | Notes |
|----------|-----|-------|
| RNDIS    | 3   | Windows network (must be first) |
| ECM      | 3   | Linux/macOS network |
| NCM      | 3   | High-throughput network (Linux 3.10+) |
| EEM      | 2   | Simple bulk Ethernet |
| Subset   | 2   | Broadest compatibility |
| HID      | 1   | Keyboard / mouse / custom |
| ACM      | 3   | Serial (CDC ACM) |
| UAC1/UAC2 | varies | USB audio |
| MassStorage | 2 | USB mass storage |

**Max viable configs:**

| Config | Total EPs |
|--------|-----------|
| RNDIS + ECM + HID | 7 ✓ |
| RNDIS + MassStorage | 5 ✓ |
| RNDIS + ECM + MassStorage | 8 ✗ |

If the bind fails silently (budget exceeded), udc.go re-reads the UDC file and
returns a clear error instead of leaving the gadget inactive.

## Functions reference

Use `WithFunc(f)` when you need a reference to the function object (e.g. to call
`IfName()`, `ReadStats()`, `ReadLEDs()`, `DevPath()`). Use `WithRNDIS(...)` /
`WithECM(...)` etc. for fire-and-forget configuration.

```go
// Need IfName() later → use WithFunc
rndis := functions.RNDIS(...)
g, _ := usbgadget.New(usbgadget.WithFunc(rndis), ...)

// Don't need a reference → use convenience option
g, _ := usbgadget.New(usbgadget.WithRNDIS(), ...)
```

## License

MIT — see [LICENSE](../../LICENSE).
