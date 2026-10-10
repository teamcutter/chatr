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

if command -v chatr &>/dev/null; then
    CURRENT=$(chatr version 2>/dev/null | sed -E 's/chatr-v?([0-9]+\.[0-9]+\.[0-9]+).*/\1/')
    LATEST="${VERSION#v}"
    if [[ "$CURRENT" == "$LATEST" ]]; then
        echo "chatr $VERSION is already installed"
        exit 0
    fi
fi

ensure_prefix() {
    [[ -d "$PREFIX" && -w "$PREFIX" ]] && return 0
    echo "chatr installs packages under $PREFIX. Creating it, sudo may ask for your password."
    mkdir -p "$PREFIX" 2>/dev/null && [[ -w "$PREFIX" ]] && return 0
    sudo mkdir -p "$PREFIX" < /dev/tty && sudo chown "$(id -un)" "$PREFIX" < /dev/tty
}

# Updates replace the binary where it already is. New installs put it in the
# package prefix so a single PATH entry covers chatr and its packages.
EXISTING=$(command -v chatr 2>/dev/null || true)
if [[ -n "$EXISTING" && -w "$(dirname "$EXISTING")" ]]; then
    INSTALL_DIR=$(dirname "$EXISTING")
elif [[ -f "$HOME/.chatr/config.toml" || "$OS" == "windows" ]]; then
    INSTALL_DIR="$HOME/.chatr/bin"
elif ensure_prefix; then
    INSTALL_DIR="$PREFIX/bin"
else
    INSTALL_DIR="$HOME/.chatr/bin"
    echo "Could not create $PREFIX. Create it before installing packages:"
    echo ""
    echo "  sudo mkdir -p $PREFIX && sudo chown \$(whoami) $PREFIX"
    echo ""
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
