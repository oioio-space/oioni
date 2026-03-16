// tools/containers/manager.go
package containers

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Config describes a managed container.
type Config struct {
	Image          string   // e.g. "oioni/impacket:arm64"
	Name           string   // container name, unique per instance
	Network        string   // "host" for USB gadget interface access
	Caps           []string // Linux capabilities, e.g. ["NET_RAW", "NET_ADMIN"]
	LocalImagePath string   // if set, load image from this .tar/.tar.gz instead of pulling
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

// gokrazySearchDirs lists directories where gokrazy installs binaries, in order.
// podman lives in /usr/local/bin; other helpers (busybox, etc.) in /user.
var gokrazySearchDirs = []string{"/user", "/usr/local/bin", "/usr/bin", "/bin"}

// defaultCmdFactory resolves the binary against gokrazy search dirs before
// calling exec.Command — exec.LookPath uses the parent process PATH which may
// not include /usr/local/bin on gokrazy.
func defaultCmdFactory(name string, args ...string) *exec.Cmd {
	resolved := resolveBinary(name)
	cmd := exec.Command(resolved, args...)
	cmd.Env = gokrazyEnv(os.Environ())
	return cmd
}

// resolveBinary finds name in gokrazySearchDirs, falling back to the bare name
// (which lets exec.Command return a clear "not found" error).
func resolveBinary(name string) string {
	if strings.ContainsRune(name, '/') {
		return name // already an absolute or relative path
	}
	for _, dir := range gokrazySearchDirs {
		if _, err := os.Stat(dir + "/" + name); err == nil {
			return dir + "/" + name
		}
	}
	return name // not found — exec.Command will return exec.ErrNotFound
}

// gokrazyEnv returns env with /user:/usr/local/bin prepended to PATH and
// TMPDIR=/tmp ensured (podman requires a writable temp dir, /tmp on gokrazy).
func gokrazyEnv(env []string) []string {
	const extra = "/user:/usr/local/bin"
	hasPath, hasTMPDIR := false, false
	for i, v := range env {
		if strings.HasPrefix(v, "PATH=") {
			env[i] = "PATH=" + extra + ":" + v[5:]
			hasPath = true
		}
		if strings.HasPrefix(v, "TMPDIR=") {
			hasTMPDIR = true
		}
	}
	if !hasPath {
		env = append(env, "PATH="+extra+":/usr/local/sbin:/sbin:/usr/sbin:/bin:/usr/bin")
	}
	if !hasTMPDIR {
		env = append(env, "TMPDIR=/tmp")
	}
	return env
}

// gokrazyPath is an alias kept for backward compatibility.
func gokrazyPath(env []string) []string { return gokrazyEnv(env) }

// initContainer loads or pulls the image, then starts the long-running container.
// Called exactly once via sync.Once.
func (m *ProcManager) initContainer(ctx context.Context) error {
	// Ensure image is available — load from local file or pull from registry.
	if m.cfg.LocalImagePath != "" {
		if err := m.loadImage(m.cfg.LocalImagePath); err != nil {
			return err
		}
	} else {
		pullCmd := m.cmdFactory("podman", "pull", m.cfg.Image)
		pullCmd.Stderr = os.Stderr
		if err := pullCmd.Run(); err != nil {
			if isNotFound(err) {
				return ErrPodmanNotFound
			}
			return fmt.Errorf("containers: podman pull %s: %w", m.cfg.Image, err)
		}
	}

	// Clean up any leftover container from a previous run (name collision prevention).
	stopCmd := m.cmdFactory("podman", "stop", m.cfg.Name)
	stopCmd.Stdout = io.Discard
	stopCmd.Stderr = io.Discard
	_ = stopCmd.Run()
	rmCmd := m.cmdFactory("podman", "rm", m.cfg.Name)
	rmCmd.Stdout = io.Discard
	rmCmd.Stderr = io.Discard
	_ = rmCmd.Run()

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

// loadImage loads a container image from a local .tar or .tar.gz file.
// It first checks whether the image is already present to avoid redundant loads.
func (m *ProcManager) loadImage(path string) error {
	// Check if already loaded — fast path on subsequent runs.
	existsCmd := m.cmdFactory("podman", "image", "exists", m.cfg.Image)
	if err := existsCmd.Run(); err == nil {
		return nil // already present
	} else if isNotFound(err) {
		return ErrPodmanNotFound // podman binary itself not found
	}
	// Image not present — load from file.
	// Use /perm/tmp as TMPDIR: podman needs temp space proportional to the
	// uncompressed image size. /tmp is a tmpfs limited by RAM; /perm (SD card)
	// has ample space for a ~125 MiB image.
	if err := os.MkdirAll("/perm/tmp", 0o700); err != nil {
		return fmt.Errorf("containers: mkdir /perm/tmp: %w", err)
	}
	loadCmd := m.cmdFactory("podman", "load", "-i", path)
	loadCmd.Stderr = os.Stderr // surface podman errors to gokrazy logs
	// Override TMPDIR in the child env to use /perm/tmp.
	env := make([]string, 0, len(loadCmd.Env))
	for _, v := range loadCmd.Env {
		if !strings.HasPrefix(v, "TMPDIR=") {
			env = append(env, v)
		}
	}
	loadCmd.Env = append(env, "TMPDIR=/perm/tmp")
	if err := loadCmd.Run(); err != nil {
		if isNotFound(err) {
			return ErrPodmanNotFound
		}
		return fmt.Errorf("containers: podman load %s: %w", path, err)
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
	// executable is single-quoted to prevent shell injection.
	execArgs := []string{"exec", m.cfg.Name, "sh", "-c",
		fmt.Sprintf("echo $$; exec %s %s", shellQuote(executable), shellJoin(args))}
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

	// Register entry and add watcher to wg under the same lock so that a
	// concurrent Close() either sees the entry (and kills it) or sees
	// m.closed=true (and we bail here). wg.Add inside the lock guarantees
	// Close()'s wg.Wait() cannot return before our watcher goroutine starts.
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		_ = kill()
		_ = cmd.Wait()
		return nil, ErrManagerClosed
	}
	m.wg.Add(1)
	m.procs[capturedName] = entry
	m.mu.Unlock()

	// Background watcher: deregister once process exits.
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

// Stop sends SIGTERM, waits up to 10 s, then escalates to SIGKILL.
// The effective deadline is min(ctx, 10 s) for SIGTERM, then 5 s more for SIGKILL.
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
	stopCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	select {
	case <-entry.gone:
		return nil
	case <-stopCtx.Done():
		// Escalate to SIGKILL.
		_ = entry.proc.Kill()
		select {
		case <-entry.gone:
		case <-time.After(5 * time.Second):
			return errors.New("containers: process did not exit after SIGKILL")
		}
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

// shellQuote single-quotes one argument, escaping embedded single quotes.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// shellJoin builds a shell-safe argument string from a slice.
func shellJoin(args []string) string {
	quoted := make([]string, len(args))
	for i, a := range args {
		quoted[i] = shellQuote(a)
	}
	return strings.Join(quoted, " ")
}
