// system/wifi/nat.go — IPv4 NAT (masquerade) via nftables for AP mode
//
// When AP mode is active, clients on uap0 (192.168.4.x) reach the internet
// through the Pi's STA interface (wlan0).  This requires:
//   1. IP forwarding enabled in the kernel (/proc/sys/net/ipv4/ip_forward)
//   2. NAT masquerade rule on the STA interface so reply packets are routed back
//
// We use a dedicated nftables table ("oioni_nat") so our rules do not conflict
// with any system-level firewall.  The table is created on AP start and deleted
// on AP stop — a clean, idempotent lifecycle.
package wifi

import (
	"fmt"
	"log"
	"os"

	"github.com/google/nftables"
	"github.com/google/nftables/expr"
)

const natTableName = "oioni_nat"

// enableNAT enables IPv4 forwarding and installs a masquerade rule for
// traffic leaving staIface (e.g. "wlan0").  Safe to call multiple times —
// any previous oioni_nat table is replaced.
func enableNAT(staIface string) error {
	if err := setIPForward(true); err != nil {
		return fmt.Errorf("nat: enable forwarding: %w", err)
	}
	if err := applyNATTable(staIface); err != nil {
		return fmt.Errorf("nat: apply table: %w", err)
	}
	log.Printf("wifi/nat: NAT enabled (masquerade via %s)", staIface)
	return nil
}

// disableNAT removes the oioni_nat nftables table.  IP forwarding is left
// enabled — disabling it could interfere with other consumers (e.g. netconf).
func disableNAT() {
	if err := removeNATTable(); err != nil {
		log.Printf("wifi/nat: remove table: %v", err)
		return
	}
	log.Printf("wifi/nat: NAT disabled")
}

// setIPForward writes to /proc/sys/net/ipv4/ip_forward.
func setIPForward(enable bool) error {
	v := []byte("0\n")
	if enable {
		v = []byte("1\n")
	}
	return os.WriteFile("/proc/sys/net/ipv4/ip_forward", v, 0644)
}

// applyNATTable creates (or replaces) the oioni_nat table with a single
// postrouting chain that masquerades all traffic leaving staIface.
func applyNATTable(staIface string) error {
	// Remove any leftover table from a previous run before recreating.
	_ = removeNATTable()

	c, err := nftables.New()
	if err != nil {
		return err
	}

	t := c.AddTable(&nftables.Table{
		Family: nftables.TableFamilyIPv4,
		Name:   natTableName,
	})

	ch := c.AddChain(&nftables.Chain{
		Name:     "postrouting",
		Table:    t,
		Type:     nftables.ChainTypeNAT,
		Hooknum:  nftables.ChainHookPostrouting,
		Priority: nftables.ChainPriorityNATSource,
	})

	// Match oifname == staIface, then masquerade.
	c.AddRule(&nftables.Rule{
		Table: t,
		Chain: ch,
		Exprs: []expr.Any{
			// meta oifname "wlan0"
			&expr.Meta{Key: expr.MetaKeyOIFNAME, Register: 1},
			&expr.Cmp{
				Op:       expr.CmpOpEq,
				Register: 1,
				Data:     ifnameBytes(staIface),
			},
			// masquerade
			&expr.Masq{},
		},
	})

	return c.Flush()
}

// removeNATTable deletes the oioni_nat table if it exists.
func removeNATTable() error {
	c, err := nftables.New()
	if err != nil {
		return err
	}
	tables, err := c.ListTablesOfFamily(nftables.TableFamilyIPv4)
	if err != nil {
		return err
	}
	for _, t := range tables {
		if t.Name == natTableName {
			c.DelTable(t)
			return c.Flush()
		}
	}
	return nil // table not present — nothing to do
}

// ifnameBytes returns a 16-byte zero-padded interface name as expected by
// the nft oifname expression (IFNAMSIZ = 16 on Linux).
func ifnameBytes(name string) []byte {
	b := make([]byte, 16)
	copy(b, name)
	return b
}
