# USB Gadget Package — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Créer le package Go `awesomeProject/usbgadget` permettant de créer des composite USB gadgets sur Pi Zero 2W via libcomposite/configfs, avec modules .ko embarqués.

**Architecture:** Package `usbgadget/` avec functional options, sous-package `usbgadget/modules/` pour les .ko embarqués (go:embed). Les .ko sont cross-compilés pour arm64/6.12.47-v8 et committés. Le programme `hello/` est transformé en démo composite USB (RNDIS+ECM+HID+MassStorage).

**Tech Stack:** Go 1.26 (CGO_ENABLED=0), go:embed, configfs (/sys/kernel/config/), insmod syscall, kernel 6.12.47-v8 arm64, Docker pour cross-compilation one-time des .ko.

---

## Contexte technique

- **Pi** : 192.168.0.33
- **OTA** : `make update` (GOWORK=off gok --parent_dir . -i oioio update)
- **Build vérif** : `GOWORK=off gok --parent_dir . -i oioio overwrite --gaf /tmp/gokrazy-oioio.gaf`
- **UDC** : `fe980000.usb` (DWC2 OTG)
- **Kernel** : 6.12.47-v8 arm64 — USB_GADGET=y DWC2_DUAL_ROLE=y built-in, aucun function driver
- **Module racine** : `awesomeProject` (local uniquement, pas github.com/...)
- **go.work** : nécessite `use (. ./oioio/builddir/awesomeProject ...)` — les builddir sont gérés par gok

---

### Task 1: Créer la structure du package usbgadget

**Files:**
- Create: `usbgadget/gadget.go`
- Create: `usbgadget/functions/function.go`

**Step 1: Créer l'interface Function**

```go
// usbgadget/functions/function.go
package functions

// Function représente un USB function driver (HID, RNDIS, ECM, etc.)
type Function interface {
    // TypeName retourne le nom du driver (ex: "hid", "rndis", "ecm")
    TypeName() string
    // InstanceName retourne le nom d'instance (ex: "usb0", "usb1")
    InstanceName() string
    // Configure écrit les attributs spécifiques dans le répertoire configfs du function
    Configure(dir string) error
}
```

**Step 2: Créer le type Gadget avec functional options**

```go
// usbgadget/gadget.go
package usbgadget

import (
    "awesomeProject/usbgadget/functions"
)

type Gadget struct {
    name         string
    vendorID     uint16
    productID    uint16
    manufacturer string
    product      string
    serialNumber string
    langID       string
    usbMajor     uint8
    usbMinor     uint8
    funcs        []functions.Function
}

type Option func(*Gadget)

func New(opts ...Option) (*Gadget, error) {
    g := &Gadget{
        name:      "g1",
        vendorID:  0x1d6b,
        productID: 0x0104,
        langID:    "0x409",
        usbMajor:  2,
        usbMinor:  0,
    }
    for _, opt := range opts {
        opt(g)
    }
    return g, nil
}

func WithName(name string) Option {
    return func(g *Gadget) { g.name = name }
}

func WithVendorID(vendor, product uint16) Option {
    return func(g *Gadget) {
        g.vendorID = vendor
        g.productID = product
    }
}

func WithStrings(langID, manufacturer, product, serial string) Option {
    return func(g *Gadget) {
        g.langID = langID
        g.manufacturer = manufacturer
        g.product = product
        g.serialNumber = serial
    }
}

func WithUSBVersion(major, minor uint8) Option {
    return func(g *Gadget) {
        g.usbMajor = major
        g.usbMinor = minor
    }
}

func (g *Gadget) Enable() error  { return nil } // implémenté task 4
func (g *Gadget) Disable() error { return nil } // implémenté task 4
func (g *Gadget) Reload() error  { return g.Disable(); return g.Enable() }
```

**Step 3: Vérifier la compilation**

```bash
cd /home/oioio/Documents/GolandProjects/awesomeProject
go build ./usbgadget/...
```
Expected: pas d'erreur

**Step 4: Commit**

```bash
git add usbgadget/
git commit -m "feat(usbgadget): skeleton package with Function interface and Gadget type"
```

---

### Task 2: Créer le Dockerfile de cross-compilation des .ko

**Files:**
- Create: `usbgadget/modules/build/Dockerfile`
- Create: `usbgadget/modules/build/Makefile`

**Step 1: Créer le Dockerfile**

