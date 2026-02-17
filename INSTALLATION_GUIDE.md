# RocketCTL Installation Guide

Complete guide for installing RocketCTL on macOS.

## For End Users (No Go Required)

### Method 1: One-Command Install (Easiest)

```bash
curl -sSL https://raw.githubusercontent.com/cjairm/rocketctl/main/install.sh | bash
source ~/.zshrc  # or source ~/.bashrc for bash
```

**What happens:**
1. Script detects your Mac architecture (Intel or Apple Silicon)
2. Downloads the appropriate pre-built binary from GitHub Releases
3. Installs to `~/.local/bin/rocketctl`
4. Adds `~/.local/bin` to your PATH
5. Ready to use!

### Method 2: Manual Download

**Step 1:** Determine your Mac type
```bash
uname -m
# x86_64 = Intel Mac
# arm64 = Apple Silicon Mac
```

**Step 2:** Download the appropriate binary

**For Intel Macs:**
```bash
curl -L https://github.com/cjairm/rocketctl/releases/latest/download/rocketctl-darwin-amd64 -o rocketctl
chmod +x rocketctl
sudo mv rocketctl /usr/local/bin/
```

**For Apple Silicon Macs:**
```bash
curl -L https://github.com/cjairm/rocketctl/releases/latest/download/rocketctl-darwin-arm64 -o rocketctl
chmod +x rocketctl
sudo mv rocketctl /usr/local/bin/
```

**Step 3:** Verify installation
```bash
rocketctl --help
```

## For Developers (With Go Installed)

### Method 1: Install via Go

```bash
go install github.com/cjairm/rocketctl@latest
```

### Method 2: Build from Source

```bash
git clone https://github.com/cjairm/rocketctl.git
cd rocketctl
make build
make install
```

Or using the install script (auto-builds if no releases):
```bash
./install.sh
```

## Verifying Installation

After installation, verify it works:

```bash
# Check if rocketctl is in PATH
which rocketctl

# Test the binary
rocketctl --help

# Check version (shows help for now)
rocketctl version
```

Expected output:
```
RocketCTL is a convention-based CLI tool that orchestrates Docker image 
building, versioning, pushing, and deployment for any project.
...
```

## Troubleshooting

### "command not found: rocketctl"

**Solution 1:** Add to PATH manually
```bash
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.zshrc
source ~/.zshrc
```

**Solution 2:** Check installation location
```bash
ls -la ~/.local/bin/rocketctl
# If it exists, the PATH issue is the problem

# Try absolute path
~/.local/bin/rocketctl --help
```

### "Permission denied"

Make the binary executable:
```bash
chmod +x ~/.local/bin/rocketctl
# Or if manually downloaded:
chmod +x rocketctl
```

### "Bad CPU type in executable"

You downloaded the wrong architecture:
- Intel Macs need: `rocketctl-darwin-amd64`
- Apple Silicon needs: `rocketctl-darwin-arm64`

Check your architecture:
```bash
uname -m
```

Re-download the correct binary.

### Install script fails to download

If there are no releases yet:
```bash
# Install from source instead
git clone https://github.com/cjairm/rocketctl.git
cd rocketctl
go build -o rocketctl
sudo mv rocketctl /usr/local/bin/
```

### "Go is not installed" but I don't want to install Go

Wait for the first release to be published, then use the one-command install which downloads pre-built binaries.

Or manually download the binary from GitHub Releases.

## Updating RocketCTL

### If installed via install.sh or curl:

```bash
# Re-run the install script
curl -sSL https://raw.githubusercontent.com/cjairm/rocketctl/main/install.sh | bash
```

### If installed via go install:

```bash
go install github.com/cjairm/rocketctl@latest
```

### If manually installed:

Download the latest binary and replace the old one.

## Uninstalling

### If installed via install.sh:

```bash
curl -sSL https://raw.githubusercontent.com/cjairm/rocketctl/main/uninstall.sh | bash
```

Or if you cloned the repo:
```bash
./uninstall.sh
```

### Manual uninstall:

```bash
# Remove the binary
rm ~/.local/bin/rocketctl
# Or if installed to /usr/local/bin:
sudo rm /usr/local/bin/rocketctl

# Optionally remove PATH configuration
# Edit ~/.zshrc or ~/.bashrc and remove the RocketCTL section
```

## Installation Paths

Different installation methods use different paths:

| Method | Binary Location | Requires sudo? |
|--------|----------------|----------------|
| install.sh | `~/.local/bin/rocketctl` | No |
| go install | `$GOPATH/bin/rocketctl` | No |
| Manual (recommended) | `/usr/local/bin/rocketctl` | Yes |
| Manual (user) | `~/.local/bin/rocketctl` | No |

## Platform Support

Currently supported:
- ✅ macOS Intel (x86_64 / amd64)
- ✅ macOS Apple Silicon (arm64)

Coming soon:
- 🔄 Linux (x86_64 / amd64)
- 🔄 Linux (arm64)
- 🔄 Windows (experimental)

## Next Steps

After installing:

1. **Initialize a project:**
   ```bash
   cd your-project
   rocketctl init
   ```

2. **Read the Quick Start Guide:**
   ```bash
   cat QUICKSTART.md
   ```

3. **Get help:**
   ```bash
   rocketctl --help
   rocketctl init --help
   ```

## Getting Help

- **Installation issues**: Check this guide or open an issue
- **Usage questions**: See [QUICKSTART.md](QUICKSTART.md)
- **Bug reports**: [GitHub Issues](https://github.com/cjairm/rocketctl/issues)
- **Discussions**: [GitHub Discussions](https://github.com/cjairm/rocketctl/discussions)
