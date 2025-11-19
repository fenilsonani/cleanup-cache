#!/bin/bash
# CleanupCache Installation Script
# Usage: curl -sSL https://raw.githubusercontent.com/yourusername/cleanup-cache/main/install.sh | bash

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
BINARY_NAME="cleanup"
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="$HOME/.config/cleanup-cache"
REPO="fenilsonani/cleanup-cache"
GITHUB_API="https://api.github.com/repos/$REPO/releases/latest"

# Print colored output
print_info() {
    echo -e "${BLUE}ℹ${NC} $1"
}

print_success() {
    echo -e "${GREEN}✓${NC} $1"
}

print_error() {
    echo -e "${RED}✗${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}⚠${NC} $1"
}

# Detect OS and architecture
detect_platform() {
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)

    case "$OS" in
        darwin)
            OS="darwin"
            ;;
        linux)
            OS="linux"
            ;;
        *)
            print_error "Unsupported operating system: $OS"
            exit 1
            ;;
    esac

    case "$ARCH" in
        x86_64)
            ARCH="amd64"
            ;;
        arm64|aarch64)
            ARCH="arm64"
            ;;
        *)
            print_error "Unsupported architecture: $ARCH"
            exit 1
            ;;
    esac

    print_info "Detected platform: ${OS}/${ARCH}"
}

# Check if running as root
check_root() {
    if [ "$EUID" -eq 0 ]; then
        print_warning "Running as root. Installation will be system-wide."
        USE_SUDO=""
    else
        USE_SUDO="sudo"
        print_info "Will use sudo for system-wide installation."
    fi
}

