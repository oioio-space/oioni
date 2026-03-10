# imgvol/build/Dockerfile
# Builds static ARM64 mkfs binaries for gokrazy (no libc / no dynamic linker at runtime).
# Run via: make build-imgvol-bins  (uses docker buildx --output)
#       or: podman build + podman cp  (see Makefile)
#
# Strategy: Alpine edge on linux/arm64, release tarballs (pre-generated configure,
# no autoreconf needed), LDFLAGS=-static for fully static binaries.
#
# Prerequisites (x86_64 host):
#   sudo dnf install qemu-user-static   # registers ARM64 binfmt_misc
#
# To verify binaries once built:
#   file imgvol/bin/mkfs.*
#   # Expected: ELF 64-bit LSB executable, ARM aarch64, statically linked

FROM --platform=linux/arm64 alpine:edge AS builder

RUN apk add --no-cache \
    build-base cmake \
    linux-headers wget tar \
    musl-dev musl-utils \
    util-linux-dev \
    attr-dev \
    libuuid

RUN mkdir /out

# ── dosfstools 4.2 (mkfs.fat) ─────────────────────────────────────────────────
# Using release tarball: configure already generated, no autoreconf needed.
WORKDIR /build/dosfstools
RUN wget -q https://github.com/dosfstools/dosfstools/releases/download/v4.2/dosfstools-4.2.tar.gz && \
    tar xf dosfstools-4.2.tar.gz --strip-components=1 && \
    LDFLAGS="-static" ./configure --disable-shared --without-udev --without-iconv && \
    make -j$(nproc) && \
    cp src/mkfs.fat /out/mkfs.fat

# ── exfatprogs 1.2.5 (mkfs.exfat) ────────────────────────────────────────────
# libtool strips LDFLAGS=-static; use -all-static which libtool recognizes.
WORKDIR /build/exfatprogs
RUN wget -q https://github.com/exfatprogs/exfatprogs/releases/download/1.2.5/exfatprogs-1.2.5.tar.gz && \
    tar xf exfatprogs-1.2.5.tar.gz --strip-components=1 && \
    ./configure --disable-shared && \
    make LDFLAGS="-all-static" -j$(nproc) && \
    cp mkfs/mkfs.exfat /out/mkfs.exfat

# ── ntfsprogs-plus (mkntfs → mkfs.ntfs) ──────────────────────────────────────
# ntfsprogs-plus is a modern standalone fork of ntfsprogs (no FUSE dependency,
# simpler build than ntfs-3g). See https://github.com/ntfsprogs-plus/ntfsprogs-plus
WORKDIR /build/ntfsprogs
RUN apk add --no-cache git automake autoconf libtool libgcrypt-dev util-linux-static && \
    git clone --depth 1 https://github.com/ntfsprogs-plus/ntfsprogs-plus.git . && \
    autoreconf -fi && \
    ./configure --disable-shared --enable-static && \
    make -C libntfs LDFLAGS="-all-static" -j$(nproc) && \
    make -C src LDFLAGS="-all-static" -j$(nproc) mkntfs && \
    cp src/mkntfs /out/mkfs.ntfs

# ── e2fsprogs 1.47.2 (mke2fs → mkfs.ext4) ────────────────────────────────────
WORKDIR /build/e2fsprogs
RUN wget -q https://www.kernel.org/pub/linux/kernel/people/tytso/e2fsprogs/v1.47.2/e2fsprogs-1.47.2.tar.gz && \
    tar xf e2fsprogs-1.47.2.tar.gz --strip-components=1 && \
    LDFLAGS="-static" ./configure --disable-backtrace && \
    make -j$(nproc) && \
    cp misc/mke2fs /out/mkfs.ext4

# ── Final stage: extract binaries via --output type=local ────────────────────
FROM scratch
COPY --from=builder /out/mkfs.fat   /mkfs.fat
COPY --from=builder /out/mkfs.exfat /mkfs.exfat
COPY --from=builder /out/mkfs.ntfs  /mkfs.ntfs
COPY --from=builder /out/mkfs.ext4  /mkfs.ext4
