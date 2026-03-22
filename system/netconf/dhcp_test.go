package netconf

import (
	"context"
	"testing"
)

func TestDHCPAppliesLease(t *testing.T) {
	nl := &fakeNetlink{}
	// dhcpApply is the internal function that takes a lease and applies it.
	lease := dhcpLease{IP: "10.0.0.5/24", Gateway: "10.0.0.1", DNS: []string{"8.8.8.8"}}
	if err := applyLease(nl, "wlan0", lease); err != nil {
		t.Fatal(err)
	}
	if len(nl.addedAddrs) == 0 {
		t.Error("expected address to be applied")
	}
	if nl.addedAddrs[0] != "10.0.0.5/24" {
		t.Errorf("unexpected addr: %s", nl.addedAddrs[0])
	}
}

// Ensure context import is used (TestDHCPAppliesLease doesn't need ctx but
// runDHCP does — this confirms the import is valid).
var _ = context.Background
