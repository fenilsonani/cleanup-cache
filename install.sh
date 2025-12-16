#!/bin/bash
# TidyUp - System Cleanup Tool Installation Script - v0.5.0
# Usage: curl -sSL https://raw.githubusercontent.com/fenilsonani/system-cleanup/main/install.sh | bash
# Update: curl -sSL https://raw.githubusercontent.com/fenilsonani/system-cleanup/main/install.sh | bash -s -- --update
# Uninstall: curl -sSL https://raw.githubusercontent.com/fenilsonani/system-cleanup/main/install.sh | bash -s -- --uninstall

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Configuration
BINARY_NAME="tidyup"
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="$HOME/.config/tidyup"
CACHE_DIR="$HOME/.cache/tidyup"
REPO="fenilsonani/cleanup-cache"
GITHUB_API="https://api.github.com/repos/$REPO/releases/latest"

# Old config location for migration
OLD_CONFIG_DIR="$HOME/.config/cleanup-cache"
OLD_BINARY_NAME="cleanup"
OLD_CACHE_DIR="$HOME/.cache/cleanup-cache"

# Command line flags
UPDATE_MODE=0
UNINSTALL_MODE=0
FORCE_MODE=0

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --update|-u)
            UPDATE_MODE=1
            shift
            ;;
        --uninstall|--remove)
            UNINSTALL_MODE=1
            shift
            ;;
        --force|-f)
            FORCE_MODE=1
            shift
            ;;
        --help|-h)
            echo "TidyUp - System Cleanup Tool Installation Script"
            echo ""
            echo "Usage:"
            echo "  install.sh [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --update, -u      Update existing installation to latest version"
            echo "  --uninstall       Remove TidyUp from the system"
            echo "  --force, -f       Skip confirmation prompts"
            echo "  --help, -h        Show this help message"
            echo ""
            echo "Performance: First scan ~10s, cached scans ~0.2s"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            echo "Use --help for usage information"
            exit 1
            ;;
    esac
done

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

# Reopen stdin from /dev/tty if we're being piped and need interactive input
# This allows interactive prompts to work with `curl | bash`
# Skip if running in force mode (no interactive input needed)
setup_tty() {
    if [ ! -t 0 ] && [ "$FORCE_MODE" = "0" ]; then
        # Try to open /dev/tty - it may exist but not be usable
        if ( exec < /dev/tty ) 2>/dev/null; then
            exec < /dev/tty
        else
            # No tty available and not in force mode - switch to force mode
            print_warning "No terminal available for interactive input. Using --force mode."
            FORCE_MODE=1
        fi
    fi
}

# Check if Tidy is already installed
check_existing_installation() {
    CURRENT_VERSION=""
    IS_INSTALLED=0

    if command -v "$BINARY_NAME" &> /dev/null; then
        IS_INSTALLED=1
        # Extract version from output like "cleanup version 0.3.0 (commit: xxx, built: xxx)"
        CURRENT_VERSION=$($BINARY_NAME --version 2>/dev/null | grep -oE 'v?[0-9]+\.[0-9]+\.[0-9]+' | head -1)
        if [ -z "$CURRENT_VERSION" ]; then
            CURRENT_VERSION="unknown"
        fi
    fi
}

# Compare semantic versions (returns 0 if $1 >= $2, 1 otherwise)
version_gte() {
    # Remove 'v' prefix if present
    local v1="${1#v}"
    local v2="${2#v}"

    # Use sort -V for version comparison
    [ "$(printf '%s\n%s' "$v1" "$v2" | sort -V | head -n1)" = "$v2" ]
}