```dockerfile
# usbgadget/modules/build/Dockerfile
# Cross-compile USB gadget kernel modules pour arm64 / kernel 6.12.47-v8
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential bc flex bison libssl-dev libelf-dev \
    gcc-aarch64-linux-gnu binutils-aarch64-linux-gnu \
    wget git ca-certificates \
    && rm -rf /var/lib/apt/lists/*

ARG KVER=6.12.47-v8
ARG ARCH=arm64
ARG CROSS_COMPILE=aarch64-linux-gnu-

WORKDIR /build

# Télécharge les sources du kernel Raspberry Pi
RUN git clone --depth=1 --branch rpi-6.12.y \
    https://github.com/raspberrypi/linux.git kernel

WORKDIR /build/kernel

# Config minimale pour compiler uniquement les modules USB gadget
RUN make ARCH=${ARCH} CROSS_COMPILE=${CROSS_COMPILE} bcm2711_defconfig

# Active les modules USB gadget nécessaires
RUN scripts/config \
    --module CONFIG_USB_GADGET \
    --module CONFIG_USB_LIBCOMPOSITE \
    --module CONFIG_USB_ETH \
    --module CONFIG_USB_ETH_RNDIS \
    --module CONFIG_USB_ETH_EEM \
    --module CONFIG_USB_G_NCM \
    --module CONFIG_USB_HID \
    --module CONFIG_USB_MASS_STORAGE \
    --module CONFIG_USB_G_SERIAL \
    --module CONFIG_USB_F_MIDI \
    --set-val CONFIG_LOCALVERSION "" \
    --set-str CONFIG_LOCALVERSION_AUTO ""

# Prépare les headers
RUN make ARCH=${ARCH} CROSS_COMPILE=${CROSS_COMPILE} \
    -j$(nproc) modules_prepare

# Compile uniquement les modules USB gadget
RUN make ARCH=${ARCH} CROSS_COMPILE=${CROSS_COMPILE} \
    -j$(nproc) \
    M=drivers/usb/gadget/function \
    M=drivers/usb/gadget \
    modules

# Collecte les .ko
RUN mkdir -p /out/${KVER} && \
    find . -name "*.ko" \
        \( -name "libcomposite.ko" \
        -o -name "u_ether.ko" \
        -o -name "usb_f_rndis.ko" \
        -o -name "usb_f_ecm.ko" \
        -o -name "usb_f_ncm.ko" \
        -o -name "usb_f_hid.ko" \
        -o -name "usb_f_mass_storage.ko" \
        -o -name "usb_f_acm.ko" \
        -o -name "u_serial.ko" \
        \) \
    -exec cp {} /out/${KVER}/ \;

CMD ["ls", "-la", "/out/"]
```

**Step 2: Créer le Makefile de build**

```makefile
# usbgadget/modules/build/Makefile
KVER   ?= 6.12.47-v8
OUTDIR := ../$(KVER)
IMAGE  := usbgadget-modules-builder

build:
	docker build \
		--build-arg KVER=$(KVER) \
		-t $(IMAGE):$(KVER) \
		-f Dockerfile .
	mkdir -p $(OUTDIR)
	docker run --rm \
		-v $(shell realpath $(OUTDIR)):/out/$(KVER) \
		$(IMAGE):$(KVER) \
		sh -c "cp /out/$(KVER)/*.ko /out/$(KVER)/"
	@echo "Modules copiés dans $(OUTDIR):"
	@ls -la $(OUTDIR)/*.ko 2>/dev/null || echo "ATTENTION: aucun .ko trouvé"

.PHONY: build
```

**Step 3: Commit**

```bash
git add usbgadget/modules/build/
git commit -m "feat(usbgadget/modules): add Dockerfile and Makefile for cross-compiling USB gadget .ko"
```

---

### Task 3: Cross-compiler les .ko et créer le package modules

**Files:**
- Create: `usbgadget/modules/6.12.47-v8/` (les .ko)
- Create: `usbgadget/modules/embed.go`
- Create: `usbgadget/modules/modules.go`

**Step 1: Lancer le build Docker**

```bash
cd /home/oioio/Documents/GolandProjects/awesomeProject/usbgadget/modules/build
make build KVER=6.12.47-v8
```
Expected: les .ko sont copiés dans `usbgadget/modules/6.12.47-v8/`

Vérifier la liste :
```bash
ls -la ../6.12.47-v8/*.ko
```
Expected: au minimum `libcomposite.ko`, `u_ether.ko`, `usb_f_rndis.ko`, `usb_f_ecm.ko`, `usb_f_hid.ko`, `usb_f_mass_storage.ko`, `usb_f_acm.ko`

**Step 2: Créer embed.go**

```go
// usbgadget/modules/embed.go
package modules

import "embed"

//go:embed 6.12.47-v8/*.ko
var koFS embed.FS
```

**Step 3: Créer modules.go**

```go
// usbgadget/modules/modules.go
package modules

import (
    "fmt"
    "os"
    "path/filepath"
    "syscall"
    "unsafe"
)

// Load charge les modules USB gadget nécessaires pour la version kernel kver.
// Les .ko sont extraits de l'embed FS vers /tmp et chargés via insmod (init_module syscall).
func Load(kver string) error {
    deps := []string{
        "libcomposite",
        "u_ether",
        "usb_f_rndis",
        "usb_f_ecm",
        "usb_f_ncm",
        "usb_f_hid",
        "usb_f_mass_storage",
        "usb_f_acm",
    }
    for _, name := range deps {
        if err := loadModule(kver, name); err != nil {
            return fmt.Errorf("loading %s: %w", name, err)
        }
    }
    return nil
}

func loadModule(kver, name string) error {
    src := filepath.Join(kver, name+".ko")
    data, err := koFS.ReadFile(src)
    if os.IsNotExist(err) {
        // .ko absent pour cette version kernel — skip silencieux
        return nil
    }
    if err != nil {
        return err
    }
    return insmod(data)
}

// insmod charge un module kernel depuis son contenu binaire.
// Utilise le syscall init_module directement (pas besoin de fichier temporaire).
func insmod(data []byte) error {
    if len(data) == 0 {
        return nil
    }
    params := ""
    paramsPtr, err := syscall.BytePtrFromString(params)
    if err != nil {
        return err
    }
    _, _, errno := syscall.Syscall(
        syscall.SYS_INIT_MODULE,
        uintptr(unsafe.Pointer(&data[0])),
        uintptr(len(data)),
        uintptr(unsafe.Pointer(paramsPtr)),
    )
    if errno != 0 && errno != syscall.EEXIST {
        return fmt.Errorf("init_module: %w", errno)
    }
    return nil
}
```

