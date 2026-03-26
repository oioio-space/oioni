// system/wifi/vif.go — virtual interface management for AP mode
package wifi

import "fmt"

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

// assignIP assigns a CIDR address to iface and brings it up.
// Runs: ip addr add <cidr> dev <iface> && ip link set <iface> up
func assignIP(proc processRunner, ipBin, iface, cidr string) error {
	if err := proc.Start(ipBin, []string{"addr", "add", cidr, "dev", iface}); err != nil {
		return fmt.Errorf("ip addr add %s dev %s: %w", cidr, iface, err)
	}
	if err := proc.Start(ipBin, []string{"link", "set", iface, "up"}); err != nil {
		return fmt.Errorf("ip link set %s up: %w", iface, err)
	}
	return nil
}
