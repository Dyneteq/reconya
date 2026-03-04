#!/bin/sh
set -e

REPO="Dyneteq/reconya"
INSTALL_DIR="./reconya"

# Detect OS
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$OS" in
  linux)  OS="linux" ;;
  darwin) OS="darwin" ;;
  *) echo "Unsupported OS: $OS"; exit 1 ;;
esac

# Detect architecture
ARCH=$(uname -m)
case "$ARCH" in
  x86_64|amd64)  ARCH="amd64" ;;
  aarch64|arm64)  ARCH="arm64" ;;
  *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

BINARY="reconya-${OS}-${ARCH}"
TARBALL="${BINARY}.tar.gz"
URL="https://github.com/${REPO}/releases/latest/download/${TARBALL}"

echo "reconYa Installer"
echo "================="
echo "Detected: ${OS}/${ARCH}"
echo "Downloading: ${TARBALL}"
echo ""

# Create install directory
mkdir -p "$INSTALL_DIR"
cd "$INSTALL_DIR"

# Download and extract
curl -sL "$URL" | tar xz

# Create .env from example if it doesn't exist
if [ ! -f .env ] && [ -f .env.example ]; then
  cp .env.example .env
  echo "Created .env from .env.example"
fi

echo ""
echo "reconYa installed to ${INSTALL_DIR}/"
echo ""
echo "To start:"
echo "  cd ${INSTALL_DIR} && sudo ./${BINARY}"
echo ""
echo "Then open http://localhost:3008"
echo "Default login: admin / password"