**Step 4: Vérifier la compilation**

```bash
cd /home/oioio/Documents/GolandProjects/awesomeProject
go build ./usbgadget/modules/
```
Expected: pas d'erreur

**Step 5: Commit**

```bash
git add usbgadget/modules/
git commit -m "feat(usbgadget/modules): embed USB gadget .ko for kernel 6.12.47-v8 arm64"
```

---

### Task 4: Implémenter configfs.go et udc.go (couche bas niveau)

**Files:**
- Create: `usbgadget/configfs.go`
- Create: `usbgadget/udc.go`

**Step 1: Créer configfs.go**

```go
// usbgadget/configfs.go
package usbgadget

import (
    "fmt"
    "os"
    "path/filepath"
    "strconv"
)

const configfsRoot = "/sys/kernel/config/usb_gadget"

// gadgetDir retourne le chemin du gadget dans configfs
func (g *Gadget) gadgetDir() string {
    return filepath.Join(configfsRoot, g.name)
}

// setupConfigfs crée et configure toute la structure configfs du gadget
func (g *Gadget) setupConfigfs() error {
    dir := g.gadgetDir()

    // Créer le répertoire du gadget
    if err := os.MkdirAll(dir, 0755); err != nil {
        return fmt.Errorf("mkdir gadget: %w", err)
    }

    // IDs USB
    if err := writeHex(filepath.Join(dir, "idVendor"), uint64(g.vendorID)); err != nil {
        return err
    }
    if err := writeHex(filepath.Join(dir, "idProduct"), uint64(g.productID)); err != nil {
        return err
    }

    // Version USB BCD (ex: 0x0200 pour USB 2.0)
    bcd := uint64(g.usbMajor)<<8 | uint64(g.usbMinor)
    if err := writeHex(filepath.Join(dir, "bcdUSB"), bcd); err != nil {
        return err
    }

    // Strings
    if g.manufacturer != "" || g.product != "" || g.serialNumber != "" {
        strDir := filepath.Join(dir, "strings", g.langID)
        if err := os.MkdirAll(strDir, 0755); err != nil {
            return fmt.Errorf("mkdir strings: %w", err)
        }
        if g.manufacturer != "" {
            if err := writeString(filepath.Join(strDir, "manufacturer"), g.manufacturer); err != nil {
                return err
            }
        }
        if g.product != "" {
            if err := writeString(filepath.Join(strDir, "product"), g.product); err != nil {
                return err
            }
        }
        if g.serialNumber != "" {
            if err := writeString(filepath.Join(strDir, "serialnumber"), g.serialNumber); err != nil {
                return err
            }
        }
    }

    // Configuration c.1
    cfgDir := filepath.Join(dir, "configs", "c.1")
    if err := os.MkdirAll(cfgDir, 0755); err != nil {
        return fmt.Errorf("mkdir config: %w", err)
    }
    if err := writeString(filepath.Join(cfgDir, "MaxPower"), "250"); err != nil {
        return err
    }

    // Créer et configurer chaque function, puis créer le symlink dans configs/c.1/
    orderedFuncs := sortFunctions(g.funcs)
    for _, f := range orderedFuncs {
        funcPath := fmt.Sprintf("%s.%s", f.TypeName(), f.InstanceName())
        funcDir := filepath.Join(dir, "functions", funcPath)
        if err := os.MkdirAll(funcDir, 0755); err != nil {
            return fmt.Errorf("mkdir function %s: %w", funcPath, err)
        }
        if err := f.Configure(funcDir); err != nil {
            return fmt.Errorf("configure %s: %w", funcPath, err)
        }
        // Symlink dans configs/c.1/ pour activer la function
        linkDst := filepath.Join(dir, "functions", funcPath)
        linkSrc := filepath.Join(cfgDir, funcPath)
        if err := os.Symlink(linkDst, linkSrc); err != nil && !os.IsExist(err) {
            return fmt.Errorf("symlink %s: %w", funcPath, err)
        }
    }

    return nil
}

// teardownConfigfs supprime toute la structure configfs du gadget
func (g *Gadget) teardownConfigfs() error {
    dir := g.gadgetDir()
    if _, err := os.Stat(dir); os.IsNotExist(err) {
        return nil
    }
    cfgDir := filepath.Join(dir, "configs", "c.1")
    // Supprimer les symlinks d'abord
    entries, _ := os.ReadDir(cfgDir)
    for _, e := range entries {
        if e.Type()&os.ModeSymlink != 0 {
            os.Remove(filepath.Join(cfgDir, e.Name()))
        }
    }
    // Supprimer dans l'ordre inverse de création
    os.Remove(filepath.Join(dir, "configs", "c.1", "strings", "0x409"))
    os.Remove(filepath.Join(dir, "configs", "c.1"))
    os.Remove(filepath.Join(dir, "configs"))
    funcsDir := filepath.Join(dir, "functions")
    entries, _ = os.ReadDir(funcsDir)
    for _, e := range entries {
        os.Remove(filepath.Join(funcsDir, e.Name()))
    }
    os.Remove(funcsDir)
    os.Remove(filepath.Join(dir, "strings", g.langID))
    os.Remove(filepath.Join(dir, "strings"))
    return os.Remove(dir)
}

func writeHex(path string, value uint64) error {
    s := "0x" + strconv.FormatUint(value, 16)
    return os.WriteFile(path, []byte(s), 0644)
}

func writeString(path, value string) error {
    return os.WriteFile(path, []byte(value), 0644)
}
```

