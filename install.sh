#!/bin/bash
# TidyUp - System Cleanup Tool Installation Script - v1.0.0
# https://github.com/fenilsonani/cleanup-cache
#
# Usage:
#   curl -sSL https://raw.githubusercontent.com/fenilsonani/cleanup-cache/main/install.sh | bash
#   curl -sSL ... | bash -s -- [OPTIONS]
#
# Options:
#   -v, --version VERSION   Install specific version (e.g., v0.4.1, 0.4.0, latest)
#   -l, --list-versions     List available versions (last 25)
#       --pre-release       Include alpha/beta/rc versions
#   -u, --update            Update existing installation
#       --uninstall         Remove tidyup completely
#   -f, --force             Skip confirmation prompts
#       --no-verify         Skip checksum verification (not recommended)
#   -h, --help              Show help message

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m' # No Color

# Configuration
BINARY_NAME="tidyup"
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="$HOME/.config/tidyup"
CACHE_DIR="$HOME/.cache/tidyup"
REPO="fenilsonani/cleanup-cache"
GITHUB_API="https://api.github.com/repos/$REPO"

# Old config location for migration
OLD_CONFIG_DIR="$HOME/.config/cleanup-cache"
OLD_BINARY_NAME="cleanup"
OLD_CACHE_DIR="$HOME/.cache/cleanup-cache"

# Command line flags
UPDATE_MODE=0
UNINSTALL_MODE=0
FORCE_MODE=0
LIST_VERSIONS=0
INCLUDE_PRERELEASE=0
SKIP_VERIFY=0
TARGET_VERSION=""
BUILD_FROM_SOURCE=0

# Print functions
print_info() {
    echo -e "${BLUE}>${NC} $1"
}

print_success() {
    echo -e "${GREEN}✓${NC} $1"
}

print_error() {
    echo -e "${RED}✗${NC} $1" >&2
}

print_warning() {
    echo -e "${YELLOW}!${NC} $1"
}

print_step() {
    echo -e "${CYAN}→${NC} $1"
}

# Print help message
print_help() {
    cat << 'HELPEOF'
TidyUp Installation Script
https://github.com/fenilsonani/cleanup-cache

Usage:
  curl -sSL https://raw.githubusercontent.com/fenilsonani/cleanup-cache/main/install.sh | bash
  curl -sSL ... | bash -s -- [OPTIONS]

Options:
  -v, --version VERSION   Install specific version (e.g., v0.4.1, 0.4.0, latest)
  -l, --list-versions     List available versions (last 25)
      --pre-release       Include alpha/beta/rc versions in list and install
  -u, --update            Update existing installation
      --uninstall         Remove tidyup completely
  -f, --force             Skip confirmation prompts
      --no-verify         Skip checksum verification (not recommended)
  -h, --help              Show this help message

Examples:
  # Install latest stable version
  curl -sSL .../install.sh | bash

  # Install specific version
  curl -sSL .../install.sh | bash -s -- -v v0.4.0
  curl -sSL .../install.sh | bash -s -- --version 0.4.1

  # List available versions
  curl -sSL .../install.sh | bash -s -- --list-versions
  curl -sSL .../install.sh | bash -s -- -l --pre-release

  # Update to latest
  curl -sSL .../install.sh | bash -s -- --update

  # Downgrade to older version
  curl -sSL .../install.sh | bash -s -- -u -v v0.3.0

  # Install pre-release version
  curl -sSL .../install.sh | bash -s -- --pre-release -v v0.5.0-beta

Security:
  Downloads are verified using SHA256 checksums by default.
  Use --no-verify to skip (not recommended).
HELPEOF
}

