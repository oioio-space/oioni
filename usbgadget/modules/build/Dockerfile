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

# Config minimale pour compiler uniquement les modules USB gadget
RUN make ARCH=${ARCH} CROSS_COMPILE=${CROSS_COMPILE} bcm2711_defconfig

# Active les modules USB gadget nécessaires
RUN scripts/config \
    --module CONFIG_USB_GADGET \
    --module CONFIG_USB_LIBCOMPOSITE \
    --module CONFIG_USB_ETH \
    --module CONFIG_USB_ETH_RNDIS \
    --module CONFIG_USB_ETH_EEM \
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
    --set-val CONFIG_LOCALVERSION "" \
    --disable CONFIG_LOCALVERSION_AUTO

# Prépare les headers
RUN make ARCH=${ARCH} CROSS_COMPILE=${CROSS_COMPILE} \
    -j$(nproc) modules_prepare

# Compile uniquement les modules USB gadget
# KBUILD_MODPOST_WARN=1 : les symboles kernel (strcpy, kfree...) ne sont pas dans
# Module.symvers lors d'un build partiel, mais ils sont bien présents dans le kernel
# gokrazy au runtime — on accepte les warnings modpost.
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