**Step 2: Créer udc.go**

```go
// usbgadget/udc.go
package usbgadget

import (
    "fmt"
    "os"
    "path/filepath"
    "strings"
)

const udcPath = "/sys/class/udc"

// detectUDC retourne le nom du premier UDC disponible (ex: "fe980000.usb")
func detectUDC() (string, error) {
    entries, err := os.ReadDir(udcPath)
    if err != nil {
        return "", fmt.Errorf("no UDC found at %s: %w", udcPath, err)
    }
    for _, e := range entries {
        return e.Name(), nil
    }
    return "", fmt.Errorf("no UDC devices found in %s", udcPath)
}

// bindUDC écrit le nom du UDC dans le gadget pour l'activer
func (g *Gadget) bindUDC() error {
    udc, err := detectUDC()
    if err != nil {
        return err
    }
    udcFile := filepath.Join(g.gadgetDir(), "UDC")
    return os.WriteFile(udcFile, []byte(udc), 0644)
}

// unbindUDC délie le gadget du UDC
func (g *Gadget) unbindUDC() error {
    udcFile := filepath.Join(g.gadgetDir(), "UDC")
    content, err := os.ReadFile(udcFile)
    if err != nil {
        return nil // already unbound
    }
    if strings.TrimSpace(string(content)) == "" {
        return nil
    }
    return os.WriteFile(udcFile, []byte(""), 0644)
}
```

**Step 3: Implémenter Enable/Disable dans gadget.go**

Modifier `gadget.go` pour implémenter les méthodes :

```go
import (
    "awesomeProject/usbgadget/functions"
    "awesomeProject/usbgadget/modules"
    "fmt"
    "os"
    "runtime"
)

// WithFunction ajoute une function USB au gadget (usage interne)
func withFunction(f functions.Function) Option {
    return func(g *Gadget) { g.funcs = append(g.funcs, f) }
}

func (g *Gadget) Enable() error {
    // 1. Vérifier qu'on est root
    if os.Getuid() != 0 {
        return fmt.Errorf("must run as root to manage USB gadgets")
    }
    // 2. Vérifier architecture arm64
    if runtime.GOARCH != "arm64" {
        return fmt.Errorf("USB gadget only supported on arm64 (current: %s)", runtime.GOARCH)
    }
    // 3. Charger les modules kernel
    kver, err := kernelVersion()
    if err != nil {
        return fmt.Errorf("kernelVersion: %w", err)
    }
    if err := modules.Load(kver); err != nil {
        return fmt.Errorf("modules.Load: %w", err)
    }
    // 4. Monter configfs si nécessaire
    if err := mountConfigfs(); err != nil {
        return fmt.Errorf("mountConfigfs: %w", err)
    }
    // 5. Configurer via configfs
    if err := g.setupConfigfs(); err != nil {
        return fmt.Errorf("setupConfigfs: %w", err)
    }
    // 6. Bind au UDC
    if err := g.bindUDC(); err != nil {
        return fmt.Errorf("bindUDC: %w", err)
    }
    return nil
}

func (g *Gadget) Disable() error {
    if err := g.unbindUDC(); err != nil {
        return fmt.Errorf("unbindUDC: %w", err)
    }
    return g.teardownConfigfs()
}

func (g *Gadget) Reload() error {
    if err := g.Disable(); err != nil {
        return err
    }
    return g.Enable()
}
```

**Step 4: Créer modules.go (kernel version + configfs mount)**

```go
// usbgadget/kernel.go
package usbgadget

import (
    "fmt"
    "os"
    "strings"
    "syscall"
)

// kernelVersion retourne la version kernel via uname (ex: "6.12.47-v8")
func kernelVersion() (string, error) {
    var uts syscall.Utsname
    if err := syscall.Uname(&uts); err != nil {
        return "", fmt.Errorf("uname: %w", err)
    }
    // Utsname.Release est [65]int8 ou [65]uint8 selon l'arch
    buf := make([]byte, 0, 65)
    for _, c := range uts.Release {
        if c == 0 {
            break
        }
        buf = append(buf, byte(c))
    }
    return strings.TrimSpace(string(buf)), nil
}

// mountConfigfs monte configfs sur /sys/kernel/config si pas déjà monté
func mountConfigfs() error {
    const target = "/sys/kernel/config"
    // Vérifie si déjà monté en essayant de lire usb_gadget
    if _, err := os.Stat(target + "/usb_gadget"); err == nil {
        return nil
    }
    // Tente le mount
    err := syscall.Mount("configfs", target, "configfs", 0, "")
    if err != nil && err != syscall.EBUSY {
        return fmt.Errorf("mount configfs: %w", err)
    }
    return nil
}
```

**Step 5: Créer priority.go**

```go
// usbgadget/priority.go
package usbgadget

import (
    "awesomeProject/usbgadget/functions"
    "sort"
)

// Priorités des function types pour l'ordre des symlinks dans configs/c.1/
// Windows lit les interfaces dans l'ordre — RNDIS DOIT être premier.
var typePriority = map[string]int{
    "rndis":        0, // Windows network — toujours premier
    "ecm":          1, // Linux/macOS network
    "ncm":          2, // High-speed network
    "hid":          3, // HID (keyboard, mouse)
    "mass_storage": 4, // USB Mass Storage
    "acm":          5, // ACM Serial
    "midi":         6, // MIDI
}

func sortFunctions(funcs []functions.Function) []functions.Function {
    sorted := make([]functions.Function, len(funcs))
    copy(sorted, funcs)
    sort.SliceStable(sorted, func(i, j int) bool {
        pi, ok1 := typePriority[sorted[i].TypeName()]
        pj, ok2 := typePriority[sorted[j].TypeName()]
        if !ok1 {
            pi = 99
        }
        if !ok2 {
            pj = 99
        }
        return pi < pj
    })
    return sorted
}
```

