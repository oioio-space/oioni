# OiOni Repository Restructure Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rename and restructure `awesomeProject` into a clean public multi-module Go workspace at `github.com/oioio-space/oioni`, organized as `drivers/` (hardware), `system/` (OS abstractions), `ui/` (interface), and `cmd/` (programs).

**Architecture:** 8 independent Go modules in a single `go.work` workspace. Each module is importable standalone via `go get github.com/oioio-space/oioni/<group>/<name>`. The `cmd/oioni` module is a gokrazy-compatible ARM64 program that wires all modules together.

**Tech Stack:** Go 1.26, multi-module go.work workspace, gokrazy, GitHub Actions, MIT license.

---

## Module map

| Old path | New path | Go module |
|---|---|---|
| `epaper/epd/` | `drivers/epd/` | `github.com/oioio-space/oioni/drivers/epd` |
| `epaper/touch/` | `drivers/touch/` | `github.com/oioio-space/oioni/drivers/touch` |
| `usbgadget/` | `drivers/usbgadget/` | `github.com/oioio-space/oioni/drivers/usbgadget` |
| `imgvol/` | `system/imgvol/` | `github.com/oioio-space/oioni/system/imgvol` |
| `storage/` | `system/storage/` | `github.com/oioio-space/oioni/system/storage` |
| `epaper/canvas/` | `ui/canvas/` | `github.com/oioio-space/oioni/ui/canvas` |
| `epaper/gui/` | `ui/gui/` | `github.com/oioio-space/oioni/ui/gui` |
| `hello/` | `cmd/oioni/` | `github.com/oioio-space/oioni/cmd/oioni` |

## Import path changes (global sed map)

```
awesomeProject/epaper/epd     → github.com/oioio-space/oioni/drivers/epd
awesomeProject/epaper/touch   → github.com/oioio-space/oioni/drivers/touch
awesomeProject/epaper/canvas  → github.com/oioio-space/oioni/ui/canvas
awesomeProject/epaper/gui     → github.com/oioio-space/oioni/ui/gui
awesomeProject/usbgadget      → github.com/oioio-space/oioni/drivers/usbgadget
awesomeProject/usbgadget/functions → github.com/oioio-space/oioni/drivers/usbgadget/functions
awesomeProject/usbgadget/modules   → github.com/oioio-space/oioni/drivers/usbgadget/modules
awesomeProject/imgvol         → github.com/oioio-space/oioni/system/imgvol
awesomeProject/storage        → github.com/oioio-space/oioni/system/storage
awesomeProject/storage/mount  → github.com/oioio-space/oioni/system/storage/mount
awesomeProject/storage/usbdetect → github.com/oioio-space/oioni/system/storage/usbdetect
```

---

## Chunk 1: Repository skeleton

### Task 1: Git remote + clean .gitignore + LICENSE

**Files:**
- Modify: `.gitignore`
- Create: `LICENSE`
- Create: `oioio/config.template.json`

- [ ] **Step 1: Connect git remote**

```bash
git remote add origin https://github.com/oioio-space/oioni.git
```

- [ ] **Step 2: Rewrite .gitignore**

Replace `.gitignore` with:

```gitignore
# Gokrazy build artifacts
oioio/builddir/
oioio/wifi.json
oioio/config.json
*.gaf

# Compiled binaries (keep mkfs.* and *.ko — they're pre-built for offline use)
hello/hello
cmd/oioni/oioni

# Go workspace local overrides
go.work.local

# IDE
.idea/
*.iml
```

- [ ] **Step 3: Create MIT LICENSE**

Create `LICENSE`:

