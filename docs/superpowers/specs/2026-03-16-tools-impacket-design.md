# Tools / Impacket — Design Spec

**Date:** 2026-03-16
**Status:** Draft

---

## Goal

Provide a Go module `tools/` that wraps Podman to run containerised security tools programmatically. The first tool is impacket. The API is designed to be consumed by Go programs (including the e-paper UI) without any knowledge of Podman or Python internals.

## Hardware context

- Raspberry Pi Zero 2W — 4× Cortex-A53 @ 1 GHz, 512 MB RAM
- USB gadget interfaces: `usb0` (RNDIS), optionally ECM
- gokrazy OS — no libc, no shell; programs are Go binaries
- Podman available via `github.com/gokrazy/podman` package

## Repository layout

```
tools/
  go.mod                      — module github.com/oioio-space/oioni/tools
  containers/
    manager.go                — ProcManager: container lifecycle + process registry
    process.go                — Process: streaming output, wait, kill; NewProcess constructor
    errors.go                 — sentinel errors
    manager_test.go
    process_test.go
  impacket/
    impacket.go               — Impacket facade + ProcessStarter interface
    runner.go                 — generic Run(ctx, name, tool, args) → *Process
    ntlmrelayx.go             — NTLMRelay() + NTLMRelayConfig + NTLMRelayEvent
    secretsdump.go            — SecretsDump() + SecretsDumpConfig + Credential
    impacket_test.go
    ntlmrelayx_test.go
    secretsdump_test.go
```

Import paths:
- `github.com/oioio-space/oioni/tools/containers`
- `github.com/oioio-space/oioni/tools/impacket`

`go.work` gains `./tools`.

---

## Package `containers`

### Responsibility

Generic Podman process manager. Knows nothing about impacket. Can be reused for any future containerised tool.

### Container strategy

One long-running container is started once (`podman run -d … sleep infinity`). Each tool invocation runs as a `podman exec` inside that container, eliminating per-invocation cold-start (3–8 s on ARM64).

Container startup — lazy, triggered on first `Start()` call:
1. `podman pull <image>` if not present locally
2. `podman run -d --network host --cap-add NET_RAW --cap-add NET_ADMIN --name <name> <image> sleep infinity`
3. Subsequent calls: `podman exec <name> <tool> <args...>`

`--network host` provides direct access to all host interfaces including USB gadget (`usb0`, `usb1`).
`NET_RAW` + `NET_ADMIN` are required for raw packet capture and interface manipulation.

### Sentinel errors

```go
// errors.go
var (
    ErrPodmanNotFound  = errors.New("containers: podman binary not found")
    ErrAlreadyRunning  = errors.New("containers: process already running")
    ErrManagerClosed   = errors.New("containers: manager is closed")
)
```

### ExitError

```go
// ExitError is returned by Process.Wait() when the process exits with a non-zero status.
type ExitError struct {
    Err *exec.ExitError // never nil
}

func (e *ExitError) Error() string  { return e.Err.Error() }
func (e *ExitError) Unwrap() error  { return e.Err }
func (e *ExitError) ExitCode() int  { return e.Err.ExitCode() }
```

### API