**Step 6: Vérifier la compilation**

```bash
cd /home/oioio/Documents/GolandProjects/awesomeProject
go build ./usbgadget/
```
Expected: pas d'erreur

**Step 7: Commit**

```bash
git add usbgadget/
git commit -m "feat(usbgadget): implement configfs setup, UDC bind, kernel module loading, priority ordering"
```

---

### Task 5: Implémenter les function drivers réseau (RNDIS, ECM, NCM)

**Files:**
- Create: `usbgadget/functions/rndis.go`
- Create: `usbgadget/functions/ecm.go`
- Create: `usbgadget/functions/ncm.go`
- Modify: `usbgadget/gadget.go` (ajouter WithRNDIS, WithECM, WithNCM)

**Step 1: Créer rndis.go**

```go
// usbgadget/functions/rndis.go
package functions

import "os"

type rndisFunc struct {
    instance string
    host     string // MAC adresse hôte (optionnel)
    dev      string // MAC adresse device (optionnel)
}

// RNDIS crée une function RNDIS (réseau Windows).
// Doit être la première function dans le composite pour compatibilité Windows.
func RNDIS() Function { return &rndisFunc{instance: "usb0"} }

func (f *rndisFunc) TypeName() string     { return "rndis" }
func (f *rndisFunc) InstanceName() string { return f.instance }
func (f *rndisFunc) Configure(dir string) error {
    if f.host != "" {
        if err := os.WriteFile(dir+"/host_addr", []byte(f.host), 0644); err != nil {
            return err
        }
    }
    if f.dev != "" {
        if err := os.WriteFile(dir+"/dev_addr", []byte(f.dev), 0644); err != nil {
            return err
        }
    }
    return nil
}
```

**Step 2: Créer ecm.go**

```go
// usbgadget/functions/ecm.go
package functions

import "os"

type ecmFunc struct {
    instance string
}

// ECM crée une function ECM (réseau Linux/macOS).
func ECM() Function { return &ecmFunc{instance: "usb1"} }

func (f *ecmFunc) TypeName() string     { return "ecm" }
func (f *ecmFunc) InstanceName() string { return f.instance }
func (f *ecmFunc) Configure(dir string) error {
    _ = dir // ECM n'a pas d'attributs requis
    return nil
}
```

**Step 3: Créer ncm.go**

```go
// usbgadget/functions/ncm.go
package functions

type ncmFunc struct {
    instance string
}

// NCM crée une function NCM (réseau haute vitesse).
func NCM() Function { return &ncmFunc{instance: "usb2"} }

func (f *ncmFunc) TypeName() string     { return "ncm" }
func (f *ncmFunc) InstanceName() string { return f.instance }
func (f *ncmFunc) Configure(_ string) error { return nil }
```

**Step 4: Ajouter les options dans gadget.go**

```go
// Dans usbgadget/gadget.go, ajouter :

import "awesomeProject/usbgadget/functions"

func WithRNDIS() Option {
    return withFunction(functions.RNDIS())
}

func WithECM() Option {
    return withFunction(functions.ECM())
}

func WithNCM() Option {
    return withFunction(functions.NCM())
}
```

**Step 5: Vérifier**

```bash
go build ./usbgadget/...
```

**Step 6: Commit**

```bash
git add usbgadget/functions/rndis.go usbgadget/functions/ecm.go usbgadget/functions/ncm.go usbgadget/gadget.go
git commit -m "feat(usbgadget): add RNDIS, ECM, NCM network function drivers"
```

---

### Task 6: Implémenter HID (clavier, souris)

**Files:**
- Create: `usbgadget/functions/hid.go`
- Modify: `usbgadget/gadget.go` (ajouter WithHID)

**Step 1: Créer hid.go**

