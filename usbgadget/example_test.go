package usbgadget_test

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"awesomeProject/usbgadget"
	"awesomeProject/usbgadget/functions"
)

// ExampleNew_rndisEcm shows a Windows + Linux dual-stack USB network gadget.
// RNDIS must be first so Windows correctly identifies the composite device.
func ExampleNew_rndisEcm() {
	rndis := functions.RNDIS(
		functions.WithRNDISHostAddr("02:00:00:aa:bb:01"), // stable MAC → same DHCP lease
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
		usbgadget.WithHID(rndis), // WithHID accepts any Function
		usbgadget.WithHID(ecm),
	)
	if err != nil {
		log.Fatal(err)
	}
	if err := g.Enable(); err != nil {
		log.Fatal(err)
	}
	defer g.Disable()

	// After Enable(), query the Pi-side interface name.
	if ifname, err := rndis.IfName(); err == nil {
		fmt.Printf("RNDIS interface: %s\n", ifname)
	}
	if stats, err := ecm.ReadStats(); err == nil {
		fmt.Printf("ECM rx=%d tx=%d bytes\n", stats.RxBytes, stats.TxBytes)
	}
}

// ExampleNew_keyboard shows a HID keyboard that reacts to LED state from the host.
func ExampleNew_keyboard() {
	kbd := functions.Keyboard()

	g, err := usbgadget.New(
		usbgadget.WithName("kbd"),
		usbgadget.WithVendorID(0x1d6b, 0x0104),
		usbgadget.WithStrings("0x409", "ACME", "USB Keyboard", "kbd001"),
		usbgadget.WithHID(kbd),
	)
	if err != nil {
		log.Fatal(err)
	}
	if err := g.Enable(); err != nil {
		log.Fatal(err)
	}
	defer g.Disable()

	// Send: Left Shift (0x02) + 'h' (0x0b) → uppercase H.
	// Keyboard report layout: [modifier, reserved, key0..key5]
	kbd.WriteReport([]byte{0x02, 0x00, 0x0b, 0, 0, 0, 0, 0}) // press
	kbd.WriteReport([]byte{0x00, 0x00, 0x00, 0, 0, 0, 0, 0}) // release

	// Read LED state changes (NumLock, CapsLock, ScrollLock toggled by host).
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	leds, err := kbd.ReadLEDs(ctx)
	if err != nil {
		log.Fatal(err)
	}
	for state := range leds {
		fmt.Printf("CapsLock=%v NumLock=%v ScrollLock=%v\n",
			state.CapsLock, state.NumLock, state.ScrollLock)
	}
}

// ExampleNew_mouse shows a HID mouse sending movement and button events.
func ExampleNew_mouse() {
	mouse := functions.Mouse()

	g, err := usbgadget.New(
		usbgadget.WithName("mouse"),
		usbgadget.WithVendorID(0x1d6b, 0x0104),
		usbgadget.WithStrings("0x409", "ACME", "USB Mouse", "mse001"),
		usbgadget.WithHID(mouse),
	)
	if err != nil {
		log.Fatal(err)
	}
	if err := g.Enable(); err != nil {
		log.Fatal(err)
	}
	defer g.Disable()

	// Mouse report: [buttons, deltaX, deltaY, wheel]
	// Move right 10px, down 5px, no buttons
	mouse.WriteReport([]byte{0x00, 10, 5, 0})
	// Left click
	mouse.WriteReport([]byte{0x01, 0, 0, 0})
	mouse.WriteReport([]byte{0x00, 0, 0, 0}) // release
}

// ExampleNew_acmSerial shows an ACM serial gadget used as a console/shell.
func ExampleNew_acmSerial() {
	acm := functions.ACMSerial()

	g, err := usbgadget.New(
		usbgadget.WithName("serial"),
		usbgadget.WithVendorID(0x1d6b, 0x0104),
		usbgadget.WithStrings("0x409", "ACME", "USB Serial", "ser001"),
		usbgadget.WithHID(acm),
	)
	if err != nil {
		log.Fatal(err)
	}
	if err := g.Enable(); err != nil {
		log.Fatal(err)
	}
	defer g.Disable()

	// Gadget side: /dev/ttyGS0 (or ttyGS1 if another serial function exists first)
	// Host side:   /dev/ttyACM0 (Linux), COMx (Windows)
	tty, err := os.OpenFile(acm.DevPath(), os.O_RDWR, 0)
	if err != nil {
		log.Fatal(err)
	}
	defer tty.Close()

	tty.WriteString("Hello from Pi!\n")
	buf := make([]byte, 256)
	n, _ := tty.Read(buf)
	fmt.Printf("received: %s", buf[:n])
}

// ExampleNew_serial shows a generic serial (GSER) gadget — simpler than ACM.
func ExampleNew_serial() {
	gser := functions.Serial()

	g, err := usbgadget.New(
		usbgadget.WithName("gser"),
		usbgadget.WithVendorID(0x1d6b, 0x0104),
		usbgadget.WithStrings("0x409", "ACME", "USB GSER", "gser001"),
		usbgadget.WithHID(gser),
	)
	if err != nil {
		log.Fatal(err)
	}
	if err := g.Enable(); err != nil {
		log.Fatal(err)
	}
	defer g.Disable()

	fmt.Printf("GSER device: %s\n", gser.DevPath()) // e.g. /dev/ttyGS0
}

