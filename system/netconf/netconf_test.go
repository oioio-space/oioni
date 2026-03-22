package netconf

import (
	"strings"
	"testing"

	"github.com/vishvananda/netlink"
)

func TestListInterfaces_FiltersVirtual(t *testing.T) {
	nl := &fakeNetlink{}
	// Add physical + virtual interfaces
	for _, name := range []string{"wlan0", "usb0", "lo", "veth0", "docker0", "br-abc"} {
		la := netlink.NewLinkAttrs()
		la.Name = name
		nl.links = append(nl.links, &netlink.Dummy{LinkAttrs: la})
	}
	m := &Manager{nl: nl, cfg: &ifaceConfig{dir: t.TempDir()}}
	ifaces, err := m.ListInterfaces()
	if err != nil {
		t.Fatal(err)
	}
	for _, iface := range ifaces {
		if iface == "lo" || strings.HasPrefix(iface, "veth") || strings.HasPrefix(iface, "docker") || strings.HasPrefix(iface, "br-") {
			t.Errorf("filtered interface %q should not appear", iface)
		}
	}
	found := false
	for _, iface := range ifaces {
		if iface == "wlan0" {
			found = true
		}
	}
	if !found {
		t.Error("wlan0 should be in list")
	}
}

func TestApply_Static_Persists(t *testing.T) {
	nl := &fakeNetlink{}
	dir := t.TempDir()
	m := &Manager{nl: nl, cfg: &ifaceConfig{dir: dir}}
	cfg := IfaceCfg{Mode: ModeStatic, IP: "10.0.0.2/24", Gateway: "10.0.0.1"}
	if err := m.Apply("eth0", cfg); err != nil {
		t.Fatal(err)
	}
	stored, _ := m.cfg.read()
	if stored["eth0"].IP != "10.0.0.2/24" {
		t.Errorf("not persisted: %+v", stored)
	}
}
