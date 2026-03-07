PARENT_DIR := /home/oioio/Documents/GolandProjects/awesomeProject
INSTANCE   := oioio
GOK        := GOWORK=off gok --parent_dir $(PARENT_DIR) -i $(INSTANCE)
SSH_KEY    := $(HOME)/.ssh/id_ed25519
HOST       := oioio.local

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

logs: ## Stream les logs d'un service  (usage: make logs PKG=awesomeProject/hello)
	$(GOK) logs --follow $(PKG)

find-pi: ## Vérifie la connectivité avec le Pi (ping)
	ping -c 3 $(HOST)

.DEFAULT_GOAL := help
.PHONY: help flash flash-auto list-sd build update ssh logs find-pi
