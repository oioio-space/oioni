# OiOni

[![Go tests](https://github.com/oioio-space/oioni/actions/workflows/test.yml/badge.svg)](https://github.com/oioio-space/oioni/actions/workflows/test.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/oioio-space/oioni.svg)](https://pkg.go.dev/github.com/oioio-space/oioni)

Firmware Go pour un **Raspberry Pi Zero 2W** tournant sous [gokrazy](https://gokrazy.org/) — un OS Linux minimal qui exécute des programmes Go directement, sans shell, sans gestionnaire de paquets, sans init system. Le Pi démarre en ~5 secondes, tous les services sont actifs dans les 170 ms qui suivent le kernel.

Le device remplit trois rôles simultanément :

- **USB composite gadget** — branché sur n'importe quelle machine hôte, il se présente comme adaptateur réseau (RNDIS pour Windows, ECM pour Linux/macOS), clavier HID, et/ou clé USB — sur un seul câble.
- **Interface locale** — un écran touch Waveshare 2.13" e-Paper HAT affiche une UI pilotée par un framework GUI sur canvas 1-bit.
- **Boîte à outils réseau** — lance des outils [impacket](https://github.com/fortra/impacket) (secretsdump, ntlmrelay, Kerberoasting, …) dans un container Podman, pilotés depuis Go.

> **Tous les usages offensifs nécessitent une autorisation explicite du propriétaire des systèmes cibles.**

---

## Hardware

| Composant | Référence |
|-----------|-----------|
| SBC | Raspberry Pi Zero 2W — quad-core ARM Cortex-A53 @ 1 GHz, 512 MB RAM |
| OS | [gokrazy](https://gokrazy.org/) — pure-Go, boot depuis SD |
| Écran | [Waveshare 2.13" Touch e-Paper HAT V4](https://www.waveshare.com/wiki/2.13inch_Touch_e-Paper_HAT) — 250×122 px N&B, SPI, touch capacitif I2C |
| USB | OTG gadget mode via BCM2835 DWC2 controller |

---

## Architecture — packages modulaires et indépendants

Chaque package est un **module Go indépendant**. Ils n'ont pas de dépendances croisées entre eux : on peut utiliser `system/wifi` sans importer `ui/gui`, `drivers/usbgadget` sans `system/netconf`, etc.

```
oioni/
├── cmd/oioni/        ← application principale (assemble tous les packages)
│
├── drivers/
│   ├── epd/          ← driver SPI display (zéro dépendance interne)
│   ├── touch/        ← driver I2C touch (zéro dépendance interne)
│   └── usbgadget/    ← gadget USB configfs (zéro dépendance interne)
│       ├── functions/ ← drivers des fonctions USB (indépendant)
│       └── ducky/    ← parser DuckyScript (indépendant)
│
├── system/
│   ├── imgvol/       ← images disque loop-mount (zéro dépendance interne)
│   ├── storage/      ← hotplug USB (dépend seulement de ses sous-packages)
│   ├── netconf/      ← configuration réseau netlink (zéro dépendance interne)
│   └── wifi/         ← gestion WiFi wpa_supplicant (zéro dépendance interne)
│
├── tools/
│   ├── containers/   ← cycle de vie Podman (zéro dépendance interne)
│   └── impacket/     ← wrappers impacket (dépend seulement de tools/containers)
│
└── ui/
    ├── canvas/       ← canvas 1-bit draw.Image (zéro dépendance interne)
    └── gui/          ← framework GUI touch (dépend seulement de ui/canvas)
```

**Point important** : `ui/gui` ne dépend **pas** de `drivers/epd` ou `drivers/touch`. Il définit ses propres interfaces (`gui.Display`, `gui.TouchPoint`, `gui.TouchEvent`). L'adaptation vers le hardware concret se fait en 5 lignes dans `cmd/oioni/epaper.go` :

```go
type epdAdapter struct{ *epd.Display }
func (a epdAdapter) Init(m gui.DisplayMode) error { return a.Display.Init(epd.Mode(m)) }
```

---

## Packages

### `drivers/epd` — Driver SPI pour écran e-ink Waveshare 2.13" V4

Driver bas niveau pour l'EPD_2in13_V4. Gère 3 modes de rafraîchissement et l'interface HAL injectable pour les tests.

```go
d, err := epd.New(epd.Config{
    SPIDevice: "/dev/spidev0.0", SPISpeed: 4_000_000,
    PinRST: 17, PinDC: 25, PinCS: 8, PinBUSY: 24,
})
d.Init(epd.ModeFull)
d.DisplayBase(buf)    // obligatoire avant les partial updates
d.DisplayPartial(buf) // ~0.3s
d.Sleep()
```

→ [drivers/epd/README.md](drivers/epd/README.md)

---

### `drivers/touch` — Driver I2C pour contrôleur touch GT1151

Détecte jusqu'à 5 points de contact simultanés. Démarre un goroutine de polling et retourne un channel d'événements.

```go
td, _ := touch.New(touch.Config{I2CDevice: "/dev/i2c-1", I2CAddr: 0x14, PinTRST: 22, PinINT: 27})
events, _ := td.Start(ctx) // <-chan touch.TouchEvent
for e := range events {
    fmt.Println(e.Points[0].X, e.Points[0].Y)
}
```

→ [drivers/touch/README.md](drivers/touch/README.md)

---

### `drivers/usbgadget` — Gadget USB composite via configfs

Configure des dispositifs USB composites (RNDIS, ECM, HID, MassStorage, Audio, Serial…) via le configfs Linux. Charge les modules kernel ARM64 embarqués automatiquement.

**Budget EP (BCM2835 DWC2, max 7 EPs hors EP0) :**

| Combinaison | EPs | Statut |
|-------------|-----|--------|
| RNDIS + ECM + HID | 3+3+1 = 7 | ✓ |
| RNDIS + MassStorage | 3+2 = 5 | ✓ |
| RNDIS + ECM + MassStorage | 3+3+2 = 8 | ✗ erreur explicite |

```go
rndis := functions.RNDIS(functions.WithRNDISHostAddr("02:00:00:aa:bb:01"))
g, _ := usbgadget.New(
    usbgadget.WithName("mygadget"),
    usbgadget.WithVendorID(0x1d6b, 0x0104),
    usbgadget.WithFunc(rndis),
)
g.Enable()
iface, _ := rndis.IfName() // "usb0"
```

**Sous-package `ducky`** — Parser DuckyScript avec layouts EN/FR :

```go
kb := functions.Keyboard()
ducky.ExecuteScript(ctx, kb, "STRING Hello\nENTER", ducky.LayoutEN)
```

→ [drivers/usbgadget/README.md](drivers/usbgadget/README.md)

---

### `system/imgvol` — Création et montage d'images disque

Crée des images disque sparse (FAT/exFAT/ext4), les loop-monte via ioctls, et expose le filesystem via `afero.Fs`. Les binaires `mkfs.*` ARM64 sont embarqués dans le binaire Go.

```go
imgvol.Create("/perm/data.img", 64<<20, imgvol.FAT)
vol, _ := imgvol.Open("/perm/data.img")
afero.WriteFile(vol.FS, "hello.txt", []byte("hi"), 0644)
vol.Close()
```

→ [system/imgvol/README.md](system/imgvol/README.md)

---

### `system/storage` — Hotplug USB et stockage persistant `/perm`

Surveille le kernel via netlink pour détecter les branchements/débranchements USB. Monte automatiquement les volumes, expose chaque volume via `afero.Fs`.

```go
sm := storage.New(
    storage.WithOnMount(func(v *storage.Volume) {
        afero.WriteFile(v.FS, "log.txt", []byte("seen"), 0644)
    }),
)
sm.Start(ctx)
```

→ [system/storage/README.md](system/storage/README.md)

---

### `system/netconf` — Configuration réseau via netlink

Applique des configurations IP (statique ou DHCP) sur des interfaces réseau via netlink. Persiste les configurations en JSON. Conçu pour gokrazy : pas de dépendance sur `ip` ou `ifconfig`.

```go
nm := netconf.New("/perm/netconf")
nm.PurgeNonWlan() // à appeler avant Start() pour virer les entrées USB gadget stales
nm.Start(ctx)
// Interface USB ECM éphémère (non persistée) :
nm.ApplyEphemeral("usb0", netconf.IfaceCfg{Mode: netconf.ModeStatic, IP: "10.42.0.1/24"})
```

→ [system/netconf/README.md](system/netconf/README.md)

---

### `system/wifi` — Gestion WiFi via wpa_supplicant

Pilote le WiFi (STA + AP simultané) via wpa_supplicant et hostapd. Charge les modules brcmfmac, gère le DHCP client en STA et le serveur DHCP en AP. Inclut les contournements pour le BCM43430 (Pi Zero 2W).

```go
mgr := wifi.New(wifi.Config{
    WpaSupplicantBin: "/user/wpa_supplicant",
    HostapdBin:       "/user/hostapd",
    IwBin:            "/user/iw",
    UdhcpcBin:        "busybox udhcpc",
    ConfDir:          "/perm/wifi",
    CtrlDir:          "/var/run/wpa_supplicant",
    Iface:            "wlan0",
})
mgr.Start(ctx)
mgr.Connect("MySSID", "passphrase", false)
mgr.SetMode(ctx, wifi.Mode{STA: true, AP: true}) // STA + AP simultané
```

Sentinelles : `wifi.ErrNotStarted`, `wifi.ErrWPATimeout`.

→ [system/wifi/README.md](system/wifi/README.md)

---

### `ui/canvas` — Canvas 1-bit pour écran e-ink

Implémente `draw.Image` sur un buffer 1-bit (1 pixel = 1 bit, MSB-first). Supporte la rotation logique (Rot90 standard pour cet écran), le texte avec polices embarquées, les rectangles, et l'export vers `image.Gray` pour les tests.

```go
c := canvas.New(122, 250, canvas.Rot90) // écran physique → logique 250×122
c.Fill(canvas.White)
c.DrawRect(image.Rect(10, 10, 100, 30), canvas.Black, false)
c.DrawText(15, 13, "Hello", canvas.EmbeddedFont(12), canvas.Black)
buf := c.Bytes() // 4000 bytes → DisplayPartial(buf)
```

→ [ui/canvas/README.md](ui/canvas/README.md)

---

### `ui/gui` — Framework GUI touch pour écran e-ink

Framework de widgets pour l'écran e-Paper 250×122. Gère la pile de scènes (Navigator), le routage des événements touch, le rafraîchissement partiel/complet automatique, et l'idle sleep.

**Zéro dépendance sur les drivers hardware** — utilise ses propres types `gui.Display`, `gui.TouchPoint`, `gui.TouchEvent`. Adaptateur vers `*epd.Display` en 2 lignes.

```go
nav := gui.NewNavigatorWithIdle(display, 60*time.Second)

scene := &gui.Scene{
    Widgets: []gui.Widget{
        gui.NewVBox(
            gui.NewLabel("Hello, e-ink!"),
            gui.NewButton("Press me", func() { log.Println("tap!") }),
        ),
    },
}
nav.Push(scene)
nav.Run(ctx, touchEvents) // bloque jusqu'à ctx.Done()
```

Widgets disponibles : Label, Button, Toggle, Checkbox, Slider, Menu, Carousel, ScrollList, TextInput, Keyboard, Image, QRCode, Clock, Arc, Alert, NetworkStatusBar, NavigationBar, SideBar, et plus.

→ [ui/gui/README.md](ui/gui/README.md)

---

### `tools/containers` — Cycle de vie container Podman

Gère un container Podman unique : chargement d'image depuis `.tar.gz`, lancement de processus, lecture stdout/stderr, envoi de signaux (contourne [Podman #19486](https://github.com/containers/podman/issues/19486)).

```go
mgr := containers.NewManager(containers.Config{
    ImageName: "oioni/impacket:arm64",
    LocalImagePath: "/usr/share/oioni/impacket-arm64.tar.gz",
})
proc, _ := mgr.Start(ctx, "dump", "/venv/bin/secretsdump", []string{"-h"})
io.Copy(os.Stdout, proc.StdoutPipe())
proc.Wait()
```

→ [tools/containers/README.md](tools/containers/README.md)

---

### `tools/impacket` — Wrappers Go pour les outils impacket

API Go typée pour les outils impacket Python. Retourne des structs Go structurés (`[]Credential`, `[]KerberosHash`, etc.) plutôt que du texte brut.

```go
imp := impacket.New()
creds, err := imp.SecretsDump(ctx, "dump1", impacket.SecretsDumpConfig{
    Target: "192.168.1.10",
    Domain: "CORP", User: "admin", Pass: "P@ssw0rd",
})
for _, c := range creds {
    fmt.Println(c.Username, c.NTHash)
}
```

→ [tools/impacket/README.md](tools/impacket/README.md)

---

## Configuration

Avant de déployer, configure tes credentials dans les deux fichiers ci-dessous. Ils sont trackés en git avec des valeurs placeholder ; les modifications locales sont masquées via `skip-worktree`.

### `oioio/wifi.json` — Credentials WiFi

```json
{ "ssid": "YOUR_WIFI_SSID", "psk": "YOUR_WIFI_PASSWORD" }
```

### `oioio/config.json` — Instance gokrazy

```json
{
  "Hostname": "oioio",
  "Update": { "Hostname": "192.168.0.33", "HTTPPassword": "CHANGE_ME" }
}
```

Pour commiter `config.json` sans exposer le vrai mot de passe :
```sh
git update-index --no-skip-worktree oioio/config.json
# Mettre HTTPPassword à "CHANGE_ME", puis :
git add oioio/config.json && git commit -m "..."
git update-index --skip-worktree oioio/config.json
```

---

## Déploiement

```sh
# OTA via WiFi — workflow normal
GOWORK=off gok update --parent_dir . -i oioio

# Flash carte SD — première fois ou Pi inaccessible
sudo setfacl -m u:$USER:rw /dev/sdX
GOWORK=off gok --parent_dir . -i oioio overwrite --full /dev/sdX
```

> `GOWORK=off` est obligatoire : gok utilise `-mod=mod` en interne, incompatible avec `go.work`.

### Livrer l'image impacket

```sh
# Build image ARM64
podman build --platform linux/arm64 -t oioni/impacket:arm64 tools/impacket/

# Exporter (~40 MB compressé)
podman save oioni/impacket:arm64 | gzip > /tmp/impacket-arm64.tar.gz
```

Puis dans `oioio/config.json` :
```json
"ExtraFilePaths": {
  "/usr/share/oioni/impacket-arm64.tar.gz": "/tmp/impacket-arm64.tar.gz"
}
```

Premier boot : chargement ~75 s (écrit dans `/perm/var`, persistant). Boots suivants : ~6 s.

---

## Développement

```sh
# Tous les tests — aucun hardware requis
go test ./...

# Cross-compilation ARM64
cd cmd/oioni && GOOS=linux GOARCH=arm64 go build .

# Lire les logs en direct depuis la Pi
curl -u 'gokrazy:PASSWORD' 'http://192.168.0.33/log?path=/user/oioni&stream=stderr'
```

Les tests nécessitant root ou hardware réel portent `//go:build ignore`.

---

## Performances (mesurées sur device)

| Métrique | Valeur |
|----------|--------|
| Boot → services actifs | ~5 s |
| OTA via WiFi | ~85 s (139 MB rootfs + 69 MB bootfs + reboot) |
| Impacket — premier run (chargement image) | ~75 s |
| Impacket — runs suivants (image cachée) | ~6 s |
| RAM au repos | ~87 MB / 402 MB total |

---

## Licence

MIT — voir [LICENSE](LICENSE).
