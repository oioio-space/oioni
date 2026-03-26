// system/wifi/vif.go — virtual interface management for AP mode
package wifi

import (
	"fmt"
	"net"

	"github.com/vishvananda/netlink"
)

// createVirtualAP creates a virtual AP interface named name on parent interface
// using "iw dev <parent> interface add <name> type __ap".
func createVirtualAP(proc processRunner, iwBin, parent, name string) error {
	if err := proc.Start(iwBin, []string{"dev", parent, "interface", "add", name, "type", "__ap"}); err != nil {
		return fmt.Errorf("create virtual AP %s on %s: %w", name, parent, err)
	}
	return nil
}

// deleteVirtualAP deletes a virtual interface using "iw dev <name> del".
func deleteVirtualAP(proc processRunner, iwBin, name string) error {
	if err := proc.Start(iwBin, []string{"dev", name, "del"}); err != nil {
		return fmt.Errorf("delete virtual AP %s: %w", name, err)
	}
	return nil
}

// assignIP assigns a CIDR address to iface and brings it up using netlink.
// No external binary required — uses vishvananda/netlink directly.
func assignIP(iface, cidr string) error {
	link, err := netlink.LinkByName(iface)
	if err != nil {
		return fmt.Errorf("link %s: %w", iface, err)
	}
	ip, netw, err := net.ParseCIDR(cidr)
	if err != nil {
		return fmt.Errorf("parse cidr %s: %w", cidr, err)
	}
	addr := &netlink.Addr{IPNet: &net.IPNet{IP: ip, Mask: netw.Mask}}
	if err := netlink.AddrAdd(link, addr); err != nil {
		return fmt.Errorf("addr add %s dev %s: %w", cidr, iface, err)
	}
	if err := netlink.LinkSetUp(link); err != nil {
		return fmt.Errorf("link set %s up: %w", iface, err)
	}
	return nil
}
