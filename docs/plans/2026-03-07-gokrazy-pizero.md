# Gokrazy Pi Zero 2W Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Déployer une appliance gokrazy sur Raspberry Pi Zero 2 W avec WiFi, SSH (breakglass) et OTA, dans le workspace Go du projet.

**Architecture:** Instance gokrazy `oioio/` dans le projet awesomeProject, liée via `go.work`. Le programme `hello/` est dans le module racine et déployé sur le Pi. Flash initial via `gok overwrite`, mises à jour via `gok update`.

**Tech Stack:** Go 1.26, gokrazy (`gok`), go workspaces, breakglass (SSH), github.com/gokrazy/wifi

---

### Task 1: Générer une clé SSH

**Files:**
- Créé par la commande: `~/.ssh/id_ed25519` et `~/.ssh/id_ed25519.pub`

**Step 1: Générer la clé**

```bash
ssh-keygen -t ed25519 -f ~/.ssh/id_ed25519 -N "" -C "oioio-pizero"
```
Expected: deux fichiers créés dans `~/.ssh/`

**Step 2: Vérifier**

```bash
cat ~/.ssh/id_ed25519.pub
```
Expected: une ligne commençant par `ssh-ed25519 AAAA...`

---

### Task 2: Créer le programme Hello World

**Files:**
- Create: `hello/main.go`

**Step 1: Créer le dossier et le fichier**

```go
// hello/main.go
package main

import (
	"fmt"
	"log"
	"time"
)

func main() {
	log.SetFlags(0)
	for {
		fmt.Println("Hello from Pi Zero 2W - oioio!")
		time.Sleep(10 * time.Second)
	}
}
```

**Step 2: Vérifier la compilation**

```bash
cd /home/oioio/Documents/GolandProjects/awesomeProject
go build ./hello/
```
Expected: pas d'erreur, binaire `hello` créé localement

**Step 3: Nettoyer le binaire local**

```bash
rm -f hello
```

**Step 4: Commit**

```bash
git add hello/main.go
git commit -m "feat: add hello world program for gokrazy deployment"
```

---

### Task 3: Initialiser l'instance gokrazy `oioio`

**Files:**
- Create: `oioio/config.json` (généré par gok)
- Create: `oioio/go.mod` (généré par gok)

**Step 1: Créer l'instance dans le projet**

```bash
cd /home/oioio/Documents/GolandProjects/awesomeProject
gok new --parent_dir . -i oioio --empty
```
Expected: dossier `oioio/` créé avec `config.json` et `go.mod`

**Step 2: Vérifier la structure**

```bash
ls oioio/
cat oioio/config.json
```
Expected: `config.json` avec un JSON valide, `go.mod` avec module gokrazy

---

### Task 4: Configurer wifi.json

**Files:**
- Create: `oioio/wifi.json`

**Step 1: Créer le fichier**

```json
{
    "ssid": "freebox_GeekHouse",
    "psk": "@rthur1709.30!"
}
```

**Step 2: Vérifier le JSON**

```bash
python3 -m json.tool oioio/wifi.json
```
Expected: JSON reformaté sans erreur

---

### Task 5: Configurer config.json

**Files:**
- Modify: `oioio/config.json`

**Step 1: Remplacer config.json avec la config complète**

Récupérer d'abord la clé SSH publique :
```bash
cat ~/.ssh/id_ed25519.pub
```

Puis écrire `oioio/config.json` (remplacer `<SSH_PUB_KEY>` par la valeur ci-dessus) :

```json
{
    "Hostname": "oioio",
    "Update": {
        "HTTPPassword": "wxcvbn.000"
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
        "awesomeProject/hello"
    ],
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
            "ExtraFileContents": {
                "/etc/breakglass.authorized_keys": "<SSH_PUB_KEY>\n"
            }
        }
    },
    "SerialConsole": "disabled"
}
```

**Step 2: Vérifier le JSON**

```bash
python3 -m json.tool oioio/config.json
```
Expected: pas d'erreur

---

### Task 6: Créer le go.work et ajouter les dépendances gokrazy

**Files:**
- Create: `go.work`

**Step 1: Initialiser le workspace**

```bash
cd /home/oioio/Documents/GolandProjects/awesomeProject
go work init . ./oioio
```
Expected: `go.work` créé avec les deux modules

