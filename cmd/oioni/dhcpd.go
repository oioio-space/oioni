package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
)

const busybox = "/usr/local/bin/busybox"

// startUDHCPD starts a udhcpd DHCP server on iface so USB clients
// receive 10.42.0.2 automatically (no manual ip addr add needed).
// Pi stays at 10.42.0.1 (configured by netconfMgr.Apply before calling this).
func startUDHCPD(ctx context.Context, iface string) {
	conf := fmt.Sprintf(`# udhcpd config for USB ECM gadget (%s)
interface %s
start 10.42.0.2
end   10.42.0.2
lease 86400
option subnet  255.255.255.0
option router  10.42.0.1
option dns     10.42.0.1
`, iface, iface)

	confPath := "/tmp/udhcpd-ecm.conf"
	leasePath := "/tmp/udhcpd-ecm.leases"

	if err := os.WriteFile(confPath, []byte(conf), 0644); err != nil {
		log.Printf("udhcpd: write conf: %v", err)
		return
	}
	// udhcpd requires the leases file to exist
	if _, err := os.Stat(leasePath); os.IsNotExist(err) {
		_ = os.WriteFile(leasePath, nil, 0644)
	}

	cmd := exec.CommandContext(ctx, busybox, "udhcpd", "-f", "-S", confPath)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	log.Printf("udhcpd: starting DHCP server on %s (10.42.0.2 → clients)", iface)
	if err := cmd.Run(); err != nil && ctx.Err() == nil {
		log.Printf("udhcpd: %v", err)
	}
}
