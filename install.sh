#!/bin/bash

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
REPO="cjairm/rocketctl"
BINARY_NAME="rocketctl"
INSTALL_DIR="$HOME/.local/bin"

echo -e "${GREEN}Installing RocketCTL...${NC}\n"

# Detect OS and architecture
OS="$(uname -s)"
ARCH="$(uname -m)"

if [ "$OS" != "Darwin" ]; then
    echo -e "${RED}Error: This installer currently only supports macOS${NC}"
    echo "Please build from source using: go build -o rocketctl"
    exit 1
fi

# Map architecture to binary naming
case "$ARCH" in
    x86_64)
        BINARY_ARCH="amd64"
        ARCH_NAME="Intel"
        ;;
    arm64)
        BINARY_ARCH="arm64"
        ARCH_NAME="Apple Silicon"
        ;;
    *)
        echo -e "${RED}Error: Unsupported architecture: $ARCH${NC}"
        exit 1
        ;;
esac

echo -e "${BLUE}Detected: macOS ${ARCH_NAME} (${ARCH})${NC}\n"

# Function to download and install binary
install_prebuilt() {
    local version=$1
    local download_url="https://github.com/${REPO}/releases/download/${version}/rocketctl-darwin-${BINARY_ARCH}"
    
    echo -e "${YELLOW}Downloading pre-built binary from GitHub Releases...${NC}"
    echo "URL: $download_url"
    
    # Create temp directory
    TMP_DIR=$(mktemp -d)
    trap "rm -rf $TMP_DIR" EXIT
    
    # Download binary
    if curl -fsSL "$download_url" -o "$TMP_DIR/$BINARY_NAME"; then
        # Make executable
        chmod +x "$TMP_DIR/$BINARY_NAME"
        
        # Create installation directory if needed
        mkdir -p "$INSTALL_DIR"
        
        # Install binary
        mv "$TMP_DIR/$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"
        
        echo -e "${GREEN}✓ Downloaded and installed pre-built binary${NC}"
        return 0
    else
        echo -e "${YELLOW}Could not download pre-built binary${NC}"
        return 1
    fi
}

# Function to build from source
build_from_source() {
    echo -e "${YELLOW}Building from source...${NC}"
    
    # Check if Go is installed
    if ! command -v go &> /dev/null; then
        echo -e "${RED}Error: Go is not installed${NC}"
        echo "Please install Go from: https://golang.org/doc/install"
        echo "Or wait for pre-built binaries to be available"
        exit 1
    fi
    
    # Build the binary
    go build -o "$BINARY_NAME"
    
    # Create installation directory if needed
    mkdir -p "$INSTALL_DIR"
    
    # Move binary to installation directory
    mv "$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"
    chmod +x "$INSTALL_DIR/$BINARY_NAME"
    
    echo -e "${GREEN}✓ Built and installed from source${NC}"
}

# Try to get latest release version
echo -e "${YELLOW}Checking for latest release...${NC}"
LATEST_VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/' 2>/dev/null || echo "")

if [ -n "$LATEST_VERSION" ]; then
    echo -e "${GREEN}Latest release: ${LATEST_VERSION}${NC}\n"
    
    # Try to install pre-built binary
    if install_prebuilt "$LATEST_VERSION"; then
        INSTALL_METHOD="pre-built binary"
    else
        echo -e "${YELLOW}Falling back to building from source...${NC}\n"
        build_from_source
        INSTALL_METHOD="source"
    fi
else
    echo -e "${YELLOW}No releases found, building from source...${NC}\n"
    build_from_source
    INSTALL_METHOD="source"
fi

echo -e "${GREEN}✓ Binary installed to $INSTALL_DIR/$BINARY_NAME${NC}"

# Detect shell configuration file
SHELL_CONFIG=""
if [ -n "$ZSH_VERSION" ]; then
    SHELL_CONFIG="$HOME/.zshrc"
elif [ -n "$BASH_VERSION" ]; then
    if [ -f "$HOME/.bash_profile" ]; then
        SHELL_CONFIG="$HOME/.bash_profile"
    else
        SHELL_CONFIG="$HOME/.bashrc"
    fi
else
    # Default to .zshrc as it's common on macOS
    if [ -f "$HOME/.zshrc" ]; then
        SHELL_CONFIG="$HOME/.zshrc"
    elif [ -f "$HOME/.bash_profile" ]; then
        SHELL_CONFIG="$HOME/.bash_profile"
    else
        SHELL_CONFIG="$HOME/.bashrc"
    fi
fi

# Add to PATH if not already present
PATH_LINE="export PATH=\"\$HOME/.local/bin:\$PATH\""

if [ -f "$SHELL_CONFIG" ]; then
    # Check if PATH is already configured
    if grep -q "\.local/bin" "$SHELL_CONFIG"; then
        echo -e "${YELLOW}PATH already configured in $SHELL_CONFIG${NC}"
    else
        echo -e "${YELLOW}Adding $INSTALL_DIR to PATH in $SHELL_CONFIG...${NC}"
        echo "" >> "$SHELL_CONFIG"
        echo "# RocketCTL" >> "$SHELL_CONFIG"
        echo "$PATH_LINE" >> "$SHELL_CONFIG"
        echo -e "${GREEN}✓ PATH updated in $SHELL_CONFIG${NC}"
    fi
else
    echo -e "${YELLOW}Creating $SHELL_CONFIG...${NC}"
    echo "# RocketCTL" > "$SHELL_CONFIG"
    echo "$PATH_LINE" >> "$SHELL_CONFIG"
    echo -e "${GREEN}✓ Created $SHELL_CONFIG with PATH configuration${NC}"
fi

# Check if .local/bin is in current PATH
if [[ ":$PATH:" != *":$HOME/.local/bin:"* ]]; then
    export PATH="$HOME/.local/bin:$PATH"
    echo -e "${YELLOW}Added $INSTALL_DIR to current session PATH${NC}"
fi

# Verify installation
if command -v rocketctl &> /dev/null; then
    VERSION_OUTPUT=$(rocketctl --help 2>&1 | head -1)
    echo ""
    echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${GREEN}✓ Installation successful!${NC}"
    echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo ""
    echo -e "Installation method: ${GREEN}${INSTALL_METHOD}${NC}"
    echo -e "Installed to: ${GREEN}$INSTALL_DIR/$BINARY_NAME${NC}"
    echo ""
    echo -e "${YELLOW}To start using rocketctl in your current shell, run:${NC}"
    echo -e "  ${GREEN}source $SHELL_CONFIG${NC}"
    echo ""
    echo -e "${YELLOW}Or simply open a new terminal window.${NC}"
    echo ""
    echo -e "${YELLOW}To get started:${NC}"
    echo -e "  ${GREEN}rocketctl init${NC}    - Initialize a new project"
    echo -e "  ${GREEN}rocketctl --help${NC}  - Show all available commands"
    echo ""
else
    echo ""
    echo -e "${GREEN}Installation complete!${NC}"
    echo ""
    echo -e "${YELLOW}Please run the following command to use rocketctl:${NC}"
    echo -e "  ${GREEN}source $SHELL_CONFIG${NC}"
    echo ""
fi
