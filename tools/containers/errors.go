package containers

import (
	"errors"
	"os/exec"
)

var (
	// ErrPodmanNotFound is returned when the podman binary cannot be located.
	ErrPodmanNotFound = errors.New("containers: podman binary not found")

	// ErrAlreadyRunning is returned by Start when a process with the given name
	// is already registered in the process registry.
	ErrAlreadyRunning = errors.New("containers: process already running")

	// ErrManagerClosed is returned by Start after Close() has been called.
	ErrManagerClosed = errors.New("containers: manager is closed")
)

// ExitError is returned by Process.Wait() when the process exits with a non-zero status.
type ExitError struct {
	Err *exec.ExitError // never nil
}

func (e *ExitError) Error() string { return e.Err.Error() }
func (e *ExitError) Unwrap() error { return e.Err }
func (e *ExitError) ExitCode() int { return e.Err.ExitCode() }