# Parse command line arguments
parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            -v|--version)
                if [[ -z "${2:-}" ]]; then
                    print_error "Option $1 requires a version argument"
                    exit 1
                fi
                TARGET_VERSION="$2"
                shift 2
                ;;
            -l|--list-versions|--list)
                LIST_VERSIONS=1
                shift
                ;;
            --pre-release|--prerelease)
                INCLUDE_PRERELEASE=1
                shift
                ;;
            -u|--update)
                UPDATE_MODE=1
                shift
                ;;
            --uninstall|--remove)
                UNINSTALL_MODE=1
                shift
                ;;
            -f|--force)
                FORCE_MODE=1
                shift
                ;;
            --no-verify|--skip-verify)
                SKIP_VERIFY=1
                shift
                ;;
            -h|--help)
                print_help
                exit 0
                ;;
            -*)
                print_error "Unknown option: $1"
                echo "Use --help for usage information"
                exit 1
                ;;
            *)
                # Positional argument - could be version without flag
                if [[ -z "$TARGET_VERSION" ]] && [[ "$1" =~ ^v?[0-9] ]]; then
                    TARGET_VERSION="$1"
                else
                    print_error "Unknown argument: $1"
                    echo "Use --help for usage information"
                    exit 1
                fi
                shift
                ;;
        esac
    done
}

# Reopen stdin from /dev/tty if we're being piped and need interactive input
setup_tty() {
    if [ ! -t 0 ] && [ "$FORCE_MODE" = "0" ]; then
        if ( exec < /dev/tty ) 2>/dev/null; then
            exec < /dev/tty
        else
            print_warning "No terminal available for interactive input. Using --force mode."
            FORCE_MODE=1
        fi
    fi
}

# List available versions from GitHub releases
list_available_versions() {
    echo ""
    print_info "Fetching available versions from GitHub..."
    echo ""

    local api_url="${GITHUB_API}/releases?per_page=25"
    local response

    response=$(curl --proto '=https' --tlsv1.2 -sS "$api_url" 2>/dev/null) || {
        print_error "Failed to fetch versions from GitHub"
        exit 1
    }

    local versions
    local prerelease_flags

    # Extract tag names and prerelease status
    versions=$(echo "$response" | grep -E '"tag_name"|"prerelease"' | paste - - | \
        sed 's/.*"tag_name": "\([^"]*\)".*"prerelease": \([^,}]*\).*/\1 \2/')

    if [ -z "$versions" ]; then
        print_error "Could not parse version list from GitHub"
        exit 1
    fi

    echo -e "${BOLD}Available versions:${NC}"
    echo ""

    local count=0
    local latest_shown=0

    while read -r tag prerelease; do
        # Skip pre-releases unless flag is set
        if [ "$prerelease" = "true" ] && [ "$INCLUDE_PRERELEASE" != "1" ]; then
            continue
        fi

        count=$((count + 1))

        local suffix=""
        if [ "$prerelease" = "true" ]; then
            suffix="${YELLOW}(pre-release)${NC}"
        elif [ "$latest_shown" = "0" ]; then
            suffix="${GREEN}(latest)${NC}"
            latest_shown=1
        fi

        if [ -n "$suffix" ]; then
            echo -e "  ${CYAN}$tag${NC} $suffix"
        else
            echo -e "  $tag"
        fi

        # Limit output
        if [ "$count" -ge 25 ]; then
            break
        fi
    done <<< "$versions"

    echo ""

    if [ "$INCLUDE_PRERELEASE" != "1" ]; then
        echo -e "  ${YELLOW}Tip:${NC} Use ${CYAN}--pre-release${NC} to include alpha/beta/rc versions"
    fi

    echo ""
    echo -e "  Install a specific version:"
    echo -e "    curl -sSL .../install.sh | bash -s -- ${CYAN}-v <version>${NC}"
    echo ""
}

# Validate version format
validate_version() {
    local version="$1"

    # Accept: latest, v0.4.1, 0.4.1, v0.5.0-beta, v0.5.0-rc.1
    if [ "$version" = "latest" ]; then
        return 0
    fi

    # Semantic version with optional pre-release suffix
    if [[ "$version" =~ ^v?[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9.]+)?$ ]]; then
        return 0
    fi

    return 1
}

# Check if version exists on GitHub
version_exists() {
    local version="$1"

    # Normalize version (add 'v' prefix if missing)
    if [[ ! "$version" =~ ^v ]]; then
        version="v$version"
    fi

    local url="${GITHUB_API}/releases/tags/${version}"
    local http_code

    http_code=$(curl --proto '=https' --tlsv1.2 -sI -o /dev/null -w "%{http_code}" "$url" 2>/dev/null) || return 1

    [ "$http_code" = "200" ]
}

