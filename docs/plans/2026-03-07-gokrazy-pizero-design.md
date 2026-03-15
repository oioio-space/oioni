# Gokrazy Appliance — Raspberry Pi Zero 2 W

Date: 2026-03-07

## Objectif

Créer une appliance gokrazy pour Raspberry Pi Zero 2 W, intégrée dans le workspace Go du projet, avec WiFi, SSH et OTA.

## Structure du projet

```
awesomeProject/
├── go.work              # workspace Go (lie les modules)
├── go.mod               # module existant (awesomeProject)
├── hello/               # programme Hello World de test
│   └── main.go
└── oioio/               # instance gokrazy (hostname: oioio)
    ├── go.mod
    ├── config.json
    └── wifi.json
```

## Packages gokrazy

| Package | Rôle |
|---|---|
| `github.com/gokrazy/wifi` | Connexion WiFi automatique |
| `github.com/gokrazy/breakglass` | Accès SSH |
| `github.com/gokrazy/fbstatus` | Status sur écran HDMI |
| `awesomeProject/hello` | Hello World de test |

## Configuration

- **Hostname**: `oioio`
- **WiFi SSID**: `freebox_GeekHouse`
- **Architecture**: `arm64` (Pi Zero 2 W)
- **OTA**: via `gok update` une fois sur le réseau
- **SSH**: breakglass sur port 22

## Commandes clés

```bash
# Flash initial sur carte SD
gok overwrite --parent_dir . -i oioio --target_drive /dev/sdX

# Mise à jour OTA
gok update --parent_dir . -i oioio
```

## Etapes suivantes (hors scope)

- Ajout de programmes custom au workspace
- Configuration avancée (services, cron, etc.)
