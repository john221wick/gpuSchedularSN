#!/usr/bin/env bash
set -euo pipefail

REPO="john221wick/gpuSchedularSN"
BASE_URL="https://github.com/${REPO}/releases/latest/download"
MODE="desktop"
CLEANUP_PATHS=("")

cleanup() {
  local path
  for path in "${CLEANUP_PATHS[@]}"; do
    if [ -n "$path" ]; then
      rm -rf "$path"
    fi
  done
}

trap cleanup EXIT

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
  curl -fsSL "$url" -o "$output"
}

make_tmpdir() {
  mktemp -d
}

install_desktop() {
  local os="$1"
  local arch="$2"

  need_cmd curl

  if [ "$os" = "darwin" ]; then
    need_cmd tar
    need_cmd ditto
    local tmpdir
    tmpdir="$(make_tmpdir)"
    CLEANUP_PATHS+=("$tmpdir")

    download "${BASE_URL}/gpusched-desktop-darwin-${arch}.tar.gz" "$tmpdir/gpusched-desktop.tar.gz"
    tar -xzf "$tmpdir/gpusched-desktop.tar.gz" -C "$tmpdir"
    run_as_root ditto "$tmpdir/gpusched.app" /Applications/gpusched.app
    echo "Installed GPU Scheduler desktop app to /Applications/gpusched.app"
    open /Applications/gpusched.app >/dev/null 2>&1 || true
    return
  fi

  if [ "$os" = "linux" ]; then
    local bin_dir="$HOME/.local/bin"
    local app_dir="$HOME/.local/share/applications"
    local desktop_file="$app_dir/gpusched.desktop"

    mkdir -p "$bin_dir" "$app_dir"
    download "${BASE_URL}/gpusched-desktop-linux-${arch}" "$bin_dir/gpusched-desktop"
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
  tmpdir="$(make_tmpdir)"
  CLEANUP_PATHS+=("$tmpdir")

  download "${BASE_URL}/gpusched-${os}-${arch}" "$tmpdir/gpusched"
  chmod +x "$tmpdir/gpusched"
  run_as_root mkdir -p /usr/local/bin
  run_as_root cp "$tmpdir/gpusched" /usr/local/bin/gpusched
  echo "Installed gpusched CLI to /usr/local/bin/gpusched"
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
