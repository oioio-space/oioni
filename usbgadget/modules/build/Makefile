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
