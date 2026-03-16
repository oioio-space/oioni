// tools/impacket/exec_test.go
package impacket_test

import (
	"context"
	"testing"

	"github.com/oioio-space/oioni/tools/impacket"
)

func TestExec_WMI_StreamsOutput(t *testing.T) {
	fake := newFakeStarter(t)
	fake.add("exec1", []string{"Windows IP Configuration", "", "Ethernet adapter:"})

	imp := impacket.NewWithManager(fake)
	proc, err := imp.Exec(context.Background(), "exec1", impacket.ExecConfig{
		Target:   "192.168.1.100",
		Username: "Administrator",
		Password: "pass",
		Domain:   "WORKGROUP",
		Command:  "ipconfig",
	})
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}

	var got []string
	for l := range proc.Lines() {
		got = append(got, l)
	}
	if len(got) != 3 {
		t.Fatalf("got %d lines, want 3", len(got))
	}
}

func TestExec_SMBMethod(t *testing.T) {
	fake := newFakeStarter(t)
	fake.add("exec2", []string{"whoami output"})

	imp := impacket.NewWithManager(fake)
	proc, err := imp.Exec(context.Background(), "exec2", impacket.ExecConfig{
		Target:   "192.168.1.100",
		Username: "admin",
		Password: "pass",
		Command:  "whoami",
		Method:   impacket.ExecSMB,
	})
	if err != nil {
		t.Fatalf("Exec(SMB): %v", err)
	}
	proc.Wait()
}

func TestExec_ValidationError(t *testing.T) {
	imp := impacket.NewWithManager(newFakeStarter(t))
	if _, err := imp.Exec(context.Background(), "x", impacket.ExecConfig{}); err == nil {
		t.Error("expected error for empty Target")
	}
}
