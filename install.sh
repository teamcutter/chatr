#!/bin/bash
set -euo pipefail

REPO="teamcutter/chatr"
PREFIX="/opt/chatr"

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$OS" in
    darwin) OS="darwin" ;;
    linux) OS="linux" ;;
    mingw*|msys*|cygwin*) OS="windows" ;;
    *) echo "Unsupported OS: $OS"; exit 1 ;;
esac

ARCH=$(uname -m)
case "$ARCH" in
    x86_64|amd64) ARCH="amd64" ;;
    arm64|aarch64) ARCH="arm64" ;;
    *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

VERSION=$(curl -sL "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
if [[ -z "$VERSION" ]]; then
    echo "Failed to get latest version"
    exit 1
fi

ensure_prefix() {
    [[ -d "$PREFIX" && -w "$PREFIX" ]] && return 0
    echo "chatr lives under $PREFIX. Creating it, sudo may ask for your password."
    mkdir -p "$PREFIX" 2>/dev/null && [[ -w "$PREFIX" ]] && return 0
    sudo mkdir -p "$PREFIX" < /dev/tty && sudo chown "$(id -un)" "$PREFIX" < /dev/tty
}

LEGACY_DIR="$HOME/.chatr/bin"
if [[ "$OS" == "windows" ]]; then
    INSTALL_DIR="$LEGACY_DIR"
elif ensure_prefix; then
    INSTALL_DIR="$PREFIX/bin"
else
    echo "Could not create $PREFIX. Create it, then run this script again:"
    echo ""
    echo "  sudo mkdir -p $PREFIX && sudo chown \$(whoami) $PREFIX"
    exit 1
fi

if [[ -x "$INSTALL_DIR/chatr" ]]; then
    CURRENT=$("$INSTALL_DIR/chatr" version 2>/dev/null | sed -E 's/chatr-v?([0-9]+\.[0-9]+\.[0-9]+).*/\1/')
    if [[ "$CURRENT" == "${VERSION#v}" ]]; then
        echo "chatr $VERSION is already installed"
        exit 0
    fi
fi

EXT="tar.gz"
if [[ "$OS" == "windows" ]]; then
    EXT="zip"
fi

FILENAME="chatr_${VERSION#v}_${OS}_${ARCH}.${EXT}"
URL="https://github.com/$REPO/releases/download/$VERSION/$FILENAME"

echo "Downloading chatr $VERSION for $OS/$ARCH..."

TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT

curl -sL "$URL" -o "$TMP_DIR/$FILENAME"

mkdir -p "$INSTALL_DIR"
if [[ "$EXT" == "zip" ]]; then
    unzip -q "$TMP_DIR/$FILENAME" -d "$TMP_DIR"
else
    tar -xzf "$TMP_DIR/$FILENAME" -C "$TMP_DIR"
fi

if [[ "$OS" == "linux" ]]; then
    rm -f "$INSTALL_DIR/chatr"
fi
cp "$TMP_DIR/chatr" "$INSTALL_DIR/chatr"
chmod +x "$INSTALL_DIR/chatr"

if [[ "$OS" == "darwin" ]]; then
    xattr -cr "$INSTALL_DIR/chatr" 2>/dev/null || true
fi

echo "Installed chatr to $INSTALL_DIR/chatr"

if [[ "$INSTALL_DIR" != "$LEGACY_DIR" && -f "$LEGACY_DIR/chatr" && ! -L "$LEGACY_DIR/chatr" ]]; then
    rm -f "$LEGACY_DIR/chatr"
    echo "Removed the old copy at $LEGACY_DIR/chatr"
fi

if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
    case "${SHELL##*/}" in
        zsh)  RC="~/.zprofile"; LINE="eval \"\$($INSTALL_DIR/chatr shellenv)\"" ;;
        bash) [[ "$OS" == "darwin" ]] && RC="~/.bash_profile" || RC="~/.bashrc"
              LINE="eval \"\$($INSTALL_DIR/chatr shellenv)\"" ;;
        fish) RC="~/.config/fish/config.fish"; LINE="$INSTALL_DIR/chatr shellenv fish | source" ;;
        *)    RC="~/.profile"; LINE="eval \"\$($INSTALL_DIR/chatr shellenv)\"" ;;
    esac
    echo ""
    echo "Add chatr to your shell by adding this line to $RC:"
    echo ""
    echo "  $LINE"
    echo ""
    echo "Then open a new terminal, or run the line once in this one."
fi
