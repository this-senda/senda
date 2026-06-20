#!/bin/sh
# Senda installer for Linux and macOS.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/this-senda/senda/main/scripts/install.sh | sh
#
# Environment overrides:
#   SENDA_VERSION       Install a specific version (e.g. 0.1.0). Default: latest release.
#   SENDA_INSTALL_DIR   Where to put the binaries. Default: ~/.local/bin (or /usr/local/bin if writable).
#   SENDA_NO_DESKTOP=1  Skip installing the senda-desktop GUI (headless hosts).
#
# This script downloads a prebuilt release archive, verifies its SHA-256
# checksum, and installs `senda` (the everyday pure-Go binary: TUI + headless
# run/mock/docs + `senda gui` launcher) plus `senda-desktop` (the GUI app).

set -eu

REPO="this-senda/senda"
BIN_NAME="senda"
DESKTOP_NAME="senda-desktop"

# --- pretty output ----------------------------------------------------------
if [ -t 1 ]; then
  BOLD="$(printf '\033[1m')"; DIM="$(printf '\033[2m')"; RED="$(printf '\033[31m')"
  GREEN="$(printf '\033[32m')"; YELLOW="$(printf '\033[33m')"; RESET="$(printf '\033[0m')"
else
  BOLD=""; DIM=""; RED=""; GREEN=""; YELLOW=""; RESET=""
fi
info()  { printf '%s\n' "${DIM}»${RESET} $*"; }
ok()    { printf '%s\n' "${GREEN}✓${RESET} $*"; }
warn()  { printf '%s\n' "${YELLOW}!${RESET} $*" >&2; }
die()   { printf '%s\n' "${RED}✗${RESET} $*" >&2; exit 1; }

# --- prerequisites ----------------------------------------------------------
if command -v curl >/dev/null 2>&1; then
  DL="curl -fsSL"
  DLO() { curl -fsSL -o "$2" "$1"; }
elif command -v wget >/dev/null 2>&1; then
  DL="wget -qO-"
  DLO() { wget -qO "$2" "$1"; }
else
  die "need curl or wget to download Senda"
fi

fetch() { eval "$DL \"$1\""; }

# --- detect platform --------------------------------------------------------
os="$(uname -s)"
case "$os" in
  Linux)  OS="linux" ;;
  Darwin) OS="darwin" ;;
  *) die "unsupported OS: $os (this installer covers Linux and macOS; use scripts/install.ps1 on Windows)" ;;
esac

arch="$(uname -m)"
case "$arch" in
  x86_64|amd64)  ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *) die "unsupported architecture: $arch" ;;
esac

# Intel macs are no longer built — only Apple Silicon ships a darwin archive.
if [ "$OS" = "darwin" ] && [ "$ARCH" = "amd64" ]; then
  die "macOS builds are Apple Silicon (arm64) only; Intel Macs are no longer supported"
fi

# --- resolve version --------------------------------------------------------
VERSION="${SENDA_VERSION:-}"
if [ -z "$VERSION" ]; then
  info "resolving latest release…"
  VERSION="$(fetch "https://api.github.com/repos/${REPO}/releases/latest" \
    | grep '"tag_name"' | head -n1 | sed -E 's/.*"tag_name": *"v?([^"]+)".*/\1/')"
  [ -n "$VERSION" ] || die "could not determine the latest version — set SENDA_VERSION manually"
fi
VERSION="${VERSION#v}"
info "installing Senda ${BOLD}v${VERSION}${RESET} for ${OS}/${ARCH}"

# --- choose install dir -----------------------------------------------------
choose_dir() {
  if [ -n "${SENDA_INSTALL_DIR:-}" ]; then
    printf '%s' "$SENDA_INSTALL_DIR"; return
  fi
  if [ -w /usr/local/bin ] 2>/dev/null; then
    printf '%s' /usr/local/bin; return
  fi
  printf '%s' "$HOME/.local/bin"
}
INSTALL_DIR="$(choose_dir)"
mkdir -p "$INSTALL_DIR" || die "cannot create install dir: $INSTALL_DIR"

# --- download + verify ------------------------------------------------------
# Release assets are named senda_<version>_<os>-<arch>.tar.gz and contain the
# `senda` and `senda-desktop` binaries at the archive root.
ASSET="senda_${VERSION}_${OS}-${ARCH}.tar.gz"
BASE="https://github.com/${REPO}/releases/download/v${VERSION}"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

info "downloading ${ASSET}…"
DLO "${BASE}/${ASSET}" "${TMP}/${ASSET}" || die "download failed: ${BASE}/${ASSET}"

if fetch "${BASE}/checksums.txt" > "${TMP}/checksums.txt" 2>/dev/null && [ -s "${TMP}/checksums.txt" ]; then
  info "verifying checksum…"
  expected="$(grep " ${ASSET}\$" "${TMP}/checksums.txt" | awk '{print $1}')"
  if [ -n "$expected" ]; then
    if command -v sha256sum >/dev/null 2>&1; then
      actual="$(sha256sum "${TMP}/${ASSET}" | awk '{print $1}')"
    elif command -v shasum >/dev/null 2>&1; then
      actual="$(shasum -a 256 "${TMP}/${ASSET}" | awk '{print $1}')"
    else
      warn "no sha256 tool found — skipping checksum verification"
      actual="$expected"
    fi
    [ "$actual" = "$expected" ] || die "checksum mismatch for ${ASSET}"
    ok "checksum verified"
  else
    warn "no checksum entry for ${ASSET} — skipping verification"
  fi