# Uninstall Tidy
uninstall() {
    echo ""
    echo -e "${YELLOW}╔══════════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${YELLOW}║                    TidyUp Uninstallation                         ║${NC}"
    echo -e "${YELLOW}╚══════════════════════════════════════════════════════════════════╝${NC}"
    echo ""

    check_existing_installation

    if [ "$IS_INSTALLED" = "0" ]; then
        print_warning "TidyUp is not installed."
        exit 0
    fi

    print_info "Found TidyUp $CURRENT_VERSION"

    # Confirm uninstall
    if [ "$FORCE_MODE" = "0" ]; then
        echo ""
        print_warning "This will remove TidyUp from your system."
        echo -n "Are you sure you want to uninstall? (y/N): "
        read -r response
        if [[ ! "$response" =~ ^[Yy]$ ]]; then
            print_info "Uninstallation cancelled."
            exit 0
        fi
    fi

    check_root

    # Remove binary
    BINARY_PATH="$INSTALL_DIR/$BINARY_NAME"
    if [ -f "$BINARY_PATH" ]; then
        print_info "Removing binary: $BINARY_PATH"
        $USE_SUDO rm -f "$BINARY_PATH"
        print_success "Binary removed"
    fi

    # Ask about config and cache
    if [ -d "$CONFIG_DIR" ] || [ -d "$CACHE_DIR" ]; then
        echo ""
        if [ "$FORCE_MODE" = "0" ]; then
            echo -n "Remove configuration and cache directories? (y/N): "
            read -r response
            if [[ "$response" =~ ^[Yy]$ ]]; then
                [ -d "$CONFIG_DIR" ] && rm -rf "$CONFIG_DIR" && print_success "Configuration removed: $CONFIG_DIR"
                [ -d "$CACHE_DIR" ] && rm -rf "$CACHE_DIR" && print_success "Cache removed: $CACHE_DIR"
            else
                print_info "Configuration preserved at: $CONFIG_DIR"
                print_info "Cache preserved at: $CACHE_DIR"
            fi
        else
            [ -d "$CONFIG_DIR" ] && rm -rf "$CONFIG_DIR" && print_success "Configuration removed"
            [ -d "$CACHE_DIR" ] && rm -rf "$CACHE_DIR" && print_success "Cache removed"
        fi
    fi

    # Also clean up old locations if they exist
    [ -d "$OLD_CONFIG_DIR" ] && rm -rf "$OLD_CONFIG_DIR" && print_info "Removed old config: $OLD_CONFIG_DIR"
    [ -d "$OLD_CACHE_DIR" ] && rm -rf "$OLD_CACHE_DIR" && print_info "Removed old cache: $OLD_CACHE_DIR"

    echo ""
    print_success "TidyUp has been uninstalled."
    exit 0
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
    print_info "Building TidyUp from source..."

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
    go build -o "$BINARY_NAME" ./cmd/tidyup

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
    DOWNLOAD_URL="https://github.com/$REPO/releases/download/${VERSION}/tidyup-${OS}-${ARCH}.tar.gz"

    print_info "Downloading TidyUp ${VERSION}..."

    # Create temporary directory
    TMP_DIR=$(mktemp -d)
    cd "$TMP_DIR"

    # Download
    if ! curl -sSL "$DOWNLOAD_URL" -o tidyup.tar.gz; then
        print_warning "Failed to download prebuilt binary."
        print_info "Falling back to building from source..."
        BUILD_FROM_SOURCE=1
        build_from_source
        return
    fi

    # Extract
    print_info "Extracting binary..."
    tar -xzf tidyup.tar.gz

    # Debug: Show what was extracted
    print_info "Extracted files:"
    ls -la

    # Check if the expected binary exists
    EXTRACTED_BINARY="tidyup-${OS}-${ARCH}"
    if [ ! -f "$EXTRACTED_BINARY" ]; then
        print_error "Expected binary not found: $EXTRACTED_BINARY"
        print_error "Contents of directory:"
        ls -la
        exit 1
    fi

    # Rename extracted binary to standard name
    print_info "Renaming $EXTRACTED_BINARY to $BINARY_NAME..."
    mv "$EXTRACTED_BINARY" "$BINARY_NAME"

    # Verify rename succeeded
    if [ ! -f "$BINARY_NAME" ]; then
        print_error "Failed to rename binary"
        exit 1
    fi

    # Install
    print_info "Installing to $INSTALL_DIR..."

    # Ensure install directory exists
    if [ ! -d "$INSTALL_DIR" ]; then
        print_info "Creating $INSTALL_DIR..."
        $USE_SUDO mkdir -p "$INSTALL_DIR"
    fi

    $USE_SUDO mv "$BINARY_NAME" "$INSTALL_DIR/"
    $USE_SUDO chmod +x "$INSTALL_DIR/$BINARY_NAME"

    # Cleanup
    cd - > /dev/null
    rm -rf "$TMP_DIR"
}