**Step 2: Ajouter les packages gokrazy à l'instance**

```bash
cd /home/oioio/Documents/GolandProjects/awesomeProject
gok add --parent_dir . -i oioio github.com/gokrazy/fbstatus
gok add --parent_dir . -i oioio github.com/gokrazy/serial-busybox
gok add --parent_dir . -i oioio github.com/gokrazy/breakglass
gok add --parent_dir . -i oioio github.com/gokrazy/wifi
```
Expected: chaque commande met à jour `oioio/go.mod`

**Step 3: Vérifier go.work**

```bash
cat go.work
```
Expected: contient `use ( . ./oioio )`

---

### Task 7: Build de vérification

**Step 1: Tenter un build gokrazy**

```bash
cd /home/oioio/Documents/GolandProjects/awesomeProject
gok build --parent_dir . -i oioio
```
Expected: compilation réussie pour arm64, pas d'erreur

**Step 2: Si erreur sur `awesomeProject/hello`** — vérifier que le module racine est bien dans go.work :

```bash
cat go.work
# doit contenir: use ( . ./oioio )
```

---

### Task 8: Créer le Makefile

**Files:**
- Create: `Makefile`

Le Makefile automatise toutes les étapes manuelles répétitives (flash, OTA, SSH, logs, build).

```makefile
PARENT_DIR := /home/oioio/Documents/GolandProjects/awesomeProject
INSTANCE   := oioio
GOK        := gok --parent_dir $(PARENT_DIR) -i $(INSTANCE)
SSH_KEY    := $(HOME)/.ssh/id_ed25519
HOST       := oioio.local

## flash DRIVE=/dev/sdX — Flash l'image sur la carte SD
flash:
	@if [ -z "$(DRIVE)" ]; then echo "Usage: make flash DRIVE=/dev/sdX"; exit 1; fi
	$(GOK) overwrite --target_drive $(DRIVE)

## update — Mise à jour OTA via le réseau
update:
	$(GOK) update

## build — Vérification de la compilation (arm64)
build:
	$(GOK) build

## ssh — Ouvre un shell SSH sur le Pi
ssh:
	ssh -i $(SSH_KEY) root@$(HOST)

## logs PKG=awesomeProject/hello — Stream les logs d'un service
logs:
	$(GOK) logs --follow $(PKG)

## find-pi — Trouve l'IP du Pi sur le réseau
find-pi:
	ping -c 3 $(HOST)

.PHONY: flash update build ssh logs find-pi
```

**Step 1: Créer le Makefile à la racine du projet**

(Copier le contenu ci-dessus dans `Makefile`)

**Step 2: Vérifier la syntaxe**

```bash
make --dry-run build
```
Expected: affiche la commande `gok build ...` sans l'exécuter

**Step 3: Commit**

```bash
git add Makefile
git commit -m "chore: add Makefile for gokrazy operations"
```

---

### Task 9: Commit et flash sur carte SD

**Step 1: Commit de tout**

```bash
cd /home/oioio/Documents/GolandProjects/awesomeProject
git add oioio/ go.work
git commit -m "feat: add gokrazy oioio instance with wifi, ssh and hello world"
```

**Step 2: Identifier la carte SD**

```bash
lsblk
```
Repérer le device de la carte SD (ex: `/dev/sda`, `/dev/mmcblk0`).
**ATTENTION: ne pas confondre avec le disque système.**

**Step 3: Flash via Makefile**

```bash
make flash DRIVE=/dev/sdX
```
Remplacer `/dev/sdX` par le bon device. Sudo sera demandé.

Expected: message `successfully wrote ...` sans erreur

---

### Task 10: Premier boot et vérification SSH

**Step 1: Insérer la SD dans le Pi Zero 2 W et démarrer**

Attendre ~60 secondes le premier boot.

**Step 2: Trouver l'IP du Pi**

```bash
make find-pi
```
Ou chercher `oioio` dans les appareils connectés sur ton routeur freebox.

**Step 3: Tester SSH**

```bash
make ssh
```
Expected: shell root sur le Pi

**Step 4: Vérifier les logs hello**

```bash
make logs PKG=awesomeProject/hello
```
Expected: "Hello from Pi Zero 2W - oioio!" toutes les 10 secondes
