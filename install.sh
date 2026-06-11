#!/bin/sh
# Caged CLI installer
# Usage: curl -fsSL https://get.caged.dev | sh
#
# Installs the latest caged CLI binary for your platform.

set -e

REPO="caged-dev/cli"
BINARY="caged"
INSTALL_DIR="/usr/local/bin"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

info() { printf "${GREEN}▸${NC} %s\n" "$1"; }
warn() { printf "${YELLOW}▸${NC} %s\n" "$1"; }
error() { printf "${RED}▸${NC} %s\n" "$1" >&2; exit 1; }

# Detect OS
detect_os() {
    case "$(uname -s)" in
        Linux*)  echo "linux" ;;
        Darwin*) echo "darwin" ;;
        *)       error "Unsupported OS: $(uname -s). Use macOS or Linux." ;;
    esac
}

# Detect architecture
detect_arch() {
    case "$(uname -m)" in
        x86_64|amd64)  echo "amd64" ;;
        arm64|aarch64) echo "arm64" ;;
        *)             error "Unsupported architecture: $(uname -m). Use amd64 or arm64." ;;
    esac
}

# Get latest version from GitHub
get_latest_version() {
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/'
    elif command -v wget >/dev/null 2>&1; then
        wget -qO- "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/'
    else
        error "curl or wget is required"
    fi
}

# Download and install
install() {
    OS=$(detect_os)
    ARCH=$(detect_arch)
    VERSION=$(get_latest_version)

    if [ -z "$VERSION" ]; then
        error "Failed to determine latest version"
    fi

    # Strip 'v' prefix for archive name
    VERSION_NUM="${VERSION#v}"

    ARCHIVE="caged_${VERSION_NUM}_${OS}_${ARCH}.tar.gz"
    URL="https://github.com/${REPO}/releases/download/${VERSION}/${ARCHIVE}"

    info "Installing caged ${VERSION} (${OS}/${ARCH})..."

    # Create temp directory
    TMP_DIR=$(mktemp -d)
    trap "rm -rf ${TMP_DIR}" EXIT

    # Download
    info "Downloading ${URL}..."
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL "$URL" -o "${TMP_DIR}/${ARCHIVE}"
    elif command -v wget >/dev/null 2>&1; then
        wget -q "$URL" -O "${TMP_DIR}/${ARCHIVE}"
    fi

    # Extract
    tar xzf "${TMP_DIR}/${ARCHIVE}" -C "${TMP_DIR}"

    # Install
    if [ -w "$INSTALL_DIR" ]; then
        mv "${TMP_DIR}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
    else
        info "Installing to ${INSTALL_DIR} (requires sudo)..."
        sudo mv "${TMP_DIR}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
    fi

    chmod +x "${INSTALL_DIR}/${BINARY}"

    # Verify
    if command -v caged >/dev/null 2>&1; then
        info "Installed successfully: $(caged version)"
        echo ""
        info "Get started:"
        echo "  caged login"
        echo "  caged sandboxes create --template node"
        echo ""
        info "Docs: https://docs.caged.dev"
    else
        warn "Installed to ${INSTALL_DIR}/${BINARY} but it's not in your PATH."
        warn "Add ${INSTALL_DIR} to your PATH, or move the binary:"
        echo "  export PATH=\"${INSTALL_DIR}:\$PATH\""
    fi
}

install
