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

func main() {
	log.SetFlags(0)

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
