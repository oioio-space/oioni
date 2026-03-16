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
    process.go                — Process: streaming output, wait, kill
    manager_test.go
    process_test.go
  impacket/
    impacket.go               — Impacket facade (embeds *containers.ProcManager)
    runner.go                 — generic Run(ctx, name, tool, args) → *Process
    ntlmrelayx.go             — NTLMRelay() + NTLMRelayConfig + NTLMRelayEvent
    secretsdump.go            — SecretsDump() + SecretsDumpConfig + Credential
    impacket_test.go
    ntlmrelayx_test.go
    secretsdump_test.go
```

`go.work` gains `./tools`.

---

## Package `containers`

### Responsibility

Generic Podman process manager. Knows nothing about impacket. Can be reused for any future containerised tool (nmap, metasploit, etc.).

### Container strategy

One long-running container is started once (`podman start`). Each tool invocation runs as a `podman exec` inside that container. This eliminates per-invocation cold-start (Python interpreter start, 3–8 s on ARM64).

Container startup sequence on first `Start()` call:
1. `podman pull <image>` if not present locally
2. `podman run -d --network host --cap-add NET_RAW --cap-add NET_ADMIN --name <name> <image> sleep infinity`
3. Subsequent calls: `podman exec <name> <tool> <args...>`

`--network host` gives the container direct access to all host interfaces including USB gadget interfaces (`usb0`, `usb1`).
`NET_RAW` + `NET_ADMIN` are required for raw packet capture and interface configuration.

### API

```go
// Config describes a managed container.
type Config struct {
    Image   string   // e.g. "ghcr.io/fortra/impacket:latest"
    Name    string   // container name, unique per instance
    Network string   // "host" for gadget interface access
    Caps    []string // Linux capabilities, e.g. ["NET_RAW", "NET_ADMIN"]
}

// NewManager creates a ProcManager. The container is not started until
// the first call to Start().
func NewManager(cfg Config) *ProcManager

type ProcManager struct { /* ... */ }

// Start launches a named process inside the container via podman exec.
// Returns an error if a process with that name is already running.
func (m *ProcManager) Start(ctx context.Context, name, executable string, args []string) (*Process, error)

// Stop sends SIGTERM to the named process and waits for it to exit.
func (m *ProcManager) Stop(name string) error

// Kill sends SIGKILL to the named process.
func (m *ProcManager) Kill(name string) error

// List returns the names of all currently running processes.
func (m *ProcManager) List() []string

// Close stops all running processes and removes the container.
func (m *ProcManager) Close() error
```

### Process

```go
type Process struct { /* ... */ }

// Lines returns a channel of stdout+stderr lines. Closed when the process exits.
func (p *Process) Lines() <-chan string

// Wait blocks until the process exits and returns its exit error.
func (p *Process) Wait() error

// Kill sends SIGKILL immediately.
func (p *Process) Kill() error

// Running reports whether the process is still alive.
func (p *Process) Running() bool
```

### Concurrency

- `ProcManager` is safe for concurrent use.
- Process registry is guarded by a `sync.Mutex`.
- Container startup is guarded by a `sync.Once`.

### Error handling

- `podman` binary not found → `ErrPodmanNotFound`
- Container already exists with different config → re-used as-is (idempotent)
- Process name already running → `ErrAlreadyRunning`
- `podman exec` exits non-zero → `Wait()` returns `*ExitError`

---

## Package `impacket`

### Responsibility

Impacket-specific facade. Provides:
1. A generic runner for any impacket script.
2. Typed wrappers for `ntlmrelayx` and `secretsdump` with structured output.

### Image

`ghcr.io/fortra/impacket:latest` — official impacket image, ~350 MB, supports arm64.

### API

#### Facade

```go
// New returns an Impacket instance backed by a ProcManager using the
// official impacket image with host networking and raw packet capabilities.
func New() *Impacket

