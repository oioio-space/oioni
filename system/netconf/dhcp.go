// system/netconf/dhcp.go — DHCP client using insomniacslk/dhcp (CGo-free)
package netconf

import (
	"context"
	"fmt"
	"net"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv4/nclient4"
)

// dhcpLease holds the result of a DHCP negotiation.
type dhcpLease struct {
	IP      string   // CIDR, e.g. "192.168.1.10/24"
	Gateway string   // e.g. "192.168.1.1"
	DNS     []string // e.g. ["8.8.8.8"]
}

// applyLease applies a lease to an interface via the netlinkClient.
func applyLease(nl netlinkClient, iface string, lease dhcpLease) error {
	return applyStatic(nl, iface, lease.IP, lease.Gateway)
}

// runDHCP runs a DHCP client for iface, applies the lease, and returns it.
// Call in a goroutine; pass ctx to cancel.
func runDHCP(ctx context.Context, nl netlinkClient, iface string) (dhcpLease, error) {
	client, err := nclient4.New(iface)
	if err != nil {
		return dhcpLease{}, fmt.Errorf("dhcp client %s: %w", iface, err)
	}
	defer client.Close()

	lease, err := client.Request(ctx)
	if err != nil {
		return dhcpLease{}, fmt.Errorf("dhcp request %s: %w", iface, err)
	}

	ip := lease.ACK.YourIPAddr
	mask := lease.ACK.SubnetMask()
	cidr := fmt.Sprintf("%s/%d", ip, maskBits(mask))

	var gw string
	if routers := lease.ACK.Options.Get(dhcpv4.OptionRouter); len(routers) >= 4 {
		gw = net.IP(routers[:4]).String()
	}

	var dns []string
	if dnsOpt := lease.ACK.Options.Get(dhcpv4.OptionDomainNameServer); len(dnsOpt) >= 4 {
		for i := 0; i+4 <= len(dnsOpt); i += 4 {
			dns = append(dns, net.IP(dnsOpt[i:i+4]).String())
		}
	}

	result := dhcpLease{IP: cidr, Gateway: gw, DNS: dns}
	if err := applyLease(nl, iface, result); err != nil {
		return dhcpLease{}, err
	}
	return result, nil
}

func maskBits(mask net.IPMask) int {
	ones, _ := mask.Size()
	return ones
}
