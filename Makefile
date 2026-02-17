.PHONY: build clean release install test help

# Binary name
BINARY_NAME=rocketctl

# Version - can be overridden: make release VERSION=1.0.1
VERSION?=1.0.0

# Build directory
BUILD_DIR=dist

# Platforms
PLATFORMS=darwin/amd64 darwin/arm64

# Default target
.DEFAULT_GOAL := help

## build: Build binary for current platform
build:
	@echo "Building $(BINARY_NAME)..."
	@go build -o $(BINARY_NAME) .
	@echo "✓ Build complete: $(BINARY_NAME)"

## clean: Remove build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -f $(BINARY_NAME)
	@rm -rf $(BUILD_DIR)
	@rm -f $(BINARY_NAME)-*
	@rm -f checksums.txt
	@echo "✓ Clean complete"

## release: Build release binaries for all platforms
release: clean
	@echo "Building release $(VERSION) for macOS..."
	@mkdir -p $(BUILD_DIR)
	
	# Build for Intel Macs (amd64)
	@echo "Building for macOS Intel (amd64)..."
	@GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 .
	
	# Build for Apple Silicon (arm64)
	@echo "Building for macOS Apple Silicon (arm64)..."
	@GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 .
	
	# Generate checksums
	@echo "Generating checksums..."
	@cd $(BUILD_DIR) && shasum -a 256 $(BINARY_NAME)-* > checksums.txt
	
	@echo ""
	@echo "✓ Release build complete!"
	@echo ""
	@echo "Binaries created:"
	@ls -lh $(BUILD_DIR)/$(BINARY_NAME)-*
	@echo ""
	@echo "Checksums:"
	@cat $(BUILD_DIR)/checksums.txt
	@echo ""
	@echo "Next steps:"
	@echo "  1. Test the binaries"
	@echo "  2. Create a new release on GitHub: https://github.com/cjairm/rocketctl/releases/new"
	@echo "  3. Tag: v$(VERSION)"
	@echo "  4. Upload files from $(BUILD_DIR)/"
	@echo "  5. Publish the release"

## install: Install binary to ~/.local/bin
install: build
	@echo "Installing $(BINARY_NAME) to ~/.local/bin..."
	@mkdir -p ~/.local/bin
	@mv $(BINARY_NAME) ~/.local/bin/$(BINARY_NAME)
	@chmod +x ~/.local/bin/$(BINARY_NAME)
	@echo "✓ Installed to ~/.local/bin/$(BINARY_NAME)"
	@echo ""
	@echo "Make sure ~/.local/bin is in your PATH"

## test: Run tests
test:
	@echo "Running tests..."
	@go test ./...

## fmt: Format code
fmt:
	@echo "Formatting code..."
	@go fmt ./...
	@echo "✓ Format complete"

## vet: Run go vet
vet:
	@echo "Running go vet..."
	@go vet ./...
	@echo "✓ Vet complete"

## check: Run fmt, vet, and test
check: fmt vet test
	@echo "✓ All checks passed"

## help: Show this help message
help:
	@echo "RocketCTL Makefile"
	@echo ""
	@echo "Usage:"
	@echo "  make [target]"
	@echo ""
	@echo "Targets:"
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' | sed -e 's/^/ /'
	@echo ""
	@echo "Examples:"
	@echo "  make build                    # Build for current platform"
	@echo "  make release                  # Build release for macOS (both architectures)"
	@echo "  make release VERSION=1.0.1    # Build release with specific version"
	@echo "  make install                  # Build and install locally"
	@echo "  make clean                    # Remove all build artifacts"