# Normalize version string (ensure 'v' prefix)
normalize_version() {
    local version="$1"
    if [[ ! "$version" =~ ^v ]]; then
        echo "v$version"
    else
        echo "$version"
    fi
}

# Verify checksum of downloaded file
verify_checksum() {
    local file="$1"
    local version="$2"
    local filename="$3"

    if [ "$SKIP_VERIFY" = "1" ]; then
        print_warning "Skipping checksum verification (--no-verify)"
        return 0
    fi

    print_step "Verifying checksum..."

    # Download checksum file from release
    local checksum_url="https://github.com/${REPO}/releases/download/${version}/checksums.txt"
    local checksums

    checksums=$(curl --proto '=https' --tlsv1.2 -sL "$checksum_url" 2>/dev/null) || true

    if [ -z "$checksums" ]; then
        print_warning "Checksum file not available for this release"
        print_info "Skipping verification (older releases may not have checksums)"
        return 0
    fi

    local expected
    expected=$(echo "$checksums" | grep "$filename" | awk '{print $1}')

    if [ -z "$expected" ]; then
        print_warning "No checksum found for $filename"
        return 0
    fi

    # Use sha256sum on Linux, shasum on macOS
    local actual
    if command -v sha256sum >/dev/null 2>&1; then
        actual=$(sha256sum "$file" | awk '{print $1}')
    else
        actual=$(shasum -a 256 "$file" | awk '{print $1}')
    fi

    if [ "$expected" = "$actual" ]; then
        print_success "Checksum verified: ${actual:0:16}..."
        return 0
    else
        echo "" >&2
        echo -e "${RED}╔══════════════════════════════════════════════════════════════╗${NC}" >&2
        echo -e "${RED}║  SECURITY ERROR: Checksum verification failed!              ║${NC}" >&2
        echo -e "${RED}╚══════════════════════════════════════════════════════════════╝${NC}" >&2
        echo "" >&2
        echo -e "  Expected: ${CYAN}$expected${NC}" >&2
        echo -e "  Actual:   ${YELLOW}$actual${NC}" >&2
        echo "" >&2
        echo "  This could indicate:" >&2
        echo "    - Corrupted download" >&2
        echo "    - Man-in-the-middle attack" >&2
        echo "    - Tampered release file" >&2
        echo "" >&2
        echo "  Installation aborted for security." >&2
        echo -e "  If you trust this download, use ${YELLOW}--no-verify${NC} (not recommended)" >&2
        exit 1
    fi
}

# Check if TidyUp is already installed
check_existing_installation() {
    CURRENT_VERSION=""
    IS_INSTALLED=0

    if command -v "$BINARY_NAME" &> /dev/null; then
        IS_INSTALLED=1
        CURRENT_VERSION=$($BINARY_NAME --version 2>/dev/null | grep -oE 'v?[0-9]+\.[0-9]+\.[0-9]+' | head -1) || true
        if [ -z "$CURRENT_VERSION" ]; then
            CURRENT_VERSION="unknown"
        fi
    fi
}

# Compare semantic versions (returns 0 if $1 >= $2, 1 otherwise)
version_gte() {
    local v1="${1#v}"
    local v2="${2#v}"
    [ "$(printf '%s\n%s' "$v1" "$v2" | sort -V | head -n1)" = "$v2" ]
}

