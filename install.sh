#!/usr/bin/env bash
set -euo pipefail

REPO="john221wick/gpuSchedularSN"
BASE_URL="https://github.com/${REPO}/releases/latest/download"
MODE="desktop"

usage() {
  cat <<'EOF'
Usage: install.sh [--desktop|--cli|--both]

Installs GPU Scheduler from the latest GitHub release.

Options:
  --desktop  Install the desktop app (default)
  --cli      Install only the gpusched CLI command
  --both     Install the desktop app and CLI command
EOF
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --desktop)
      MODE="desktop"
      ;;
    --cli)
      MODE="cli"
      ;;
    --both)
      MODE="both"
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
  shift
done

need_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "Missing required command: $1" >&2
    exit 1
  fi
}

run_as_root() {
  if [ "$(id -u)" -eq 0 ]; then
    "$@"
  else
    need_cmd sudo
    sudo "$@"
  fi
}

refresh_macos_app_icon() {
  local app_path="$1"
  local lsregister="/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister"

  run_as_root touch "$app_path" >/dev/null 2>&1 || true

  if [ -x "$lsregister" ]; then
    "$lsregister" -f "$app_path" >/dev/null 2>&1 || true
  fi

  if command -v qlmanage >/dev/null 2>&1; then
    qlmanage -r cache >/dev/null 2>&1 || true
  fi

  killall Dock >/dev/null 2>&1 || true
}

detect_os() {
  case "$(uname -s)" in
    Darwin) echo "darwin" ;;
    Linux) echo "linux" ;;
    *)
      echo "Unsupported OS: $(uname -s)" >&2
      exit 1
      ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64) echo "amd64" ;;
    arm64|aarch64) echo "arm64" ;;
    *)
      echo "Unsupported architecture: $(uname -m)" >&2
      exit 1
      ;;
  esac
}

download() {
  local url="$1"
  local output="$2"
  echo "Downloading ${url}"
  # -f fail on HTTP error, -L follow redirects, --progress-bar show a progress bar
  curl -fL --progress-bar "$url" -o "$output"
}

# Pick the webkit2gtk variant this distro provides. Newer distros (Ubuntu
# 24.04+, Debian 13, Fedora 40+, current Arch) dropped 4.0 and ship only 4.1.
# We prefer 4.0 when available (broadest), else fall back to 4.1. Echoes
# "40" or "41".
detect_linux_webkit() {
  if command -v apt-get >/dev/null 2>&1; then
    if apt-cache show libwebkit2gtk-4.0-37 2>/dev/null | grep -q '^Package:'; then
      echo "40"
    else
      echo "41"
    fi
  elif command -v dnf >/dev/null 2>&1; then
    if dnf -q list webkit2gtk4.0 >/dev/null 2>&1; then echo "40"; else echo "41"; fi
  elif command -v pacman >/dev/null 2>&1; then
    if pacman -Sp webkit2gtk >/dev/null 2>&1; then echo "40"; else echo "41"; fi
  elif command -v zypper >/dev/null 2>&1; then
    if zypper -q se -x libwebkit2gtk-4_0-37 >/dev/null 2>&1; then echo "40"; else echo "41"; fi
  else
    echo "40"
  fi
}

