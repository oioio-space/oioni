# usbgadget/modules/build/Dockerfile
# Cross-compile USB gadget kernel modules pour arm64 / kernel 6.12.47-v8
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential bc flex bison libssl-dev libelf-dev \
    gcc-aarch64-linux-gnu binutils-aarch64-linux-gnu \
    wget git ca-certificates \
    && rm -rf /var/lib/apt/lists/*

ARG KVER=6.12.47-v8
ARG ARCH=arm64
ARG CROSS_COMPILE=aarch64-linux-gnu-

WORKDIR /build

# Télécharge les sources du kernel Raspberry Pi — tag exact utilisé par gokrazy kernel.rpi
RUN wget -q https://github.com/raspberrypi/linux/archive/refs/tags/stable_20250916.tar.gz \
    && tar xf stable_20250916.tar.gz \
    && mv linux-stable_20250916 kernel \
    && rm stable_20250916.tar.gz

WORKDIR /build/kernel

# Reconstruit le .config exactement comme gokrazy :
#   1. bcm2711_defconfig (base Raspberry Pi arm64)
#   2. config.addendum.txt (ajouts gokrazy)
#   3. olddefconfig (résout les dépendances)
# Cette séquence produit le même config que le kernel.rpi package.
RUN make ARCH=${ARCH} CROSS_COMPILE=${CROSS_COMPILE} bcm2711_defconfig

COPY config.addendum.txt /tmp/config.addendum.txt
RUN cat /tmp/config.addendum.txt >> .config && \
    make ARCH=${ARCH} CROSS_COMPILE=${CROSS_COMPILE} olddefconfig

# Active les modules USB gadget (gokrazy a USB_GADGET=y built-in, on passe en =m)
RUN scripts/config \
    --module CONFIG_USB_GADGET \
    --module CONFIG_USB_LIBCOMPOSITE \
    --module CONFIG_USB_ETH \
    --module CONFIG_USB_G_NCM \
    --module CONFIG_USB_F_HID \
    --module CONFIG_USB_MASS_STORAGE \
    --module CONFIG_USB_G_SERIAL \
    --module CONFIG_USB_F_MIDI \
    --module CONFIG_USB_U_ETHER \
    --module CONFIG_USB_F_RNDIS \
    --module CONFIG_USB_F_ECM \
    --module CONFIG_USB_F_MASS_STORAGE \
    --module CONFIG_USB_F_ACM \
    --module CONFIG_USB_U_SERIAL \
    --disable CONFIG_LOCALVERSION_AUTO

# Prépare les headers (syncconfig → autoconf.h)
RUN make ARCH=${ARCH} CROSS_COMPILE=${CROSS_COMPILE} \
    -j$(nproc) modules_prepare

# Diagnostic : vérifier que l'offset exit dans struct module est 0x4a8
RUN grep -E "DYNAMIC_DEBUG|FTRACE|TRACING|KPROBES|BPF_EVENT|JUMP_LABEL|KALLSYMS|EVENT_TRACING" \
    include/generated/autoconf.h | sort

# Compile uniquement les modules USB gadget
# KBUILD_MODPOST_WARN=1 : les symboles kernel ne sont pas dans Module.symvers
# lors d'un build partiel, mais présents dans le kernel gokrazy au runtime.
RUN make ARCH=${ARCH} CROSS_COMPILE=${CROSS_COMPILE} \
    -j$(nproc) \
    KBUILD_MODPOST_WARN=1 \
    modules \
    M=drivers/usb/gadget

# Collecte les .ko
RUN mkdir -p /out/${KVER} && \
    find . -name "*.ko" \
        \( -name "libcomposite.ko" \
        -o -name "u_ether.ko" \
        -o -name "usb_f_rndis.ko" \
        -o -name "usb_f_ecm.ko" \
        -o -name "usb_f_ncm.ko" \
        -o -name "usb_f_hid.ko" \
        -o -name "usb_f_mass_storage.ko" \
        -o -name "usb_f_acm.ko" \
        -o -name "u_serial.ko" \
        -o -name "usb_f_midi.ko" \
        \) \
    -exec cp {} /out/${KVER}/ \;

CMD ["ls", "-la", "/out/"]
