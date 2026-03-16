// tools/containers/process.go
package containers

import "sync"

// Process represents a running process inside a container.
// Use NewProcess to construct — do not copy a Process.
type Process struct {
	lines  <-chan string
	waitFn func() error
	killFn func() error
	done   chan struct{} // closed when waitFn() has been called and returned
}

// NewProcess constructs a Process from pre-built components.
// Stable public API — used by impacket tests to create fake processes without instantiating ProcManager.
//
//	lines — channel of lines, closed by the provider when the process exits.
//	wait  — blocks until exit; returns nil or *ExitError. Wrapped with sync.Once:
//	        subsequent calls return the cached result without blocking.
//	kill  — sends SIGKILL; may return an error if the process is already gone.
func NewProcess(lines <-chan string, wait func() error, kill func() error) *Process {
	p := &Process{
		lines:  lines,
		killFn: kill,
		done:   make(chan struct{}),
	}
	var (
		once   sync.Once
		result error
	)
	p.waitFn = func() error {
		once.Do(func() {
			result = wait()
			close(p.done)
		})
		return result
	}
	return p
}

// Lines returns the channel of stdout+stderr lines. See spec for capacity/drop behaviour.
func (p *Process) Lines() <-chan string { return p.lines }

// Wait blocks until the process exits. Returns nil or *ExitError.
// Idempotent: subsequent calls return the cached result immediately.
func (p *Process) Wait() error { return p.waitFn() }

// Kill sends SIGKILL immediately.
func (p *Process) Kill() error { return p.killFn() }

// Running returns false once the OS process has exited (Wait() has returned).
func (p *Process) Running() bool {
	select {
	case <-p.done:
		return false
	default:
		return true
	}
}