# Install the GTK/WebKit runtime libraries the desktop app links against.
# $1 is the webkit variant ("40" or "41"); gtk3 is needed either way.
install_linux_gui_deps() {
  local variant="$1"
  echo "Installing desktop runtime dependencies (webkit2gtk-4.${variant#4}, gtk3)..."
  # Best-effort and non-interactive: a partial failure here must not abort the
  # install — the user may already have the libraries, or only need a subset.
  local ok=1
  if command -v apt-get >/dev/null 2>&1; then
    export DEBIAN_FRONTEND=noninteractive
    local webkit_pkg="libwebkit2gtk-4.0-37"
    [ "$variant" = "41" ] && webkit_pkg="libwebkit2gtk-4.1-0"
    run_as_root apt-get update -y || ok=0
    run_as_root apt-get install -y "$webkit_pkg" libgtk-3-0 || ok=0
  elif command -v dnf >/dev/null 2>&1; then
    local webkit_pkg="webkit2gtk4.0"
    [ "$variant" = "41" ] && webkit_pkg="webkit2gtk4.1"
    run_as_root dnf install -y "$webkit_pkg" gtk3 || ok=0
  elif command -v pacman >/dev/null 2>&1; then
    local webkit_pkg="webkit2gtk"
    [ "$variant" = "41" ] && webkit_pkg="webkit2gtk-4.1"
    run_as_root pacman -Sy --needed --noconfirm "$webkit_pkg" gtk3 || ok=0
  elif command -v zypper >/dev/null 2>&1; then
    local webkit_pkg="libwebkit2gtk-4_0-37"
    [ "$variant" = "41" ] && webkit_pkg="libwebkit2gtk-4_1-0"
    run_as_root zypper --non-interactive install "$webkit_pkg" gtk3 || ok=0
  else
    ok=0
    echo "WARNING: Could not detect a supported package manager." >&2
  fi

  if [ "$ok" -ne 1 ]; then
    echo "WARNING: Could not auto-install all desktop dependencies." >&2
    echo "If the app fails to launch, install webkit2gtk and gtk3 for your distro." >&2
  fi
}

install_desktop() {
  local os="$1"
  local arch="$2"

  need_cmd curl

  if [ "$os" = "darwin" ]; then
    need_cmd tar
    need_cmd ditto
    local tmpdir
    tmpdir="$(mktemp -d)"

    download "${BASE_URL}/gpusched-desktop-darwin-${arch}.tar.gz" "$tmpdir/gpusched-desktop.tar.gz"
    tar -xzf "$tmpdir/gpusched-desktop.tar.gz" -C "$tmpdir"
    run_as_root ditto "$tmpdir/gpusched.app" /Applications/gpusched.app
    refresh_macos_app_icon /Applications/gpusched.app
    echo "Installed GPU Scheduler desktop app to /Applications/gpusched.app"
    open /Applications/gpusched.app >/dev/null 2>&1 || true
    rm -rf "$tmpdir"
    return
  fi

  if [ "$os" = "linux" ]; then
    local bin_dir="$HOME/.local/bin"
    local app_dir="$HOME/.local/share/applications"
    local desktop_file="$app_dir/gpusched.desktop"

    local webkit_variant asset_suffix=""
    webkit_variant="$(detect_linux_webkit)"
    [ "$webkit_variant" = "41" ] && asset_suffix="-webkit41"

    install_linux_gui_deps "$webkit_variant"

    mkdir -p "$bin_dir" "$app_dir"
    download "${BASE_URL}/gpusched-desktop-linux-${arch}${asset_suffix}" "$bin_dir/gpusched-desktop"
    chmod +x "$bin_dir/gpusched-desktop"

    cat > "$desktop_file" <<EOF
[Desktop Entry]
Name=GPU Scheduler
Comment=Topology-aware GPU scheduler
Exec=$bin_dir/gpusched-desktop
Icon=utilities-system-monitor
Terminal=false
Type=Application
Categories=Development;System;
EOF

    if command -v update-desktop-database >/dev/null 2>&1; then
      update-desktop-database "$app_dir" >/dev/null 2>&1 || true
    fi

    echo "Installed GPU Scheduler desktop app to $bin_dir/gpusched-desktop"
    echo "Installed desktop launcher to $desktop_file"
    return
  fi
}

install_cli() {
  local os="$1"
  local arch="$2"

  need_cmd curl

  local tmpdir
  tmpdir="$(mktemp -d)"

  download "${BASE_URL}/gpusched-${os}-${arch}" "$tmpdir/gpusched"
  chmod +x "$tmpdir/gpusched"
  run_as_root mkdir -p /usr/local/bin
  run_as_root cp "$tmpdir/gpusched" /usr/local/bin/gpusched
  echo "Installed gpusched CLI to /usr/local/bin/gpusched"
  rm -rf "$tmpdir"
}

main() {
  local os
  local arch
  os="$(detect_os)"
  arch="$(detect_arch)"

  case "$MODE" in
    desktop)
      install_desktop "$os" "$arch"
      ;;
    cli)
      install_cli "$os" "$arch"
      ;;
    both)
      install_desktop "$os" "$arch"
      install_cli "$os" "$arch"
      ;;
  esac
}

main