```go
// Config describes a managed container.
type Config struct {
    Image   string   // e.g. "ghcr.io/fortra/impacket:latest"
    Name    string   // container name, unique per instance
    Network string   // "host" for gadget interface access
    Caps    []string // e.g. ["NET_RAW", "NET_ADMIN"]
}

// Option is a functional option for NewManager.
type Option func(*ProcManager)

// WithCmdFactory replaces the internal exec.Cmd factory used for all podman
// invocations (pull, run, exec, rm). The factory is called once per
// podman invocation, never reused — exec.Cmd cannot be started more than once.
// Note: `podman stop` is never issued by ProcManager; signal delivery goes directly
// to the host-side podman exec OS process PID.
// This is the injection seam for containers-level tests; it is orthogonal to the
// ProcessStarter+NewProcess seam used by impacket-level tests.
func WithCmdFactory(factory func(name string, args ...string) *exec.Cmd) Option

// NewManager creates a ProcManager. No I/O occurs at construction time.
func NewManager(cfg Config, opts ...Option) *ProcManager

type ProcManager struct { /* ... */ }

// Start ensures the backing container is running (lazy init, idempotent across
// concurrent calls), then launches a named process inside it via `podman exec`.
//
// Container init uses sync.Once. The startup error, if any, is stored in a companion
// startErr field written exclusively inside Once.Do. Reads of startErr occur only
// after once.Do() returns — no read is attempted before or outside Once — so the
// field is safe without an additional mutex (Go memory model: once.Do() establishes
// a happens-before). Panics inside Once.Do are not recovered; they propagate to the
// caller as a fatal runtime panic (acceptable on gokrazy, which has no crash recovery).
//
// After a failed container init, startErr is returned on all subsequent Start() calls
// without retrying. The manager must be discarded and a new one created to retry.
//
// Start() registers the name in the process registry and launches a background watcher
// goroutine that calls proc.Wait() internally and deregisters the name automatically
// when the process exits — whether by natural exit, Kill(), or Stop(). This means
// callers never need to manually deregister; stopping a process via any mechanism
// always leaves the registry consistent.
//
// Returns ErrAlreadyRunning if the name is currently registered.
// Returns ErrManagerClosed if Close() has already been called.
func (m *ProcManager) Start(ctx context.Context, name, executable string, args []string) (*Process, error)

// Stop signals the named process to terminate and waits for it to exit.
//
// Signal delivery: SIGTERM sent to the host-side `podman exec` OS process.
// ProcManager's internal registry entry stores both the `*Process` returned to
// the caller and the `*exec.Cmd` that launched `podman exec`; Stop uses
// cmd.Process.Signal(syscall.SIGTERM) from that stored Cmd. Podman propagates
// the signal into the container process.
//
// Deadline: Stop wraps ctx internally as context.WithTimeout(ctx, 10s), applying
// whichever deadline comes first (minimum of caller deadline and 10 s).
//
// If the process has not exited by that deadline, Stop escalates to SIGKILL.
// After SIGKILL, Stop waits up to an additional 5 s (fixed internal timeout) for
// the OS to deliver the signal and for the background watcher goroutine to
// deregister the name. If the watcher has not completed within 5 s, Stop returns
// an error but the name may still be registered — a pathological case (zombie
// process) that callers should treat as fatal. On a successful SIGKILL, Stop
// returns nil.
//
// Stop always returns only after the name has been deregistered from the registry,
// so subsequent Start() with the same name is always safe after Stop() returns
// (absent the zombie-process error path).
func (m *ProcManager) Stop(ctx context.Context, name string) error

// Kill sends SIGKILL to the named process immediately. Does not wait for exit;
// deregistration is handled asynchronously by the background watcher goroutine.
// Callers who need to reuse the same name immediately after stopping must use
// Stop() instead, which waits for deregistration before returning.
func (m *ProcManager) Kill(name string) error

// List returns the names of all currently registered (running) processes.
func (m *ProcManager) List() []string

// Close kills all running processes (SIGKILL), waits for all background watcher
// goroutines to complete deregistration, and removes the container via `podman rm -f`.
// Close() returns only after all watcher goroutines have exited, so callers (e.g.
// tests) can safely inspect the registry after Close() returns and find it empty.
// After Close() returns, all subsequent Start() calls return ErrManagerClosed.
// Close() is safe to call concurrently with Start(); any Start() that begins after
// Close() is called will receive ErrManagerClosed.
func (m *ProcManager) Close() error
```

### Process

`NewProcess` is an exported constructor that lets `impacket` package tests construct
`*containers.Process` values backed by in-process pipes — the injection seam for
impacket-level tests. This API is stable: changing its signature is a breaking change.