# Uninstall TidyUp
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

    BINARY_PATH="$INSTALL_DIR/$BINARY_NAME"
    if [ -f "$BINARY_PATH" ]; then
        print_step "Removing binary: $BINARY_PATH"
        $USE_SUDO rm -f "$BINARY_PATH"
        print_success "Binary removed"
    fi

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
            echo ""
            echo "Supported platforms:"
            echo "  - macOS (darwin) - Intel x86_64, Apple Silicon arm64"
            echo "  - Linux - x86_64, aarch64"
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
    if [ "${EUID:-$(id -u)}" -eq 0 ]; then
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
            missing_deps+=("$cmd")
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
    print_step "Fetching latest version from GitHub..."

    local api_url="${GITHUB_API}/releases/latest"
    local response

    response=$(curl --proto '=https' --tlsv1.2 -sS "$api_url" 2>/dev/null) || {
        print_warning "Could not fetch latest version from GitHub."
        print_info "Building from source instead..."
        BUILD_FROM_SOURCE=1
        return
    }

    VERSION=$(echo "$response" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

    if [ -z "$VERSION" ]; then
        print_warning "Could not parse version from GitHub response."
        print_info "Building from source instead..."
        BUILD_FROM_SOURCE=1
    else
        print_success "Latest version: $VERSION"
    fi
}

# Build from source (fallback if no release available)
build_from_source() {
    print_info "Building TidyUp from source..."

    if ! command -v go &> /dev/null; then
        print_error "Go is not installed. Please install Go 1.21+ and try again."
        print_info "Visit https://go.dev/doc/install for installation instructions."
        exit 1
    fi

    TMP_DIR=$(mktemp -d)
    # Cleanup on exit
    trap 'rm -rf "$TMP_DIR"' EXIT

    cd "$TMP_DIR"

    print_step "Cloning repository..."
    git clone --depth 1 "https://github.com/$REPO.git" . >/dev/null 2>&1

    print_step "Building binary..."
    go build -o "$BINARY_NAME" ./cmd/tidyup

    print_step "Installing binary to $INSTALL_DIR..."
    $USE_SUDO mv "$BINARY_NAME" "$INSTALL_DIR/"
    $USE_SUDO chmod +x "$INSTALL_DIR/$BINARY_NAME"

    cd - > /dev/null
    trap - EXIT
    rm -rf "$TMP_DIR"
}

# Download and install binary
download_and_install() {
    local version="${TARGET_VERSION:-$VERSION}"

    if [ "$BUILD_FROM_SOURCE" = "1" ]; then
        build_from_source
        return
    fi

    # Normalize version
    version=$(normalize_version "$version")

    # Validate version exists
    if [ -n "$TARGET_VERSION" ]; then
        print_step "Checking if version $version exists..."
        if ! version_exists "$version"; then
            print_error "Version $version not found on GitHub"
            echo ""
            echo "Use --list-versions to see available versions"
            exit 1
        fi
        print_success "Version $version found"
    fi

    # Construct download URL
    local filename="tidyup-${OS}-${ARCH}.tar.gz"
    local download_url="https://github.com/$REPO/releases/download/${version}/${filename}"

    print_step "Downloading TidyUp ${version}..."

    TMP_DIR=$(mktemp -d)
    trap 'rm -rf "$TMP_DIR"' EXIT

    cd "$TMP_DIR"

    # Download with retry
    local max_attempts=3
    local attempt=1

    while [ $attempt -le $max_attempts ]; do
        if curl --proto '=https' --tlsv1.2 -sSL "$download_url" -o tidyup.tar.gz 2>/dev/null; then
            break
        fi

        attempt=$((attempt + 1))
        if [ $attempt -le $max_attempts ]; then
            print_warning "Download failed, retrying... ($attempt/$max_attempts)"
            sleep 2
        fi
    done

    if [ $attempt -gt $max_attempts ]; then
        print_warning "Failed to download prebuilt binary after $max_attempts attempts."
        print_info "Falling back to building from source..."
        cd - > /dev/null
        trap - EXIT
        rm -rf "$TMP_DIR"
        build_from_source
        return
    fi

    # Check if download is a valid tar.gz
    if ! tar -tzf tidyup.tar.gz > /dev/null 2>&1; then
        print_warning "Downloaded file is not a valid archive."
        print_info "Falling back to building from source..."
        cd - > /dev/null
        trap - EXIT
        rm -rf "$TMP_DIR"
        build_from_source
        return
    fi

    # Verify checksum
    verify_checksum "tidyup.tar.gz" "$version" "$filename"

    # Extract
    print_step "Extracting binary..."
    tar -xzf tidyup.tar.gz

    # Find the binary
    EXTRACTED_BINARY=""
    for name in "tidyup-${OS}-${ARCH}" "tidyup" "${BINARY_NAME}"; do
        if [ -f "$name" ]; then
            EXTRACTED_BINARY="$name"
            break
        fi
    done

    if [ -z "$EXTRACTED_BINARY" ]; then
        print_warning "Binary not found in archive."
        print_info "Falling back to building from source..."
        cd - > /dev/null
        trap - EXIT
        rm -rf "$TMP_DIR"
        build_from_source
        return
    fi

    if [ "$EXTRACTED_BINARY" != "$BINARY_NAME" ]; then
        mv "$EXTRACTED_BINARY" "$BINARY_NAME"
    fi

    # Install
    print_step "Installing to $INSTALL_DIR..."

    if [ ! -d "$INSTALL_DIR" ]; then
        print_info "Creating $INSTALL_DIR..."
        $USE_SUDO mkdir -p "$INSTALL_DIR"
    fi

    $USE_SUDO mv "$BINARY_NAME" "$INSTALL_DIR/"
    $USE_SUDO chmod +x "$INSTALL_DIR/$BINARY_NAME"

    cd - > /dev/null
    trap - EXIT
    rm -rf "$TMP_DIR"
}

