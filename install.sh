#!/bin/sh
set -e

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
if [ -z "$VERSION" ]; then
    echo "Failed to get latest version"
    exit 1
fi

EXT="tar.gz"
if [ "$OS" = "windows" ]; then
    EXT="zip"
fi

FILENAME="chatr_${VERSION#v}_${OS}_${ARCH}.${EXT}"
URL="https://github.com/$REPO/releases/download/$VERSION/$FILENAME"

echo "Downloading chatr $VERSION for $OS/$ARCH..."

TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT

curl -sL "$URL" -o "$TMP_DIR/$FILENAME"

mkdir -p "$INSTALL_DIR"
if [ "$EXT" = "zip" ]; then
    unzip -q "$TMP_DIR/$FILENAME" -d "$TMP_DIR"
else
    tar -xzf "$TMP_DIR/$FILENAME" -C "$TMP_DIR"
fi

cp "$TMP_DIR/chatr" "$INSTALL_DIR/chatr"
chmod +x "$INSTALL_DIR/chatr"

if [ "$OS" = "darwin" ]; then
    xattr -cr "$INSTALL_DIR/chatr" 2>/dev/null || true
fi

echo "Installed chatr to $INSTALL_DIR/chatr"

case ":$PATH:" in
    *":$INSTALL_DIR:"*) ;;
    *)
        echo ""
        echo "Add chatr to your PATH by adding this to your shell config:"
        echo ""
        echo "  export PATH=\"\$HOME/.chatr/bin:\$PATH\""
        ;;
esac
