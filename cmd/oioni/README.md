# cmd/oioni — gokrazy main program

The main program that runs on the Pi Zero 2W under [gokrazy](https://gokrazy.org/).

## What it does

- Configures a USB composite gadget (RNDIS + ECM network, HID keyboard, mass storage)
- Manages disk image files on `/perm` (persistent) and USB drives (hotplug)
- Drives the Waveshare 2.13" Touch e-Paper HAT (display + touch input)
- Runs [impacket](https://github.com/fortra/impacket) network security tools inside a Podman container

## Flags

### USB gadget

| Flag | Default | Description |
|------|---------|-------------|
| `-rndis` | false | Enable RNDIS network function (Windows, 3 EP) |
| `-ecm` | false | Enable ECM network function (Linux/macOS, 3 EP) |
| `-hid` | false | Enable HID keyboard function (1 EP) |
| `-mass-storage` | false | Enable USB mass storage using `--img` (2 EP) |

### Disk image

| Flag | Default | Description |
|------|---------|-------------|
| `-img` | `/perm/data.img` | Disk image path |
| `-img-fs` | `vfat` | Filesystem: `vfat` \| `exfat` \| `ext4` |
| `-img-size` | `64` | Image size in MiB |
| `-img-create` | false | Create and format the image (fails if it already exists) |
| `-img-write` | false | Open image, write test files, close |
| `-img-read` | false | Open image, print contents, close |

### Storage manager

| Flag | Default | Description |
|------|---------|-------------|
| `-storage` | false | Enable USB hotplug storage manager |

### E-paper display

| Flag | Default | Description |
|------|---------|-------------|
| `-epaper` | false | Enable e-ink display and capacitive touch controller |

### Impacket tools

When any `-impacket-*` tool flag is set, the program runs that tool and exits (no gadget/display started). The container image is loaded from `/usr/share/oioni/impacket-arm64.tar.gz` on first run (~75 s); subsequent runs use the cached image from `/perm/var` (~6 s total).

**Common options** (apply to most tools):

| Flag | Description |
|------|-------------|
| `-impacket-target` | Target host IP or hostname |
| `-impacket-domain` | Domain name (e.g. `corp.local`) |
| `-impacket-user` | Username |
| `-impacket-pass` | Password |
| `-impacket-hash` | Pass-the-hash: `LMHASH:NTHASH` |

**Tool flags**:

| Flag | Tool | Description |
|------|------|-------------|
| `-impacket-secretsdump` | `secretsdump.py` | Dump SAM/LSA/NTDS hashes from a Windows host |
| `-impacket-ntlmrelay` | `ntlmrelayx.py` | NTLM relay daemon (runs until SIGTERM) |
| `-impacket-kerberoast` | `GetUserSPNs.py` | Kerberoasting — request TGS for SPNs |
| `-impacket-asreproast` | `GetNPUsers.py` | AS-REP Roasting — target accounts without pre-auth |
| `-impacket-lookupsid` | `lookupsid.py` | SID enumeration |
| `-impacket-samrdump` | `samrdump.py` | SAMR user/group enumeration |
| `-impacket-exec` | `wmiexec.py` / `psexec.py` / `smbexec.py` | Remote command execution |

**Extra flags for specific tools**:

| Flag | Default | For |
|------|---------|-----|
| `-impacket-relay-target` | | ntlmrelay: relay target URL, e.g. `smb://192.168.1.1` |
| `-impacket-command` | | exec: command to run, e.g. `whoami` |
| `-impacket-exec-method` | `wmi` | exec: `wmi` \| `smb` \| `smbexec` |

## EP budget (DWC2 BCM2835)

The Pi Zero 2W has only **7 usable endpoints** beyond EP0:

| Config | EPs | Works? |
|--------|-----|--------|
| RNDIS + ECM + HID | 7 | ✓ |
| RNDIS + mass-storage | 5 | ✓ |
| RNDIS + ECM + mass-storage | 8 | ✗ |

## Deploy

```sh
# From repo root
GOWORK=off gok update --parent_dir . -i oioio
```

See [oioio/config.json](../../oioio/config.json) for gokrazy instance configuration (packages, flags, ExtraFilePaths for the impacket image).

## gokrazy logs

```sh
# Live stderr (includes impacket output)
curl -u 'gokrazy:PASSWORD' 'http://PI_IP/log?path=/user/oioni&stream=stderr'

# Service status
curl -u 'gokrazy:PASSWORD' 'http://PI_IP/status?path=/user/oioni'
```