# Create default configuration
create_config() {
    print_info "Setting up configuration..."

    # Create config directory
    if [ ! -d "$CONFIG_DIR" ]; then
        mkdir -p "$CONFIG_DIR"
        print_success "Created config directory: $CONFIG_DIR"
    fi

    # Create cache directory for scan cache
    if [ ! -d "$CACHE_DIR" ]; then
        mkdir -p "$CACHE_DIR"
        print_success "Created cache directory: $CACHE_DIR"
    fi

    # Migrate from old config location if it exists
    if [ -d "$OLD_CONFIG_DIR" ] && [ ! -f "$CONFIG_DIR/config.yaml" ]; then
        if [ -f "$OLD_CONFIG_DIR/config.yaml" ]; then
            print_info "Migrating config from $OLD_CONFIG_DIR..."
            cp "$OLD_CONFIG_DIR/config.yaml" "$CONFIG_DIR/config.yaml"
            print_success "Config migrated successfully"
        fi
    fi

    if [ ! -f "$CONFIG_DIR/config.yaml" ]; then
        print_info "Creating default configuration file..."
        cat > "$CONFIG_DIR/config.yaml" << 'EOF'
# TidyUp - System Cleanup Tool Configuration
# https://github.com/fenilsonani/system-cleanup

# Enable/disable cleanup categories
categories:
  cache: true              # Browser and app caches
  temp: true               # Temporary files
  logs: true               # Log files
  downloads: false         # Old downloads (disabled by default - review before enabling)
  package_managers: true   # Package manager caches (npm, brew, pip, etc.)
  docker: false            # Docker cleanup (requires Docker installation)
  # Development artifacts - enabled by default
  node_modules: true       # Node.js dependencies (finds node_modules with package.json)
  virtual_envs: true       # Python virtual environments (venv, .venv)
  build_artifacts: true    # Build outputs (.next, dist, build, target, __pycache__)
  # Large and old file detection - uses Spotlight for speed on macOS
  large_files: true        # Large files (>500MB by default)
  old_files: true          # Old unused files (>180 days by default)

# Age thresholds (in days)
age_thresholds:
  logs: 30                 # Clean log files older than 30 days
  downloads: 90            # Clean downloads older than 90 days
  temp: 7                  # Clean temp files older than 7 days

# Size limits
size_limits:
  min_file_size: "1KB"     # Ignore files smaller than this
  max_file_size: "10GB"    # Skip files larger than this (safety measure)

# Development project directories to scan
dev:
  project_dirs:
    - "~/Projects"
    - "~/Developer"
    - "~/Code"
    - "~/work"
    - "~/src"
    - "~/repos"
  build_patterns:
    - "node_modules"
    - ".next"
    - "dist"
    - "build"
    - "target"
    - "__pycache__"
    - ".gradle"
    - "*.egg-info"
    - ".tox"
    - ".pytest_cache"
    - "vendor"
    - ".bundle"
    - "Pods"

# Large files detection config
large_files:
  min_size: "500MB"        # Files larger than this are flagged
  scan_paths:
    - "~"
  exclude_paths:
    - "~/Library"
    - "~/.Trash"
    - "/System"
    - "/Applications"
    - "~/.local"
  file_types:              # Focus on these file types
    - ".mp4"
    - ".mkv"
    - ".avi"
    - ".mov"
    - ".iso"
    - ".dmg"
    - ".zip"
    - ".tar.gz"
    - ".rar"
    - ".7z"

# Old files detection config
old_files:
  min_age_days: 180        # Files not accessed in this many days
  scan_paths:
    - "~/Downloads"
    - "~/Documents"
    - "~/Desktop"
  exclude_paths:
    - "~/Documents/Work"
    - "~/Documents/Important"

# Exclude patterns (glob patterns)
exclude_patterns:
  - "*/important/*"
  - "*.keep"
  - "*/Documents/*"
  - "*/Pictures/*"
  - "*/Music/*"
  - "*/Videos/*"
  - "*/Movies/*"

# Protected paths - these will never be touched
protected_paths:
  - "/"
  - "/System"
  - "/Applications"
  - "/usr"
  - "/etc"
  - "/var"

# Runtime settings
dry_run: false             # Set to true to preview without deleting
min_file_age: 1            # Never delete files younger than 1 hour
verbose: false             # Show detailed output

# Docker cleanup settings (only if docker category is enabled)
docker:
  enabled: false
  clean_images: true
  clean_containers: true
  clean_volumes: false     # Disabled - volumes may contain data
  clean_build_cache: true
  only_dangling_images: true
  only_stopped_containers: true
  image_age_days: 7
  container_age_days: 1
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
    echo -e "${GREEN}╔══════════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║              TidyUp Successfully Installed!                      ║${NC}"
    echo -e "${GREEN}╚══════════════════════════════════════════════════════════════════╝${NC}"
    echo ""
    echo -e "${BLUE}Quick Start:${NC}"
    echo ""
    echo -e "  ${YELLOW}tidyup scan${NC}                  Scan system for cleanable files"
    echo -e "  ${YELLOW}tidyup clean${NC}                 Clean files based on configuration"
    echo -e "  ${YELLOW}tidyup scan --detailed${NC}       Show detailed breakdown by location"
    echo -e "  ${YELLOW}tidyup scan --live${NC}           Show live progress during scan"
    echo ""
    echo -e "${BLUE}Category-specific scanning:${NC}"
    echo ""
    echo -e "  ${YELLOW}tidyup clean --category cache${NC}         Clean only cache files"
    echo -e "  ${YELLOW}tidyup clean --category node_modules${NC}  Clean only node_modules"
    echo -e "  ${YELLOW}tidyup clean --category build_artifacts${NC} Clean build outputs"
    echo ""
    echo -e "${BLUE}Examples:${NC}"
    echo ""
    echo -e "  # Preview what would be deleted (dry-run)"
    echo -e "  ${YELLOW}tidyup clean --dry-run${NC}"
    echo ""
    echo -e "  # Clean a specific category"
    echo -e "  ${YELLOW}tidyup clean --category temp${NC}"
    echo ""
    echo -e "${BLUE}Performance:${NC}"
    echo ""
    echo -e "  ${CYAN}First scan:${NC}   ~10 seconds (builds cache)"
    echo -e "  ${CYAN}Cached scan:${NC}  ~0.2 seconds (near-instant)"
    echo ""
    echo -e "  TidyUp uses intelligent caching and Spotlight (macOS) for"
    echo -e "  blazing fast scans. Cache is stored at: ${YELLOW}$CACHE_DIR${NC}"
    echo ""
    echo -e "${BLUE}Configuration:${NC}"
    echo ""
    echo -e "  Config file: ${YELLOW}$CONFIG_DIR/config.yaml${NC}"
    echo -e "  Edit config: ${YELLOW}nano $CONFIG_DIR/config.yaml${NC}"
    echo ""
    echo -e "${BLUE}Categories available:${NC}"
    echo ""
    echo -e "  cache, temp, logs, downloads, package_managers, docker"
    echo -e "  node_modules, virtual_envs, build_artifacts"
    echo -e "  large_files, old_files"
    echo ""
    echo -e "${BLUE}Documentation:${NC}"
    echo ""
    echo -e "  GitHub: ${YELLOW}https://github.com/$REPO${NC}"
    echo ""
}

