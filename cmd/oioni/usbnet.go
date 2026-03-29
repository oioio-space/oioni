// cmd/oioni/usbnet.go — SetupUSBNetLink: shared setup for USB network gadget functions.
//
// Both RNDIS and ECM go through waitAndSetupUSBNet, which:
//   1. Waits up to 5 s for the kernel to assign an interface name.
//   2. Applies a static IP via netconf (ephemeral — not persisted).
//   3. Starts a keepalive goroutine that re-applies the IP every 3 s.
//
// ECM additionally installs a permanent ARP neighbor entry (Pi→PC) and sends
// a gratuitous ARP reply (PC→Pi) because the CDC-ECM interface on this kernel
// has hw_type=14 instead of ARPHRD_ETHER=1, which disables automatic ARP on
// both sides (see memory: ecm-netlink-neigh-fix).
package main

import (
	"context"
	"log"
	"net"
	"os/exec"
	"strings"
	"time"

	"github.com/vishvananda/netlink"
	netconf "github.com/oioio-space/oioni/system/netconf"
)

// usbNetIface is satisfied by ECMFunc and RNDISFunc.
type usbNetIface interface {
	IfName() (string, error)
}

// usbNetConfig holds per-function setup parameters.
type usbNetConfig struct {
	label  string // "ECM" or "RNDIS" — used only for log messages
	ipCIDR string // static IP for gadget side, e.g. "10.42.0.1/24"

	// ARP config — non-empty only for ECM (hw_type=14 workaround).
	arpHostMAC string // host-side MAC to install as permanent neighbor
	arpHostIP  string // host-side IP to install as permanent neighbor
	arpSelfIP  string // gadget IP for gratuitous arping -A
}

// waitAndSetupUSBNet blocks until the interface name is known (up to 5 s),
// applies the static IP, and launches a keepalive goroutine.
// It also starts a DHCP server on the interface via startUDHCPD.
// Returns the interface name, or "" on timeout.
func waitAndSetupUSBNet(ctx context.Context, fn usbNetIface, cfg usbNetConfig,
	nm *netconf.Manager, busybox string) string {

	// 1. Wait for a valid interface name.
	var iface string
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if name, err := fn.IfName(); err == nil &&
			name != "" && !strings.Contains(name, "unnamed") {
			iface = name
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if iface == "" {
		log.Printf("%s: interface name not ready after 5s", cfg.label)
		return ""
	}
	log.Printf("%s → %s", cfg.label, iface)

	// 2. Apply static IP.
	netCfg := netconf.IfaceCfg{Mode: netconf.ModeStatic, IP: cfg.ipCIDR}
	if err := nm.ApplyEphemeral(iface, netCfg); err != nil {
		log.Printf("%s netconf: %v", cfg.label, err)
		return ""
	}
	log.Printf("%s OK: %s sur %s", cfg.label, cfg.ipCIDR, iface)

	// 3. Start keepalive goroutine.
	go usbNetKeepalive(ctx, iface, cfg, netCfg, nm, busybox)
	return iface
}

// usbNetKeepalive re-applies the IP every 3 s and, for ECM, refreshes the ARP
// neighbor entry and sends a gratuitous ARP reply.
func usbNetKeepalive(ctx context.Context, iface string, cfg usbNetConfig,
	netCfg netconf.IfaceCfg, nm *netconf.Manager, busybox string) {

	t := time.NewTicker(3 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := nm.ApplyEphemeral(iface, netCfg); err != nil {
				log.Printf("%s keepalive: %v", cfg.label, err)
				continue
			}
			if cfg.arpHostMAC == "" {
				continue
			}
			// ECM hw_type=14 ARP workaround.
			link, err := netlink.LinkByName(iface)
			if err != nil {
				log.Printf("%s neigh link: %v", cfg.label, err)
			} else {
				pcMAC, _ := net.ParseMAC(cfg.arpHostMAC)
				if err := netlink.NeighSet(&netlink.Neigh{
					LinkIndex:    link.Attrs().Index,
					IP:           net.ParseIP(cfg.arpHostIP),
					HardwareAddr: pcMAC,
					State:        netlink.NUD_PERMANENT,
				}); err != nil {
					log.Printf("%s neigh set: %v", cfg.label, err)
				}
			}
			if out, err := exec.Command(busybox, "arping", "-A", "-I", iface, "-c", "1",
				cfg.arpSelfIP).CombinedOutput(); err != nil {
				log.Printf("%s arping: %v: %s", cfg.label, err, strings.TrimSpace(string(out)))
			}
		}
	}
}