# Check dependencies
check_dependencies() {
    local missing_deps=()

    for cmd in curl tar; do
        if ! command -v $cmd &> /dev/null; then
            missing_deps+=($cmd)
        fi
    done

    if [ ${#missing_deps[@]} -ne 0 ]; then
        print_error "Missing required dependencies: ${missing_deps[*]}"
        print_info "Please install them and try again."
        exit 1
    fi
}

# Get latest release version from GitHub
get_latest_version() {
    print_info "Fetching latest version from GitHub..."

    if command -v curl &> /dev/null; then
        VERSION=$(curl -s "$GITHUB_API" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    else
        print_error "curl is required but not installed."
        exit 1
    fi

    if [ -z "$VERSION" ]; then
        print_warning "Could not fetch latest version from GitHub."
        print_info "Building from source instead..."
        BUILD_FROM_SOURCE=1
    else
        print_success "Latest version: $VERSION"
    fi
}

# Build from source (fallback if no release available)
build_from_source() {
    print_info "Building CleanupCache from source..."

    # Check if Go is installed
    if ! command -v go &> /dev/null; then
        print_error "Go is not installed. Please install Go 1.21+ and try again."
        print_info "Visit https://go.dev/doc/install for installation instructions."
        exit 1
    fi

    # Create temporary directory
    TMP_DIR=$(mktemp -d)
    cd "$TMP_DIR"

    print_info "Cloning repository..."
    git clone "https://github.com/$REPO.git" .

    print_info "Building binary..."
    go build -o "$BINARY_NAME" ./cmd/cleanup

    print_info "Installing binary to $INSTALL_DIR..."
    $USE_SUDO mv "$BINARY_NAME" "$INSTALL_DIR/"
    $USE_SUDO chmod +x "$INSTALL_DIR/$BINARY_NAME"

    # Cleanup
    cd - > /dev/null
    rm -rf "$TMP_DIR"
}

# Download and install binary
download_and_install() {
    if [ "$BUILD_FROM_SOURCE" = "1" ]; then
        build_from_source
        return
    fi

    # Construct download URL
    DOWNLOAD_URL="https://github.com/$REPO/releases/download/${VERSION}/cleanup-${OS}-${ARCH}.tar.gz"

    print_info "Downloading CleanupCache ${VERSION}..."

    # Create temporary directory
    TMP_DIR=$(mktemp -d)
    cd "$TMP_DIR"

    # Download
    if ! curl -sSL "$DOWNLOAD_URL" -o cleanup.tar.gz; then
        print_warning "Failed to download prebuilt binary."
        print_info "Falling back to building from source..."
        BUILD_FROM_SOURCE=1
        build_from_source
        return
    fi

    # Extract
    print_info "Extracting binary..."
    tar -xzf cleanup.tar.gz

    # Rename extracted binary to standard name
    mv "cleanup-${OS}-${ARCH}" "$BINARY_NAME"

    # Install
    print_info "Installing to $INSTALL_DIR..."
    $USE_SUDO mv "$BINARY_NAME" "$INSTALL_DIR/"
    $USE_SUDO chmod +x "$INSTALL_DIR/$BINARY_NAME"

    # Cleanup
    cd - > /dev/null
    rm -rf "$TMP_DIR"
}

# Create default configuration
create_config() {
    print_info "Setting up configuration..."

    if [ ! -d "$CONFIG_DIR" ]; then
        mkdir -p "$CONFIG_DIR"
        print_success "Created config directory: $CONFIG_DIR"
    fi

    if [ ! -f "$CONFIG_DIR/config.yaml" ]; then
        print_info "Creating default configuration file..."
        cat > "$CONFIG_DIR/config.yaml" << 'EOF'
# CleanupCache Configuration File
categories:
  cache: true
  temp: true
  logs: true
  duplicates: false
  downloads: false
  package_managers: true

age_thresholds:
  logs: 30
  downloads: 90
  temp: 7

size_limits:
  min_file_size: "1KB"
  max_file_size: "10GB"

exclude_patterns:
  - "*/important/*"
  - "*.keep"
  - "*/Documents/*"
  - "*/Pictures/*"
  - "*/Music/*"
  - "*/Videos/*"

dry_run: false
min_file_age: 1
verbose: false
EOF
        print_success "Created default config: $CONFIG_DIR/config.yaml"
    else
        print_info "Config file already exists, skipping..."
    fi
}

# Verify installation
verify_installation() {
    print_info "Verifying installation..."

    if command -v "$BINARY_NAME" &> /dev/null; then
        VERSION_OUTPUT=$($BINARY_NAME --version)
        print_success "Installation successful!"
        print_success "$VERSION_OUTPUT"
    else
        print_error "Installation failed. Binary not found in PATH."
        exit 1
    fi
}

# Print usage instructions
print_usage() {
    echo ""
    echo -e "${GREEN}╔══════════════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║         CleanupCache Successfully Installed!             ║${NC}"
    echo -e "${GREEN}╚══════════════════════════════════════════════════════════╝${NC}"
    echo ""
    echo -e "${BLUE}Quick Start:${NC}"
    echo ""
    echo -e "  ${YELLOW}cleanup scan${NC}             - Scan system for cleanable files"
    echo -e "  ${YELLOW}cleanup interactive${NC}      - Interactive mode with file browser"
    echo -e "  ${YELLOW}cleanup clean${NC}            - Clean based on configuration"
    echo -e "  ${YELLOW}cleanup report${NC}           - Generate detailed report"
    echo ""
    echo -e "${BLUE}Examples:${NC}"
    echo ""
    echo -e "  # Preview what would be deleted (dry-run)"
    echo -e "  ${YELLOW}cleanup clean --dry-run${NC}"
    echo ""
    echo -e "  # Interactive mode with file selection"
    echo -e "  ${YELLOW}cleanup interactive${NC}"
    echo ""
    echo -e "  # Clean specific category only"
    echo -e "  ${YELLOW}cleanup clean --category cache${NC}"
    echo ""
    echo -e "${BLUE}Configuration:${NC}"
    echo -e "  Config file: ${YELLOW}$CONFIG_DIR/config.yaml${NC}"
    echo -e "  Edit config: ${YELLOW}nano $CONFIG_DIR/config.yaml${NC}"
    echo ""
    echo -e "${BLUE}Documentation:${NC}"
    echo -e "  GitHub: ${YELLOW}https://github.com/$REPO${NC}"
    echo ""
}

# Main installation flow
main() {
    echo ""
    echo -e "${BLUE}╔══════════════════════════════════════════════════════════╗${NC}"
    echo -e "${BLUE}║           CleanupCache Installation Script              ║${NC}"
    echo -e "${BLUE}╚══════════════════════════════════════════════════════════╝${NC}"
    echo ""

    detect_platform
    check_root
    check_dependencies
    get_latest_version
    download_and_install
    create_config
    verify_installation
    print_usage
}

# Run main function
main
