# containers — Podman process manager

Package `containers` manages a single long-running Podman container and the processes inside it. Designed for use on [gokrazy](https://gokrazy.org/) (Raspberry Pi Zero 2W, arm64).

## Concepts

- **One container, many processes** — a single `podman run … sleep infinity` container is kept alive. Each tool invocation runs inside it via `podman exec`.
- **Local image loading** — on gokrazy there is no registry access; images are shipped as `.tar.gz` via `ExtraFilePaths` and loaded with `podman load`.
- **Signal delivery** — Podman does not forward signals to `exec` subprocesses ([Podman issue #19486](https://github.com/containers/podman/issues/19486)). The manager captures the container PID via `echo $$` and sends signals with `podman exec … kill -TERM/-KILL <pid>`.
- **gokrazy PATH** — the `podman` binary lives in `/usr/local/bin` which is not in gokrazy's parent process PATH. The manager resolves binaries via `os.Stat` across known dirs before calling `exec.Command`.

## API

### ProcManager

```go
// Create a manager. No I/O happens at construction time.
mgr := containers.NewManager(containers.Config{
    Image:          "oioni/impacket:arm64",
    Name:           "oioni-impacket",       // unique container name
    Network:        "host",                 // needed for USB gadget interface access
    Caps:           []string{"NET_RAW", "NET_ADMIN"},
    LocalImagePath: "/usr/share/oioni/impacket-arm64.tar.gz",
})

// Start a process (initialises the container on first call).
proc, err := mgr.Start(ctx, "run1", "secretsdump.py", []string{"admin@192.168.1.1", "-p", "pass"})

// Read output line by line.
for line := range proc.Lines() {
    fmt.Println(line)
}

// Wait for exit.
err = proc.Wait()  // returns nil or *containers.ExitError

// Graceful stop: SIGTERM → 10 s timeout → SIGKILL.
err = mgr.Stop(ctx, "run1")

// Immediate kill.
err = mgr.Kill("run1")

// List running processes.
names := mgr.List()

// Tear down: kills all procs, removes container.
err = mgr.Close()
```

### Process

```go
proc.Lines()    // <-chan string — output lines, closed on exit
proc.Wait()     // blocks until exit; idempotent; returns nil or *ExitError
proc.Kill()     // SIGKILL immediately
proc.Running()  // false once Wait() has returned
```

### Errors

```go
containers.ErrPodmanNotFound  // podman binary not found on this system
containers.ErrAlreadyRunning  // Start called with a name already in use
containers.ErrManagerClosed   // Start called after Close()

var exitErr *containers.ExitError
if errors.As(err, &exitErr) {
    fmt.Println(exitErr.ExitCode())
}
```

### Testability

Inject a custom `exec.Cmd` factory to unit-test without a real podman installation:

```go
mgr := containers.NewManager(cfg, containers.WithCmdFactory(func(name string, args ...string) *exec.Cmd {
    // return a fake command
}))
```

## gokrazy specifics

| Concern | Solution |
|---------|----------|
| `podman` not in PATH | `resolveBinary` stat-checks `/user`, `/usr/local/bin`, `/usr/bin`, `/bin` |
| `/tmp` tmpfs too small for `podman load` | Uses `TMPDIR=/perm/tmp` (SD card) only during image load |
| Container name collision on reboot | `podman stop` + `podman rm` before every `podman run` |
| Image persistence across reboots | `/var` → `/perm/var` on gokrazy; Podman image store persists |
