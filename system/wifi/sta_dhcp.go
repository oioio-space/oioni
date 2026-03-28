// system/wifi/sta_dhcp.go — DHCP client for the STA interface
//
// When wpa_supplicant completes an association (wpa_state=COMPLETED), we run
// udhcpc in one-shot mode to obtain an IPv4 lease.  Without this, the STA
// interface has no IP and the default route is absent — making NAT forwarding
// useless even if the nftables rules are in place.
//
// startSTADHCPWatcher polls wpa_supplicant every 2 s and runs udhcpc whenever
// a DISCONNECTED→COMPLETED transition is detected.  It also handles the initial
// boot-up association so no explicit trigger in Connect() is needed.
package wifi

import (
	"context"
	"log"
	"os/exec"
	"strings"
	"time"
)

// startSTADHCPWatcher starts a background goroutine that monitors the STA
// link state and runs udhcpc when the interface becomes associated.
// udhcpcBin must be the path to the udhcpc binary (e.g. "/bin/udhcpc").
// If udhcpcBin is empty, the watcher is a no-op.
func (m *Manager) startSTADHCPWatcher(ctx context.Context, udhcpcBin string) {
	if udhcpcBin == "" {
		return
	}
	go func() {
		var lastSSID string
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				st, err := m.Status()
				if err != nil {
					continue
				}
				// Trigger DHCP whenever the SSID is present and wasn't before.
				// On BCM43430, wpa_state may stay "ASSOCIATED" (not "COMPLETED")
				// even when fully connected — use SSID presence as the signal.
				connected := st.SSID != ""
				wasConnected := lastSSID != ""
				if connected && !wasConnected {
					// Re-disable power save on each reconnect: BCM43430 re-enables
					// it during association. Without this, power save beacon misses
					// cause disconnections every ~20s.
					m.disablePowerSave()
					go runSTADHCP(ctx, m.cfg.Iface, udhcpcBin)
				}
				lastSSID = st.SSID
			}
		}
	}()
}

// runSTADHCP runs udhcpc on iface in one-shot mode (exits after first lease).
// udhcpcBin may include a leading applet name (e.g. "/usr/local/bin/busybox udhcpc"),
// in which case it is split on spaces before appending the interface flags.
// Blocks until the lease is obtained, the context is cancelled, or 30 s elapse.
func runSTADHCP(ctx context.Context, iface, udhcpcBin string) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	// Split "binary [applet]" into argv[0] + optional prefix args.
	parts := strings.Fields(udhcpcBin)
	if len(parts) == 0 {
		return
	}
	// Flags: -f foreground, -q quit after lease, -n fail if no lease, -t 5 retries.
	args := append(parts[1:], "-i", iface, "-f", "-q", "-n", "-t", "5")
	cmd := exec.CommandContext(ctx, parts[0], args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("wifi/dhcp-sta: udhcpc %s: %v: %s", iface, err, out)
		return
	}
	log.Printf("wifi/dhcp-sta: DHCP lease obtained on %s", iface)
}
