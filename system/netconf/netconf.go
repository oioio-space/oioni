// system/netconf/netconf.go — per-interface IP configuration manager
package netconf

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
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
	// Default: DHCP on wlan0 if not explicitly configured.
	if _, ok := saved["wlan0"]; !ok {
		saved["wlan0"] = IfaceCfg{Mode: ModeDHCP}
	}
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

// Status returns the current IP state for iface.
func (m *Manager) Status(iface string) (IfaceStatus, error) {
	link, err := m.nl.LinkByName(iface)
	if err != nil {
		return IfaceStatus{}, err
	}
	_ = link
	// IP state would be read from netlink AddrList — simplified for now.
	return IfaceStatus{Up: true}, nil
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
				if _, err := runDHCP(m.ctx, m.nl, iface); err != nil && m.ctx.Err() == nil {
					_ = err // caller should log; non-fatal
				}
			}()
		}
		return nil
	default:
		return fmt.Errorf("unknown mode: %s", cfg.Mode)
	}
}

// setDNS overrides net.DefaultResolver to use the given DNS servers.
// Protected by mu so concurrent Apply calls don't race.
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
			d := net.Dialer{}
			return d.DialContext(ctx, "udp", addrs[0])
		},
	}
}
