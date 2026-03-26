// system/wifi/ap.go — AP mode management via hostapd + uap0 virtual interface
//
// Flow: SetMode(AP:true) → APManager.Start() → createVirtualAP → assignIP →
//       writeHostapdConf → startHostapd (supervised) → apDHCPServer.Start()
//
// The BCM43430 chip supports concurrent STA+AP only when both use the same channel.
// Manager.SetMode auto-detects the STA channel and passes it in APConfig.Channel.
package wifi

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"
)

// APConfig configures the WiFi access point.
type APConfig struct {
	SSID    string   `json:"ssid"`
	PSK     string   `json:"psk"`     // empty = open network
	Channel int      `json:"channel"` // default 6 (BCM43430 is 2.4GHz only)
	IP      string   `json:"ip"`      // CIDR for uap0, e.g. "192.168.4.1/24"
	DNS     []string `json:"dns"`     // DNS servers advertised to DHCP clients
}

// defaultAPConfig returns the default AP configuration.
func defaultAPConfig() APConfig {
	return APConfig{
		Channel: 6,
		IP:      "192.168.4.1/24",
		DNS:     []string{"8.8.8.8", "8.8.4.4"},
	}
}

// APStatus is the current AP state.
type APStatus struct {
	Running bool
	IP      string // uap0's IP (CIDR)
	Clients int    // connected DHCP clients
}

// APManager manages the hostapd process and DHCP server on the uap0 virtual
// interface. Created and destroyed by Manager.SetMode.
type APManager struct {
	cfg        APConfig
	hostapdBin string
	iwBin      string
	conf       *confManager
	proc       processRunner
	assignIPFn func(iface, cidr string) error // injectable; defaults to assignIP

	mu      sync.Mutex  // guards process
	dhcp    *apDHCPServer
	process *os.Process // running hostapd; nil when stopped; guarded by mu
	cancel  context.CancelFunc
}

// newAPManager creates an APManager. Called by Manager.SetMode.
func newAPManager(cfg APConfig, conf *confManager, proc processRunner, hostapdBin, iwBin string) *APManager {
	return &APManager{
		cfg:        cfg,
		hostapdBin: hostapdBin,
		iwBin:      iwBin,
		conf:       conf,
		proc:       proc,
		assignIPFn: assignIP,
	}
}

// Start creates uap0, assigns the CIDR address, writes hostapd.conf, starts
// hostapd under supervision, and starts the DHCP server.
// DHCP failure is non-fatal (logged); all other failures clean up and return.
func (a *APManager) Start(ctx context.Context) error {
	if err := createVirtualAP(a.proc, a.iwBin, "wlan0", "uap0"); err != nil {
		return fmt.Errorf("ap start: create uap0: %w", err)
	}
	if err := a.assignIPFn("uap0", a.cfg.IP); err != nil {
		_ = deleteVirtualAP(a.proc, a.iwBin, "uap0")
		return fmt.Errorf("ap start: assign IP: %w", err)
	}
	if err := a.conf.writeHostapdConf(a.cfg); err != nil {
		_ = deleteVirtualAP(a.proc, a.iwBin, "uap0")
		return fmt.Errorf("ap start: write hostapd.conf: %w", err)
	}

	// Start hostapd in foreground; goroutine monitors and restarts on crash.
	innerCtx, cancel := context.WithCancel(ctx)
	a.cancel = cancel
	if err := a.startHostapd(innerCtx); err != nil {
		cancel()
		_ = deleteVirtualAP(a.proc, a.iwBin, "uap0")
		return fmt.Errorf("ap start: hostapd: %w", err)
	}

	// Start DHCP server.
	a.dhcp = newAPDHCPServer("uap0", a.cfg)
	if err := a.dhcp.Start(innerCtx); err != nil {
		// DHCP failure is non-fatal: AP is still operational.
		log.Printf("wifi/ap: DHCP server error: %v", err)
	}

	return nil
}

// startHostapd launches hostapd and starts a goroutine that restarts it on
// unexpected exit (same pattern as the Manager's wpa_supplicant polling).
func (a *APManager) startHostapd(ctx context.Context) error {
	confPath := a.conf.dir + "/hostapd.conf"
	proc, err := a.proc.StartProcess(a.hostapdBin, []string{confPath})
	if err != nil {
		return err
	}
	a.mu.Lock()
	a.process = proc
	a.mu.Unlock()

	go func() {
		for {
			state, err := proc.Wait()
			if ctx.Err() != nil {
				return // intentional shutdown
			}
			if err != nil || (state != nil && !state.Success()) {
				log.Printf("wifi/ap: hostapd exited (%v), restarting in 3s", err)
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(3 * time.Second):
			}
			proc, err = a.proc.StartProcess(a.hostapdBin, []string{confPath})
			if err != nil {
				log.Printf("wifi/ap: hostapd restart failed: %v", err)
				continue
			}
			a.mu.Lock()
			a.process = proc
			a.mu.Unlock()
		}
	}()
	return nil
}

// Stop cancels the supervisor goroutine, shuts down the DHCP server, sends
// SIGINT to hostapd, and deletes the uap0 interface.
func (a *APManager) Stop() {
	if a.cancel != nil {
		a.cancel()
	}
	if a.dhcp != nil {
		a.dhcp.Stop()
	}
	a.mu.Lock()
	proc := a.process
	a.process = nil
	a.mu.Unlock()
	if proc != nil {
		_ = proc.Signal(os.Interrupt)
		_, _ = proc.Wait()
	}
	if err := deleteVirtualAP(a.proc, a.iwBin, "uap0"); err != nil {
		log.Printf("wifi/ap: delete uap0: %v", err)
	}
}

// Status returns the current AP state. Safe to call concurrently with Start/Stop.
func (a *APManager) Status() APStatus {
	a.mu.Lock()
	running := a.process != nil
	a.mu.Unlock()
	clients := 0
	if a.dhcp != nil {
		clients = a.dhcp.ClientCount()
	}
	return APStatus{Running: running, IP: a.cfg.IP, Clients: clients}
}