```go
// usbgadget/functions/hid.go
package functions

import (
    "fmt"
    "os"
    "sync/atomic"
)

// Compteur pour instances HID multiples (clavier=0, souris=1...)
var hidCounter atomic.Int32

type hidFunc struct {
    instance    string
    protocol    uint8  // 1=keyboard, 2=mouse
    subclass    uint8  // 1=boot interface
    reportLen   uint16 // taille du rapport HID
    reportDesc  []byte // descripteur HID
}

type HIDOption func(*hidFunc)

func newHID(opts ...HIDOption) *hidFunc {
    n := hidCounter.Add(1) - 1
    f := &hidFunc{
        instance: fmt.Sprintf("usb%d", n),
    }
    for _, o := range opts {
        o(f)
    }
    return f
}

// Keyboard crée un HID keyboard standard (boot protocol).
func Keyboard(opts ...HIDOption) Function {
    f := newHID(opts...)
    f.protocol = 1
    f.subclass = 1
    f.reportLen = 8
    // Descripteur HID clavier standard (boot keyboard)
    f.reportDesc = []byte{
        0x05, 0x01, // Usage Page (Generic Desktop)
        0x09, 0x06, // Usage (Keyboard)
        0xa1, 0x01, // Collection (Application)
        0x05, 0x07, // Usage Page (Keyboard)
        0x19, 0xe0, 0x29, 0xe7, // Usage Minimum/Maximum (modifier keys)
        0x15, 0x00, 0x25, 0x01, // Logical Min/Max
        0x75, 0x01, 0x95, 0x08, // Report Size 1, Count 8
        0x81, 0x02, // Input (Data, Variable, Absolute)
        0x95, 0x01, 0x75, 0x08, // Report Count 1, Size 8
        0x81, 0x03, // Input (Constant) — padding
        0x95, 0x06, 0x75, 0x08, // Report Count 6, Size 8
        0x15, 0x00, 0x25, 0x65, // Logical Min 0, Max 101
        0x05, 0x07, // Usage Page (Keyboard)
        0x19, 0x00, 0x29, 0x65, // Usage Min/Max
        0x81, 0x00, // Input (Data, Array)
        0xc0,       // End Collection
    }
    return f
}

// Mouse crée un HID mouse standard (boot protocol).
func Mouse(opts ...HIDOption) Function {
    f := newHID(opts...)
    f.protocol = 2
    f.subclass = 1
    f.reportLen = 4
    f.reportDesc = []byte{
        0x05, 0x01, // Usage Page (Generic Desktop)
        0x09, 0x02, // Usage (Mouse)
        0xa1, 0x01, // Collection (Application)
        0x09, 0x01, // Usage (Pointer)
        0xa1, 0x00, // Collection (Physical)
        0x05, 0x09, // Usage Page (Button)
        0x19, 0x01, 0x29, 0x03, // Usage Min/Max (buttons 1-3)
        0x15, 0x00, 0x25, 0x01, // Logical Min/Max
        0x95, 0x03, 0x75, 0x01, // Count 3, Size 1
        0x81, 0x02, // Input (Data, Variable, Absolute)
        0x95, 0x01, 0x75, 0x05, // Count 1, Size 5 (padding)
        0x81, 0x03, // Input (Constant)
        0x05, 0x01, // Usage Page (Generic Desktop)
        0x09, 0x30, 0x09, 0x31, // Usage X, Y
        0x15, 0x81, 0x25, 0x7f, // Logical Min -127, Max 127
        0x75, 0x08, 0x95, 0x02, // Size 8, Count 2
        0x81, 0x06, // Input (Data, Variable, Relative)
        0xc0, 0xc0, // End Collections
    }
    return f
}

func (f *hidFunc) TypeName() string     { return "hid" }
func (f *hidFunc) InstanceName() string { return f.instance }
func (f *hidFunc) Configure(dir string) error {
    writeU := func(name string, val uint64) error {
        return os.WriteFile(fmt.Sprintf("%s/%s", dir, name),
            []byte(fmt.Sprintf("%d", val)), 0644)
    }
    if err := writeU("protocol", uint64(f.protocol)); err != nil {
        return err
    }
    if err := writeU("subclass", uint64(f.subclass)); err != nil {
        return err
    }
    if err := writeU("report_length", uint64(f.reportLen)); err != nil {
        return err
    }
    if len(f.reportDesc) > 0 {
        if err := os.WriteFile(dir+"/report_desc", f.reportDesc, 0644); err != nil {
            return err
        }
    }
    return nil
}
```

**Step 2: Ajouter WithHID dans gadget.go**

```go
func WithHID(f functions.Function) Option {
    return withFunction(f)
}
```

**Step 3: Vérifier**

```bash
go build ./usbgadget/...
```

**Step 4: Commit**

```bash
git add usbgadget/functions/hid.go usbgadget/gadget.go
git commit -m "feat(usbgadget): add HID function driver (keyboard, mouse) with standard descriptors"
```

---

### Task 7: Implémenter Mass Storage et ACM Serial

**Files:**
- Create: `usbgadget/functions/mass_storage.go`
- Create: `usbgadget/functions/acm.go`
- Modify: `usbgadget/gadget.go` (ajouter WithMassStorage, WithACMSerial)

**Step 1: Créer mass_storage.go**

```go
// usbgadget/functions/mass_storage.go
package functions

import (
    "fmt"
    "os"
)

type massStorageFunc struct {
    instance string
    file     string // chemin vers l'image disque (ex: /perm/disk.img)
    cdrom    bool
    readOnly bool
    removable bool
}

type MassStorageOption func(*massStorageFunc)

func WithCDROM(v bool) MassStorageOption {
    return func(f *massStorageFunc) { f.cdrom = v }
}

func WithReadOnly(v bool) MassStorageOption {
    return func(f *massStorageFunc) { f.readOnly = v }
}

func WithRemovable(v bool) MassStorageOption {
    return func(f *massStorageFunc) { f.removable = v }
}

// MassStorage crée une function Mass Storage.
// file est le chemin vers l'image disque (ex: /perm/disk.img).
func MassStorage(file string, opts ...MassStorageOption) Function {
    f := &massStorageFunc{
        instance:  "usb0",
        file:      file,
        removable: true,
    }
    for _, o := range opts {
        o(f)
    }
    return f
}

func (f *massStorageFunc) TypeName() string     { return "mass_storage" }
func (f *massStorageFunc) InstanceName() string { return f.instance }
func (f *massStorageFunc) Configure(dir string) error {
    boolStr := func(v bool) string {
        if v {
            return "1"
        }
        return "0"
    }
    // Configurer le LUN 0
    lun0 := fmt.Sprintf("%s/lun.0", dir)
    if err := os.MkdirAll(lun0, 0755); err != nil {
        return fmt.Errorf("mkdir lun.0: %w", err)
    }
    if err := os.WriteFile(lun0+"/file", []byte(f.file), 0644); err != nil {
        return err
    }
    if err := os.WriteFile(lun0+"/cdrom", []byte(boolStr(f.cdrom)), 0644); err != nil {
        return err
    }
    if err := os.WriteFile(lun0+"/ro", []byte(boolStr(f.readOnly)), 0644); err != nil {
        return err
    }
    if err := os.WriteFile(lun0+"/removable", []byte(boolStr(f.removable)), 0644); err != nil {
        return err
    }
    return nil
}
```

