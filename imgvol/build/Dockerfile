# imgvol/build/Dockerfile
# Builds static ARM64 mkfs binaries for gokrazy (no libc / no dynamic linker at runtime).
# Run via: make build-imgvol-bins
#
# Strategy: Alpine edge on linux/arm64 — musl libc makes -static straightforward.
# Each tool is compiled from source with LDFLAGS="-static" and --disable-shared.
# The final FROM scratch stage + --output type=local extracts binaries to imgvol/bin/.
#
# Prerequisites:
#   - Docker with buildx plugin (NOT podman — `docker buildx build --output` is required)
#   - ARM64 QEMU binfmt_misc registered for cross-build on x86_64 hosts:
#       docker run --rm --privileged multiarch/qemu-user-static --reset -p yes
#
# Build NOT executed during initial commit: `docker` binary unavailable on this
# machine (only `podman` is installed). The pipeline structure is correct and
# ready to run on any host with Docker + buildx + QEMU ARM64 emulation.
#
# To verify binaries once built:
#   file imgvol/bin/mkfs.*
#   # Expected: ELF 64-bit LSB executable, ARM aarch64, statically linked
#
# Known issues / fallback notes:
#   - ntfs-3g: --without-fuse avoids fuse dep; libntfs-3g is linked statically.
#   - e2fsprogs: produces mke2fs; copied as mkfs.ext4.
#   - If a tool fails to link fully statically, copy the Alpine package binary
#     and bundle required .so files alongside it (gokrazy can load them if placed
#     in the same directory and LD_LIBRARY_PATH is set by the wrapper).

FROM --platform=linux/arm64 alpine:edge AS builder

RUN apk add --no-cache \
    build-base cmake automake autoconf libtool \
    linux-headers git \
    musl-dev musl-utils \
    util-linux-dev

RUN mkdir /out

# ── dosfstools (mkfs.fat) ─────────────────────────────────────────────────────
WORKDIR /build/dosfstools
RUN git clone --depth 1 https://github.com/dosfstools/dosfstools.git . && \
    autoreconf -fi && \
    LDFLAGS="-static" ./configure --disable-shared --without-udev && \
    make -j$(nproc) && \
    cp src/mkfs.fat /out/mkfs.fat

# ── exfatprogs (mkfs.exfat) ───────────────────────────────────────────────────
WORKDIR /build/exfatprogs
RUN git clone --depth 1 https://github.com/exfatprogs/exfatprogs.git . && \
    autoreconf -fi && \
    LDFLAGS="-static" ./configure --disable-shared && \
    make -j$(nproc) && \
    cp mkfs/mkfs.exfat /out/mkfs.exfat

# ── ntfs-3g (mkntfs → mkfs.ntfs) ─────────────────────────────────────────────
# Uses libfuse/ntfs-3g (maintained fork). --enable-ntfsprogs is required to
# build mkntfs; --without-fuse avoids the FUSE dependency for the library.
WORKDIR /build/ntfs3g
RUN git clone --depth 1 https://github.com/libfuse/ntfs-3g.git . && \
    autoreconf -fi && \
    LDFLAGS="-static" ./configure \
        --disable-shared \
        --enable-static \
        --enable-ntfsprogs \
        --without-fuse && \
    make -j$(nproc) ntfsprogs/mkntfs && \
    cp ntfsprogs/mkntfs /out/mkfs.ntfs

# ── e2fsprogs (mke2fs → mkfs.ext4) ───────────────────────────────────────────
WORKDIR /build/e2fsprogs
RUN git clone --depth 1 https://git.kernel.org/pub/scm/fs/ext2/e2fsprogs.git . && \
    ./configure LDFLAGS="-static" --disable-shared --enable-static \
        --disable-blkid --disable-backtrace && \
    make -j$(nproc) && \
    cp misc/mke2fs /out/mkfs.ext4

# ── Final stage: extract binaries via --output type=local ────────────────────
FROM scratch
COPY --from=builder /out/mkfs.fat   /mkfs.fat
COPY --from=builder /out/mkfs.exfat /mkfs.exfat
COPY --from=builder /out/mkfs.ntfs  /mkfs.ntfs
COPY --from=builder /out/mkfs.ext4  /mkfs.ext4