// ExampleNew_massStorage shows a USB flash drive gadget backed by a disk image.
func ExampleNew_massStorage() {
	g, err := usbgadget.New(
		usbgadget.WithName("storage"),
		usbgadget.WithVendorID(0x1d6b, 0x0104),
		usbgadget.WithStrings("0x409", "ACME", "USB Drive", "drv001"),
		usbgadget.WithMassStorage("/perm/disk.img",
			functions.WithRemovable(true),
			functions.WithReadOnly(false),
		),
	)
	if err != nil {
		log.Fatal(err)
	}
	if err := g.Enable(); err != nil {
		log.Fatal(err)
	}
	defer g.Disable()
	// Host sees /dev/sdX — partition and format it with fdisk + mkfs.
	fmt.Println("mass storage active")
}

// ExampleNew_cdrom shows a bootable read-only CD-ROM image.
func ExampleNew_cdrom() {
	g, err := usbgadget.New(
		usbgadget.WithName("cdrom"),
		usbgadget.WithVendorID(0x1d6b, 0x0104),
		usbgadget.WithStrings("0x409", "ACME", "USB CD-ROM", "cd001"),
		usbgadget.WithMassStorage("/perm/image.iso",
			functions.WithCDROM(true),
			functions.WithReadOnly(true),
		),
	)
	if err != nil {
		log.Fatal(err)
	}
	if err := g.Enable(); err != nil {
		log.Fatal(err)
	}
	defer g.Disable()
	fmt.Println("cd-rom active")
}

// ExampleNew_uac2 shows a USB speaker + microphone using Audio Class 2.
func ExampleNew_uac2() {
	g, err := usbgadget.New(
		usbgadget.WithName("audio"),
		usbgadget.WithVendorID(0x1d6b, 0x0104),
		usbgadget.WithStrings("0x409", "ACME", "USB Audio", "aud001"),
		usbgadget.WithUAC2(
			functions.WithUAC2PlaybackChannels(3), // stereo: bit0=L, bit1=R
			functions.WithUAC2PlaybackRate(48000),
			functions.WithUAC2PlaybackSampleSize(2), // 16-bit
			functions.WithUAC2CaptureChannels(3),
			functions.WithUAC2CaptureRate(48000),
			functions.WithUAC2CaptureSampleSize(2),
		),
	)
	if err != nil {
		log.Fatal(err)
	}
	if err := g.Enable(); err != nil {
		log.Fatal(err)
	}
	defer g.Disable()
	// Host: appears as standard USB audio (snd-usb-audio on Linux).
	// Play audio: aplay -D hw:CARD=<name> audio.wav
	// Record:     arecord -D hw:CARD=<name> -f S16_LE -r 48000 out.wav
	fmt.Println("uac2 audio active")
}

// ExampleNew_uac1 shows a USB Audio Class 1 gadget (wider OS compatibility).
func ExampleNew_uac1() {
	g, err := usbgadget.New(
		usbgadget.WithName("uac1"),
		usbgadget.WithVendorID(0x1d6b, 0x0104),
		usbgadget.WithStrings("0x409", "ACME", "USB Audio v1", "uac1001"),
		usbgadget.WithUAC1(
			functions.WithUAC1PlaybackChannels(3),
			functions.WithUAC1PlaybackRate(44100),
			functions.WithUAC1PlaybackSampleSize(2),
			functions.WithUAC1CaptureChannels(1), // mono mic
			functions.WithUAC1CaptureRate(44100),
			functions.WithUAC1CaptureSampleSize(2),
		),
	)
	if err != nil {
		log.Fatal(err)
	}
	if err := g.Enable(); err != nil {
		log.Fatal(err)
	}
	defer g.Disable()
	fmt.Println("uac1 audio active")
}

// ExampleNew_midi shows a USB MIDI instrument sending note events to the host.
func ExampleNew_midi() {
	g, err := usbgadget.New(
		usbgadget.WithName("midi"),
		usbgadget.WithVendorID(0x1d6b, 0x0104),
		usbgadget.WithStrings("0x409", "ACME", "USB MIDI", "mid001"),
		usbgadget.WithMIDI(
			functions.WithMIDIBufLen(512),
			functions.WithMIDIQLen(64),
		),
	)
	if err != nil {
		log.Fatal(err)
	}
	if err := g.Enable(); err != nil {
		log.Fatal(err)
	}
	defer g.Disable()

	// Send a MIDI Note On (channel 1, middle C, velocity 100), then Note Off.
	midiDev, err := os.OpenFile("/dev/snd/midiC0D0", os.O_WRONLY, 0)
	if err != nil {
		log.Fatal(err)
	}
	defer midiDev.Close()
	midiDev.Write([]byte{0x90, 60, 100}) // note on
	time.Sleep(500 * time.Millisecond)
	midiDev.Write([]byte{0x80, 60, 0}) // note off
}

