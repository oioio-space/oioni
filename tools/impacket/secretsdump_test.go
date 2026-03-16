// tools/impacket/secretsdump_test.go
package impacket_test

import (
	"context"
	"testing"

	"github.com/oioio-space/oioni/tools/impacket"
)

var secretsdump_output = []string{
	"[*] Target: 192.168.1.1",
	"[*] Dumping local SAM hashes (uid:rid:lmhash:nthash)",
	"Administrator:500:aad3b435b51404eeaad3b435b51404ee:31d6cfe0d16ae931b73c59d7e0c089c0:::",
	"Guest:501:aad3b435b51404eeaad3b435b51404ee:31d6cfe0d16ae931b73c59d7e0c089c0:::",
	"[*] Cleaning up...",
}

func TestSecretsDump_ParsesCredentials(t *testing.T) {
	fake := newFakeStarter(t)
	fake.add("dump1", secretsdump_output)

	imp := impacket.NewWithManager(fake)
	creds, err := imp.SecretsDump(context.Background(), "dump1", impacket.SecretsDumpConfig{
		Target:   "192.168.1.1",
		Username: "Administrator",
		Password: "Password1",
	})
	if err != nil {
		t.Fatalf("SecretsDump: %v", err)
	}
	if len(creds) != 2 {
		t.Fatalf("got %d creds, want 2; creds=%v", len(creds), creds)
	}
	if creds[0].Username != "Administrator" {
		t.Errorf("creds[0] = %+v", creds[0])
	}
	if creds[0].Type != "NTLM" {
		t.Errorf("creds[0].Type = %q, want NTLM", creds[0].Type)
	}
}

func TestSecretsDump_ContextCancellation(t *testing.T) {
	// Fake that blocks — simulates a long-running secretsdump.
	proc, complete := fakeProcess([]string{}, nil)
	var mgr fakeStarterWithProc
	mgr.proc = proc
	mgr.stopFn = complete

	imp := impacket.NewWithManager(&mgr)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := imp.SecretsDump(ctx, "dump2", impacket.SecretsDumpConfig{Target: "1.2.3.4"})
	if err == nil {
		t.Fatal("want error on cancelled context, got nil")
	}
}