# Create default configuration
create_config() {
    print_step "Setting up configuration..."

    if [ ! -d "$CONFIG_DIR" ]; then
        mkdir -p "$CONFIG_DIR"
        print_success "Created config directory: $CONFIG_DIR"
    fi

    if [ ! -d "$CACHE_DIR" ]; then
        mkdir -p "$CACHE_DIR"
        print_success "Created cache directory: $CACHE_DIR"
    fi

    # Migrate from old config location
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
# https://github.com/fenilsonani/cleanup-cache

# Enable/disable cleanup categories
categories:
  cache: true              # Browser and app caches
  temp: true               # Temporary files
  logs: true               # Log files
  downloads: false         # Old downloads (disabled by default - review before enabling)
  package_managers: true   # Package manager caches (npm, brew, pip, etc.)
  docker: false            # Docker cleanup (requires Docker installation)
  node_modules: true       # Node.js dependencies
  virtual_envs: true       # Python virtual environments
  build_artifacts: true    # Build outputs (.next, dist, build, target)
  large_files: true        # Large files (>500MB by default)
  old_files: true          # Old unused files (>180 days by default)

# Age thresholds (in days)
age_thresholds:
  logs: 30
  downloads: 90
  temp: 7

# Size limits
size_limits:
  min_file_size: "1KB"
  max_file_size: "10GB"

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

# Large files detection
large_files:
  min_size: "500MB"
  scan_paths:
    - "~"
  exclude_paths:
    - "~/Library"
    - "~/.Trash"

# Old files detection
old_files:
  min_age_days: 180
  scan_paths:
    - "~/Downloads"
    - "~/Documents"
    - "~/Desktop"

# Runtime settings
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
    print_step "Verifying installation..."

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
    echo -e "${BOLD}Quick Start:${NC}"
    echo ""
    echo -e "  ${CYAN}tidyup scan${NC}              Scan system for cleanable files"
    echo -e "  ${CYAN}tidyup clean${NC}             Clean files based on configuration"
    echo -e "  ${CYAN}tidyup dev${NC}               Find dev artifacts (node_modules, etc.)"
    echo -e "  ${CYAN}tidyup large${NC}             Find large files"
    echo -e "  ${CYAN}tidyup old${NC}               Find old unused files"
    echo ""
    echo -e "${BOLD}Examples:${NC}"
    echo ""
    echo -e "  ${CYAN}tidyup clean --dry-run${NC}  Preview what would be deleted"
    echo -e "  ${CYAN}tidyup dev --clean${NC}      Clean all dev artifacts"
    echo ""
    echo -e "${BOLD}Performance:${NC}"
    echo ""
    echo -e "  First scan:   ~10 seconds (builds cache)"
    echo -e "  Cached scan:  ~0.2 seconds (near-instant)"
    echo ""
    echo -e "${BOLD}Configuration:${NC}"
    echo ""
    echo -e "  Config: ${YELLOW}$CONFIG_DIR/config.yaml${NC}"
    echo ""
    echo -e "${BOLD}Update/Uninstall:${NC}"
    echo ""
    echo -e "  Update:    curl -sSL .../install.sh | bash -s -- --update"
    echo -e "  Uninstall: curl -sSL .../install.sh | bash -s -- --uninstall"
    echo ""
    echo -e "Documentation: ${CYAN}https://github.com/$REPO${NC}"
    echo ""
}

