// hello/main.go — démo composite USB gadget sur Pi Zero 2W (gokrazy)
//
// Contrainte matérielle : DWC2 BCM2835 (Pi Zero 2W) dispose de 7 endpoints
// utilisables hors EP0. Budget par fonction :
//   RNDIS        : 3 EP (bulk in/out + interrupt)
//   ECM          : 3 EP (bulk in/out + interrupt)
//   HID keyboard : 1 EP (interrupt in)
// Total          : 7 EP — limite absolue du contrôleur.
// ACM Serial et MassStorage nécessiteraient 3 EP et 2 EP supplémentaires,
// ce qui dépasse la capacité → can't bind, err -19 (ENODEV).
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"awesomeProject/usbgadget"
	"awesomeProject/usbgadget/functions"
)

func main() {
	log.SetFlags(log.Ltime)

	// MACs stables → même interface réseau côté hôte à chaque boot,
	// même bail DHCP si le routeur mémorise les MACs.
	rndis := functions.RNDIS(
		functions.WithRNDISHostAddr("02:00:00:aa:bb:01"),
		functions.WithRNDISDevAddr("02:00:00:aa:bb:02"),
	)
	ecm := functions.ECM(
		functions.WithECMHostAddr("02:00:00:cc:dd:01"),
		functions.WithECMDevAddr("02:00:00:cc:dd:02"),
	)
	kbd := functions.Keyboard()

	g, err := usbgadget.New(
		usbgadget.WithName("geekhouse"),
		usbgadget.WithVendorID(0x1d6b, 0x0104),
		usbgadget.WithStrings("0x409", "GeekHouse", "oioio Composite", "pi0001"),
		// RNDIS en premier — Windows identifie le composite gadget correctement
		usbgadget.WithFunc(rndis),
		usbgadget.WithFunc(ecm),
		usbgadget.WithFunc(kbd),
	)
	if err != nil {
		log.Fatalf("usbgadget.New: %v", err)
	}

	if err := g.Enable(); err != nil {
		log.Printf("gadget.Enable: %v (gadget désactivé, WiFi toujours actif)", err)
		// Ne pas Fatalf : le process continue pour que gokrazy reste stable.
		// Le gadget USB sera inactif mais les autres services (WiFi, breakglass) fonctionnent.
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGTERM, syscall.SIGINT)
		<-ch
		return
	}
	log.Println("USB composite gadget actif : RNDIS + ECM + HID Keyboard")

	// Affiche les noms d'interfaces réseau côté Pi
	if ifname, err := rndis.IfName(); err == nil {
		log.Printf("RNDIS → interface Pi : %s", ifname)
	}
	if ifname, err := ecm.IfName(); err == nil {
		log.Printf("ECM   → interface Pi : %s", ifname)
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Stats réseau toutes les 30 secondes
	go func() {
		t := time.NewTicker(30 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if s, err := rndis.ReadStats(); err == nil {
					log.Printf("RNDIS stats: rx=%d tx=%d bytes (rx_err=%d tx_err=%d)",
						s.RxBytes, s.TxBytes, s.RxErrors, s.TxErrors)
				}
				if s, err := ecm.ReadStats(); err == nil {
					log.Printf("ECM   stats: rx=%d tx=%d bytes",
						s.RxBytes, s.TxBytes)
				}
			}
		}
	}()

	// Lecture des LEDs clavier (NumLock / CapsLock / ScrollLock)
	go func() {
		leds, err := kbd.ReadLEDs(ctx)
		if err != nil {
			log.Printf("ReadLEDs: %v", err)
			return
		}
		for state := range leds {
			log.Printf("LED clavier → NumLock=%v CapsLock=%v ScrollLock=%v",
				state.NumLock, state.CapsLock, state.ScrollLock)
		}
	}()

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGTERM, syscall.SIGINT)
	<-ch

	cancel()
	log.Println("Arrêt du gadget USB...")
	if err := g.Disable(); err != nil {
		log.Printf("gadget.Disable: %v", err)
	}
}
