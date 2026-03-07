PARENT_DIR := /home/oioio/Documents/GolandProjects/awesomeProject
INSTANCE   := oioio
GOK        := GOWORK=off gok --parent_dir $(PARENT_DIR) -i $(INSTANCE)
SSH_KEY    := $(HOME)/.ssh/id_ed25519
HOST       := oioio.local

## flash DRIVE=/dev/sdX — Flash l'image sur la carte SD
flash:
	@if [ -z "$(DRIVE)" ]; then echo "Usage: make flash DRIVE=/dev/sdX"; exit 1; fi
	$(GOK) overwrite --target_drive $(DRIVE)

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

.PHONY: flash build update ssh logs find-pi
