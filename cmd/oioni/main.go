// hello/main.go — demo composite USB gadget on Pi Zero 2W (gokrazy)
//
// EP budget reference (DWC2 BCM2835, max 7 usable EPs beyond EP0):
//   RNDIS: 3 EP  |  ECM: 3 EP  |  HID: 1 EP  |  MassStorage: 2 EP
//   RNDIS + MassStorage = 5 EP  ✓
//   RNDIS + ECM + HID   = 7 EP  ✓
//   RNDIS + ECM + MassStorage = 8 EP  ✗ → clear error from udc.go
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/oioio-space/oioni/system/imgvol"
	"github.com/oioio-space/oioni/system/storage"
	netconf "github.com/oioio-space/oioni/system/netconf"
	wifi "github.com/oioio-space/oioni/system/wifi"
	"github.com/oioio-space/oioni/drivers/usbgadget"
	"github.com/oioio-space/oioni/drivers/usbgadget/functions"

	"github.com/spf13/afero"
)

func main() {
	log.SetFlags(log.Ldate | log.Ltime)

	// Mirror logs to /perm so boot failures are readable without WiFi.
	var logFile *os.File
	if f, err := os.OpenFile("/perm/oioni-boot.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644); err == nil {
		logFile = f
		defer f.Close()
		log.SetOutput(io.MultiWriter(os.Stderr, f))
		log.Printf("=== oioni boot %s ===", time.Now().Format(time.RFC3339))
	}
	// Copy relevant kernel messages (USB gadget, brcmfmac) to boot log.
	go logKernelMessages(logFile)

	// Gadget flags
	withRNDIS := flag.Bool("rndis", false, "enable RNDIS network function (3 EP)")
	withECM := flag.Bool("ecm", false, "enable ECM network function (3 EP)")
	withHID := flag.Bool("hid", false, "enable HID keyboard function (1 EP)")
	withMassStorage := flag.Bool("mass-storage", false, "enable MassStorage function using --img (2 EP)")

	// Image flags
	imgPath := flag.String("img", "/perm/data.img", "disk image path")
	imgFSStr := flag.String("img-fs", "vfat", "filesystem: vfat|exfat|ext4")
	imgSizeMiB := flag.Int64("img-size", 64, "image size in MiB")
	withImgCreate := flag.Bool("img-create", false, "create and format the image (fails if exists)")
	withImgWrite := flag.Bool("img-write", false, "open image, write test files via afero, close")
	withImgRead := flag.Bool("img-read", false, "open image, print contents via afero, close")

	// Storage hotplug
	withStorage := flag.Bool("storage", false, "enable USB hotplug storage manager")

	// E-ink display + touch
	withEPaper := flag.Bool("epaper", false, "enable e-ink display and touch controller")

	// Impacket tools (test integration)
	imp := defineImpacketFlags()

	flag.Parse()

	// ── Impacket tools ────────────────────────────────────────────────────────
	anyImpacket := imp.secretsdump || imp.ntlmrelay || imp.kerberoast ||
		imp.asreproast || imp.lookupsid || imp.samrdump || imp.exec
	if anyImpacket {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
		impCtx, impCancel := context.WithCancel(context.Background())
		go func() { <-sigCh; impCancel() }()
		runImpacket(impCtx, imp)
		impCancel()
		return
	}

	// ── Image operations (before gadget, so Pi owns the image first) ──────────
	fstype := imgvol.FSType(*imgFSStr)

	if *withImgCreate {
		log.Printf("img: creating %s (%d MiB, %s)", *imgPath, *imgSizeMiB, fstype)
		if err := imgvol.Create(*imgPath, *imgSizeMiB<<20, fstype); err != nil {
			log.Printf("img-create: %v", err)
		} else {
			log.Printf("img: created %s", *imgPath)
		}
	}

	if *withImgWrite {
		vol, err := imgvol.Open(*imgPath)
		if err != nil {
			log.Printf("img-write open: %v", err)
		} else {
			demoWrite(vol)
			if err := vol.Close(); err != nil {
				log.Printf("img-write close: %v", err)
			}
		}
	}

	if *withImgRead {
		vol, err := imgvol.Open(*imgPath)
		if err != nil {
			log.Printf("img-read open: %v", err)
		} else {
			demoRead(vol)
			if err := vol.Close(); err != nil {
				log.Printf("img-read close: %v", err)
			}
		}
	}

	// ── Gadget ────────────────────────────────────────────────────────────────
	anyGadget := *withRNDIS || *withECM || *withHID || *withMassStorage
	anyBackground := anyGadget || *withStorage || *withEPaper

	if !anyBackground {
		return // only image operations requested — done
	}

	var rndis *functions.RNDISFunc
	var ecm *functions.ECMFunc

	opts := []usbgadget.Option{
		usbgadget.WithName("geekhouse"),
		usbgadget.WithVendorID(0x1d6b, 0x0104),
		usbgadget.WithStrings("0x409", "GeekHouse", "oioio Composite", "pi0001"),
	}

	if *withRNDIS {
		rndis = functions.RNDIS(
			functions.WithRNDISHostAddr("02:00:00:aa:bb:01"),
			functions.WithRNDISDevAddr("02:00:00:aa:bb:02"),
		)
		opts = append(opts, usbgadget.WithFunc(rndis))
	}
	if *withECM {
		ecm = functions.ECM(
			functions.WithECMHostAddr("02:00:00:cc:dd:01"),
			functions.WithECMDevAddr("02:00:00:cc:dd:02"),
		)
		opts = append(opts, usbgadget.WithFunc(ecm))
	}
	if *withHID {
		opts = append(opts, usbgadget.WithFunc(functions.Keyboard()))
	}
	if *withMassStorage {
		opts = append(opts, usbgadget.WithFunc(functions.MassStorage(*imgPath,
			functions.WithRemovable(true),
		)))
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// ── Network config (always started so USB gadget can use it too) ──────────
	netconfMgr := netconf.New("/perm/netconf")
	if err := netconfMgr.Start(ctx); err != nil {
		log.Printf("netconf: %v", err)
	}

	// ── E-ink display ─────────────────────────────────────────────────────────
	var ep *epaperState
	if *withEPaper {
		wifiMgr := wifi.New(wifi.Config{
			WpaSupplicantBin: "/user/wpa_supplicant",
			ConfDir:          "/perm/wifi",
			CtrlDir:          "/var/run/wpa_supplicant",
			Iface:            "wlan0",
		})
		if err := wifiMgr.Start(ctx); err != nil {
			log.Printf("wifi: %v", err)
		}
		ep = startEPaper(ctx, wifiMgr, netconfMgr)
		if ep != nil {
			defer ep.Close()
		}
	}

	if anyGadget {
		g, err := usbgadget.New(opts...)
		if err != nil {
			log.Printf("usbgadget.New: %v", err)
		} else if err := g.Enable(); err != nil {
			log.Printf("gadget.Enable: %v (USB inactif, WiFi OK)", err)
		} else {
			log.Println("USB gadget actif")
			var gadgetFuncs []string
			if *withRNDIS {
				gadgetFuncs = append(gadgetFuncs, "RNDIS")
			}
			if *withECM {
				gadgetFuncs = append(gadgetFuncs, "ECM")
			}
			if *withHID {
				gadgetFuncs = append(gadgetFuncs, "HID")
			}
			if *withMassStorage {
				gadgetFuncs = append(gadgetFuncs, "Mass")
			}
			ep.UpdateStatus("USB: "+strings.Join(gadgetFuncs, " "), "") // TODO: wire via ep.nsb.SetInterfaces/SetTools
			if rndis != nil {
				if ifname, err := rndis.IfName(); err == nil {
					log.Printf("RNDIS → %s", ifname)
				}
				go logStats(ctx, rndis, ecm)
			}
			if ecm != nil {
				// Wait up to 5s for the kernel to assign the ECM interface name.
				// g.Enable() returns before configfs ifname is written.
				var ecmIface string
				ecmDeadline := time.Now().Add(5 * time.Second)
				for time.Now().Before(ecmDeadline) {
					if name, err := ecm.IfName(); err == nil &&
						name != "" && !strings.Contains(name, "unnamed") {
						ecmIface = name
						break
					}
					time.Sleep(100 * time.Millisecond)
				}
				if ecmIface != "" {
					log.Printf("ECM → %s", ecmIface)
					if err := netconfMgr.Apply(ecmIface, netconf.IfaceCfg{
						Mode: netconf.ModeStatic,
						IP:   "10.42.0.1/24",
					}); err != nil {
						log.Printf("ECM netconf: %v", err)
					} else {
						log.Printf("ECM OK: 10.42.0.1/24 sur %s — SSH: ssh root@10.42.0.1", ecmIface)
						go startUDHCPD(ctx, ecmIface)
					}
				} else {
					log.Printf("ECM: interface name not ready after 5s")
				}
			}
			defer func() {
				if err := g.Disable(); err != nil {
					log.Printf("gadget.Disable: %v", err)
				}
			}()
		}
	}

	if *withStorage {
		sm := storage.New(
			storage.WithOnMount(func(v *storage.Volume) {
				log.Printf("storage: mounted %s (%s) @ %s", v.Name, v.FSType, v.MountPath)
			}),
			storage.WithOnUnmount(func(v *storage.Volume) {
				log.Printf("storage: removed %s", v.Name)
			}),
		)
		go func() {
			if err := sm.Start(ctx); err != nil {
				log.Printf("storage: %v", err)
			}
		}()
	}

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGTERM, syscall.SIGINT)
	<-ch
	log.Println("shutting down...")
}