```go
// NewProcess constructs a Process from pre-built components.
//   lines — channel of text lines, closed by the provider when the process exits.
//   wait  — blocks until exit; returns nil or *ExitError. The implementation wraps
//           wait with sync.Once so it is safe and idempotent to call multiple times;
//           subsequent calls return the cached result without blocking.
//   kill  — sends SIGKILL; may return an error if the process is already gone.
func NewProcess(lines <-chan string, wait func() error, kill func() error) *Process

type Process struct { /* ... */ }

// Lines returns a channel of stdout+stderr lines, closed when the process exits.
//
// For processes created by ProcManager (the real implementation): the channel has
// capacity 64 and is fed by a goroutine tied to process lifetime via non-blocking
// sends. When the channel is full, lines are dropped to prevent the feeder from
// stalling the child process. Callers that cannot afford dropped lines must drain
// the channel faster than lines arrive.
//
// For processes created by NewProcess (test fakes): the channel capacity and feeding
// behaviour are determined by the caller-supplied `lines` argument. The drop-on-full
// contract does not apply; the test fake controls its own channel.
func (p *Process) Lines() <-chan string

// Wait blocks until the process exits. Returns nil on clean exit, *ExitError otherwise.
// Running() transitions to false when Wait() would return, i.e., when the OS process
// has exited (not when Lines() closes; stdout drain may lag slightly).
func (p *Process) Wait() error

// Kill sends SIGKILL immediately.
func (p *Process) Kill() error

// Running returns false once the OS process has exited (i.e., once Wait() would return).
func (p *Process) Running() bool
```

### Concurrency

- `ProcManager` is safe for concurrent use.
- Process registry is guarded by a `sync.Mutex`.
- Container startup uses `sync.Once`; `startErr` is written inside `Once.Do` and read
  only after `once.Do()` returns.
- `Close()` sets a closed flag under the registry mutex; `Start()` checks this flag
  while holding the same mutex to guarantee ErrManagerClosed is returned consistently.

---

## Package `impacket`

### Responsibility

Impacket-specific facade. Provides:
1. A generic runner for any impacket script.
2. Typed wrappers for `ntlmrelayx` and `secretsdump` with structured output.

### Image

`ghcr.io/fortra/impacket:latest` — official impacket image, ~350 MB compressed on disk
(actual RSS of the Python process is ~80–120 MB). Supports arm64.

### ProcessStarter interface

The `impacket` package depends on this interface, not on `*containers.ProcManager`.
Test fakes implement it entirely in-process by returning `*containers.Process` values
built with `containers.NewProcess`. No `ProcManager` is ever instantiated in impacket tests.

These two seams are orthogonal:
- `WithCmdFactory` → containers tests (fake podman binary)
- `ProcessStarter` + `containers.NewProcess` → impacket tests (fake in-process starter)

```go
// ProcessStarter is the full lifecycle interface that impacket needs.
// *containers.ProcManager satisfies this interface.
// Test fakes must implement all three methods; Stop and Kill may be no-ops if
// the test does not exercise lifecycle management.
type ProcessStarter interface {
    Start(ctx context.Context, name, executable string, args []string) (*containers.Process, error)
    Stop(ctx context.Context, name string) error
    Kill(name string) error
}
```

### API

#### Facade

```go
// impacketConfig holds configuration for the Impacket facade.
// Extended by future ImpacketOption values.
type impacketConfig struct {
    // reserved for future options (custom image, registry credentials, etc.)
}

// ImpacketOption is a functional option for New().
type ImpacketOption func(*impacketConfig)

// No options are defined today; the type exists for forward compatibility.

// New returns an Impacket backed by a real ProcManager (official impacket image,
// host networking, raw packet capabilities). The container is not started until
// the first tool call.
func New(opts ...ImpacketOption) *Impacket

// NewWithManager injects a custom ProcessStarter (for tests).
func NewWithManager(mgr ProcessStarter) *Impacket
```

#### Generic runner

```go
// Run launches any impacket script. name must be unique among currently running procs.
func (i *Impacket) Run(ctx context.Context, name, tool string, args []string) (*containers.Process, error)
```

#### NTLMRelay wrapper

