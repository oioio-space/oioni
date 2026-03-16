// tools/containers/manager.go
package containers

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

// Config describes a managed container.
type Config struct {
	Image   string   // e.g. "oioni/impacket:arm64"
	Name    string   // container name, unique per instance
	Network string   // "host" for USB gadget interface access
	Caps    []string // Linux capabilities, e.g. ["NET_RAW", "NET_ADMIN"]
}

// Option is a functional option for NewManager.
type Option func(*ProcManager)

// WithCmdFactory replaces the internal exec.Cmd factory for all podman invocations.
// The factory is called once per invocation; the returned *exec.Cmd is used once only.
func WithCmdFactory(factory func(name string, args ...string) *exec.Cmd) Option {
	return func(m *ProcManager) { m.cmdFactory = factory }
}

// procEntry is a registry slot for a running process.
type procEntry struct {
	proc         *Process
	cmd          *exec.Cmd
	containerPID int
	gone         chan struct{} // closed by the background watcher goroutine after deregistration
}

// ProcManager manages a single long-running container and the processes inside it.
type ProcManager struct {
	cfg        Config
	cmdFactory func(string, ...string) *exec.Cmd

	once     sync.Once
	startErr error

	mu     sync.Mutex
	closed bool
	procs  map[string]*procEntry

	wg sync.WaitGroup // tracks background watcher goroutines
}

// NewManager creates a ProcManager. No I/O occurs at construction time.
func NewManager(cfg Config, opts ...Option) *ProcManager {
	m := &ProcManager{
		cfg:   cfg,
		procs: make(map[string]*procEntry),
	}
	m.cmdFactory = defaultCmdFactory
	for _, o := range opts {
		o(m)
	}
	return m
}

func defaultCmdFactory(name string, args ...string) *exec.Cmd {
	return exec.Command(name, args...)
}

// initContainer pulls the image and starts the long-running container.
// Called exactly once via sync.Once.
func (m *ProcManager) initContainer(ctx context.Context) error {
	// Pull image
	pullCmd := m.cmdFactory("podman", "pull", m.cfg.Image)
	pullCmd.Stderr = io.Discard
	if err := pullCmd.Run(); err != nil {
		if isNotFound(err) {
			return ErrPodmanNotFound
		}
		return fmt.Errorf("containers: podman pull %s: %w", m.cfg.Image, err)
	}

	// Build podman run args
	runArgs := []string{"run", "-d", "--name", m.cfg.Name}
	if m.cfg.Network != "" {
		runArgs = append(runArgs, "--network", m.cfg.Network)
	}
	for _, cap := range m.cfg.Caps {
		runArgs = append(runArgs, "--cap-add", cap)
	}
	runArgs = append(runArgs, m.cfg.Image, "sleep", "infinity")

	runCmd := m.cmdFactory("podman", runArgs...)
	runCmd.Stderr = io.Discard
	if err := runCmd.Run(); err != nil {
		return fmt.Errorf("containers: podman run: %w", err)
	}
	return nil
}

func isNotFound(err error) bool {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return false // podman exists but failed for another reason
	}
	// exec.ErrNotFound or similar path errors
	return true
}

