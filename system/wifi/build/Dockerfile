# system/wifi/build/Dockerfile — builds static ARM64 binaries: wpa_supplicant, hostapd, iw
#
# Requires these files in the build context (system/wifi/build/):
#   wpa_2.11.tar.xz   — Debian upstream tarball (contains wpa_supplicant + hostapd)
#   iw-6.9.tar.gz     — from https://www.kernel.org/pub/software/network/iw/
#
# Build:
#   podman build --platform linux/arm64 --output type=local,dest=system/wifi/bin system/wifi/build/
FROM --platform=linux/arm64 alpine:3.21 AS builder

RUN apk add --no-cache \
    build-base \
    openssl-dev \
    openssl-libs-static \
    libnl3-dev \
    libnl3-static \
    dbus-dev \
    linux-headers \
    xz

# Debian's wpa_2.11.orig.tar.xz extracts to wpa-2.11/ containing both
# wpa_supplicant/ and hostapd/ subdirectories.
COPY wpa_2.11.tar.xz /
RUN tar xf /wpa_2.11.tar.xz

# ── wpa_supplicant ────────────────────────────────────────────────────────────
WORKDIR /wpa-2.11/wpa_supplicant
COPY defconfig .config
RUN make -j$(nproc) LDFLAGS="-static" CC="gcc" \
    && strip wpa_supplicant

# ── hostapd ───────────────────────────────────────────────────────────────────
WORKDIR /wpa-2.11/hostapd
COPY hostapd_defconfig .config
RUN make -j$(nproc) LDFLAGS="-static" CC="gcc" \
    && strip hostapd

# ── iw ────────────────────────────────────────────────────────────────────────
COPY iw-6.9.tar.gz /
RUN tar xf /iw-6.9.tar.gz -C /

WORKDIR /iw-6.9
RUN make -j$(nproc) \
        CFLAGS="-static -I/usr/include/libnl3" \
        LDFLAGS="-static" \
        NL3_FOUND=y \
        LIBS="-lnl-3 -lnl-genl-3 -lm" \
    && strip iw

# ── collect outputs ───────────────────────────────────────────────────────────
FROM scratch
COPY --from=builder /wpa-2.11/wpa_supplicant/wpa_supplicant /wpa_supplicant
COPY --from=builder /wpa-2.11/hostapd/hostapd               /hostapd
COPY --from=builder /iw-6.9/iw                               /iw