```go
type NTLMRelayConfig struct {
    Target      string // relay target, e.g. "smb://192.168.1.1"
    SMB2Support bool   // enables SMB2 relay (recommended)
    OutputFile  string // write captured hashes inside the container; optional
}

type NTLMRelayEvent struct {
    Username string
    Domain   string
    Hash     string // NTLMv2 hash
    Target   string
}

// NTLMRelayProcess wraps the underlying container process.
// Does NOT embed *containers.Process to avoid ambiguous Kill()/Wait() methods.
// Use Process() for raw process access, or Stop()/Kill() for lifecycle management.
type NTLMRelayProcess struct {
    // unexported: proc *containers.Process, mgr ProcessStarter,
    //             name string, events chan NTLMRelayEvent,
    //             mu sync.Mutex, err error
}

// NTLMRelayProcess obtains Stop/Kill capability directly from the ProcessStarter
// (which now includes Stop and Kill). No separate processKiller interface is needed.

// Process returns the underlying container process for Wait()/Running()/Lines() access.
func (p *NTLMRelayProcess) Process() *containers.Process

// Events returns a buffered channel (capacity 16) of parsed NTLMRelayEvent values.
// The parsing goroutine sends events with a non-blocking send and drops them if the
// channel is full, preventing the parser from blocking under a slow consumer.
// The channel is closed when the process exits (under p.mu — see Err()).
func (p *NTLMRelayProcess) Events() <-chan NTLMRelayEvent

// Err returns the error that caused the process to exit, if any.
// Concurrency: the parsing goroutine uses the following exact sequence:
//   (1) p.mu.Lock()
//   (2) p.err = exitErr
//   (3) close(p.events)    ← channel closed while holding the mutex
//   (4) p.mu.Unlock()
// Closing the channel inside the lock means there is no observable window where
// Err() returns non-nil but Events() is not yet closed. Receivers of Events() do
// not acquire p.mu (p.mu is unexported, so no external code can hold it), so closing
// the channel under the lock does not risk deadlock.
// Err() acquires p.mu before reading p.err, so it is safe for concurrent calls.
// Note: a caller ranging over Events() on the same goroutine as a Stop() call would
// deadlock on its own (Stop blocks waiting for the process to exit, which cannot
// happen if the goroutine is also consuming events) — this is caller misuse, not a
// spec defect.
// Returns nil while the process is running.
func (p *NTLMRelayProcess) Err() error

// Stop signals the ntlmrelayx daemon to terminate gracefully via the ProcessStarter
// (which includes Stop), waits for exit, and deregisters the name.
func (p *NTLMRelayProcess) Stop(ctx context.Context) error

// Kill sends SIGKILL immediately via the ProcessStarter. Deregistration is asynchronous
// (handled by the background watcher goroutine in ProcManager; no-op in test fakes).
func (p *NTLMRelayProcess) Kill() error

// NTLMRelay starts ntlmrelayx as a background daemon. name must be unique.
func (i *Impacket) NTLMRelay(ctx context.Context, name string, cfg NTLMRelayConfig) (*NTLMRelayProcess, error)
```

#### SecretsDump wrapper

```go
type SecretsDumpConfig struct {
    Target   string // host IP or hostname
    Username string
    Password string // one of Password or Hash required
    Hash     string // pass-the-hash, format "LMHASH:NTHASH"
    Domain   string // optional; defaults to target hostname
}

type Credential struct {
    Username string
    Domain   string
    Hash     string
    Type     string // "NTLM", "Kerberos", "Plaintext"
}

// SecretsDump runs secretsdump.py and returns parsed credentials.
// name must be unique among currently running procs.
//
// Implementation:
//   proc, err := i.mgr.Start(ctx, name, "secretsdump.py", args)
//   if err != nil { return nil, err }
//
//   // Capacity 1 is load-bearing: when ctx is cancelled, nobody reads from done
//   // after SecretsDump returns. Without capacity 1 the goroutine would leak,
//   // blocked forever on the send.
//   done := make(chan result, 1)
//   go func() {
//       creds, err := parseSecretsDump(proc.Lines()) // drains Lines() in this goroutine
//       done <- result{creds, err} // always succeeds: buffered, or caller is waiting
//   }()
//
//   select {
//   case r := <-done:
//       // errors.Join(r.err, proc.Wait()) surfaces both parse errors and
//       // non-zero exit status; returns nil if both are nil.
//       return r.creds, errors.Join(r.err, proc.Wait())
//   case <-ctx.Done():
//       _ = i.mgr.Kill(name)
//       // After Kill, wait up to 5 s for the goroutine to finish (proc.Lines() closes
//       // once the container process exits after SIGKILL). This bounds the goroutine
//       // lifetime even when Kill appears to succeed but signal delivery into the container
//       // is delayed or lost. If the goroutine does not finish within 5 s, return a
//       // non-nil error alongside ctx.Err() so callers know cleanup is incomplete.
//       // In that case the name may remain registered and the goroutine leaks — treated
//       // as a fatal OS-level failure outside the spec's recovery scope.
//       select {
//       case <-done: // goroutine finished cleanly
//       case <-time.After(5 * time.Second):
//           return nil, errors.Join(ctx.Err(), errors.New("secretsdump: kill did not stop process within 5s"))
//       }
//       // Deregistration is asynchronous (Kill is async); callers who immediately retry
//       // with the same name may receive ErrAlreadyRunning. Use distinct names per
//       // invocation to avoid this.
//       return nil, ctx.Err()
//   }
//
// Lines() is drained in the launched goroutine, not in the select caller.
// If ctx.Done() and done fire simultaneously, whichever case the runtime selects
// determines the return: credentials on done, ctx.Err() on cancellation.
func (i *Impacket) SecretsDump(ctx context.Context, name string, cfg SecretsDumpConfig) ([]Credential, error)
```

