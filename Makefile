PARENT_DIR := /home/oioio/Documents/GolandProjects/awesomeProject
INSTANCE   := oioio
GOK        := GOWORK=off gok --parent_dir $(PARENT_DIR) -i $(INSTANCE)
SSH_KEY    := $(HOME)/.ssh/id_ed25519
HOST       := oioio.local

# Détecte le premier périphérique amovible (carte SD)
DETECTED_SD := $(shell lsblk -d -o NAME,RM | awk '$$2=="1" {print "/dev/"$$1}' | head -1)

## flash DRIVE=/dev/sdX — Flash l'image sur la carte SD (device explicite)
flash:
	@if [ -z "$(DRIVE)" ]; then echo "Usage: make flash DRIVE=/dev/sdX"; exit 1; fi
	$(GOK) overwrite --full $(DRIVE)

## flash-auto — Détecte la carte SD et flash avec confirmation
flash-auto:
	@if [ -z "$(DETECTED_SD)" ]; then \
		echo "Aucun périphérique amovible détecté. Insère la carte SD et réessaie."; \
		exit 1; \
	fi
	@echo "Carte SD détectée : $(DETECTED_SD)"
	@lsblk -d -o NAME,SIZE,MODEL $(DETECTED_SD) 2>/dev/null || true
	@printf "Flasher gokrazy sur $(DETECTED_SD) ? [y/N] " && read confirm && [ "$$confirm" = "y" ]
	$(GOK) overwrite --full $(DETECTED_SD)

## list-sd — Liste les périphériques amovibles disponibles
list-sd:
	@echo "Périphériques amovibles (cartes SD, clés USB) :"
	@lsblk -d -o NAME,SIZE,TYPE,RM,MODEL | awk 'NR==1 || $$4=="1"'

## build — Vérifie la compilation (arm64) sans device
build:
	$(GOK) overwrite --gaf /tmp/gokrazy-$(INSTANCE).gaf

## update — Mise à jour OTA via le réseau
update:
	$(GOK) update

## ssh — Ouvre un shell SSH sur le Pi
ssh:
	ssh -i $(SSH_KEY) root@$(HOST)

## logs PKG=awesomeProject/hello — Stream les logs d'un service
logs:
	$(GOK) logs --follow $(PKG)

## find-pi — Vérifie la connectivité avec le Pi
find-pi:
	ping -c 3 $(HOST)

.PHONY: flash flash-auto list-sd build update ssh logs find-pi
