#!/bin/bash

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Default version
VERSION=${1:-}

if [ -z "$VERSION" ]; then
    echo -e "${RED}Error: Version number required${NC}"
    echo "Usage: $0 <version>"
    echo "Example: $0 1.0.0"
    exit 1
fi

# Remove 'v' prefix if present
VERSION=${VERSION#v}

echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}  RocketCTL Release Builder${NC}"
echo -e "${BLUE}  Version: v${VERSION}${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

# Check if git repo is clean
if [ -n "$(git status --porcelain)" ]; then
    echo -e "${YELLOW}Warning: Git working directory is not clean${NC}"
    echo "Uncommitted changes:"
    git status --short
    echo ""
    read -p "Continue anyway? (y/n) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
    fi
fi

# Update CHANGELOG
echo -e "${YELLOW}Step 1/5: Checking CHANGELOG.md...${NC}"
if ! grep -q "## \[$VERSION\]" CHANGELOG.md 2>/dev/null; then
    echo -e "${YELLOW}  Version $VERSION not found in CHANGELOG.md${NC}"
    echo -e "${YELLOW}  Please update CHANGELOG.md before releasing${NC}"
    read -p "  Continue anyway? (y/n) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
    fi
else
    echo -e "${GREEN}  ✓ CHANGELOG.md updated${NC}"
fi

# Build release binaries
echo ""
echo -e "${YELLOW}Step 2/5: Building release binaries...${NC}"
make release VERSION=$VERSION

# Create git tag
echo ""
echo -e "${YELLOW}Step 3/5: Creating git tag...${NC}"
if git rev-parse "v$VERSION" >/dev/null 2>&1; then
    echo -e "${YELLOW}  Tag v$VERSION already exists${NC}"
    read -p "  Delete and recreate? (y/n) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        git tag -d "v$VERSION"
        git tag -a "v$VERSION" -m "Release v$VERSION"
        echo -e "${GREEN}  ✓ Tag recreated: v$VERSION${NC}"
    fi
else
    git tag -a "v$VERSION" -m "Release v$VERSION"
    echo -e "${GREEN}  ✓ Tag created: v$VERSION${NC}"
fi

# Generate release notes template
echo ""
echo -e "${YELLOW}Step 4/5: Generating release notes...${NC}"
cat > dist/release-notes.md << EOF
# RocketCTL v${VERSION}

## What's New

<!-- Add your release highlights here -->

## Installation

### Quick Install (Recommended)

\`\`\`bash
curl -sSL https://raw.githubusercontent.com/cjairm/rocketctl/main/install.sh | bash
\`\`\`

### Manual Download

**macOS Intel (x86_64):**
\`\`\`bash
curl -L https://github.com/cjairm/rocketctl/releases/download/v${VERSION}/rocketctl-darwin-amd64 -o rocketctl
chmod +x rocketctl
sudo mv rocketctl /usr/local/bin/
\`\`\`

**macOS Apple Silicon (ARM64):**
\`\`\`bash
curl -L https://github.com/cjairm/rocketctl/releases/download/v${VERSION}/rocketctl-darwin-arm64 -o rocketctl
chmod +x rocketctl
sudo mv rocketctl /usr/local/bin/
\`\`\`

### Verify Installation

\`\`\`bash
rocketctl --help
\`\`\`

## Checksums

\`\`\`
$(cat dist/checksums.txt)
\`\`\`

## Full Changelog

See [CHANGELOG.md](https://github.com/cjairm/rocketctl/blob/main/CHANGELOG.md)
EOF

echo -e "${GREEN}  ✓ Release notes created: dist/release-notes.md${NC}"

# Summary
echo ""
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${GREEN}✓ Release v${VERSION} prepared successfully!${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo -e "${YELLOW}Step 5/5: Next Steps${NC}"
echo ""
echo "1. Review the release notes:"
echo -e "   ${GREEN}cat dist/release-notes.md${NC}"
echo ""
echo "2. Test the binaries:"
echo -e "   ${GREEN}./dist/rocketctl-darwin-amd64 --help${NC}"
echo -e "   ${GREEN}./dist/rocketctl-darwin-arm64 --help${NC}"
echo ""
echo "3. Push the tag to GitHub:"
echo -e "   ${GREEN}git push origin v${VERSION}${NC}"
echo ""
echo "4. Create GitHub Release:"
echo -e "   ${GREEN}https://github.com/cjairm/rocketctl/releases/new?tag=v${VERSION}${NC}"
echo ""
echo "5. Upload these files:"
echo -e "   ${GREEN}dist/rocketctl-darwin-amd64${NC}"
echo -e "   ${GREEN}dist/rocketctl-darwin-arm64${NC}"
echo -e "   ${GREEN}dist/checksums.txt${NC}"
echo ""
echo "6. Copy release notes from:"
echo -e "   ${GREEN}dist/release-notes.md${NC}"
echo ""
echo "7. Publish the release!"
echo ""
