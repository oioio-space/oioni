PARENT_DIR := /home/oioio/Documents/GolandProjects/oioni
INSTANCE   := oioio
GOK        := GOWORK=off gok --parent_dir $(PARENT_DIR) -i $(INSTANCE)
SSH_KEY    := $(HOME)/.ssh/id_ed25519
HOST       := 192.168.0.33

# Détecte le premier disque amovible via /dev/disk/by-id (méthode gokrazy)
DETECTED_SD := $(shell ls -l /dev/disk/by-id/ 2>/dev/null \
	| awk '/(usb-|mmc-)/ && !/-part[0-9]/ {print $$NF}' \
	| sed 's|../../||' \
	| head -1 \
	| xargs -I{} echo /dev/{})

help: ## Affiche cette aide
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z_-]+:.*##/ {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

list-sd: ## Liste les cartes SD/USB via /dev/disk/by-id
	@echo "Périphériques amovibles détectés (/dev/disk/by-id) :"
	@ls -l /dev/disk/by-id/ | awk '/(usb-|mmc-)/ && !/-part[0-9]/ {print "  " $$NF " -> " $$(NF-2)}' | sed 's|../../||'

flash-auto: ## Détecte la carte SD automatiquement et flash avec confirmation
	@if [ -z "$(DETECTED_SD)" ]; then \
		echo "Aucune carte SD détectée dans /dev/disk/by-id/"; \
		echo "Insère la carte SD et surveille avec :"; \
		echo "  watch -d1 ls -l /dev/disk/by-id/"; \
		exit 1; \
	fi
	@echo "Carte SD détectée : $(DETECTED_SD)"
	@lsblk -d -o NAME,SIZE,MODEL $(DETECTED_SD) 2>/dev/null || true
	@printf "Flasher gokrazy sur $(DETECTED_SD) ? [y/N] " && read confirm && [ "$$confirm" = "y" ]
	@echo "Démontage des partitions de $(DETECTED_SD)..."
	@sudo umount $(DETECTED_SD)p* $(DETECTED_SD)[0-9]* 2>/dev/null || true
	$(GOK) overwrite --full $(DETECTED_SD)

flash: ## Flash sur un device explicite  (usage: make flash DRIVE=/dev/sdX)
	@if [ -z "$(DRIVE)" ]; then echo "Usage: make flash DRIVE=/dev/sdX"; exit 1; fi
	@echo "Démontage des partitions de $(DRIVE)..."
	@sudo umount $(DRIVE)p* $(DRIVE)[0-9]* 2>/dev/null || true
	$(GOK) overwrite --full $(DRIVE)

build: ## Vérifie la compilation arm64 sans device physique
	$(GOK) overwrite --gaf /tmp/gokrazy-$(INSTANCE).gaf

update: ## Mise à jour OTA via le réseau
	$(GOK) update

ssh: ## Ouvre un shell SSH sur le Pi
	ssh -i $(SSH_KEY) root@$(HOST)

logs: ## Stream les logs d'un service  (usage: make logs PKG=github.com/oioio-space/oioni/cmd/oioni)
	$(GOK) logs --follow $(PKG)

find-pi: ## Vérifie la connectivité avec le Pi (ping)
	ping -c 3 $(HOST)

test: ## Run unit tests across all modules
	cd drivers/epd     && go test ./...
	cd drivers/touch   && go test ./...
	cd drivers/usbgadget && go test ./...
	cd system/storage  && go test ./...
	cd system/netconf  && go test ./...
	cd system/wifi     && go test ./...
	cd ui/canvas       && go test ./...
	cd ui/gui          && go test ./...
	cd cmd/oioni       && go test ./ui/...

build-modules: ## Compile les modules kernel ARM64 (USB gadget) pour usbgadget
	podman build --platform linux/arm64 \
	    --output type=local,dest=drivers/usbgadget/modules/build/out \
	    drivers/usbgadget/modules/build/
	@echo "Modules générés :"
	@ls -lh drivers/usbgadget/modules/build/out/6.12.47-v8/*.ko 2>/dev/null || echo "(aucun .ko trouvé)"
	@cp drivers/usbgadget/modules/build/out/6.12.47-v8/*.ko drivers/usbgadget/modules/6.12.47-v8/

build-imgvol-bins: ## Compile les binaires mkfs statiques ARM64 pour imgvol
	podman build --platform linux/arm64 \
	    --output type=local,dest=system/imgvol/bin \
	    system/imgvol/build/
	@echo "Binaires générés dans system/imgvol/bin/ :"
	@ls -lh system/imgvol/bin/mkfs.* 2>/dev/null || echo "(aucun binaire trouvé — vérifier le Dockerfile)"
	@file system/imgvol/bin/mkfs.* 2>/dev/null || true

build-wifi-bins: ## Compile wpa_supplicant static ARM64 binary for system/wifi
	podman build --platform linux/arm64 \
	    --output type=local,dest=system/wifi/bin \
	    system/wifi/build/
	@echo "Binary generated in system/wifi/bin/:"
	@ls -lh system/wifi/bin/wpa_supplicant 2>/dev/null || echo "(not found -- check Dockerfile)"
	@file system/wifi/bin/wpa_supplicant 2>/dev/null || true

build-wifi-ap-bins: ## Compile hostapd, iw, ip static ARM64 binaries for AP mode
	podman build --platform linux/arm64 \
	    --output type=local,dest=system/wifi/bin \
	    system/wifi/bin/
	@echo "AP binaries generated in system/wifi/bin/:"
	@ls -lh system/wifi/bin/hostapd system/wifi/bin/iw system/wifi/bin/ip 2>/dev/null \
	    || echo "(not found -- check system/wifi/bin/Dockerfile)"

build-all: build-modules build-imgvol-bins build-wifi-bins build-wifi-ap-bins build ## Build all static bins then verify gokrazy compilation

.DEFAULT_GOAL := help
.PHONY: help flash flash-auto list-sd build update ssh logs find-pi test build-modules build-imgvol-bins build-wifi-bins build-wifi-ap-bins build-all
