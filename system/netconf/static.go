// system/netconf/static.go — apply static IP configuration via netlink
package netconf

import (
	"fmt"
	"net"

	"github.com/vishvananda/netlink"
)

// applyStatic sets a static IP and gateway on iface using nl.
// cidr is "192.168.1.10/24", gateway is "192.168.1.1" (empty = no default route).
func applyStatic(nl netlinkClient, iface, cidr, gateway string) error {
	link, err := nl.LinkByName(iface)
	if err != nil {
		return fmt.Errorf("link %s: %w", iface, err)
	}
	addr, err := netlink.ParseAddr(cidr)
	if err != nil {
		return fmt.Errorf("parse addr %s: %w", cidr, err)
	}
	if err := nl.AddrAdd(link, addr); err != nil {
		return fmt.Errorf("addr add: %w", err)
	}
	if gateway != "" {
		gw := net.ParseIP(gateway)
		if gw == nil {
			return fmt.Errorf("invalid gateway: %s", gateway)
		}
		route := &netlink.Route{
			LinkIndex: link.Attrs().Index,
			Gw:        gw,
		}
		if err := nl.RouteAdd(route); err != nil {
			return fmt.Errorf("route add: %w", err)
		}
	}
	return nil
}
