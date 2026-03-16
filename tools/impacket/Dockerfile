# tools/impacket/Dockerfile
# Build for arm64: podman build --platform linux/arm64 -t oioni/impacket:arm64 .
FROM python:3.13-alpine

# Use a venv so pip does not conflict with the system Python (PEP 668).
# Alpine busybox provides sh and kill — required by ProcManager signal delivery.
RUN python -m venv /opt/impacket \
 && /opt/impacket/bin/pip install --no-cache-dir impacket

ENV PATH="/opt/impacket/bin:$PATH"
