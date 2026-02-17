#!/bin/bash

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

INSTALL_DIR="$HOME/.local/bin"
BINARY_NAME="rocketctl"

echo -e "${YELLOW}Uninstalling RocketCTL...${NC}\n"

# Remove binary
if [ -f "$INSTALL_DIR/$BINARY_NAME" ]; then
    rm "$INSTALL_DIR/$BINARY_NAME"
    echo -e "${GREEN}✓ Removed $INSTALL_DIR/$BINARY_NAME${NC}"
else
    echo -e "${YELLOW}Binary not found at $INSTALL_DIR/$BINARY_NAME${NC}"
fi

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
    if [ -f "$HOME/.zshrc" ]; then
        SHELL_CONFIG="$HOME/.zshrc"
    elif [ -f "$HOME/.bash_profile" ]; then
        SHELL_CONFIG="$HOME/.bash_profile"
    else
        SHELL_CONFIG="$HOME/.bashrc"
    fi
fi

# Remove PATH configuration (optional - commented out to preserve user's PATH)
# Uncomment the following lines if you want to remove the PATH entry
# if [ -f "$SHELL_CONFIG" ]; then
#     if grep -q "# RocketCTL" "$SHELL_CONFIG"; then
#         echo -e "${YELLOW}Removing PATH configuration from $SHELL_CONFIG...${NC}"
#         sed -i.bak '/# RocketCTL/,/export PATH.*\.local\/bin/d' "$SHELL_CONFIG"
#         echo -e "${GREEN}✓ Removed PATH configuration${NC}"
#     fi
# fi

echo ""
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${GREEN}✓ RocketCTL uninstalled successfully!${NC}"
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo -e "${YELLOW}Note: The PATH configuration in $SHELL_CONFIG was left intact.${NC}"
echo -e "${YELLOW}If you want to remove it, edit $SHELL_CONFIG manually.${NC}"
echo ""