```
MIT License

Copyright (c) 2026 oioio-space

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

- [ ] **Step 4: Create gokrazy config template**

Create `oioio/config.template.json`:

```json
{
  "Hostname": "oioni",
  "Update": {
    "Hostname": "192.168.1.x",
    "HTTPPassword": "your-password-here"
  },
  "Environment": [
    "GOOS=linux",
    "GOARCH=arm64"
  ],
  "Packages": [
    "github.com/gokrazy/fbstatus",
    "github.com/gokrazy/serial-busybox",
    "github.com/gokrazy/breakglass",
    "github.com/gokrazy/wifi",
    "github.com/gokrazy/mkfs",
    "github.com/oioio-space/oioni/cmd/oioni"
  ],
  "PackageConfig": {
    "github.com/gokrazy/gokrazy/cmd/randomd": {
      "ExtraFileContents": {
        "/etc/machine-id": "your-machine-id-here\n"
      }
    },
    "github.com/gokrazy/wifi": {
      "ExtraFilePaths": {
        "/etc/wifi.json": "wifi.json"
      }
    },
    "github.com/gokrazy/breakglass": {
      "CommandLineFlags": [
        "-authorized_keys=/etc/breakglass.authorized_keys"
      ],
      "ExtraFileContents": {
        "/etc/breakglass.authorized_keys": "ssh-ed25519 YOUR_PUBLIC_KEY_HERE\n"
      }
    },
    "github.com/oioio-space/oioni/cmd/oioni": {
      "CommandLineFlags": [
        "-epaper",
        "-rndis"
      ]
    }
  },
  "BootloaderExtraLines": [
    "dtoverlay=dwc2,dr_mode=peripheral",
    "dtparam=spi=on",
    "dtparam=i2c_arm=on"
  ],
  "SerialConsole": "disabled"
}
```

- [ ] **Step 5: Create docs/claude/memory/ directory and copy memories**

```bash
mkdir -p docs/claude/memory
cp /home/oioio/.claude/projects/-home-oioio-Documents-GolandProjects-awesomeProject/memory/*.md docs/claude/memory/
```

Create `docs/claude/memory/README.md`:

```markdown
# Claude AI Session Memories

This directory contains persistent knowledge accumulated across Claude Code sessions
for this project. These memories help Claude understand project-specific conventions,
decisions, and patterns without needing to rediscover them each session.

Files are maintained automatically by Claude Code using the `bd remember` and
memory management tools.
```

- [ ] **Step 6: Commit skeleton**

```bash
git add .gitignore LICENSE oioio/config.template.json docs/claude/
git commit -m "chore: add LICENSE (MIT), .gitignore, gokrazy config template, Claude memories"
```

---

### Task 2: Create directory structure

**Files:**
- Create: all new directories

- [ ] **Step 1: Create all module directories**

```bash
mkdir -p drivers/epd drivers/touch drivers/usbgadget/functions drivers/usbgadget/modules
mkdir -p system/imgvol/bin system/imgvol/build system/storage/mount system/storage/usbdetect
mkdir -p ui/canvas ui/gui
mkdir -p cmd/oioni
```

- [ ] **Step 2: Commit empty structure**

```bash
git add drivers/ system/ ui/ cmd/
git commit -m "chore: create multi-module directory structure (drivers/, system/, ui/, cmd/)"
```

---

## Chunk 2: Code migration — modules

> For each module: copy source files → update imports → create go.mod → run tests → commit.
> Use `sed -i` to rewrite imports in batch. Do NOT delete old files yet (delete at end of chunk).

### Task 3: drivers/epd

**Files:**
- Copy: `epaper/epd/*.go` → `drivers/epd/`
- Create: `drivers/epd/go.mod`

- [ ] **Step 1: Copy source files**

```bash
cp epaper/epd/epd.go epaper/epd/hal.go epaper/epd/epd_test.go drivers/epd/
```

- [ ] **Step 2: No import changes needed** — epd has no internal imports.

- [ ] **Step 3: Create drivers/epd/go.mod**

```
module github.com/oioio-space/oioni/drivers/epd

go 1.26

require golang.org/x/sys v0.42.0
```

- [ ] **Step 4: Create drivers/epd/go.sum**

```bash
cd drivers/epd && go mod tidy && cd ../..
```

- [ ] **Step 5: Run tests**

```bash
go test github.com/oioio-space/oioni/drivers/epd -v -count=1
```

Expected: epd tests pass (openSPI/openGPIO tests expect failures on non-Pi, that is OK).

- [ ] **Step 6: Commit**

```bash
git add drivers/epd/
git commit -m "feat(drivers/epd): migrate EPD_2in13_V4 driver to drivers/epd module"
```

---

### Task 4: drivers/touch

**Files:**
- Copy: `epaper/touch/*.go` → `drivers/touch/`
- Create: `drivers/touch/go.mod`

- [ ] **Step 1: Copy source files**

```bash
cp epaper/touch/touch.go epaper/touch/hal.go epaper/touch/touch_test.go drivers/touch/
```

- [ ] **Step 2: No internal imports to update.**

- [ ] **Step 3: Create drivers/touch/go.mod**

```
module github.com/oioio-space/oioni/drivers/touch

go 1.26

require golang.org/x/sys v0.42.0
```

- [ ] **Step 4: go mod tidy**

```bash
cd drivers/touch && go mod tidy && cd ../..
```

- [ ] **Step 5: Run tests**

```bash
go test github.com/oioio-space/oioni/drivers/touch -v -count=1
```

- [ ] **Step 6: Commit**

```bash
git add drivers/touch/
git commit -m "feat(drivers/touch): migrate GT1151 touch driver to drivers/touch module"
```

---

### Task 5: drivers/usbgadget

**Files:**
- Copy: `usbgadget/*.go` → `drivers/usbgadget/`
- Copy: `usbgadget/functions/*.go` → `drivers/usbgadget/functions/`
- Copy: `usbgadget/modules/*.go` → `drivers/usbgadget/modules/`
- Copy: `usbgadget/modules/6.12.47-v8/` → `drivers/usbgadget/modules/6.12.47-v8/`
- Copy: `usbgadget/modules/build/` → `drivers/usbgadget/modules/build/`
- Create: `drivers/usbgadget/go.mod`

- [ ] **Step 1: Copy source files**

```bash
cp usbgadget/*.go drivers/usbgadget/
cp usbgadget/functions/*.go drivers/usbgadget/functions/
cp usbgadget/modules/*.go drivers/usbgadget/modules/
cp -r usbgadget/modules/6.12.47-v8/ drivers/usbgadget/modules/
cp -r usbgadget/modules/build/ drivers/usbgadget/modules/build/
```

- [ ] **Step 2: Update internal imports in drivers/usbgadget/**

```bash
find drivers/usbgadget -name "*.go" -exec sed -i \
  's|awesomeProject/usbgadget/functions|github.com/oioio-space/oioni/drivers/usbgadget/functions|g;
   s|awesomeProject/usbgadget/modules|github.com/oioio-space/oioni/drivers/usbgadget/modules|g;
   s|awesomeProject/usbgadget|github.com/oioio-space/oioni/drivers/usbgadget|g' {} \;
```

- [ ] **Step 3: Create drivers/usbgadget/go.mod**

```
module github.com/oioio-space/oioni/drivers/usbgadget

go 1.26

require golang.org/x/sys v0.42.0
```

- [ ] **Step 4: go mod tidy**

```bash
cd drivers/usbgadget && go mod tidy && cd ../..
```

- [ ] **Step 5: Run tests (build only — hardware required for full tests)**

```bash
go build github.com/oioio-space/oioni/drivers/usbgadget/...
go test github.com/oioio-space/oioni/drivers/usbgadget -v -run TestExample -count=1 2>/dev/null || true
```

- [ ] **Step 6: Commit**

```bash
git add drivers/usbgadget/
git commit -m "feat(drivers/usbgadget): migrate USB gadget framework to drivers/usbgadget module"
```

---

### Task 6: system/imgvol

**Files:**
- Copy: `imgvol/*.go` → `system/imgvol/`
- Copy: `imgvol/bin/` → `system/imgvol/bin/`
- Copy: `imgvol/build/` → `system/imgvol/build/`
- Create: `system/imgvol/go.mod`

- [ ] **Step 1: Copy source files and binaries**

```bash
cp imgvol/imgvol.go imgvol/format.go imgvol/format_test.go imgvol/loop.go imgvol/loop_test.go system/imgvol/
cp imgvol/bin/mkfs.* system/imgvol/bin/ 2>/dev/null || true
cp -r imgvol/build/ system/imgvol/build/
```

- [ ] **Step 2: No internal imports — imgvol has no internal cross-references.**

- [ ] **Step 3: Update go:embed paths if needed**

The `imgvol.go` embeds `bin/mkfs.*` — path is relative, stays valid after move. Verify:

```bash
grep -n 'go:embed' system/imgvol/imgvol.go
```

Expected: `//go:embed bin/mkfs.*` — no change needed.

- [ ] **Step 4: Create system/imgvol/go.mod**

```
module github.com/oioio-space/oioni/system/imgvol

go 1.26

require (
	github.com/spf13/afero v1.15.0
	golang.org/x/sys v0.42.0
)
```

- [ ] **Step 5: go mod tidy**

```bash
cd system/imgvol && go mod tidy && cd ../..
```

- [ ] **Step 6: Run tests**

```bash
go test github.com/oioio-space/oioni/system/imgvol -v -count=1
```

- [ ] **Step 7: Update .gitignore — remove old imgvol/bin exclusion, binaries now committed**

Remove this line from `.gitignore`:
```
imgvol/bin/mkfs.*
```

- [ ] **Step 8: Commit**

```bash
git add system/imgvol/ .gitignore
git commit -m "feat(system/imgvol): migrate disk image volume manager to system/imgvol module"
```

---

### Task 7: system/storage

**Files:**
- Copy: `storage/*.go` → `system/storage/`
- Copy: `storage/mount/*.go` → `system/storage/mount/`
- Copy: `storage/usbdetect/*.go` → `system/storage/usbdetect/`
- Create: `system/storage/go.mod`

- [ ] **Step 1: Copy source files**

```bash
cp storage/doc.go storage/manager.go storage/manager_test.go storage/perm.go storage/volume.go system/storage/
cp storage/mount/mount.go storage/mount/detect.go storage/mount/detect_test.go system/storage/mount/
cp storage/usbdetect/doc.go storage/usbdetect/detector.go storage/usbdetect/netlink.go \
   storage/usbdetect/netlink_test.go storage/usbdetect/sysfs.go storage/usbdetect/sysfs_test.go \
   system/storage/usbdetect/
```

- [ ] **Step 2: Update internal imports**

```bash
find system/storage -name "*.go" -exec sed -i \
  's|awesomeProject/storage/mount|github.com/oioio-space/oioni/system/storage/mount|g;
   s|awesomeProject/storage/usbdetect|github.com/oioio-space/oioni/system/storage/usbdetect|g;
   s|awesomeProject/storage|github.com/oioio-space/oioni/system/storage|g' {} \;
```

- [ ] **Step 3: Create system/storage/go.mod**

```
module github.com/oioio-space/oioni/system/storage

go 1.26

require (
	github.com/spf13/afero v1.15.0
	golang.org/x/sys v0.42.0
)
```

- [ ] **Step 4: go mod tidy**

```bash
cd system/storage && go mod tidy && cd ../..
```

- [ ] **Step 5: Run tests**

```bash
go test github.com/oioio-space/oioni/system/storage/... -v -count=1
```

- [ ] **Step 6: Commit**

```bash
git add system/storage/
git commit -m "feat(system/storage): migrate USB storage manager to system/storage module"
```

---

### Task 8: ui/canvas

**Files:**
- Copy: `epaper/canvas/*.go` → `ui/canvas/`
- Create: `ui/canvas/go.mod`

- [ ] **Step 1: Copy source files**

```bash
cp epaper/canvas/canvas.go epaper/canvas/canvas_test.go epaper/canvas/draw.go epaper/canvas/font.go ui/canvas/
```

- [ ] **Step 2: No internal imports.**

- [ ] **Step 3: Create ui/canvas/go.mod**

```
module github.com/oioio-space/oioni/ui/canvas

go 1.26

require (
	golang.org/x/image v0.37.0
	golang.org/x/text v0.35.0
)
```

- [ ] **Step 4: go mod tidy**

```bash
cd ui/canvas && go mod tidy && cd ../..
```

- [ ] **Step 5: Run tests**

```bash
go test github.com/oioio-space/oioni/ui/canvas -v -count=1
```

- [ ] **Step 6: Commit**

```bash
git add ui/canvas/
git commit -m "feat(ui/canvas): migrate 1-bit e-ink canvas to ui/canvas module"
```

---

### Task 9: ui/gui

**Files:**
- Copy: `epaper/gui/*.go` → `ui/gui/`
- Create: `ui/gui/go.mod`

- [ ] **Step 1: Copy source files**

```bash
cp epaper/gui/gui.go epaper/gui/layout.go epaper/gui/navigator.go \
   epaper/gui/refresh.go epaper/gui/widgets.go epaper/gui/gui_test.go ui/gui/
```

- [ ] **Step 2: Update imports**

```bash
find ui/gui -name "*.go" -exec sed -i \
  's|awesomeProject/epaper/canvas|github.com/oioio-space/oioni/ui/canvas|g;
   s|awesomeProject/epaper/epd|github.com/oioio-space/oioni/drivers/epd|g;
   s|awesomeProject/epaper/touch|github.com/oioio-space/oioni/drivers/touch|g' {} \;
```

- [ ] **Step 3: Create ui/gui/go.mod**

```
module github.com/oioio-space/oioni/ui/gui

go 1.26

require (
	github.com/oioio-space/oioni/drivers/epd v0.0.0-00010101000000-000000000000
	github.com/oioio-space/oioni/drivers/touch v0.0.0-00010101000000-000000000000
	github.com/oioio-space/oioni/ui/canvas v0.0.0-00010101000000-000000000000
)
```

> Note: The pseudo-version `v0.0.0-00010101000000-000000000000` is the standard placeholder for workspace-replaced modules. `go mod tidy` within the workspace context will keep this.

- [ ] **Step 4: Update go.work to include new modules before running tidy**

Rewrite `go.work` completely:

```go
go 1.26.0

use (
	./drivers/epd
	./drivers/touch
	./drivers/usbgadget
	./system/imgvol
	./system/storage
	./ui/canvas
	./ui/gui
	./cmd/oioni
)
```

- [ ] **Step 5: go mod tidy (from root, workspace-aware)**

```bash
cd ui/gui && go mod tidy && cd ../..
```

- [ ] **Step 6: Run tests**

```bash
go test github.com/oioio-space/oioni/ui/gui -v -count=1
```

Expected: 38 tests pass.

- [ ] **Step 7: Commit**

```bash
git add ui/gui/ go.work go.work.sum
git commit -m "feat(ui/gui): migrate e-ink GUI toolkit to ui/gui module"
```

---

### Task 10: cmd/oioni

**Files:**
- Copy: `hello/main.go` → `cmd/oioni/main.go`
- Copy: `hello/epaper.go` → `cmd/oioni/epaper.go`
- Create: `cmd/oioni/go.mod`

- [ ] **Step 1: Copy source files**

```bash
cp hello/main.go hello/epaper.go cmd/oioni/
```

- [ ] **Step 2: Update all imports**

```bash
find cmd/oioni -name "*.go" -exec sed -i \
  's|awesomeProject/epaper/epd|github.com/oioio-space/oioni/drivers/epd|g;
   s|awesomeProject/epaper/gui|github.com/oioio-space/oioni/ui/gui|g;
   s|awesomeProject/epaper/touch|github.com/oioio-space/oioni/drivers/touch|g;
   s|awesomeProject/usbgadget/functions|github.com/oioio-space/oioni/drivers/usbgadget/functions|g;
   s|awesomeProject/usbgadget|github.com/oioio-space/oioni/drivers/usbgadget|g;
   s|awesomeProject/imgvol|github.com/oioio-space/oioni/system/imgvol|g;
   s|awesomeProject/storage|github.com/oioio-space/oioni/system/storage|g' {} \;
```

- [ ] **Step 3: Create cmd/oioni/go.mod**

```
module github.com/oioio-space/oioni/cmd/oioni

go 1.26

require (
	github.com/oioio-space/oioni/drivers/epd v0.0.0-00010101000000-000000000000
	github.com/oioio-space/oioni/drivers/touch v0.0.0-00010101000000-000000000000
	github.com/oioio-space/oioni/drivers/usbgadget v0.0.0-00010101000000-000000000000
	github.com/oioio-space/oioni/system/imgvol v0.0.0-00010101000000-000000000000
	github.com/oioio-space/oioni/system/storage v0.0.0-00010101000000-000000000000
	github.com/oioio-space/oioni/ui/gui v0.0.0-00010101000000-000000000000
	github.com/spf13/afero v1.15.0
)
```

- [ ] **Step 4: go mod tidy**

```bash
cd cmd/oioni && go mod tidy && cd ../..
```

- [ ] **Step 5: Verify compilation (cross-compile ARM64)**

```bash
GOOS=linux GOARCH=arm64 go build github.com/oioio-space/oioni/cmd/oioni
```

Expected: exits 0. Binary built at `oioni` (delete it after).

- [ ] **Step 6: Commit**

```bash
git add cmd/oioni/ go.work.sum
git commit -m "feat(cmd/oioni): migrate main program to cmd/oioni module"
```

---

### Task 11: Remove old directories + run full test suite

- [ ] **Step 1: Run full test suite across all modules**

```bash
go test ./drivers/... ./system/... ./ui/... 2>&1
```

Expected: all tests pass (hardware tests fail gracefully on missing device).

- [ ] **Step 2: Remove old source directories**

```bash
git rm -r epaper/ hello/ usbgadget/ imgvol/ storage/
```

- [ ] **Step 3: Remove old root go.mod (replaced by workspace)**

```bash
git rm go.mod go.sum
```

- [ ] **Step 4: Run tests again to confirm nothing broken**

```bash
go test ./drivers/... ./system/... ./ui/...
```

- [ ] **Step 5: Commit removal**

```bash
git add -A
git commit -m "chore: remove old source directories (migrated to multi-module workspace)"
```

---

## Chunk 3: Documentation

> For each module: doc.go (package-level godoc), README.md (usage + hardware links + example), example_test.go (runnable Example functions). Then global README.md.

### Task 12: drivers/epd documentation

**Files:**
- Create: `drivers/epd/doc.go`
- Create: `drivers/epd/README.md`
- Create: `drivers/epd/example_test.go`

- [ ] **Step 1: Create doc.go**

```go
// Package epd drives the Waveshare EPD 2.13" V4 e-ink display over SPI.
//
// The display is 122×250 pixels, 1 bit per pixel (black/white).
// It supports three refresh modes: full (~2 s, best quality), fast (~0.5 s),
// and partial (~0.3 s, no full-screen flash — requires a prior DisplayBase call).
//
// Hardware: Waveshare 2.13inch Touch e-Paper HAT
// https://www.waveshare.com/wiki/2.13inch_Touch_e-Paper_HAT
//
// Typical usage:
//
//	d, err := epd.New(epd.Config{
//	    SPIDevice: "/dev/spidev0.0", SPISpeed: 4_000_000,
//	    PinRST: 17, PinDC: 25, PinCS: 8, PinBUSY: 24,
//	})
//	d.Init(epd.ModeFull)
//	d.DisplayBase(buf)          // sets reference frame
//	d.DisplayPartial(buf)       // fast update, no ghost flash
//	d.Sleep()
//	d.Close()
package epd
```

- [ ] **Step 2: Create README.md**

```markdown
# epd — Waveshare EPD 2.13" V4 driver

[![Go Reference](https://pkg.go.dev/badge/github.com/oioio-space/oioni/drivers/epd.svg)](https://pkg.go.dev/github.com/oioio-space/oioni/drivers/epd)

Driver for the **Waveshare 2.13inch Touch e-Paper HAT (V4)** — a 122×250 px
black/white e-ink display connected over SPI.

**Hardware:** https://www.waveshare.com/wiki/2.13inch_Touch_e-Paper_HAT

## Install

```sh
go get github.com/oioio-space/oioni/drivers/epd
```

## Wiring (Raspberry Pi Zero 2W)

| EPD pin | BCM GPIO | Function |
|---------|----------|----------|
| RST     | 17       | Reset    |
| DC      | 25       | Data/Command |
| CS      | 8        | Chip Select (SPI CE0) |
| BUSY    | 24       | Busy signal |
| SPI     | /dev/spidev0.0 | 4 MHz |

Enable SPI in `config.txt`: `dtparam=spi=on`

## Quick start

```go
d, err := epd.New(epd.Config{
    SPIDevice: "/dev/spidev0.0",
    SPISpeed:  4_000_000,
    PinRST:    17, PinDC: 25, PinCS: 8, PinBUSY: 24,
})
if err != nil {
    log.Fatal(err)
}
defer d.Close()

buf := make([]byte, epd.BufferSize) // 4000 bytes, 1 bpp
// Fill buf with your image (0=black, 1=white per bit)

d.Init(epd.ModeFull)
d.DisplayBase(buf)          // full refresh + set reference frame
d.DisplayPartial(buf)       // fast partial update (~0.3 s)
d.Sleep()
```

## Refresh modes

| Mode | Duration | Notes |
|------|----------|-------|
| `ModeFull` | ~2 s | Best quality, full redraw |
| `ModeFast` | ~0.5 s | Fast full refresh |
| `ModePartial` | ~0.3 s | No flash, requires prior `DisplayBase` |

## Buffer layout

`BufferSize = ((Width+7)/8) * Height = 4000 bytes`
MSB first: bit 7 of byte 0 = pixel (0,0). `0` = black, `1` = white.
```

- [ ] **Step 3: Create example_test.go**

```go
package epd_test

import (
	"fmt"
	"github.com/oioio-space/oioni/drivers/epd"
)

func ExampleNew_missingDevice() {
	_, err := epd.New(epd.Config{
		SPIDevice: "/dev/spidev_nonexistent",
		SPISpeed:  4_000_000,
		PinRST:    17, PinDC: 25, PinCS: 8, PinBUSY: 24,
	})
	fmt.Println(err != nil)
	// Output: true
}

func ExampleBufferSize() {
	fmt.Println(epd.BufferSize) // 122 px wide → 16 bytes/row × 250 rows
	// Output: 4000
}
```

- [ ] **Step 4: Run tests**

```bash
go test github.com/oioio-space/oioni/drivers/epd -v -count=1
```

- [ ] **Step 5: Commit**

```bash
git add drivers/epd/
git commit -m "docs(drivers/epd): add doc.go, README, example_test"
```

---

### Task 13: drivers/touch documentation

**Files:**
- Create: `drivers/touch/doc.go`
- Create: `drivers/touch/README.md`
- Create: `drivers/touch/example_test.go`

- [ ] **Step 1: Create doc.go**

```go
// Package touch drives the Waveshare GT1151 capacitive touch controller over I2C.
//
// The GT1151 supports up to 5 simultaneous touch points. It is used on the
// Waveshare 2.13inch Touch e-Paper HAT alongside the EPD_2in13_V4 display.
//
// Hardware: Waveshare 2.13inch Touch e-Paper HAT
// https://www.waveshare.com/wiki/2.13inch_Touch_e-Paper_HAT
//
// Usage:
//
//	td, err := touch.New(touch.Config{
//	    I2CDevice: "/dev/i2c-1", I2CAddr: 0x14,
//	    PinTRST: 22, PinINT: 27,
//	})
//	events, err := td.Start(ctx)
//	for ev := range events {
//	    for _, pt := range ev.Points {
//	        log.Printf("touch at (%d,%d)", pt.X, pt.Y)
//	    }
//	}
package touch
```

- [ ] **Step 2: Create README.md**

```markdown
# touch — Waveshare GT1151 capacitive touch driver

[![Go Reference](https://pkg.go.dev/badge/github.com/oioio-space/oioni/drivers/touch.svg)](https://pkg.go.dev/github.com/oioio-space/oioni/drivers/touch)

Driver for the **GT1151 capacitive touch controller** on the Waveshare 2.13inch
Touch e-Paper HAT. Supports 5 simultaneous touch points over I2C.

**Hardware:** https://www.waveshare.com/wiki/2.13inch_Touch_e-Paper_HAT

## Install

```sh
go get github.com/oioio-space/oioni/drivers/touch
```

## Wiring (Raspberry Pi Zero 2W)

| Touch pin | BCM GPIO | Function |
|-----------|----------|----------|
| TRST      | 22       | Touch reset (output) |
| INT       | 27       | Interrupt (falling edge) |
| I2C       | /dev/i2c-1 | Address 0x14 |

Enable I2C in `config.txt`: `dtparam=i2c_arm=on`

## Quick start

```go
td, err := touch.New(touch.Config{
    I2CDevice: "/dev/i2c-1",
    I2CAddr:   0x14,
    PinTRST:   22,
    PinINT:    27,
})
if err != nil {
    log.Fatal(err)
}

ctx, cancel := context.WithCancel(context.Background())
events, err := td.Start(ctx)
if err != nil {
    log.Fatal(err)
}
defer td.Close()

for ev := range events {
    for _, pt := range ev.Points {
        log.Printf("touch id=%d x=%d y=%d", pt.ID, pt.X, pt.Y)
    }
}
```

## Coordinate system

Physical display pixels: X ∈ [0, 121], Y ∈ [0, 249].
Origin is top-left of the display (not the PCB).
```

- [ ] **Step 3: Create example_test.go**

```go
package touch_test

import (
	"fmt"
	"github.com/oioio-space/oioni/drivers/touch"
)

func ExampleNew_missingDevice() {
	_, err := touch.New(touch.Config{
		I2CDevice: "/dev/i2c-nonexistent",
		I2CAddr:   0x14,
		PinTRST:   22,
		PinINT:    27,
	})
	fmt.Println(err != nil)
	// Output: true
}
```

- [ ] **Step 4: Run tests**

```bash
go test github.com/oioio-space/oioni/drivers/touch -v -count=1
```

- [ ] **Step 5: Commit**

```bash
git add drivers/touch/
git commit -m "docs(drivers/touch): add doc.go, README, example_test"
```

---

### Task 14: drivers/usbgadget documentation

**Files:**
- Modify: `drivers/usbgadget/doc.go` (already exists — update module path in examples)
- Create: `drivers/usbgadget/README.md`

- [ ] **Step 1: Update doc.go import path in examples**

Check existing doc.go and update any import references:

```bash
sed -i 's|awesomeProject/usbgadget|github.com/oioio-space/oioni/drivers/usbgadget|g' \
    drivers/usbgadget/doc.go
```

- [ ] **Step 2: Create README.md**

```markdown
# usbgadget — Linux USB Gadget framework

[![Go Reference](https://pkg.go.dev/badge/github.com/oioio-space/oioni/drivers/usbgadget.svg)](https://pkg.go.dev/github.com/oioio-space/oioni/drivers/usbgadget)

Compose Linux USB gadgets from 20 pluggable functions (RNDIS, ECM, HID, Mass Storage,
Audio, MIDI, Serial…) using Linux ConfigFS. Designed for gokrazy on Raspberry Pi Zero 2W.

> **DWC2 EP budget** (Pi Zero 2W): max 7 endpoints beyond EP0.
> RNDIS=3, ECM=3, HID=1, MassStorage=2. Plan accordingly.

## Install

```sh
go get github.com/oioio-space/oioni/drivers/usbgadget
```

Requires Linux with `libcomposite` kernel module and ConfigFS mounted at
`/sys/kernel/config`. Pre-built ARM64 modules for kernel 6.12.47-v8 are embedded.

## Quick start

```go
rndis := functions.RNDIS(
    functions.WithRNDISHostAddr("02:00:00:aa:bb:01"),
    functions.WithRNDISDevAddr("02:00:00:aa:bb:02"),
)
g, err := usbgadget.New(
    usbgadget.WithName("my-gadget"),
    usbgadget.WithVendorID(0x1d6b, 0x0104),
    usbgadget.WithStrings("0x409", "MyOrg", "MyDevice", "001"),
    usbgadget.WithFunc(rndis),
    usbgadget.WithFunc(functions.Keyboard()),
)
g.Enable()
defer g.Disable()
```

## Available functions

| Function | EPs | Description |
|----------|-----|-------------|
| RNDIS | 3 | Windows-compatible USB network |
| ECM | 3 | Linux/macOS USB network |
| HID | 1 | Keyboard / mouse |
| MassStorage | 2 | USB drive (from disk image) |
| UAC1/UAC2 | varies | USB audio |
| MIDI | 2 | USB MIDI |
| Serial/ACM | 3 | USB serial port |
| NCM | 3 | USB network (modern) |

## Kernel modules

Pre-built `.ko` files for kernel `6.12.47-v8` (Raspberry Pi ARM64) are embedded
in `modules/6.12.47-v8/`. To rebuild for another kernel version:

```sh
cd drivers/usbgadget/modules/build
podman build --platform linux/arm64 --output type=local,dest=../6.x.y-v8/ .
```
```

- [ ] **Step 3: Commit**

```bash
git add drivers/usbgadget/
git commit -m "docs(drivers/usbgadget): add README, update import paths in doc.go"
```

---

### Task 15: system/imgvol + system/storage documentation

**Files:**
- Create: `system/imgvol/doc.go`, `system/imgvol/README.md`, `system/imgvol/example_test.go`
- Modify: `system/storage/doc.go` (update imports), Create: `system/storage/README.md`

- [ ] **Step 1: Create system/imgvol/doc.go**

```go
// Package imgvol creates, formats, and mounts disk image files on Linux.
//
// Supported filesystems: FAT (vfat), exFAT, ext4.
// Static mkfs binaries for ARM64 are embedded — no host tools required.
// Images are loop-mounted and exposed as an afero.Fs for filesystem-agnostic access.
//
// Constraint: only one Open per image file at a time (exclusive loop device).
//
// Usage:
//
//	// Create a 64 MiB FAT image:
//	imgvol.Create("/perm/data.img", 64<<20, imgvol.FAT)
//
//	// Mount and use:
//	vol, _ := imgvol.Open("/perm/data.img")
//	afero.WriteFile(vol.FS, "hello.txt", []byte("hi"), 0644)
//	vol.Close()
package imgvol
```

- [ ] **Step 2: Create system/imgvol/README.md**

```markdown
# imgvol — Disk image volume manager

[![Go Reference](https://pkg.go.dev/badge/github.com/oioio-space/oioni/system/imgvol.svg)](https://pkg.go.dev/github.com/oioio-space/oioni/system/imgvol)

Create, format, and mount disk image files on Linux. Supports FAT, exFAT, and ext4.
Static ARM64 `mkfs` binaries are embedded — no host tools required at runtime.

## Install

```sh
go get github.com/oioio-space/oioni/system/imgvol
```

Requires Linux (loop devices via `ioctl`). Runs on Raspberry Pi / gokrazy out of the box.

## Quick start

```go
import (
    "github.com/oioio-space/oioni/system/imgvol"
    "github.com/spf13/afero"
)

// Create a 64 MiB FAT image (fails if file exists)
err := imgvol.Create("/perm/data.img", 64<<20, imgvol.FAT)

// Mount and read/write via afero.Fs
vol, err := imgvol.Open("/perm/data.img")
defer vol.Close()

afero.WriteFile(vol.FS, "boot.txt", []byte("hello"), 0644)
entries, _ := afero.ReadDir(vol.FS, ".")
```

## Supported filesystems

| Constant | mkfs tool | Notes |
|----------|-----------|-------|
| `imgvol.FAT` | mkfs.fat | Compatible with all OS |
| `imgvol.ExFAT` | mkfs.exfat | Large file support |
| `imgvol.Ext4` | mkfs.ext4 | Linux-native |

## Using as USB mass storage

```go
// In usbgadget: expose the image as a USB drive
functions.MassStorage("/perm/data.img", functions.WithRemovable(true))
```
```

- [ ] **Step 3: Create system/imgvol/example_test.go**

```go
package imgvol_test

import (
	"fmt"
	"github.com/oioio-space/oioni/system/imgvol"
)

func ExampleFSType() {
	fmt.Println(imgvol.FAT)
	fmt.Println(imgvol.ExFAT)
	fmt.Println(imgvol.Ext4)
	// Output:
	// vfat
	// exfat
	// ext4
}
```

- [ ] **Step 4: Create system/storage/README.md**

```markdown
# storage — USB hotplug storage manager

[![Go Reference](https://pkg.go.dev/badge/github.com/oioio-space/oioni/system/storage.svg)](https://pkg.go.dev/github.com/oioio-space/oioni/system/storage)

Detects USB storage devices via Linux netlink and mounts them automatically.
Also manages the gokrazy `/perm` persistent partition.

## Install

```sh
go get github.com/oioio-space/oioni/system/storage
```

## Quick start

```go
sm := storage.New(
    storage.WithOnMount(func(v *storage.Volume) {
        log.Printf("mounted %s (%s) @ %s", v.Name, v.FSType, v.MountPath)
    }),
    storage.WithOnUnmount(func(v *storage.Volume) {
        log.Printf("removed %s", v.Name)
    }),
)
go sm.Start(ctx)
```

## Sub-packages

- `storage/mount` — low-level `mount(2)` / `umount(2)` syscall wrappers + FS type detection
- `storage/usbdetect` — USB hotplug via `AF_NETLINK` KOBJECT_UEVENT + sysfs initial scan
```

- [ ] **Step 5: Run all tests**

```bash
go test github.com/oioio-space/oioni/system/... -v -count=1
```

- [ ] **Step 6: Commit**

```bash
git add system/imgvol/ system/storage/
git commit -m "docs(system): add doc.go, README, examples for imgvol and storage"
```

---

### Task 16: ui/canvas + ui/gui documentation

**Files:**
- Create: `ui/canvas/doc.go`, `ui/canvas/README.md`, `ui/canvas/example_test.go`
- Create: `ui/gui/doc.go`, `ui/gui/README.md`, `ui/gui/example_test.go`

- [ ] **Step 1: Create ui/canvas/doc.go**

```go
// Package canvas provides a 1-bit (black/white) drawing surface for e-ink displays.
//
// Canvas implements draw.Image and supports text rendering via embedded bitmap fonts,
// rectangles, and lines. It handles rotation (0°, 90°, 180°, 270°) and produces
// a packed byte buffer compatible with epd.Display methods.
//
// Usage:
//
//	c := canvas.New(epd.Width, epd.Height, canvas.Rot90)
//	c.DrawRect(image.Rect(0,0,100,20), canvas.Black, true)
//	c.DrawText(5, 3, "Hello", canvas.EmbeddedFont(12), canvas.White)
//	buf := c.Bytes() // 4000-byte buffer ready for epd.DisplayBase
package canvas
```

- [ ] **Step 2: Create ui/canvas/README.md**

```markdown
# canvas — 1-bit drawing canvas for e-ink displays

[![Go Reference](https://pkg.go.dev/badge/github.com/oioio-space/oioni/ui/canvas.svg)](https://pkg.go.dev/github.com/oioio-space/oioni/ui/canvas)

A zero-dependency 1-bit (black/white) drawing surface for e-ink displays.
Implements `draw.Image` and produces packed byte buffers for direct display transfer.

## Install

```sh
go get github.com/oioio-space/oioni/ui/canvas
```

## Quick start

```go
import (
    "image"
    "github.com/oioio-space/oioni/ui/canvas"
)

c := canvas.New(250, 122, canvas.Rot90)
c.DrawRect(image.Rect(0, 0, 250, 18), canvas.Black, true) // filled header
c.DrawText(4, 3, "OiOni", canvas.EmbeddedFont(12), canvas.White)
buf := c.Bytes() // 4000 bytes, 1 bpp, MSB first
```

## Rotation

| Constant | Use case |
|----------|----------|
| `canvas.Rot0` | Portrait (no rotation) |
| `canvas.Rot90` | Landscape (Waveshare HAT default) |
| `canvas.Rot180` | Portrait flipped |
| `canvas.Rot270` | Landscape flipped |

## Fonts

`canvas.EmbeddedFont(size)` returns an embedded bitmap font (sizes: 8, 12, 16, 20, 24).
```

- [ ] **Step 3: Create ui/canvas/example_test.go**

```go
package canvas_test

import (
	"fmt"
	"image"

	"github.com/oioio-space/oioni/ui/canvas"
)

func ExampleCanvas_Bytes() {
	c := canvas.New(16, 1, canvas.Rot0) // 16 px wide, 1 px tall
	c.DrawRect(image.Rect(0, 0, 16, 1), canvas.Black, true)
	buf := c.Bytes()
	// 16 pixels / 8 bits = 2 bytes, all black = 0x00
	fmt.Printf("len=%d bytes=[%02x %02x]\n", len(buf), buf[0], buf[1])
	// Output: len=2 bytes=[00 00]
}
```

- [ ] **Step 4: Create ui/gui/doc.go**

```go
// Package gui provides a widget toolkit for e-ink displays.
//
// The toolkit is designed for the Waveshare 2.13" Touch e-Paper HAT (250×122 px
// logical, after Rot90). It coordinates rendering, touch input, debounce, and
// anti-ghost full refreshes transparently.
//
// Core concepts:
//   - Widget: drawable element with bounds, dirty flag, and optional touch handling
//   - Scene: a tree of widgets displayed as a unit
//   - Navigator: manages a scene stack; Push/Pop trigger full refreshes
//   - refreshManager: decides partial vs full refresh based on dirty state and counter
//
// Minimal usage:
//
//	nav := gui.NewNavigator(display)
//	label := gui.NewLabel("Hello")
//	btn := gui.NewButton("OK")
//	btn.OnClick(func() { log.Println("clicked") })
//	nav.Push(&gui.Scene{
//	    Widgets: []gui.Widget{
//	        gui.NewVBox(label, gui.Expand(btn)),
//	    },
//	})
//	nav.Run(ctx, touchEvents)
package gui
```

- [ ] **Step 5: Create ui/gui/README.md**

```markdown
# gui — e-ink widget toolkit

[![Go Reference](https://pkg.go.dev/badge/github.com/oioio-space/oioni/ui/gui.svg)](https://pkg.go.dev/github.com/oioio-space/oioni/ui/gui)

A minimal, hardware-aware widget toolkit for e-ink displays. Handles partial/full
refresh scheduling, touch debounce (200 ms), anti-ghost full refresh (every 50 partials),
and scene navigation.

## Install

```sh
go get github.com/oioio-space/oioni/ui/gui
```

## Quick start

```go
nav := gui.NewNavigator(display) // display implements gui.Display interface

status := gui.NewStatusBar("", "Pi Zero 2W")
btn := gui.NewButton("Hello")
btn.OnClick(func() { log.Println("tapped!") })

nav.Push(&gui.Scene{
    Widgets: []gui.Widget{
        gui.NewVBox(
            gui.NewLabel("OiOni"),
            gui.NewDivider(),
            gui.Expand(btn),
            status,
        ),
    },
})

events, _ := touchDetector.Start(ctx)
nav.Run(ctx, events) // blocks until ctx cancelled
```

## Layout widgets

| Widget | Description |
|--------|-------------|
| `VBox` | Stack children vertically |
| `HBox` | Stack children horizontally |
| `Fixed` | Absolute pixel positioning |
| `Overlay` | Float content over a scene |
| `WithPadding` | Add uniform padding |
| `Expand(w)` | Fill remaining space |
| `FixedSize(w, px)` | Pin to exact size |

## Display widgets

| Widget | Description |
|--------|-------------|
| `Label` | Single-line text |
| `Button` | Touchable button with pressed state |
| `ProgressBar` | 0.0–1.0 horizontal fill bar |
| `StatusBar` | Full-width black bar with left/right text |
| `Spacer` | Invisible flexible gap |
| `Divider` | 1 px separator line |

## Refresh strategy

| Trigger | Strategy |
|---------|----------|
| `Push` / `Pop` | Full refresh (Init+DisplayBase) |
| Dirty widget | Partial refresh (~0.3 s) |
| Every 50 partials | Auto full refresh (anti-ghost) |
| No dirty widget | Noop |

## Implementing a custom widget

```go
type MyWidget struct {
    gui.BaseWidget
}

func (w *MyWidget) Draw(c *canvas.Canvas) {
    c.DrawRect(w.Bounds(), canvas.Black, false) // border
}
func (w *MyWidget) PreferredSize() image.Point { return image.Pt(60, 20) }
func (w *MyWidget) MinSize() image.Point       { return image.Pt(20, 10) }
```
```

- [ ] **Step 6: Create ui/gui/example_test.go**

```go
package gui_test

import (
	"image"

	"github.com/oioio-space/oioni/ui/canvas"
	"github.com/oioio-space/oioni/ui/gui"
)

// ExampleNewVBox shows how to build a vertical layout with a fixed header,
// an expanding body, and a status bar.
func ExampleNewVBox() {
	header := gui.NewLabel("OiOni")
	body := gui.NewLabel("Ready")
	status := gui.NewStatusBar("12:00", "Pi Zero 2W")

	root := gui.NewVBox(
		header,
		gui.NewDivider(),
		gui.Expand(body),
		status,
	)
	root.SetBounds(image.Rect(0, 0, 250, 122))
	_ = root
}

// ExampleButton shows how to create a touchable button with a click handler.
func ExampleNewButton() {
	btn := gui.NewButton("OK")
	btn.OnClick(func() {
		// handle tap
	})
	btn.SetBounds(image.Rect(0, 0, 80, 30))

	c := canvas.New(250, 122, canvas.Rot90)
	btn.Draw(c)
}
```

- [ ] **Step 7: Run tests**

```bash
go test github.com/oioio-space/oioni/ui/... -v -count=1
```

- [ ] **Step 8: Commit**

```bash
git add ui/canvas/ ui/gui/
git commit -m "docs(ui): add doc.go, README, examples for canvas and gui"
```

---

### Task 17: Global README + cmd/oioni README

**Files:**
- Create: `README.md` (root)
- Create: `cmd/oioni/README.md`

- [ ] **Step 1: Create root README.md**

```markdown
# OiOni

A collection of Go modules for building embedded Linux applications on
**Raspberry Pi Zero 2W** with the **Waveshare 2.13" Touch e-Paper HAT**.

Built for [gokrazy](https://gokrazy.org) — a minimal Go-native OS for Pi.

## Modules

### drivers/

Hardware device drivers — each independently importable.

| Module | Description | Hardware |
|--------|-------------|----------|
| [`drivers/epd`](./drivers/epd) | EPD 2.13" V4 e-ink display (SPI) | [Waveshare 2.13" HAT](https://www.waveshare.com/wiki/2.13inch_Touch_e-Paper_HAT) |
| [`drivers/touch`](./drivers/touch) | GT1151 capacitive touch (I2C) | [Waveshare 2.13" HAT](https://www.waveshare.com/wiki/2.13inch_Touch_e-Paper_HAT) |
| [`drivers/usbgadget`](./drivers/usbgadget) | Linux USB Gadget (RNDIS, ECM, HID, Storage, Audio…) | Raspberry Pi DWC2 |

### system/

Linux/kernel abstractions — no hardware dependency.

| Module | Description |
|--------|-------------|
| [`system/imgvol`](./system/imgvol) | Create, format, mount disk image files (FAT/exFAT/ext4) |
| [`system/storage`](./system/storage) | USB hotplug detection + automount (netlink + sysfs) |

### ui/

User interface toolkit — no hardware dependency.

| Module | Description |
|--------|-------------|
| [`ui/canvas`](./ui/canvas) | 1-bit drawing canvas (text, shapes, rotation) |
| [`ui/gui`](./ui/gui) | Widget toolkit with layout, navigation, and refresh management |

### cmd/

| Program | Description |
|---------|-------------|
| [`cmd/oioni`](./cmd/oioni) | Main gokrazy program — wires all modules together |

## Getting started

### Use a single module

```sh
go get github.com/oioio-space/oioni/drivers/epd
go get github.com/oioio-space/oioni/ui/gui
```

### Build the full app for gokrazy

Install [gokrazy](https://gokrazy.org/quickstart/):

```sh
go install github.com/gokrazy/tools/cmd/gok@latest
```

Create your instance config (see [`oioio/config.template.json`](./oioio/config.template.json)):

```sh
cp oioio/config.template.json oioio/config.json
# Edit oioio/config.json with your Pi IP, password, and SSH key
# Add your WiFi credentials to oioio/wifi.json
GOWORK=off gok update --parent_dir . -i oioio
```

### Hardware

| Component | Model | Link |
|-----------|-------|------|
| SBC | Raspberry Pi Zero 2W | https://www.raspberrypi.com/products/raspberry-pi-zero-2-w/ |
| Display | Waveshare 2.13" Touch e-Paper HAT | https://www.waveshare.com/wiki/2.13inch_Touch_e-Paper_HAT |

## Development

```sh
# Run all tests
go test ./drivers/... ./system/... ./ui/...

# Cross-compile for ARM64
GOOS=linux GOARCH=arm64 go build github.com/oioio-space/oioni/cmd/oioni

# Rebuild pre-built ARM64 kernel modules
make build-modules

# Rebuild pre-built ARM64 mkfs binaries
make build-imgvol-bins
```

## License

MIT — see [LICENSE](./LICENSE)
```

- [ ] **Step 2: Create cmd/oioni/README.md**

```markdown
# cmd/oioni — OiOni main program

Gokrazy-compatible application for Raspberry Pi Zero 2W. Combines:

- USB gadget (RNDIS network, ECM, HID keyboard, mass storage)
- E-ink display + touch UI via `ui/gui`
- USB hotplug storage manager
- Disk image volume management

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-rndis` | false | Enable RNDIS USB network (3 EP) |
| `-ecm` | false | Enable ECM USB network (3 EP) |
| `-hid` | false | Enable HID keyboard (1 EP) |
| `-mass-storage` | false | Enable USB drive from `--img` (2 EP) |
| `-epaper` | false | Enable e-ink display + touch |
| `-storage` | false | Enable USB hotplug storage |
| `-img` | `/perm/data.img` | Disk image path |
| `-img-fs` | `vfat` | Filesystem: vfat/exfat/ext4 |
| `-img-size` | 64 | Image size in MiB |
| `-img-create` | false | Create and format image |
| `-img-write` | false | Write test files to image |
| `-img-read` | false | List files in image |

> EP budget (DWC2/Pi Zero 2W): max 7 EPs. RNDIS+ECM+HID = 7 ✓ RNDIS+ECM+MassStorage = 8 ✗

## Deploy with gokrazy

See root [README.md](../../README.md) for gokrazy setup instructions.
```

- [ ] **Step 3: Commit**

```bash
git add README.md cmd/oioni/README.md
git commit -m "docs: add global README and cmd/oioni README"
```

---

## Chunk 4: CI/CD, Makefile, cleanup, and push

### Task 18: GitHub Actions workflows

**Files:**
- Create: `.github/workflows/test.yml`
- Create: `.github/workflows/build.yml`

- [ ] **Step 1: Create .github/workflows/test.yml**

```yaml
name: Test

on:
  push:
    branches: [ main, master ]
  pull_request:
    branches: [ main, master ]

jobs:
  test:
    name: Test all modules
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.26'

      - name: Test drivers/epd
        run: go test ./drivers/epd/... -v -count=1

      - name: Test drivers/touch
        run: go test ./drivers/touch/... -v -count=1

      - name: Test drivers/usbgadget (build only — needs Linux ConfigFS)
        run: go build ./drivers/usbgadget/...

      - name: Test system/imgvol (build only — needs loop devices)
        run: go build ./system/imgvol/...

      - name: Test system/storage
        run: go test ./system/storage/... -v -count=1

      - name: Test ui/canvas
        run: go test ./ui/canvas/... -v -count=1

      - name: Test ui/gui
        run: go test ./ui/gui/... -v -count=1
```

- [ ] **Step 2: Create .github/workflows/build.yml**

```yaml
name: Build ARM64

on:
  push:
    branches: [ main, master ]
  pull_request:
    branches: [ main, master ]

jobs:
  build-arm64:
    name: Cross-compile cmd/oioni for ARM64
    runs-on: ubuntu-latest
    env:
      GOOS: linux
      GOARCH: arm64
      # Gokrazy credentials (set in repo secrets for full build)
      GOKRAZY_UPDATE_HOSTNAME: ${{ secrets.GOKRAZY_UPDATE_HOSTNAME }}
      GOKRAZY_HTTP_PASSWORD: ${{ secrets.GOKRAZY_HTTP_PASSWORD }}
    steps:
      - uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.26'

      - name: Build cmd/oioni (ARM64)
        run: go build -o /tmp/oioni github.com/oioio-space/oioni/cmd/oioni

      - name: Upload binary
        uses: actions/upload-artifact@v4
        with:
          name: oioni-arm64
          path: /tmp/oioni
```

- [ ] **Step 3: Commit**

```bash
mkdir -p .github/workflows
git add .github/
git commit -m "ci: add GitHub Actions test + ARM64 build workflows"
```

---

### Task 19: Update Makefile

**Files:**
- Modify: `Makefile`

- [ ] **Step 1: Rewrite Makefile for new structure**

```makefile
PARENT_DIR := $(shell pwd)
INSTANCE   := oioio
GOK        := GOWORK=off gok --parent_dir $(PARENT_DIR) -i $(INSTANCE)
SSH_KEY    := $(HOME)/.ssh/id_ed25519
HOST       := 192.168.0.33
MODULE     := github.com/oioio-space/oioni

DETECTED_SD := $(shell ls -l /dev/disk/by-id/ 2>/dev/null \
	| awk '/(usb-|mmc-)/ && !/-part[0-9]/ {print $$NF}' \
	| sed 's|../../||' | head -1 | xargs -I{} echo /dev/{})

## ── Development ───────────────────────────────────────────────────────────────

.PHONY: help test build build-arm64 lint

help: ## Show this help
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z_-]+:.*##/ {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

test: ## Run all tests (x86_64, unit tests only)
	go test ./drivers/epd/... ./drivers/touch/... \
	         ./system/storage/... \
	         ./ui/canvas/... ./ui/gui/...

build: ## Verify gokrazy build (no physical device)
	$(GOK) overwrite --gaf /tmp/gokrazy-$(INSTANCE).gaf

build-arm64: ## Cross-compile cmd/oioni for ARM64
	GOOS=linux GOARCH=arm64 go build -o /tmp/oioni $(MODULE)/cmd/oioni
	@echo "Built /tmp/oioni (ARM64)"

## ── Deployment ───────────────────────────────────────────────────────────────

.PHONY: update flash flash-auto list-sd ssh logs find-pi

update: ## OTA update over the network
	$(GOK) update

flash-auto: ## Auto-detect SD card and flash (with confirmation)
	@if [ -z "$(DETECTED_SD)" ]; then \
		echo "No SD card detected in /dev/disk/by-id/"; exit 1; fi
	@echo "Detected: $(DETECTED_SD)"
	@lsblk -d -o NAME,SIZE,MODEL $(DETECTED_SD) 2>/dev/null || true
	@printf "Flash gokrazy to $(DETECTED_SD)? [y/N] " && read c && [ "$$c" = "y" ]
	@sudo umount $(DETECTED_SD)p* $(DETECTED_SD)[0-9]* 2>/dev/null || true
	$(GOK) overwrite --full $(DETECTED_SD)

flash: ## Flash to explicit device (usage: make flash DRIVE=/dev/sdX)
	@if [ -z "$(DRIVE)" ]; then echo "Usage: make flash DRIVE=/dev/sdX"; exit 1; fi
	@sudo umount $(DRIVE)p* $(DRIVE)[0-9]* 2>/dev/null || true
	$(GOK) overwrite --full $(DRIVE)

list-sd: ## List removable storage devices
	@ls -l /dev/disk/by-id/ | awk '/(usb-|mmc-)/ && !/-part[0-9]/ {print $$NF}' | sed 's|../../||'

ssh: ## SSH into the Pi
	ssh -i $(SSH_KEY) root@$(HOST)

logs: ## Stream service logs (usage: make logs PKG=cmd/oioni)
	$(GOK) logs --follow $(PKG)

find-pi: ## Ping the Pi
	ping -c 3 $(HOST)

## ── Build tools (cross-compile pre-built binaries) ───────────────────────────

.PHONY: build-modules build-imgvol-bins build-all

build-modules: ## Rebuild ARM64 kernel modules (usbgadget) via podman
	podman build --platform linux/arm64 \
	    --output type=local,dest=drivers/usbgadget/modules/build/out \
	    drivers/usbgadget/modules/build/
	cp drivers/usbgadget/modules/build/out/6.*.ko drivers/usbgadget/modules/6.12.47-v8/

build-imgvol-bins: ## Rebuild ARM64 static mkfs binaries (imgvol) via podman
	podman build --platform linux/arm64 \
	    --output type=local,dest=system/imgvol/bin \
	    system/imgvol/build/
	@ls -lh system/imgvol/bin/mkfs.*

build-all: build-modules build-imgvol-bins build ## Rebuild all pre-built artifacts + verify gokrazy build

.DEFAULT_GOAL := help
```

- [ ] **Step 2: Run tests via Makefile**

```bash
make test
```

Expected: all unit tests pass.

- [ ] **Step 3: Commit**

```bash
git add Makefile
git commit -m "chore(Makefile): update for multi-module structure and new paths"
```

---

### Task 20: Beads + Claude memories in repo + final push

**Files:**
- Verify: `.beads/` is committed (not gitignored)
- Verify: `docs/claude/memory/` is committed

- [ ] **Step 1: Ensure .beads/ is committed**

```bash
git status .beads/
```

If not tracked, add it:

```bash
git add .beads/
git commit -m "chore: include beads project tracking database in repo"
```

- [ ] **Step 2: Verify docs/claude/memory/ is committed**

```bash
git status docs/claude/
```

Should be committed from Task 1. If not:

```bash
git add docs/claude/
git commit -m "chore: include Claude AI session memories in repo"
```

- [ ] **Step 3: Run full test suite one final time**

```bash
go test ./drivers/epd/... ./drivers/touch/... \
         ./system/storage/... \
         ./ui/canvas/... ./ui/gui/...
```

Expected: all pass.

- [ ] **Step 4: Verify ARM64 cross-compile**

```bash
GOOS=linux GOARCH=arm64 go build github.com/oioio-space/oioni/cmd/oioni && echo "OK"
```

- [ ] **Step 5: Close beads issue**

```bash
bd close awesomeProject-woh --reason="Restructuring complete, all modules migrated, docs written, CI added, pushed to GitHub"
```

- [ ] **Step 6: Push to GitHub**

```bash
git push -u origin master
```

If the default branch on GitHub is `main`:

```bash
git push -u origin master:main
```

- [ ] **Step 7: Verify on GitHub**

Visit https://github.com/oioio-space/oioni and confirm:
- All modules visible with README
- Actions tab shows workflow runs
- No credentials in any file (grep -r "wxcvbn\|@rthur\|AAAA" --include="*.json" --include="*.go")

---

## Summary of commits

| Commit | Scope |
|--------|-------|
| `chore: add LICENSE, .gitignore, config template, memories` | Skeleton |
| `chore: create multi-module directory structure` | Skeleton |
| `feat(drivers/epd): migrate EPD driver` | Code |
| `feat(drivers/touch): migrate touch driver` | Code |
| `feat(drivers/usbgadget): migrate USB gadget framework` | Code |
| `feat(system/imgvol): migrate disk image manager` | Code |
| `feat(system/storage): migrate USB storage manager` | Code |
| `feat(ui/canvas): migrate 1-bit canvas` | Code |
| `feat(ui/gui): migrate e-ink GUI toolkit` | Code |
| `feat(cmd/oioni): migrate main program` | Code |
| `chore: remove old source directories` | Cleanup |
| `docs(drivers/epd): doc.go, README, examples` | Docs |
| `docs(drivers/touch): doc.go, README, examples` | Docs |
| `docs(drivers/usbgadget): README, import fix` | Docs |
| `docs(system): doc.go, README, examples` | Docs |
| `docs(ui): doc.go, README, examples` | Docs |
| `docs: global README + cmd/oioni README` | Docs |
| `ci: GitHub Actions workflows` | CI |
| `chore(Makefile): update for new structure` | CI |
| `chore: beads + Claude memories` | Meta |