**Step 2: Créer acm.go**

```go
// usbgadget/functions/acm.go
package functions

type acmFunc struct {
    instance string
}

// ACMSerial crée une function ACM serial (port série USB).
func ACMSerial() Function { return &acmFunc{instance: "usb0"} }

func (f *acmFunc) TypeName() string     { return "acm" }
func (f *acmFunc) InstanceName() string { return f.instance }
func (f *acmFunc) Configure(_ string) error { return nil }
```

**Step 3: Ajouter les options dans gadget.go**

```go
func WithMassStorage(file string, opts ...functions.MassStorageOption) Option {
    return withFunction(functions.MassStorage(file, opts...))
}

func WithACMSerial() Option {
    return withFunction(functions.ACMSerial())
}
```

**Step 4: Vérifier**

```bash
go build ./usbgadget/...
```

**Step 5: Commit**

```bash
git add usbgadget/functions/mass_storage.go usbgadget/functions/acm.go usbgadget/gadget.go
git commit -m "feat(usbgadget): add Mass Storage and ACM Serial function drivers"
```

---

### Task 8: Implémenter MIDI

**Files:**
- Create: `usbgadget/functions/midi.go`
- Modify: `usbgadget/gadget.go` (ajouter WithMIDI)

**Step 1: Créer midi.go**

```go
// usbgadget/functions/midi.go
package functions

import (
    "fmt"
    "os"
)

type midiFunc struct {
    instance string
    bufLen   uint32
    qLen     uint32
}

// MIDI crée une function MIDI USB.
func MIDI() Function {
    return &midiFunc{
        instance: "usb0",
        bufLen:   256,
        qLen:     32,
    }
}

func (f *midiFunc) TypeName() string     { return "midi" }
func (f *midiFunc) InstanceName() string { return f.instance }
func (f *midiFunc) Configure(dir string) error {
    writeU := func(name string, val uint32) error {
        return os.WriteFile(fmt.Sprintf("%s/%s", dir, name),
            []byte(fmt.Sprintf("%d", val)), 0644)
    }
    if err := writeU("buflen", f.bufLen); err != nil {
        return err
    }
    return writeU("qlen", f.qLen)
}
```

**Step 2: Ajouter WithMIDI dans gadget.go**

```go
func WithMIDI() Option {
    return withFunction(functions.MIDI())
}
```

**Step 3: Vérifier compilation complète**

```bash
go build ./usbgadget/...
go vet ./usbgadget/...
```
Expected: 0 erreur, 0 warning

**Step 4: Commit**

```bash
git add usbgadget/functions/midi.go usbgadget/gadget.go
git commit -m "feat(usbgadget): add MIDI function driver"
```

---

### Task 9: Intégration gokrazy — OTG config et config.json

**Files:**
- Create: `usbgadget/gokrazy.go`
- Modify: `oioio/config.json`

**Step 1: Créer gokrazy.go**

```go
// usbgadget/gokrazy.go
package usbgadget

// OTGConfigContent retourne le contenu de la config OTG pour le Pi Zero 2W.
// À copier dans PackageConfig ExtraFileContents pour activer le port USB en mode peripheral.
func OTGConfigContent() string {
    return "dtoverlay=dwc2,dr_mode=peripheral\n"
}
```

**Step 2: Ajouter l'OTG config dans oioio/config.json**

Modifier `oioio/config.json` pour ajouter dans `PackageConfig["awesomeProject/hello"]` :

```json
"awesomeProject/hello": {
    "ExtraFileContents": {
        "/boot/firmware/usb-gadget.conf": "dtoverlay=dwc2,dr_mode=peripheral\n"
    }
}
```

**Note :** gokrazy lit les fichiers `*.conf` dans `/boot/firmware/` — vérifier la doc exacte de gokrazy pour l'injection dans config.txt. Alternativement, utiliser le path direct `/boot/firmware/config.txt.d/usb-gadget.txt` si supporté.

Contenu final de la section `PackageConfig` de `oioio/config.json` :

```json
"PackageConfig": {
    "github.com/gokrazy/gokrazy/cmd/randomd": {
        "ExtraFileContents": {
            "/etc/machine-id": "819d2606b0db419c9a5bdddb9ca8daf1\n"
        }
    },
    "github.com/gokrazy/wifi": {
        "ExtraFilePaths": {
            "/etc/wifi.json": "wifi.json"
        }
    },
    "github.com/gokrazy/breakglass": {
        "CommandLineFlags": ["-authorized_keys=/etc/breakglass.authorized_keys"],
        "ExtraFileContents": {
            "/etc/breakglass.authorized_keys": "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIMTRUHZxjWLlN4Vrg9CEdZCrwMvvaJ8dOpSzna/Nr/K6 oioio-pizero\n"
        }
    },
    "awesomeProject/hello": {
        "ExtraFileContents": {
            "/boot/firmware/config.txt.d/usb-gadget.txt": "dtoverlay=dwc2,dr_mode=peripheral\n"
        }
    }
}
```

