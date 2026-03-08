// hello/main.go
package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"awesomeProject/usbgadget"
	"awesomeProject/usbgadget/functions"
)

const diskImg = "/perm/disk.img"
const diskSize = 64 << 20 // 64 MiB

func ensureDiskImg() {
	if _, err := os.Stat(diskImg); err == nil {
		return
	}
	f, err := os.Create(diskImg)
	if err != nil {
		log.Printf("create %s: %v", diskImg, err)
		return
	}
	if err := f.Truncate(diskSize); err != nil {
		log.Printf("truncate %s: %v", diskImg, err)
	}
	f.Close()
	log.Printf("created %s (%d MiB sparse)", diskImg, diskSize>>20)
}

func main() {
	log.SetFlags(0)

	ensureDiskImg()

	g, err := usbgadget.New(
		usbgadget.WithName("geekhouse"),
		usbgadget.WithVendorID(0x1d6b, 0x0104),
		usbgadget.WithStrings("0x409", "GeekHouse", "oioio Composite", "pi0001"),
		usbgadget.WithRNDIS(),
		usbgadget.WithECM(),
		usbgadget.WithHID(functions.Keyboard()),
		usbgadget.WithMassStorage("/perm/disk.img"),
	)
	if err != nil {
		log.Fatalf("usbgadget.New: %v", err)
	}

	if err := g.Enable(); err != nil {
		log.Fatalf("gadget.Enable: %v", err)
	}
	log.Println("USB composite gadget actif : RNDIS + ECM + HID Keyboard + MassStorage")

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGTERM, syscall.SIGINT)
	<-ch

	log.Println("Arrêt du gadget USB...")
	if err := g.Disable(); err != nil {
		log.Printf("gadget.Disable: %v", err)
	}
}
