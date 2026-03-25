// Package netconf configures network interfaces on gokrazy via netlink.
//
// Saved configurations live in confDir/interfaces.json (JSON map of iface→IfaceCfg).
// Call PurgeNonWlan() before Start() to evict stale USB gadget entries, then
// Start() applies all saved configs. Use ApplyEphemeral() for transient
// interfaces that must not survive a reboot (e.g. USB ECM gadget).
package netconf

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

// IfaceStatus holds the runtime IP state of an interface.
type IfaceStatus struct {
	IP      string
	Gateway string
	Up      bool
}

// Manager configures network interfaces (DHCP or static) and persists config.
type Manager struct {
	nl  netlinkClient
	cfg *ifaceConfig
	ctx context.Context // set in Start; used by DHCP goroutines
	mu  sync.Mutex      // guards net.DefaultResolver writes
}

// New creates a Manager with configuration stored in confDir.
func New(confDir string) *Manager {
	return &Manager{
		nl:  &realNetlink{},
		cfg: &ifaceConfig{dir: confDir},
	}
}

// Start applies saved configuration for all known interfaces.
// Interfaces not in config default to DHCP.
func (m *Manager) Start(ctx context.Context) error {
	m.ctx = ctx
	saved, err := m.cfg.read()
	if err != nil {
		return fmt.Errorf("netconf load: %w", err)
	}
	// Note: wlan0 DHCP is handled by github.com/gokrazy/wifi, not by this manager.
	for iface, cfg := range saved {
		if err := m.applyNow(iface, cfg); err != nil {
			_ = err // non-fatal per spec: log in caller
		}
	}
	return nil
}

// ListInterfaces returns physical/USB interfaces, excluding lo, veth*, docker*, br-*.
func (m *Manager) ListInterfaces() ([]string, error) {
	links, err := m.nl.LinkList()
	if err != nil {
		return nil, err
	}
	var result []string
	for _, l := range links {
		name := l.Attrs().Name
		if name == "lo" ||
			strings.HasPrefix(name, "veth") ||
			strings.HasPrefix(name, "docker") ||
			strings.HasPrefix(name, "br-") {
			continue
		}
		result = append(result, name)
	}
	return result, nil
}

// Get returns the saved configuration for an interface (defaults to DHCP).
func (m *Manager) Get(iface string) (IfaceCfg, error) {
	saved, err := m.cfg.read()
	if err != nil {
		return IfaceCfg{}, err
	}
	cfg, ok := saved[iface]
	if !ok {
		return IfaceCfg{Mode: ModeDHCP}, nil
	}
	return cfg, nil
}

// Apply configures an interface live and persists the configuration.
func (m *Manager) Apply(iface string, cfg IfaceCfg) error {
	if err := m.applyNow(iface, cfg); err != nil {
		return err
	}
	saved, err := m.cfg.read()
	if err != nil {
		return err
	}
	saved[iface] = cfg
	return m.cfg.write(saved)
}

// ApplyEphemeral configures an interface without persisting to disk.
// Use for transient interfaces (USB gadget ECM) that must not be saved.
func (m *Manager) ApplyEphemeral(iface string, cfg IfaceCfg) error {
	return m.applyNow(iface, cfg)
}

// PurgeNonWlan removes non-wireless interface entries from the saved config.
// Call before Start() to clean up stale USB gadget interface entries that
// would otherwise cause a DHCP goroutine to fight with our static ECM config.
func (m *Manager) PurgeNonWlan() {
	saved, err := m.cfg.read()
	if err != nil {
		return
	}
	filtered := make(map[string]IfaceCfg)
	for iface, cfg := range saved {
		if strings.HasPrefix(iface, "wlan") || strings.HasPrefix(iface, "eth") {
			filtered[iface] = cfg
		}
	}
	_ = m.cfg.write(filtered)
}

// Status returns the current IP state for iface.
func (m *Manager) Status(iface string) (IfaceStatus, error) {
	link, err := m.nl.LinkByName(iface)
	if err != nil {
		return IfaceStatus{}, err
	}
	up := link.Attrs().Flags&net.FlagUp != 0
	addrs, err := m.nl.AddrList(link, netlinkFamilyV4)
	if err != nil || len(addrs) == 0 {
		return IfaceStatus{Up: up}, nil
	}
	return IfaceStatus{IP: addrs[0].IPNet.String(), Up: up}, nil
}

func (m *Manager) applyNow(iface string, cfg IfaceCfg) error {
	switch cfg.Mode {
	case ModeStatic:
		if err := applyStatic(m.nl, iface, cfg.IP, cfg.Gateway); err != nil {
			return err
		}
		if len(cfg.DNS) > 0 {
			m.setDNS(cfg.DNS)
		}
		return nil
	case ModeDHCP:
		if m.ctx != nil {
			go func() {
				// Retry until DHCP succeeds (wpa_supplicant may not have
				// associated yet when this goroutine first runs).
				for {
					_, err := runDHCP(m.ctx, m.nl, iface)
					if err == nil {
						return
					}
					select {
					case <-m.ctx.Done():
						return
					case <-time.After(5 * time.Second):
					}
				}
			}()
		}
		return nil
	default:
		return fmt.Errorf("unknown mode: %s", cfg.Mode)
	}
}

// setDNS overrides net.DefaultResolver to use the given DNS servers.
// All servers are tried in order; the first reachable one wins.
// Protected by mu so concurrent Apply calls don't race on the global resolver.
func (m *Manager) setDNS(servers []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	addrs := make([]string, len(servers))
	for i, s := range servers {
		addrs[i] = s + ":53"
	}
	net.DefaultResolver = &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := &net.Dialer{}
			var lastErr error
			for _, addr := range addrs {
				conn, err := d.DialContext(ctx, "udp", addr)
				if err == nil {
					return conn, nil
				}
				lastErr = err
			}
			return nil, lastErr
		},
	}
}
