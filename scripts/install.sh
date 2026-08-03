#!/bin/bash
# skillf installer
# Usage: curl -fsSL https://raw.githubusercontent.com/Sanix-Darker/skill-md.dev/main/scripts/install.sh | bash

set -e

# Configuration
REPO="Sanix-Darker/skill-md.dev"
BINARY_NAME="skillf"
INSTALL_DIR="/usr/local/bin"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Detect OS and architecture
detect_platform() {
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)

    case $OS in
        linux) OS="linux" ;;
        darwin) OS="darwin" ;;
        mingw*|msys*|cygwin*) OS="windows" ;;
        *)
            echo -e "${RED}Unsupported OS: $OS${NC}"
            exit 1
            ;;
    esac

    case $ARCH in
        x86_64|amd64) ARCH="amd64" ;;
        arm64|aarch64) ARCH="arm64" ;;
        *)
            echo -e "${RED}Unsupported architecture: $ARCH${NC}"
            exit 1
            ;;
    esac

    echo "${OS}-${ARCH}"
}

# Get latest version
get_latest_version() {
    curl -s "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/'
}

# Download and install
install() {
    PLATFORM=$(detect_platform)
    VERSION=$(get_latest_version)

    if [ -z "$VERSION" ]; then
        echo -e "${YELLOW}Could not determine latest version, using 'latest'${NC}"
        VERSION="latest"
    fi

    echo -e "${GREEN}Installing skillf ${VERSION} for ${PLATFORM}...${NC}"

    # Construct download URL
    if [ "$OS" = "windows" ]; then
        FILENAME="${BINARY_NAME}-${PLATFORM}.exe"
    else
        FILENAME="${BINARY_NAME}-${PLATFORM}"
    fi

    DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${VERSION}/${FILENAME}"

    # Create temp directory
    TMP_DIR=$(mktemp -d)
    trap "rm -rf $TMP_DIR" EXIT

    # Download
    echo "Downloading from ${DOWNLOAD_URL}..."
    if ! curl -fsSL "$DOWNLOAD_URL" -o "$TMP_DIR/$FILENAME"; then
        echo -e "${RED}Download failed. Trying to build from source...${NC}"
        install_from_source
        return
    fi

    # Verify checksum if available
    CHECKSUM_URL="https://github.com/${REPO}/releases/download/${VERSION}/checksums.txt"
    echo "Verifying checksum..."
    if curl -fsSL "$CHECKSUM_URL" -o "$TMP_DIR/checksums.txt" 2>/dev/null; then
        cd "$TMP_DIR"
        grep " ${FILENAME}\$" checksums.txt > "${FILENAME}.sha256" || true
        if command -v sha256sum &> /dev/null; then
            if [ -s "${FILENAME}.sha256" ] && ! sha256sum -c "${FILENAME}.sha256" 2>/dev/null; then
                echo -e "${RED}Checksum verification failed!${NC}"
                echo -e "${RED}The downloaded file may be corrupted or tampered with.${NC}"
                exit 1
            fi
            if [ -s "${FILENAME}.sha256" ]; then
                echo -e "${GREEN}Checksum verified successfully.${NC}"
            else
                echo -e "${YELLOW}Warning: No checksum entry found for ${FILENAME}, skipping verification${NC}"
            fi
        elif command -v shasum &> /dev/null; then
            # macOS fallback
            if [ -s "${FILENAME}.sha256" ] && ! shasum -a 256 -c "${FILENAME}.sha256" 2>/dev/null; then
                echo -e "${RED}Checksum verification failed!${NC}"
                echo -e "${RED}The downloaded file may be corrupted or tampered with.${NC}"
                exit 1
            fi
            if [ -s "${FILENAME}.sha256" ]; then
                echo -e "${GREEN}Checksum verified successfully.${NC}"
            else
                echo -e "${YELLOW}Warning: No checksum entry found for ${FILENAME}, skipping verification${NC}"
            fi
        else
            echo -e "${YELLOW}Warning: sha256sum not available, skipping verification${NC}"
        fi
        cd - > /dev/null
    else
        echo -e "${YELLOW}Warning: Checksum file not available, skipping verification${NC}"
    fi

    # Make executable
    chmod +x "$TMP_DIR/$FILENAME"

    # Install
    if [ -w "$INSTALL_DIR" ]; then
        mv "$TMP_DIR/$FILENAME" "$INSTALL_DIR/$BINARY_NAME"
    else
        echo "Installing to $INSTALL_DIR (requires sudo)..."
        sudo mv "$TMP_DIR/$FILENAME" "$INSTALL_DIR/$BINARY_NAME"
    fi

    echo -e "${GREEN}skillf installed successfully!${NC}"
    echo ""
    echo "Run 'skillf --help' to get started."
    echo "Start the server with 'skillf serve'"
}

# Install from source
install_from_source() {
    echo "Building from source..."

    if ! command -v go &> /dev/null; then
        echo -e "${RED}Go is required to build from source. Please install Go 1.23+${NC}"
        exit 1
    fi

    if ! command -v git &> /dev/null; then
        echo -e "${RED}git is required to build from source fallback.${NC}"
        exit 1
    fi

    SRC_DIR="$TMP_DIR/src"
    git clone --depth 1 "https://github.com/${REPO}.git" "$SRC_DIR" >/dev/null 2>&1
    (
        cd "$SRC_DIR"
        go build -trimpath -ldflags="-s -w" -o "$TMP_DIR/$BINARY_NAME" ./cmd/skillf
    )

    if [ -w "$INSTALL_DIR" ]; then
        mv "$TMP_DIR/$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"
    else
        echo "Installing to $INSTALL_DIR (requires sudo)..."
        sudo mv "$TMP_DIR/$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"
    fi

    echo -e "${GREEN}skillf installed from source!${NC}"
}

# Main
main() {
    echo ""
    echo "  _____ _    _ _ _   _____"
    echo " / ____| |  (_) | | |  ___|"
    echo "| (___ | | ___| | | | |_ ___  _ __ __ _  ___"
    echo " \\___ \\| |/ / | | | |  _/ _ \\| '__/ _\` |/ _ \\"
    echo " ____) |   <| | | | | || (_) | | | (_| |  __/"
    echo "|_____/|_|\\_\\_|_|_| |_| \\___/|_|  \\__, |\\___|"
    echo "                                   __/ |"
    echo "                                  |___/"
    echo ""

    install
}

main