func demoWrite(vol *imgvol.Volume) {
	name := fmt.Sprintf("boot-%s.txt", time.Now().Format("2006-01-02T15-04-05"))
	content := fmt.Sprintf("boot at %s on %s (%s)\n", time.Now().Format(time.RFC3339), vol.Path, vol.FSType)
	if err := afero.WriteFile(vol.FS, name, []byte(content), 0644); err != nil {
		log.Printf("img-write: %v", err)
		return
	}
	log.Printf("img-write: wrote %s", name)
}

func demoRead(vol *imgvol.Volume) {
	entries, err := afero.ReadDir(vol.FS, ".")
	if err != nil {
		log.Printf("img-read readdir: %v", err)
		return
	}
	log.Printf("img-read: %d file(s) in %s (%s)", len(entries), vol.Path, vol.FSType)
	for _, e := range entries {
		log.Printf("  %s (%d bytes)", e.Name(), e.Size())
	}
}

func logStats(ctx context.Context, rndis *functions.RNDISFunc, ecm *functions.ECMFunc) {
	t := time.NewTicker(30 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if s, err := rndis.ReadStats(); err == nil {
				log.Printf("RNDIS stats: rx=%d tx=%d bytes", s.RxBytes, s.TxBytes)
			}
			if ecm != nil {
				if s, err := ecm.ReadStats(); err == nil {
					log.Printf("ECM stats: rx=%d tx=%d bytes", s.RxBytes, s.TxBytes)
				}
			}
		}
	}
}
