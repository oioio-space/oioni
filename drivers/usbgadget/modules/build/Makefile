# usbgadget/modules/build/Makefile
KVER              ?= 6.12.47-v8
OUTDIR            := ../$(KVER)
IMAGE             := usbgadget-modules-builder
CONTAINER_RUNTIME ?= $(shell which podman 2>/dev/null || echo docker)

build:
	$(CONTAINER_RUNTIME) build \
		--build-arg KVER=$(KVER) \
		-t $(IMAGE):$(KVER) \
		-f Dockerfile .
	mkdir -p $(OUTDIR)
	$(CONTAINER_RUNTIME) run --rm \
		$(IMAGE):$(KVER) \
		tar -C /out/$(KVER) -cf - . \
		| tar -C $(abspath $(OUTDIR)) -xf -
	@echo "Modules copiés dans $(OUTDIR):"
	@ls -la $(OUTDIR)/*.ko 2>/dev/null || echo "ATTENTION: aucun .ko trouvé"

.PHONY: build
