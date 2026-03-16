// tools/impacket/ntlmrelayx_test.go
package impacket_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/oioio-space/oioni/tools/containers"
	"github.com/oioio-space/oioni/tools/impacket"
)

// Typical ntlmrelayx log lines that should produce NTLMRelayEvents.
// Includes a hyphenated domain (CORP-DC) to cover real AD naming.
var ntlmrelayx_lines = []string{
	"[*] SMBD-Thread-2: Connection from WORKGROUP/Administrator@192.168.1.100",
	"[*] WORKGROUP\\Administrator::192.168.1.100:aabbccdd:aad3b435b51404eeaad3b435b51404ee:31d6cfe0d16ae931b73c59d7e0c089c0",
	"[*] Some other log line",
	"[*] CORP-DC\\jdoe::192.168.1.101:aabbccdd:aad3b435b51404eeaad3b435b51404ee:8846f7eaee8fb117ad06bdd830b7586c",
}

func TestNTLMRelay_EventsParsed(t *testing.T) {
	fake := newFakeStarter(t)
	fake.add("relay1", ntlmrelayx_lines)

	imp := impacket.NewWithManager(fake)
	relay, err := imp.NTLMRelay(context.Background(), "relay1", impacket.NTLMRelayConfig{
		Target: "smb://192.168.1.1",
	})
	if err != nil {
		t.Fatalf("NTLMRelay: %v", err)
	}

	var events []impacket.NTLMRelayEvent
	for e := range relay.Events() {
		events = append(events, e)
	}

	if len(events) != 2 {
		t.Fatalf("got %d events, want 2; events=%v", len(events), events)
	}
	if events[0].Username != "Administrator" || events[0].Domain != "WORKGROUP" {
		t.Errorf("event[0] = %+v", events[0])
	}
	if events[1].Username != "jdoe" || events[1].Domain != "CORP-DC" {
		t.Errorf("event[1] = %+v", events[1])
	}
	if err := relay.Err(); err != nil {
		t.Fatalf("Err() after exit: %v", err)
	}
}

func TestNTLMRelay_ErrAfterExit(t *testing.T) {
	// fakeProcess with a non-nil wait error simulates a crash.
	proc, complete := fakeProcess([]string{"[*] crash log"}, errors.New("exit status 1"))

	var fakeMgr fakeStarterWithProc
	fakeMgr.proc = proc

	imp := impacket.NewWithManager(&fakeMgr)
	relay, err := imp.NTLMRelay(context.Background(), "x", impacket.NTLMRelayConfig{Target: "smb://1.2.3.4"})
	if err != nil {
		t.Fatal(err)
	}

	complete()
	// Drain events
	for range relay.Events() {}

	if relay.Err() == nil {
		t.Fatal("want non-nil Err() after crash, got nil")
	}
}

func TestNTLMRelay_StopStopsEvents(t *testing.T) {
	proc, complete := fakeProcess([]string{"line1", "line2"}, nil)
	var fakeMgr fakeStarterWithProc
	fakeMgr.proc = proc
	fakeMgr.stopFn = func() { complete() }

	imp := impacket.NewWithManager(&fakeMgr)
	relay, err := imp.NTLMRelay(context.Background(), "x", impacket.NTLMRelayConfig{Target: "smb://1.2.3.4"})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := relay.Stop(ctx); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	// Events channel should be closed
	for range relay.Events() {}
}

// fakeStarterWithProc is a ProcessStarter that returns a pre-built *Process.
type fakeStarterWithProc struct {
	proc   *containers.Process // set before Start() is called
	stopFn func()
}

func (f *fakeStarterWithProc) Start(_ context.Context, _, _ string, _ []string) (*containers.Process, error) {
	return f.proc, nil
}
func (f *fakeStarterWithProc) Stop(_ context.Context, _ string) error {
	if f.stopFn != nil {
		f.stopFn()
	}
	return nil
}
func (f *fakeStarterWithProc) Kill(_ string) error {
	if f.stopFn != nil {
		f.stopFn()
	}
	return nil
}
