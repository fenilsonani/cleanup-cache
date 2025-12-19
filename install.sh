#!/bin/bash
# TidyUp Quick Install Script
# https://github.com/fenilsonani/cleanup-cache

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

echo ""
echo -e "${BLUE}╔═══════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║              TidyUp Installation Options                  ║${NC}"
echo -e "${BLUE}╚═══════════════════════════════════════════════════════════╝${NC}"
echo ""

# Detect OS
OS=$(uname -s | tr '[:upper:]' '[:lower:]')

if [ "$OS" = "darwin" ]; then
    echo -e "${GREEN}Recommended for macOS:${NC}"
    echo ""
    echo -e "  ${CYAN}brew install fenilsonani/tidyup/tidyup${NC}"
    echo ""
    echo -e "  Or tap first:"
    echo -e "  ${CYAN}brew tap fenilsonani/tidyup${NC}"
    echo -e "  ${CYAN}brew install tidyup${NC}"
    echo ""
elif [ "$OS" = "linux" ]; then
    echo -e "${GREEN}Recommended for Linux:${NC}"
    echo ""
    if command -v brew &> /dev/null; then
        echo -e "  ${CYAN}brew install fenilsonani/tidyup/tidyup${NC}"
        echo ""
    fi
fi

echo -e "${GREEN}Alternative - Go Install (requires Go 1.21+):${NC}"
echo ""
echo -e "  ${CYAN}go install github.com/fenilsonani/cleanup-cache/cmd/tidyup@latest${NC}"
echo ""

echo -e "${GREEN}Alternative - Direct Binary Download:${NC}"
echo ""
echo -e "  ${CYAN}https://github.com/fenilsonani/cleanup-cache/releases/latest${NC}"
echo ""

# Quick install option
echo -e "${YELLOW}Quick Install (downloads latest binary):${NC}"
echo ""

if [ "$1" = "--quick" ] || [ "$1" = "-q" ]; then
    echo -e "${BLUE}Installing latest binary...${NC}"

    ARCH=$(uname -m)
    case "$ARCH" in
        x86_64) ARCH="amd64" ;;
        arm64|aarch64) ARCH="arm64" ;;
        *) echo -e "${RED}Unsupported architecture: $ARCH${NC}"; exit 1 ;;
    esac

    DOWNLOAD_URL="https://github.com/fenilsonani/cleanup-cache/releases/latest/download/tidyup-${OS}-${ARCH}.tar.gz"

    TMP_DIR=$(mktemp -d)
    cd "$TMP_DIR"

    echo -e "  Downloading from ${CYAN}$DOWNLOAD_URL${NC}..."
    curl -sL "$DOWNLOAD_URL" -o tidyup.tar.gz
    tar -xzf tidyup.tar.gz

    echo -e "  Installing to /usr/local/bin (requires sudo)..."
    sudo mv tidyup /usr/local/bin/
    sudo chmod +x /usr/local/bin/tidyup

    rm -rf "$TMP_DIR"

    echo ""
    echo -e "${GREEN}✓ Installed successfully!${NC}"
    tidyup --version
else
    echo -e "  Run with ${CYAN}--quick${NC} flag to auto-install:"
    echo -e "  ${CYAN}curl -sSL .../install.sh | bash -s -- --quick${NC}"
fi

echo ""
