// tools/impacket/lookupsid_test.go
package impacket_test

import (
	"context"
	"testing"

	"github.com/oioio-space/oioni/tools/impacket"
)

var lookupsid_lines = []string{
	"[*] Brute forcing SIDs at WORKGROUP",
	"[*] StringBinding ncacn_np:192.168.1.100[\\pipe\\lsarpc]",
	"[*] Domain SID is: S-1-5-21-123456789-123456789-123456789",
	"500: WORKGROUP\\Administrator (SidTypeUser)",
	"501: WORKGROUP\\Guest (SidTypeUser)",
	"512: WORKGROUP\\Domain Admins (SidTypeGroup)",
	"[*] some other line",
	"1001: CORP-DC\\jdoe (SidTypeUser)",
}

func TestLookupSID_ParsesObjects(t *testing.T) {
	fake := newFakeStarter(t)
	fake.add("sid1", lookupsid_lines)

	imp := impacket.NewWithManager(fake)
	objs, err := imp.LookupSID(context.Background(), "sid1", impacket.SIDLookupConfig{
		Target:   "192.168.1.100",
		Username: "Administrator",
		Password: "pass",
	})
	if err != nil {
		t.Fatalf("LookupSID: %v", err)
	}
	if len(objs) != 4 {
		t.Fatalf("got %d objects, want 4; objs=%v", len(objs), objs)
	}
	if objs[0].RID != 500 || objs[0].Name != "Administrator" || objs[0].Domain != "WORKGROUP" {
		t.Errorf("objs[0] = %+v", objs[0])
	}
	if objs[2].Name != "Domain Admins" || objs[2].Type != "SidTypeGroup" {
		t.Errorf("objs[2] = %+v", objs[2])
	}
	// Domain with hyphen
	if objs[3].Domain != "CORP-DC" || objs[3].Name != "jdoe" {
		t.Errorf("objs[3] = %+v", objs[3])
	}
}