// ExampleNew_printer shows a USB printer gadget that reads raw print data.
func ExampleNew_printer() {
	g, err := usbgadget.New(
		usbgadget.WithName("printer"),
		usbgadget.WithVendorID(0x1d6b, 0x0104),
		usbgadget.WithStrings("0x409", "ACME", "USB Printer", "prt001"),
		usbgadget.WithPrinter(
			functions.WithPrinterPnP("MFG:ACME;MDL:Pi Printer;CMD:PCL;CLS:PRINTER;"),
			functions.WithPrinterQLen(20),
		),
	)
	if err != nil {
		log.Fatal(err)
	}
	if err := g.Enable(); err != nil {
		log.Fatal(err)
	}
	defer g.Disable()

	// Print jobs (PCL, PostScript, PDF, raw) arrive on /dev/usb/lp0.
	lp, err := os.Open("/dev/usb/lp0")
	if err != nil {
		log.Fatal(err)
	}
	defer lp.Close()
	buf := make([]byte, 4096)
	n, _ := lp.Read(buf)
	fmt.Printf("print job: %d bytes\n", n)
}

// ExampleNew_loopback shows a USB loopback gadget for bandwidth benchmarking.
func ExampleNew_loopback() {
	g, err := usbgadget.New(
		usbgadget.WithName("lb"),
		usbgadget.WithVendorID(0x1d6b, 0x0104),
		usbgadget.WithStrings("0x409", "ACME", "USB Loopback", "lb001"),
		usbgadget.WithLoopback(
			functions.WithLoopbackBufLen(65536), // 64 KiB for high throughput
			functions.WithLoopbackQLen(64),
		),
	)
	if err != nil {
		log.Fatal(err)
	}
	if err := g.Enable(); err != nil {
		log.Fatal(err)
	}
	defer g.Disable()
	// Test with: modprobe usbtest on the host, then run testusb -D /dev/bus/usb/...
	fmt.Println("loopback active")
}

// ExampleNew_eem shows a CDC EEM network gadget (Linux-to-Linux, minimal overhead).
func ExampleNew_eem() {
	eem := functions.EEM(
		functions.WithEEMDevAddr("02:00:00:ee:00:01"),
		functions.WithEEMHostAddr("02:00:00:ee:00:02"),
	)

	g, err := usbgadget.New(
		usbgadget.WithName("eem"),
		usbgadget.WithVendorID(0x1d6b, 0x0104),
		usbgadget.WithStrings("0x409", "ACME", "USB EEM", "eem001"),
		usbgadget.WithHID(eem),
	)
	if err != nil {
		log.Fatal(err)
	}
	if err := g.Enable(); err != nil {
		log.Fatal(err)
	}
	defer g.Disable()

	if ifname, err := eem.IfName(); err == nil {
		fmt.Printf("EEM interface: %s\n", ifname)
	}
}

// ExampleNew_obex shows an OBEX file-transfer gadget.
func ExampleNew_obex() {
	obex := functions.OBEX()

	g, err := usbgadget.New(
		usbgadget.WithName("obex"),
		usbgadget.WithVendorID(0x1d6b, 0x0104),
		usbgadget.WithStrings("0x409", "ACME", "USB OBEX", "obex001"),
		usbgadget.WithHID(obex),
	)
	if err != nil {
		log.Fatal(err)
	}
	if err := g.Enable(); err != nil {
		log.Fatal(err)
	}
	defer g.Disable()

	// On the host: obexftp --usb 0 --get file.txt
	// On the gadget, implement the OBEX server on obex.DevPath()
	fmt.Printf("OBEX device: %s\n", obex.DevPath())
}

// ExampleNew_composite shows a full composite gadget combining major functions.
func ExampleNew_composite() {
	rndis := functions.RNDIS()
	kbd := functions.Keyboard()
	acm := functions.ACMSerial()

	g, err := usbgadget.New(
		usbgadget.WithName("composite"),
		usbgadget.WithVendorID(0x1d6b, 0x0104),
		usbgadget.WithStrings("0x409", "ACME", "USB Composite", "comp001"),
		// Network — RNDIS must be first for Windows
		usbgadget.WithHID(rndis),
		usbgadget.WithECM(),
		// Input
		usbgadget.WithHID(kbd),
		usbgadget.WithHID(functions.Mouse()),
		// Storage
		usbgadget.WithMassStorage("/perm/disk.img"),
		// Serial console
		usbgadget.WithHID(acm),
		// Audio
		usbgadget.WithUAC2(),
	)
	if err != nil {
		log.Fatal(err)
	}
	if err := g.Enable(); err != nil {
		log.Fatal(err)
	}
	defer g.Disable()

	// Monitor network + read LED state concurrently
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		leds, _ := kbd.ReadLEDs(ctx)
		for state := range leds {
			fmt.Printf("CapsLock=%v\n", state.CapsLock)
		}
	}()

	go func() {
		for range time.Tick(time.Second) {
			stats, _ := rndis.ReadStats()
			fmt.Printf("rx=%d tx=%d\n", stats.RxBytes, stats.TxBytes)
		}
	}()

	<-ctx.Done()
}