# Print update success message
print_update_success() {
    echo ""
    echo -e "${GREEN}╔══════════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║              TidyUp Successfully Updated!                        ║${NC}"
    echo -e "${GREEN}╚══════════════════════════════════════════════════════════════════╝${NC}"
    echo ""
    local installed_version
    installed_version=$($BINARY_NAME --version 2>/dev/null | grep -oE 'v?[0-9]+\.[0-9]+\.[0-9]+' | head -1) || true
    echo -e "  ${YELLOW}$CURRENT_VERSION${NC} → ${GREEN}${installed_version:-$VERSION}${NC}"
    echo ""
    echo -e "  Run ${CYAN}tidyup --version${NC} to verify the update."
    echo -e "  Run ${CYAN}tidyup scan${NC} to test."
    echo ""
}

# Main installation flow
main() {
    # Parse arguments first
    parse_args "$@"

    # Handle list-versions mode
    if [ "$LIST_VERSIONS" = "1" ]; then
        list_available_versions
        exit 0
    fi

    # Setup tty for interactive input
    setup_tty

    # Handle uninstall mode
    if [ "$UNINSTALL_MODE" = "1" ]; then
        uninstall
    fi

    # Validate target version if specified
    if [ -n "$TARGET_VERSION" ]; then
        if ! validate_version "$TARGET_VERSION"; then
            print_error "Invalid version format: $TARGET_VERSION"
            echo ""
            echo "Expected formats: v0.4.1, 0.4.1, v0.5.0-beta, latest"
            exit 1
        fi
        print_info "Target version: $TARGET_VERSION"
    fi

    # Check for existing installation
    check_existing_installation

    # Determine mode based on existing installation and flags
    if [ "$IS_INSTALLED" = "1" ]; then
        if [ "$UPDATE_MODE" = "1" ] || [ -n "$TARGET_VERSION" ]; then
            echo ""
            echo -e "${BLUE}╔══════════════════════════════════════════════════════════════════╗${NC}"
            echo -e "${BLUE}║                      TidyUp Update                                ║${NC}"
            echo -e "${BLUE}╚══════════════════════════════════════════════════════════════════╝${NC}"
            echo ""
            print_info "Current version: $CURRENT_VERSION"
        else
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
                UPDATE_MODE=1
            fi
        fi
    else
        echo ""
        echo -e "${BLUE}╔══════════════════════════════════════════════════════════════════╗${NC}"
        echo -e "${BLUE}║                  TidyUp Installation Script                       ║${NC}"
        echo -e "${BLUE}╚══════════════════════════════════════════════════════════════════╝${NC}"
        echo ""
    fi

    detect_platform
    check_root
    check_dependencies

    # Get version to install
    if [ -n "$TARGET_VERSION" ]; then
        VERSION=$(normalize_version "$TARGET_VERSION")
        print_info "Installing version: $VERSION"
    else
        get_latest_version
    fi

    # Check if update is actually needed (unless specific version requested)
    if [ "$UPDATE_MODE" = "1" ] && [ "$IS_INSTALLED" = "1" ] && [ -z "$TARGET_VERSION" ]; then
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
            fi
            print_info "Building from source (installed version is newer than release)..."
            BUILD_FROM_SOURCE=1
        else
            print_info "New version available: $VERSION"
        fi
    fi

    download_and_install
    create_config
    verify_installation

    if [ "$UPDATE_MODE" = "1" ] && [ "$IS_INSTALLED" = "1" ]; then
        print_update_success
    else
        print_usage
    fi
}

# Run main function with all arguments
main "$@"
