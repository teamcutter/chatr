#!/bin/bash
set -euo pipefail

REPO="teamcutter/chatr"
INSTALL_DIR="$HOME/.chatr/bin"

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

# Packages live under a short prefix so paths compiled into Homebrew bottles
# can be rewritten in place. Existing installs keep the prefix in their config.
PREFIX="/opt/chatr"
if [[ ! -f "$HOME/.chatr/config.toml" ]]; then
    if [[ ! -d "$PREFIX" ]]; then
        echo ""
        echo "chatr installs packages under $PREFIX. Creating it, sudo may ask for your password."
        if mkdir -p "$PREFIX" 2>/dev/null ||
            { sudo mkdir -p "$PREFIX" < /dev/tty && sudo chown "$(id -un)" "$PREFIX" < /dev/tty; }; then
            echo "Created $PREFIX"
        else
            echo "Could not create $PREFIX. Create it before installing packages:"
            echo ""
            echo "  sudo mkdir -p $PREFIX && sudo chown \$(whoami) $PREFIX"
        fi
    fi
    PKG_BIN="$PREFIX/bin"
else
    PKG_BIN="$INSTALL_DIR"
fi

MISSING=""
[[ ":$PATH:" != *":$INSTALL_DIR:"* ]] && MISSING="\$HOME/.chatr/bin"
if [[ "$PKG_BIN" != "$INSTALL_DIR" && ":$PATH:" != *":$PKG_BIN:"* ]]; then
    MISSING="$PKG_BIN${MISSING:+:$MISSING}"
fi
if [[ -n "$MISSING" ]]; then
    echo ""
    echo "Add chatr to your PATH by adding this to your shell config:"
    echo ""
    echo "  export PATH=\"$MISSING:\$PATH\""
fi
