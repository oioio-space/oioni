// system/wifi/iface.go — network interface helpers
package wifi

import (
	"github.com/vishvananda/netlink"
)

// ifaceIPv4 returns the first IPv4 address assigned to iface, or "" if none.
func ifaceIPv4(iface string) string {
	link, err := netlink.LinkByName(iface)
	if err != nil {
		return ""
	}
	addrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
	if err != nil || len(addrs) == 0 {
		return ""
	}
	if ip := addrs[0].IP; ip != nil {
		return ip.String()
	}
	return ""
}
