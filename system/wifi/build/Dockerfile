# system/wifi/build/Dockerfile — builds static wpa_supplicant for ARM64
# Output: /wpa_supplicant (static ARM64 ELF)
FROM --platform=linux/arm64 alpine:3.21 AS builder

RUN apk add --no-cache \
    build-base \
    openssl-dev \
    openssl-libs-static \
    libnl3-dev \
    libnl3-static \
    dbus-dev \
    linux-headers

ARG WPA_VERSION=2.11
RUN wget -q https://w1.fi/releases/wpa_supplicant-${WPA_VERSION}.tar.gz \
    && tar xf wpa_supplicant-${WPA_VERSION}.tar.gz

WORKDIR /wpa_supplicant-${WPA_VERSION}/wpa_supplicant

COPY defconfig .config

RUN make -j$(nproc) LDFLAGS="-static" CC="gcc" \
    && strip wpa_supplicant

FROM scratch
COPY --from=builder /wpa_supplicant-2.11/wpa_supplicant/wpa_supplicant /wpa_supplicant