# Print update success message
print_update_success() {
    echo ""
    echo -e "${GREEN}╔══════════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║              TidyUp Successfully Updated!                        ║${NC}"
    echo -e "${GREEN}╚══════════════════════════════════════════════════════════════════╝${NC}"
    echo ""
    echo -e "  Updated: ${YELLOW}$CURRENT_VERSION${NC} → ${GREEN}$VERSION${NC}"
    echo ""
    echo -e "  Run ${YELLOW}tidyup --version${NC} to verify the update."
    echo -e "  Run ${YELLOW}tidyup scan${NC} to test (cached scans are near-instant!)"
    echo ""
}

# Main installation flow
main() {
    # Setup tty for interactive input if needed
    setup_tty

    # Handle uninstall mode
    if [ "$UNINSTALL_MODE" = "1" ]; then
        uninstall
    fi

    # Check for existing installation
    check_existing_installation

    # Determine mode based on existing installation and flags
    if [ "$IS_INSTALLED" = "1" ]; then
        if [ "$UPDATE_MODE" = "1" ]; then
            # Explicit update mode
            echo ""
            echo -e "${BLUE}╔══════════════════════════════════════════════════════════════════╗${NC}"
            echo -e "${BLUE}║                      TidyUp Update                                ║${NC}"
            echo -e "${BLUE}╚══════════════════════════════════════════════════════════════════╝${NC}"
            echo ""
            print_info "Current version: $CURRENT_VERSION"
        else
            # Already installed, prompt for action
            echo ""
            echo -e "${BLUE}╔══════════════════════════════════════════════════════════════════╗${NC}"
            echo -e "${BLUE}║                  TidyUp Already Installed                        ║${NC}"
            echo -e "${BLUE}╚══════════════════════════════════════════════════════════════════╝${NC}"
            echo ""
            print_info "Current version: $CURRENT_VERSION"
            echo ""

            if [ "$FORCE_MODE" = "0" ]; then
                echo "What would you like to do?"
                echo ""
                echo "  1) Update to latest version"
                echo "  2) Reinstall current version"
                echo "  3) Cancel"
                echo ""
                echo -n "Enter choice [1-3]: "
                read -r choice

                case $choice in
                    1)
                        UPDATE_MODE=1
                        ;;
                    2)
                        print_info "Reinstalling..."
                        ;;
                    3|*)
                        print_info "Installation cancelled."
                        exit 0
                        ;;
                esac
            else
                # Force mode - just update
                UPDATE_MODE=1
            fi
        fi
    else
        # Fresh install
        echo ""
        echo -e "${BLUE}╔══════════════════════════════════════════════════════════════════╗${NC}"
        echo -e "${BLUE}║                  TidyUp Installation Script                       ║${NC}"
        echo -e "${BLUE}╚══════════════════════════════════════════════════════════════════╝${NC}"
        echo ""
    fi

    detect_platform
    check_root
    check_dependencies
    get_latest_version

    # Check if update is needed
    if [ "$UPDATE_MODE" = "1" ] && [ "$IS_INSTALLED" = "1" ]; then
        # Compare versions
        LATEST_VERSION="${VERSION#v}"
        INSTALLED_VERSION="${CURRENT_VERSION#v}"

        if [ "$CURRENT_VERSION" != "unknown" ] && version_gte "$INSTALLED_VERSION" "$LATEST_VERSION"; then
            print_success "You already have the latest version ($CURRENT_VERSION)"
            echo ""
            if [ "$FORCE_MODE" = "0" ]; then
                echo -n "Reinstall anyway? (y/N): "
                read -r response
                if [[ ! "$response" =~ ^[Yy]$ ]]; then
                    print_info "No changes made."
                    exit 0
                fi
            else
                print_info "No changes made."
                exit 0
            fi
        else
            print_info "New version available: $VERSION"
        fi
    fi

    download_and_install
    create_config
    verify_installation

    # Show appropriate message
    if [ "$UPDATE_MODE" = "1" ] && [ "$IS_INSTALLED" = "1" ]; then
        print_update_success
    else
        print_usage
    fi
}

# Run main function
main