---

## Testing strategy

### `containers/` tests — injection via `WithCmdFactory`

Inject a fake podman binary via `WithCmdFactory`. The factory is called once per
podman invocation and returns a fresh `*exec.Cmd` each time (never reused).
The fake accepts `pull`, `run`, `exec`, `rm` subcommands, writes predictable stdout,
exits 0. (`podman stop` is never called by ProcManager — signal delivery goes directly
to the host-side `podman exec` OS process PID, not via the podman CLI.)
No real Podman required.

`containers.NewProcess` is also used directly in `process_test.go` to test `Process`
in isolation. Required test cases include:
- `TestProcess_LinesOpenAfterRunningFalse`: kill the process, assert `Running()` returns
  false, then assert `Lines()` still yields buffered data and eventually closes. This
  guards against callers mistakenly using `Running() == false` as a signal to stop
  reading `Lines()`, since stdout drain may lag OS process exit.

### `impacket/` tests — injection via `ProcessStarter` + `containers.NewProcess`

Inject a fake `ProcessStarter` via `NewWithManager`. The fake:
- Implements `ProcessStarter` in-process; never instantiates `ProcManager`
- Returns `*containers.Process` values built with `containers.NewProcess` (io.Pipe-backed)
- Simulates ntlmrelayx log lines → verifies `Events()` parsing
- Simulates secretsdump output → verifies `[]Credential` parsing
- Simulates early process exit → verifies `Err()` returns the exit error after Events() closes

The two test seams are orthogonal and do not interact.

### What is NOT tested

- Real Podman interaction (integration test, requires hardware)
- Real impacket output (E2E, requires a target network)

---

## Performance considerations

| Concern | Assessment |
|---------|------------|
| RAM | Container image is ~350 MB compressed on disk. RSS footprint of the running Python process is ~80–120 MB. Total with Go progs + Podman daemon: ~200–250 MB. Leaves ~260 MB headroom on 512 MB. Acceptable. |
| Container cold-start | ~10–20 s on ARM64 (image pull + container create). Lazy: paid on first tool invocation, not at New(). |
| Per-invocation start | `podman exec` on a running container: ~300–500 ms. Acceptable for interactive use. |
| `ntlmrelayx` latency | Daemon stays running; no per-event overhead. |
| SecretsDump duration | 5–30 s depending on target. Blocking call with context cancellation. |

---

## Future extensions

The `containers/` package is intentionally generic. Adding a new tool (e.g. `tools/nmap/`)
requires only:
1. A new package under `tools/` with its own `ProcessStarter`-consumer facade
2. A `New()` function that calls `containers.NewManager` with the tool's image and passes
   the resulting `*ProcManager` (which satisfies `ProcessStarter`) to the facade

No changes to `containers/` needed.

---

## Out of scope

- GUI integration (handled in `cmd/oioni` / `ui/gui`)
- Credential storage / persistence (handled in `system/storage`)
- Container image build (uses upstream impacket image)
- Authentication to private registries
- Network interface binding for ntlmrelayx (`ntlmrelayx.py` does not expose a stable
  flag for NIC selection; excluded until verified against the actual binary)
