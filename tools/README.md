# tools — Podman container runtime + impacket wrappers

This module (`github.com/oioio-space/oioni/tools`) provides two packages:

| Package | Description |
|---------|-------------|
| [`containers`](containers/) | Manages a long-running Podman container and processes inside it |
| [`impacket`](impacket/) | Typed Go wrappers for [impacket](https://github.com/fortra/impacket) scripts |

## containers

A minimal Podman process manager designed for [gokrazy](https://gokrazy.org/):

- Loads a container image from a local `.tar.gz` file (no registry required on device)
- Keeps a single long-running container alive (`sleep infinity`)
- Runs tools via `podman exec` — each invocation is a fresh process inside the shared container
- Delivers output line-by-line over a channel
- Handles graceful shutdown: SIGTERM → wait 10 s → SIGKILL
- Resolves the `podman` binary via `os.Stat` across gokrazy search dirs (`/user`, `/usr/local/bin`, …) because gokrazy's parent process PATH may not include `/usr/local/bin`

See [containers/README.md](containers/README.md) for the full API.

## impacket

Typed Go wrappers for the [impacket](https://github.com/fortra/impacket) Python toolkit, running inside the `containers.ProcManager`:

| Go method | Script | Purpose |
|-----------|--------|---------|
| `SecretsDump` | `secretsdump.py` | Dump SAM/LSA/NTDS hashes |
| `NTLMRelay` | `ntlmrelayx.py` | NTLM relay daemon |
| `Kerberoast` | `GetUserSPNs.py` | Kerberoasting |
| `ASREPRoast` | `GetNPUsers.py` | AS-REP Roasting |
| `LookupSID` | `lookupsid.py` | SID enumeration |
| `SAMRDump` | `samrdump.py` | User/group enumeration |
| `Exec` | `wmiexec.py` / `psexec.py` / `smbexec.py` | Remote command execution |
| `Run` | any | Raw invocation of any impacket script |

See [impacket/README.md](impacket/README.md) for the full API and container build instructions.

## Module layout

```
tools/
├── go.mod                  # module github.com/oioio-space/oioni/tools
├── containers/
│   ├── errors.go           # sentinel errors + ExitError
│   ├── manager.go          # ProcManager: container lifecycle + process registry
│   ├── manager_test.go
│   ├── process.go          # Process: lines channel, Wait, Kill, Running
│   └── process_test.go
└── impacket/
    ├── Dockerfile           # python:3.13-alpine + impacket 0.12.0 via pipx venv
    ├── auth.go              # shared credential argument builder
    ├── exec.go              # Exec (wmiexec/psexec/smbexec)
    ├── impacket.go          # Impacket struct + New / NewWithManager
    ├── kerberoast.go        # Kerberoast + ASREPRoast
    ├── lookupsid.go         # LookupSID
    ├── ntlmrelayx.go        # NTLMRelay
    ├── runner.go            # Run (raw)
    ├── samr.go              # SAMRDump
    ├── secretsdump.go       # SecretsDump
    └── *_test.go
```

## gokrazy deployment

The impacket image is shipped via `ExtraFilePaths` in `config.json` — no registry access needed on the Pi:

```sh
# 1. Build arm64 image on dev machine
podman build --platform linux/arm64 -t oioni/impacket:arm64 tools/impacket/

# 2. Export as compressed tar
podman save oioni/impacket:arm64 | gzip > /tmp/impacket-arm64.tar.gz

# 3. Add to config.json PackageConfig for cmd/oioni:
#    "ExtraFilePaths": {"/usr/share/oioni/impacket-arm64.tar.gz": "/tmp/impacket-arm64.tar.gz"}

# 4. Deploy
GOWORK=off gok update --parent_dir . -i oioio
```

On first boot the manager loads the image from `/usr/share/oioni/impacket-arm64.tar.gz` into Podman (~75 s). Subsequent runs use the cached image in `/perm/var/lib/containers` (~6 s).
