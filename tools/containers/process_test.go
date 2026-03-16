// tools/containers/process_test.go
package containers_test

import (
	"errors"
	"os/exec"
	"testing"

	"github.com/oioio-space/oioni/tools/containers"
)

// makeProcess creates a Process backed by an in-process channel and functions.
func makeProcess(lines []string, waitErr error) (*containers.Process, func()) {
	ch := make(chan string, len(lines)+1)
	for _, l := range lines {
		ch <- l
	}
	killed := make(chan struct{})
	waitCalled := make(chan struct{}, 1)

	wait := func() error {
		waitCalled <- struct{}{}
		<-killed
		return waitErr
	}
	kill := func() error {
		select {
		case <-killed:
		default:
			close(killed)
		}
		return nil
	}
	p := containers.NewProcess(ch, wait, kill)
	// Caller must close ch to signal process exit.
	done := func() {
		close(ch)
		<-waitCalled
		kill()
	}
	return p, done
}

func TestProcess_LinesAndWait(t *testing.T) {
	ch := make(chan string, 2)
	ch <- "line1"
	ch <- "line2"
	done := make(chan struct{})
	wait := func() error { <-done; return nil }
	kill := func() error { return nil }
	p := containers.NewProcess(ch, wait, kill)

	if !p.Running() {
		t.Fatal("expected Running()=true before wait")
	}

	close(done) // simulate process exit
	if err := p.Wait(); err != nil {
		t.Fatalf("Wait() = %v, want nil", err)
	}
	if err := p.Wait(); err != nil { // idempotent
		t.Fatalf("second Wait() = %v, want nil", err)
	}
	if p.Running() {
		t.Fatal("expected Running()=false after wait")
	}

	close(ch) // simulate end of process output (in real impl, the feed goroutine closes it)
	got := []string{}
	for l := range ch { // channel still readable; range terminates when ch closed
		got = append(got, l)
	}
	if len(got) != 2 {
		t.Fatalf("Lines got %v, want [line1 line2]", got)
	}
}

func TestProcess_LinesOpenAfterRunningFalse(t *testing.T) {
	// Guard: Lines() is still readable after Running()=false (OS exit precedes drain).
	ch := make(chan string, 3)
	ch <- "a"
	ch <- "b"
	ch <- "c"

	done := make(chan struct{})
	p := containers.NewProcess(ch, func() error { close(done); return nil }, func() error { return nil })

	// Trigger wait in goroutine
	go p.Wait()
	<-done // wait returned, OS process is "gone"

	if p.Running() {
		t.Fatal("expected Running()=false")
	}
	// channel must still yield buffered lines
	n := 0
	for range ch {
		n++
		if n == 3 {
			break
		}
	}
	if n != 3 {
		t.Fatalf("expected 3 lines after Running()=false, got %d", n)
	}
}

func TestProcess_WaitExitError(t *testing.T) {
	// When wait returns *ExitError, Wait() propagates it.
	exitErr := &containers.ExitError{Err: fakeExitError(t)}
	done := make(chan struct{})
	p := containers.NewProcess(make(chan string), func() error { <-done; return exitErr }, func() error { return nil })
	close(done)
	err := p.Wait()
	if err == nil {
		t.Fatal("want error, got nil")
	}
	var ee *containers.ExitError
	if !errors.As(err, &ee) {
		t.Fatalf("expected *ExitError, got %T: %v", err, err)
	}
}

func fakeExitError(t *testing.T) *exec.ExitError {
	t.Helper()
	cmd := exec.Command("sh", "-c", "exit 1")
	err := cmd.Run()
	var ee *exec.ExitError
	if !errors.As(err, &ee) {
		t.Fatalf("could not get *exec.ExitError: %v", err)
	}
	return ee
}