// Start initialises the container (once) then launches the named executable inside it.
// name must be unique among running processes; ErrAlreadyRunning is returned otherwise.
func (m *ProcManager) Start(ctx context.Context, name, executable string, args []string) (*Process, error) {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil, ErrManagerClosed
	}
	if _, exists := m.procs[name]; exists {
		m.mu.Unlock()
		return nil, ErrAlreadyRunning
	}
	m.mu.Unlock()

	// Initialise container exactly once.
	m.once.Do(func() {
		m.startErr = m.initContainer(ctx)
	})
	if m.startErr != nil {
		return nil, m.startErr
	}

	// Launch tool via podman exec.
	// First line of stdout is the containerPID; subsequent lines are tool output.
	execArgs := append([]string{"exec", m.cfg.Name, "sh", "-c",
		fmt.Sprintf("echo $$; exec %s %s", executable, shellJoin(args))})
	cmd := m.cmdFactory("podman", execArgs...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("containers: stdout pipe: %w", err)
	}
	cmd.Stderr = io.Discard

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("containers: podman exec start: %w", err)
	}

	// Read containerPID from first line.
	scanner := bufio.NewScanner(stdout)
	var containerPID int
	if scanner.Scan() {
		pid, parseErr := strconv.Atoi(strings.TrimSpace(scanner.Text()))
		if parseErr == nil {
			containerPID = pid
		}
	}

	// Feed remaining lines into a buffered channel.
	linesCh := make(chan string, 64)
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		defer close(linesCh)
		for scanner.Scan() {
			select {
			case linesCh <- scanner.Text():
			default:
				// Drop line if consumer is slow; prevents blocking the goroutine.
			}
		}
	}()

	capturedPID := containerPID
	capturedName := name

	kill := func() error {
		if capturedPID <= 0 {
			return nil
		}
		killCmd := m.cmdFactory("podman", "exec", m.cfg.Name, "kill", "-KILL",
			strconv.Itoa(capturedPID))
		killCmd.Stdout = io.Discard
		killCmd.Stderr = io.Discard
		return killCmd.Run()
	}

	wait := func() error {
		err := cmd.Wait()
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return &ExitError{Err: exitErr}
		}
		return err
	}

	proc := NewProcess(linesCh, wait, kill)

	entry := &procEntry{
		proc:         proc,
		cmd:          cmd,
		containerPID: containerPID,
		gone:         make(chan struct{}),
	}

	m.mu.Lock()
	m.procs[capturedName] = entry
	m.mu.Unlock()

	// Background watcher: deregister once process exits.
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		proc.Wait() //nolint:errcheck
		m.mu.Lock()
		delete(m.procs, capturedName)
		m.mu.Unlock()
		close(entry.gone)
	}()

	return proc, nil
}

// Stop sends SIGTERM to the named process, then waits for it to exit.
func (m *ProcManager) Stop(ctx context.Context, name string) error {
	m.mu.Lock()
	entry, ok := m.procs[name]
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("containers: process %q not found", name)
	}
	if entry.containerPID > 0 {
		termCmd := m.cmdFactory("podman", "exec", m.cfg.Name, "kill", "-TERM",
			strconv.Itoa(entry.containerPID))
		termCmd.Stdout = io.Discard
		termCmd.Stderr = io.Discard
		_ = termCmd.Run()
	}
	select {
	case <-entry.gone:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Kill immediately sends SIGKILL to the named process.
func (m *ProcManager) Kill(name string) error {
	m.mu.Lock()
	entry, ok := m.procs[name]
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("containers: process %q not found", name)
	}
	return entry.proc.Kill()
}

// List returns the names of all currently running processes.
func (m *ProcManager) List() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	names := make([]string, 0, len(m.procs))
	for n := range m.procs {
		names = append(names, n)
	}
	return names
}

// Close kills all running processes, stops the container, and releases resources.
func (m *ProcManager) Close() error {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil
	}
	m.closed = true
	// Snapshot entries to kill outside the lock.
	entries := make([]*procEntry, 0, len(m.procs))
	for _, e := range m.procs {
		entries = append(entries, e)
	}
	m.mu.Unlock()

	for _, e := range entries {
		_ = e.proc.Kill()
	}
	m.wg.Wait()

	// Remove the container.
	rmCmd := m.cmdFactory("podman", "rm", "-f", m.cfg.Name)
	rmCmd.Stdout = io.Discard
	rmCmd.Stderr = io.Discard
	_ = rmCmd.Run()

	return nil
}

// shellJoin builds a minimal shell-safe argument string.
func shellJoin(args []string) string {
	quoted := make([]string, len(args))
	for i, a := range args {
		quoted[i] = "'" + strings.ReplaceAll(a, "'", `'\''`) + "'"
	}
	return strings.Join(quoted, " ")
}
