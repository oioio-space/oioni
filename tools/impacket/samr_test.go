// tools/impacket/samr_test.go
package impacket_test

import (
	"context"
	"testing"

	"github.com/oioio-space/oioni/tools/impacket"
)

var samrdump_lines = []string{
	"[*] Retrieving endpoint list from 192.168.1.100",
	"Found domain(s):",
	". WORKGROUP",
	". Builtin",
	"[*] Looking up users in domain WORKGROUP",
	"Found user: Administrator, uid = 500",
	"Found user: Guest, uid = 501",
	"Found user: svc-backup, uid = 1105",
}

func TestSAMRDump_ParsesUsers(t *testing.T) {
	fake := newFakeStarter(t)
	fake.add("samr1", samrdump_lines)

	imp := impacket.NewWithManager(fake)
	users, err := imp.SAMRDump(context.Background(), "samr1", impacket.SAMRDumpConfig{
		Target:   "192.168.1.100",
		Username: "Administrator",
		Password: "pass",
	})
	if err != nil {
		t.Fatalf("SAMRDump: %v", err)
	}
	if len(users) != 3 {
		t.Fatalf("got %d users, want 3; users=%v", len(users), users)
	}
	if users[0].Username != "Administrator" || users[0].UID != 500 {
		t.Errorf("users[0] = %+v", users[0])
	}
	if users[2].Username != "svc-backup" || users[2].UID != 1105 {
		t.Errorf("users[2] = %+v", users[2])
	}
}