else
  warn "checksum file unavailable — skipping verification"
fi

# --- extract + install ------------------------------------------------------
tar -xzf "${TMP}/${ASSET}" -C "$TMP"

install -m 0755 "${TMP}/${BIN_NAME}" "${INSTALL_DIR}/${BIN_NAME}"
ok "installed ${INSTALL_DIR}/${BIN_NAME}"

DESKTOP_INSTALLED=0
if [ "${SENDA_NO_DESKTOP:-0}" != "1" ] && [ -f "${TMP}/${DESKTOP_NAME}" ]; then
  install -m 0755 "${TMP}/${DESKTOP_NAME}" "${INSTALL_DIR}/${DESKTOP_NAME}"
  ok "installed ${INSTALL_DIR}/${DESKTOP_NAME}"
  DESKTOP_INSTALLED=1
fi

# --- register desktop entry (Linux) -----------------------------------------
# Create a freedesktop .desktop launcher + icon so senda-desktop shows up in
# GNOME/KDE/etc app menus. User-level under $XDG_DATA_HOME (no root needed).
if [ "$OS" = "linux" ] && [ "$DESKTOP_INSTALLED" = "1" ] && [ "${SENDA_NO_DESKTOP:-0}" != "1" ]; then
  DATA_HOME="${XDG_DATA_HOME:-$HOME/.local/share}"
  ICON_DIR="${DATA_HOME}/icons/hicolor/256x256/apps"
  APPS_DIR="${DATA_HOME}/applications"
  mkdir -p "$ICON_DIR" "$APPS_DIR"

  # Icon ships at the archive root (senda.png) on Linux. Older archives didn't
  # bundle it, so fall back to fetching it from raw.
  ICON="senda"
  if [ -f "${TMP}/senda.png" ]; then
    install -m 0644 "${TMP}/senda.png" "${ICON_DIR}/senda.png"
  else
    ICON_URL="https://raw.githubusercontent.com/${REPO}/main/docs/logo/senda-mark-256.png"
    DLO "$ICON_URL" "${ICON_DIR}/senda.png" 2>/dev/null \
      || warn "could not fetch icon — desktop entry may show no icon"
  fi

  cat > "${APPS_DIR}/senda-desktop.desktop" <<EOF
[Desktop Entry]
Type=Application
Name=Senda
GenericName=API Client
Comment=Fast, git-native API client
Exec=${INSTALL_DIR}/${DESKTOP_NAME} %F
Icon=${ICON}
Terminal=false
Categories=Development;
Keywords=API;HTTP;REST;client;rest;
StartupWMClass=senda-desktop
EOF
  chmod 0644 "${APPS_DIR}/senda-desktop.desktop"
  ok "registered desktop entry (${APPS_DIR}/senda-desktop.desktop)"

  command -v update-desktop-database >/dev/null 2>&1 && update-desktop-database "$APPS_DIR" 2>/dev/null || true
  command -v gtk-update-icon-cache  >/dev/null 2>&1 && gtk-update-icon-cache -f -t "${DATA_HOME}/icons/hicolor" 2>/dev/null || true
fi

# Strip the quarantine flag on macOS so Gatekeeper does not block the unsigned
# binary on first launch. A curl|sh download isn't quarantined, but this is
# defensive and harmless if the attribute is absent.
if [ "$OS" = "darwin" ]; then
  xattr -dr com.apple.quarantine "${INSTALL_DIR}/${BIN_NAME}" 2>/dev/null || true
  [ -f "${INSTALL_DIR}/${DESKTOP_NAME}" ] && xattr -dr com.apple.quarantine "${INSTALL_DIR}/${DESKTOP_NAME}" 2>/dev/null || true
fi

# --- post-install notes -----------------------------------------------------
if [ "$OS" = "linux" ] && [ "$DESKTOP_INSTALLED" = "1" ]; then
  if ! ldconfig -p 2>/dev/null | grep -qiE 'libwebkitgtk-6\.0|libwebkit2gtk-4\.1'; then
    warn "The senda-desktop window needs a WebKitGTK runtime. Install one of:"
    warn "  Debian/Ubuntu: sudo apt install libwebkitgtk-6.0-4    (or libwebkit2gtk-4.1-0)"
    warn "  Arch:          sudo pacman -S webkitgtk-6.0           (or webkit2gtk-4.1)"
    warn "  Fedora:        sudo dnf install webkitgtk6.0          (or webkit2gtk4.1)"
  fi
fi

case ":${PATH}:" in
  *":${INSTALL_DIR}:"*) ;;
  *) warn "${INSTALL_DIR} is not on your PATH — add this to your shell profile:"
     warn "  export PATH=\"${INSTALL_DIR}:\$PATH\"" ;;
esac

printf '\n%s\n' "${GREEN}${BOLD}Senda v${VERSION} installed.${RESET} Run ${BOLD}senda${RESET} for the terminal UI, ${BOLD}senda gui${RESET} for the desktop app, or ${BOLD}senda run -h${RESET} for the headless runner."
