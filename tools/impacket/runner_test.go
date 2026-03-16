// tools/impacket/runner_test.go
package impacket_test

import (
	"context"
	"testing"

	"github.com/oioio-space/oioni/tools/impacket"
)

func TestRun_LinesPassedThrough(t *testing.T) {
	fake := newFakeStarter(t)
	fake.add("myrun", []string{"output line 1", "output line 2"})

	imp := impacket.NewWithManager(fake)
	proc, err := imp.Run(context.Background(), "myrun", "samrdump", []string{"-target", "192.168.1.1"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	var got []string
	for l := range proc.Lines() {
		got = append(got, l)
	}
	if len(got) != 2 {
		t.Fatalf("got %v lines, want 2", got)
	}
}
