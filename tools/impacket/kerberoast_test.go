// tools/impacket/kerberoast_test.go
package impacket_test

import (
	"context"
	"testing"

	"github.com/oioio-space/oioni/tools/impacket"
)

var kerberoast_lines = []string{
	"ServicePrincipalName                Name    MemberOf  PasswordLastSet  LastLogon",
	"----------------------------------  ------  --------  ---------------  ---------",
	"$krb5tgs$23$*svc_sql$CORP.LOCAL$MSSQLSvc/sql01.corp.local:1433*$aabbccdd1122334455...",
	"[*] Some other line",
	"$krb5tgs$23$*svc_web$CORP.LOCAL$HTTP/web01.corp.local*$eeff00112233...",
}

func TestKerberoast_ParsesHashes(t *testing.T) {
	fake := newFakeStarter(t)
	fake.add("krb1", kerberoast_lines)

	imp := impacket.NewWithManager(fake)
	hashes, err := imp.Kerberoast(context.Background(), "krb1", impacket.KerberoastConfig{
		Target:   "192.168.1.1",
		Domain:   "CORP.LOCAL",
		Username: "user",
		Password: "pass",
	})
	if err != nil {
		t.Fatalf("Kerberoast: %v", err)
	}
	if len(hashes) != 2 {
		t.Fatalf("got %d hashes, want 2", len(hashes))
	}
	if hashes[0].Username != "svc_sql" || hashes[0].Domain != "CORP.LOCAL" {
		t.Errorf("hashes[0] = %+v", hashes[0])
	}
	if hashes[0].SPN != "MSSQLSvc/sql01.corp.local:1433" {
		t.Errorf("hashes[0].SPN = %q", hashes[0].SPN)
	}
	if hashes[1].Username != "svc_web" {
		t.Errorf("hashes[1] = %+v", hashes[1])
	}
}

var asrep_lines = []string{
	"[*] Getting credentials for all valid users in CORP.LOCAL",
	"$krb5asrep$23$jdoe@CORP.LOCAL:aabbcc112233445566...",
	"[-] Kerberos SessionError: KDC_ERR_C_PRINCIPAL_UNKNOWN",
	"$krb5asrep$23$jsmith@CORP-DC.LOCAL:ddeeff778899...",
}

func TestASREPRoast_ParsesHashes(t *testing.T) {
	fake := newFakeStarter(t)
	fake.add("asrep1", asrep_lines)

	imp := impacket.NewWithManager(fake)
	hashes, err := imp.ASREPRoast(context.Background(), "asrep1", impacket.ASREPRoastConfig{
		Target: "192.168.1.1",
		Domain: "CORP.LOCAL",
	})
	if err != nil {
		t.Fatalf("ASREPRoast: %v", err)
	}
	if len(hashes) != 2 {
		t.Fatalf("got %d hashes, want 2; hashes=%v", len(hashes), hashes)
	}
	if hashes[0].Username != "jdoe" || hashes[0].Domain != "CORP.LOCAL" {
		t.Errorf("hashes[0] = %+v", hashes[0])
	}
	// SPN must be empty for AS-REP
	if hashes[0].SPN != "" {
		t.Errorf("hashes[0].SPN should be empty, got %q", hashes[0].SPN)
	}
	// domain with hyphen
	if hashes[1].Domain != "CORP-DC.LOCAL" {
		t.Errorf("hashes[1].Domain = %q, want CORP-DC.LOCAL", hashes[1].Domain)
	}
}

func TestKerberoast_ValidationErrors(t *testing.T) {
	imp := impacket.NewWithManager(newFakeStarter(t))
	if _, err := imp.Kerberoast(context.Background(), "x", impacket.KerberoastConfig{Domain: "CORP.LOCAL"}); err == nil {
		t.Error("expected error for empty Target")
	}
	if _, err := imp.Kerberoast(context.Background(), "x", impacket.KerberoastConfig{Target: "1.2.3.4"}); err == nil {
		t.Error("expected error for empty Domain")
	}
}
