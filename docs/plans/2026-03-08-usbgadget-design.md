# USB Gadget Package — Design Document

Date: 2026-03-08

## Contexte

- **Cible** : Raspberry Pi Zero 2W avec gokrazy
- **Kernel** : 6.12.47-v8 (arm64)
- **Contrainte kernel** : Le kernel gokrazy a `USB_GADGET=y` et `DWC2_DUAL_ROLE=y` built-in, mais AUCUN function driver compilé (pas de libcomposite, RNDIS, HID, ECM, configfs...).
- **UDC** : `fe980000.usb` (DWC2 OTG, port USB data du Pi Zero 2W)
- **OTG** : Nécessite `dtoverlay=dwc2,dr_mode=peripheral` dans config.txt
- **Référence** : P4wnP1_aloa (https://github.com/RoganDawes/P4wnP1_aloa) — même problématique sous Raspbian

## Objectif

Package Go `awesomeProject/usbgadget` permettant de créer et gérer des **composite USB gadgets** sur le Pi Zero 2W via libcomposite/configfs, avec :
- API haut niveau (functional options pattern)
- Gestion automatique des priorités (RNDIS en premier pour Windows)
- Support de tous les types USB standards
- Compatibilité totale gokrazy (pas de CGO, ExtraFiles pour OTG config)
- Programme `hello/` comme démo composite complète

---

## Architecture

### Structure du workspace

```
awesomeProject/
├── go.work
├── go.mod                    ← module awesomeProject
├── usbgadget/                ← package principal
│   ├── gadget.go             ← type Gadget, New(), Enable(), Disable()
│   ├── configfs.go           ← écriture bas niveau /sys/kernel/config/
│   ├── modules.go            ← chargement .ko embarqués (insmod)
│   ├── udc.go                ← bind/unbind UDC
│   ├── priority.go           ← tri des functions (RNDIS en 1er)
│   ├── gokrazy.go            ← ExtraFiles() pour config.txt OTG
│   └── functions/
│       ├── function.go       ← interface Function
│       ├── rndis.go          ← RNDIS (Windows réseau)
│       ├── ecm.go            ← ECM (Linux/macOS réseau)
│       ├── ncm.go            ← NCM (réseau haute vitesse)
│       ├── hid.go            ← HID (clavier, souris, gamepad)
│       ├── mass_storage.go   ← Mass Storage (clé USB, CD-ROM)
│       ├── acm.go            ← ACM Serial
│       └── midi.go           ← MIDI
├── usbgadget/modules/        ← .ko cross-compilés arm64 (go:embed)
│   ├── build/
│   │   └── Dockerfile        ← cross-compilation one-time
│   ├── embed.go              ← //go:embed 6.12.47-v8/*.ko
│   ├── modules.go            ← API: Load(), Unload()
│   └── 6.12.47-v8/
│       ├── libcomposite.ko
│       ├── u_ether.ko        ← dépendance rndis+ecm
│       ├── usb_f_rndis.ko
│       ├── usb_f_ecm.ko
│       ├── usb_f_ncm.ko
│       ├── usb_f_hid.ko
│       ├── usb_f_mass_storage.ko
│       └── usb_f_acm.ko
└── hello/
    └── main.go               ← démo : RNDIS+ECM+HID+MassStorage
```

---

## API publique (functional options pattern)

```go
// Création d'un gadget composite
g, err := usbgadget.New(
    usbgadget.WithName("g1"),
    usbgadget.WithVendorID(0x1d6b, 0x0104),
    usbgadget.WithStrings("fr", "Manufacturer", "Product", "SerialNumber"),
    usbgadget.WithUSBVersion(2, 0),
    usbgadget.WithRNDIS(),                          // auto-priorité Windows (interface 0-1)
    usbgadget.WithECM(),
    usbgadget.WithHID(functions.Keyboard()),
    usbgadget.WithHID(functions.Mouse()),
    usbgadget.WithMassStorage("/perm/disk.img",
        functions.WithCDROM(true),
        functions.WithReadOnly(true),
    ),
    usbgadget.WithACMSerial(),
    usbgadget.WithOrder(usbgadget.RNDISFirst),      // override priorité si besoin
)

err = g.Enable()    // insmod, configfs, bind UDC
err = g.Disable()   // unbind, nettoyage configfs
err = g.Reload()    // disable + enable (hot-reload)
```

---

## Gestion des priorités (Windows)

Windows lit les interfaces USB dans l'ordre de création des symlinks dans `configs/c.1/`.
**RNDIS doit être interface 0** (avant ECM, HID, etc.) pour être reconnu correctement.

Ordre appliqué automatiquement par le package :
1. RNDIS (si présent) — toujours premier
2. Réseau (ECM, NCM)
3. HID (keyboard, mouse, gamepad)
4. Mass Storage
5. ACM Serial
6. MIDI

Overridable via `usbgadget.WithOrder(...)` pour cas avancés.

---

## Kernel modules (approche B choisie)

### Cross-compilation (one-time build)

Un `Dockerfile` dans `usbgadget/modules/build/` cross-compile les .ko nécessaires
contre les headers du kernel 6.12.47-v8 fournis par le module gokrazy `kernel.rpi`.

```bash
# Build one-time (dans modules/build/)
make modules KVER=6.12.47-v8
```

Les .ko compilés sont committés dans `usbgadget/modules/6.12.47-v8/` et embarqués
via `//go:embed`.

### Chargement runtime

```go
// modules.go
func Load() error {
    kver := kernelVersion() // uname -r
    return loadEmbedded(kver, []string{
        "libcomposite",
        "u_ether",
        "usb_f_rndis",
        // ...
    })
}
```

---

## Intégration gokrazy

### OTG config (config.txt)

```go
// gokrazy.go — appelé depuis config.json PackageConfig ExtraFileContents
func ExtraFiles() map[string]string {
    return map[string]string{
        "/boot/firmware/config.txt.d/usb-gadget.txt": "dtoverlay=dwc2,dr_mode=peripheral\n",
    }
}
```

En pratique : ajout dans `oioio/config.json` PackageConfig ExtraFileContents pour le programme hello.

### config.json (extrait)

```json
"PackageConfig": {
    "awesomeProject/hello": {
        "ExtraFileContents": {
            "/boot/firmware/usb-gadget.conf": "dtoverlay=dwc2,dr_mode=peripheral\n"
        }
    }
}
```

---

## Programme hello (démo composite)

```go
// hello/main.go — remplace le hello world actuel
func main() {
    log.SetFlags(0)

    g, err := usbgadget.New(
        usbgadget.WithName("geekhouse"),
        usbgadget.WithVendorID(0x1d6b, 0x0104),
        usbgadget.WithStrings("fr", "GeekHouse", "oioio Composite", "pi0001"),
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
    log.Println("USB composite gadget actif : RNDIS + ECM + HID + MassStorage")

    // gokrazy gère le cycle de vie — on attend indéfiniment
    select {}
}
```

---

## Dépendances externes

- Aucune (CGO_ENABLED=0, compatible gokrazy)
- Kernel 6.12.47-v8 sur le Pi (fourni par gokrazy kernel.rpi)
- Docker pour le build one-time des .ko (sur la machine de dev)

---

## Étapes d'implémentation (ordre)

1. Cross-compile les .ko USB gadget (Docker, arm64, kernel 6.12.47-v8)
2. Package `usbgadget/modules/` avec go:embed
3. Package `usbgadget/` — interface Function + configfs bas niveau
4. Functions : RNDIS, ECM, HID, MassStorage, ACM, NCM, MIDI
5. Gadget composite : priorités, Enable/Disable, UDC bind
6. Intégration gokrazy (ExtraFiles OTG config)
7. Programme hello — démo composite complète
8. OTA update + test sur Pi Zero 2W