**Step 3: Vérifier le JSON**

```bash
python3 -m json.tool oioio/config.json > /dev/null && echo "JSON valid"
```

**Step 4: Commit**

```bash
git add usbgadget/gokrazy.go oioio/config.json
git commit -m "feat(usbgadget): add OTG config for gokrazy (dtoverlay=dwc2,dr_mode=peripheral)"
```

---

### Task 10: Réécrire hello/main.go comme démo composite USB

**Files:**
- Modify: `hello/main.go`

**Step 1: Réécrire hello/main.go**

```go
// hello/main.go
package main

import (
    "log"
    "os"
    "os/signal"
    "syscall"

    "awesomeProject/usbgadget"
    "awesomeProject/usbgadget/functions"
)

func main() {
    log.SetFlags(0)

    g, err := usbgadget.New(
        usbgadget.WithName("geekhouse"),
        usbgadget.WithVendorID(0x1d6b, 0x0104),
        usbgadget.WithStrings("0x409", "GeekHouse", "oioio Composite", "pi0001"),
        usbgadget.WithRNDIS(),
        usbgadget.WithECM(),
        usbgadget.WithHID(functions.Keyboard()),
        usbgadget.WithMassStorage("/perm/disk.img"),
    )
    if err != nil {
        log.Fatalf("usbgadget.New: %v", err)
    }

    if err := g.Enable(); err != nil {
        log.Fatalf("gadget.Enable: %v", err)
    }
    log.Println("USB composite gadget actif : RNDIS + ECM + HID Keyboard + MassStorage")

    // Attendre SIGTERM/SIGINT pour cleanup propre
    ch := make(chan os.Signal, 1)
    signal.Notify(ch, syscall.SIGTERM, syscall.SIGINT)
    <-ch

    log.Println("Arrêt du gadget USB...")
    if err := g.Disable(); err != nil {
        log.Printf("gadget.Disable: %v", err)
    }
}
```

**Step 2: Vérifier la compilation**

```bash
cd /home/oioio/Documents/GolandProjects/awesomeProject
go build ./hello/
```
Expected: pas d'erreur

**Step 3: Nettoyer le binaire local**

```bash
rm -f hello
```

**Step 4: Build gokrazy de vérification**

```bash
GOWORK=off gok --parent_dir . -i oioio overwrite --gaf /tmp/gokrazy-oioio.gaf
```
Expected: compilation réussie pour arm64

**Step 5: Commit**

```bash
git add hello/main.go
git commit -m "feat(hello): replace hello world with USB composite gadget demo (RNDIS+ECM+HID+MassStorage)"
```

---

### Task 11: Créer l'image disque pour Mass Storage

**Files:**
- Notes sur la création de l'image (hors git)

**Step 1: Se connecter au Pi**

```bash
make ssh
```

**Step 2: Créer l'image disque**

Sur le Pi (via breakglass SSH) :

```bash
# Créer une image disque de 64MB formatée FAT32
dd if=/dev/zero of=/perm/disk.img bs=1M count=64
mkfs.fat -F32 /perm/disk.img
```
Expected: image créée dans /perm/ (persistante entre reboots)

---

### Task 12: OTA update et test sur Pi Zero 2W

**Step 1: OTA update**

```bash
make update
```
Expected: `successfully updated` sans erreur

**Step 2: Vérifier les logs**

```bash
make logs PKG=awesomeProject/hello
```
Expected: `USB composite gadget actif : RNDIS + ECM + HID Keyboard + MassStorage`

**Step 3: Brancher le Pi Zero 2W au Mac/PC via câble USB data**

Utiliser le port USB data (pas le port power-only).

**Step 4: Vérifier la détection USB**

Sur le Mac/PC connecté :
```bash
# Linux
lsusb | grep -i "Linux Foundation"
# ou macOS
system_profiler SPUSBDataType | grep -A5 "oioio"
```
Expected: le Pi apparaît comme périphérique composite USB

**Step 5: Vérifier l'interface réseau RNDIS**

Sur Windows : vérifier dans Gestionnaire de périphériques → Network Adapters → RNDIS Gadget
Sur Linux : `ip link show` → nouvelle interface `usb0` ou similaire

---

## Notes importantes

### Résolution de problèmes courants

**insmod ENOEXEC** : Les .ko ne matchent pas le kernel en cours. Vérifier `uname -r` sur le Pi.

**configfs not found** : Le mount configfs a échoué. Vérifier que `USB_GADGET=y` est bien dans le kernel gokrazy (`/proc/config.gz`).

**UDC not found** : `fe980000.usb` absent de `/sys/class/udc`. Vérifier que `dtoverlay=dwc2,dr_mode=peripheral` est dans config.txt.

**RNDIS not recognized on Windows** : Vérifier que le symlink RNDIS est bien le premier dans `configs/c.1/` (sortFunctions doit mettre rndis en priorité 0).

### Kernel modules disponibles dans gokrazy 6.12.47-v8

Le kernel gokrazy a `USB_GADGET=y` et `DWC2_DUAL_ROLE=y` built-in mais aucun function driver. Les .ko compilés dans cette tâche ajoutent :
- `libcomposite.ko` : framework composite
- `u_ether.ko` : dépendance réseau (RNDIS, ECM)
- `usb_f_rndis.ko`, `usb_f_ecm.ko`, `usb_f_ncm.ko` : réseau
- `usb_f_hid.ko` : HID
- `usb_f_mass_storage.ko` : stockage
- `usb_f_acm.ko` : serial
