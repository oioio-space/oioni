# system/wifi/bin/Dockerfile — builds static ARM64 binaries: hostapd, iw, ip
#
# Usage (from system/wifi/):
#   podman build --platform linux/arm64 -t oioni/wifi-bins system/wifi/bin/
#   podman run --rm -v $(pwd)/system/wifi/bin:/out oioni/wifi-bins \
#       sh -c 'cp /build/out/hostapd /build/out/iw /build/out/ip /out/'
#
# Or via Makefile target: make build-wifi-bins
#
# Output binaries land in system/wifi/bin/:
#   hostapd   — from hostap project (WPA Supplicant / hostapd)
#   iw        — from iw package (nl80211 config tool)
#   ip        — from iproute2 (IP management)

FROM --platform=linux/arm64 alpine:3.21 AS builder

RUN apk add --no-cache \
    build-base \
    linux-headers \
    libnl3-dev \
    libnl3-static \
    openssl-dev \
    openssl-libs-static \
    libnl3 \
    git \
    pkgconf

WORKDIR /build

# ── iw ────────────────────────────────────────────────────────────────────────
# iw: nl80211 configuration tool
RUN apk add --no-cache libnl3-dev libnl3-static
RUN wget -q https://www.kernel.org/pub/software/network/iw/iw-6.9.tar.gz \
    && tar xzf iw-6.9.tar.gz
RUN cd iw-6.9 \
    && make -j$(nproc) \
        CFLAGS="-static" \
        LDFLAGS="-static" \
        NL3_FOUND=y \
    && cp iw /build/out-iw

# ── iproute2 / ip ─────────────────────────────────────────────────────────────
RUN apk add --no-cache libelf-dev bison flex iptables-dev
RUN wget -q https://mirrors.edge.kernel.org/pub/linux/utils/net/iproute2/iproute2-6.12.0.tar.gz \
    && tar xzf iproute2-6.12.0.tar.gz
RUN cd iproute2-6.12.0 \
    && ./configure --prefix=/build/iproute2-install \
    && make -j$(nproc) ip/ip \
    && cp ip/ip /build/out-ip

# ── hostapd ───────────────────────────────────────────────────────────────────
RUN wget -q https://w1.fi/releases/hostapd-2.11.tar.gz \
    && tar xzf hostapd-2.11.tar.gz
COPY hostapd.config /build/hostapd-2.11/hostapd/.config
RUN cd hostapd-2.11/hostapd \
    && make -j$(nproc) \
    && cp hostapd /build/out-hostapd

# ── collect outputs ───────────────────────────────────────────────────────────
FROM scratch AS outputs
COPY --from=builder /build/out-iw      /iw
COPY --from=builder /build/out-ip      /ip
COPY --from=builder /build/out-hostapd /hostapd
