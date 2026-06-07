# Linux desktop (Wails) build toolchain.
# Baked once into an image so release builds are fast and repeatable.
# Ships BOTH webkit dev packages so we can build two variants:
#   4.0 -> Ubuntu 20.04/22.04, Debian 11/12 (default build)
#   4.1 -> Ubuntu 24.04+, Debian 13, Fedora 40+, Arch (-tags webkit2_41)
FROM golang:1.25-bookworm

RUN apt-get update && apt-get install -y --no-install-recommends \
        build-essential \
        pkg-config \
        libgtk-3-dev \
        libwebkit2gtk-4.0-dev \
        libwebkit2gtk-4.1-dev \
    && rm -rf /var/lib/apt/lists/*

RUN go install github.com/wailsapp/wails/v2/cmd/wails@v2.12.0

WORKDIR /app/cmd/desktop