// NewWithManager allows injecting a custom ProcManager (useful for tests).
func NewWithManager(mgr *containers.ProcManager) *Impacket
```

#### Generic runner

```go
// Run launches any impacket script by name with raw args.
// name identifies the process in the registry (must be unique among running procs).
// tool is the impacket script name, e.g. "samrdump", "lookupsid".
func (i *Impacket) Run(ctx context.Context, name, tool string, args []string) (*containers.Process, error)
```

#### NTLMRelay wrapper

```go
type NTLMRelayConfig struct {
    // Target is the relay target, e.g. "smb://192.168.1.1".
    Target string

    // Interface is the network interface to listen on, e.g. "usb0".
    // Empty = listen on all interfaces.
    Interface string

    // SMB2Support enables SMB2 relay (recommended).
    SMB2Support bool

    // OutputFile writes captured hashes to a file inside the container.
    // Optional.
    OutputFile string
}

type NTLMRelayEvent struct {
    Username string
    Domain   string
    Hash     string // NTLMv2 hash
    Target   string
}

type NTLMRelayProcess struct {
    *containers.Process
}

// Events returns parsed NTLMRelayEvent values streamed from ntlmrelayx output.
// The channel is closed when the process exits.
func (p *NTLMRelayProcess) Events() <-chan NTLMRelayEvent

// NTLMRelay starts ntlmrelayx as a background daemon.
func (i *Impacket) NTLMRelay(ctx context.Context, name string, cfg NTLMRelayConfig) (*NTLMRelayProcess, error)
```

#### SecretsDump wrapper

```go
type SecretsDumpConfig struct {
    // Target host IP or hostname.
    Target string

    // Credentials — one of password or hash required.
    Username string
    Password string
    Hash     string // pass-the-hash, format "LMHASH:NTHASH"

    // Domain is optional; defaults to target hostname.
    Domain string
}

type Credential struct {
    Username string
    Domain   string
    Hash     string
    Type     string // "NTLM", "Kerberos", "Plaintext"
}

// SecretsDump runs secretsdump.py and returns parsed credentials.
// Blocks until completion.
func (i *Impacket) SecretsDump(ctx context.Context, cfg SecretsDumpConfig) ([]Credential, error)
```

---

## Testing strategy

### `containers/` tests

Inject a fake `podman` binary via the `ProcManager`'s internal command factory (functional option `WithCmdFactory`). The fake binary:
- Accepts `pull`, `run`, `exec`, `stop`, `rm` subcommands
- Writes predictable output to stdout
- Exits 0

No real Podman or container runtime required. Tests run on any platform.

### `impacket/` tests

Inject a fake `ProcManager` via `NewWithManager`. The fake manager:
- Returns a `*Process` backed by an `io.Pipe` with scripted output
- Allows simulating ntlmrelayx log lines → verify `Events()` parsing
- Allows simulating secretsdump output → verify `[]Credential` parsing

### What is NOT tested

- Real Podman interaction (integration test, requires hardware)
- Real impacket output (E2E test, requires a target network)

---

## Performance considerations

| Concern | Assessment |
|---------|------------|
| RAM | ~330 MB total (Go progs + Podman + Python + impacket). Leaves ~180 MB headroom on 512 MB. Acceptable. |
| Container cold-start | ~10–20 s on ARM64 (image pull + container create). Done once at `New()`. |
| Per-invocation start | `podman exec` on a running container: ~300–500 ms. Acceptable for interactive use. |
| `ntlmrelayx` latency | Daemon stays running; no per-event overhead. |
| SecretsDump duration | 5–30 s depending on target. Blocking call with context cancellation. |

---

## Future extensions

The `containers/` package is intentionally generic. Adding a new tool requires only:
1. A new package in `tools/` (e.g. `tools/nmap/`)
2. A new `*Impacket`-style facade that calls `containers.NewManager` with its own image

No changes to `containers/` needed.

---

## Out of scope

- GUI integration (handled in `cmd/oioni` / `ui/gui`)
- Credential storage / persistence (handled in `system/storage`)
- Container image build (uses upstream impacket image)
- Authentication to private registries
