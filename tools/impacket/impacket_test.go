// tools/impacket/impacket_test.go
package impacket_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/oioio-space/oioni/tools/containers"
)

// fakeStarter implements ProcessStarter entirely in-process.
// Call add() before Start() to register scripted outputs per process name.
type fakeStarter struct {
	t       *testing.T
	scripts map[string][]string // name → lines to emit
}

func newFakeStarter(t *testing.T) *fakeStarter {
	t.Helper()
	return &fakeStarter{t: t, scripts: make(map[string][]string)}
}

func (f *fakeStarter) add(name string, lines []string) {
	f.scripts[name] = lines
}

func (f *fakeStarter) Start(_ context.Context, name, _ string, _ []string) (*containers.Process, error) {
	lines, ok := f.scripts[name]
	if !ok {
		return nil, fmt.Errorf("fakeStarter: unknown process %q", name)
	}
	// Pre-fill and immediately close the channel: the process "exits" as soon as all
	// buffered lines are written. No goroutines needed — no data races possible.
	ch := make(chan string, len(lines))
	for _, l := range lines {
		ch <- l
	}
	close(ch)
	wait := func() error { return nil } // returns immediately
	kill := func() error { return nil } // no-op: process already "exited"
	return containers.NewProcess(ch, wait, kill), nil
}

func (f *fakeStarter) Stop(_ context.Context, name string) error { return nil }
func (f *fakeStarter) Kill(name string) error                     { return nil }

// fakeProcess builds a *containers.Process backed by explicit channels/funcs.
func fakeProcess(lines []string, waitErr error) (*containers.Process, func()) {
	ch := make(chan string, len(lines)+1)
	for _, l := range lines {
		ch <- l
	}
	done := make(chan struct{})
	wait := func() error {
		<-done
		return waitErr
	}
	kill := func() error {
		select {
		case <-done:
		default:
			close(done)
		}
		return nil
	}
	proc := containers.NewProcess(ch, wait, kill)
	complete := func() {
		select {
		case <-done:
		default:
			close(done)
		}
		close(ch)
	}
	return proc, complete
}

// must fails the test if err is non-nil.
func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
