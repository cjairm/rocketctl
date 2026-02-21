# RocketCTL

A convention-based CLI tool that orchestrates Docker image building, versioning, pushing, and deployment for any project.

## Features

- **Convention over Configuration**: Minimal config, maximum automation
- **Semantic Versioning**: Automatic version management per service
- **Monorepo & Single-Service**: Works with both project structures
- **AWS ECR Integration**: Built-in authentication and image pushing
- **Docker Compose Integration**: Dev and production workflows

## Installation

### Quick Install (Recommended)

```bash
curl -sSL https://raw.githubusercontent.com/cjairm/rocketctl/main/install.sh | bash
source ~/.zshrc  # or source ~/.bashrc
```

### Manual Download

**Intel Macs:**
```bash
curl -L https://github.com/cjairm/rocketctl/releases/latest/download/rocketctl-darwin-amd64 -o rocketctl
chmod +x rocketctl
sudo mv rocketctl /usr/local/bin/
```

**Apple Silicon:**
```bash
curl -L https://github.com/cjairm/rocketctl/releases/latest/download/rocketctl-darwin-arm64 -o rocketctl
chmod +x rocketctl
sudo mv rocketctl /usr/local/bin/
```

### From Source (Go 1.23+)

```bash
git clone https://github.com/cjairm/rocketctl.git
cd rocketctl
make build && make install
```

Or: `go install github.com/cjairm/rocketctl@latest`

### Troubleshooting

| Problem | Fix |
|---------|-----|
| `command not found` | `echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.zshrc && source ~/.zshrc` |
| `Permission denied` | `chmod +x ~/.local/bin/rocketctl` |
| `Bad CPU type` | Check `uname -m`: use `amd64` for Intel, `arm64` for Apple Silicon |

### Uninstall

```bash
curl -sSL https://raw.githubusercontent.com/cjairm/rocketctl/main/uninstall.sh | bash
```

## Prerequisites

- **Docker** and **Docker Compose**
- **AWS CLI** configured with ECR permissions (`aws configure`)

## Quick Start

### 1. Initialize

```bash
cd your-project
rocketctl init
```

Creates `rocket.yaml`, `.rocket-version` (0.1.0), `docker-compose.prod.yml`, and optionally `caddy/Caddyfile`.

### 2. Create Dockerfiles

RocketCTL expects `Dockerfile` (dev) and `Dockerfile.production` (production) in the project root or each service folder.

### 3. Set Up Environment

Create `.env.production` on your server with runtime secrets. This file is injected at runtime via `env_file` in `docker-compose.prod.yml` -- never commit it to git.

### 4. Build, Push, Deploy

```bash
rocketctl ecr create             # Create ECR repos (once)
rocketctl test api               # Test production build locally
rocketctl build api --bump patch # Build and bump version
rocketctl push api               # Push to registry
rocketctl deploy                 # Deploy (on production server)
```

## Configuration

### rocket.yaml

**Single-Service:**
```yaml
project: my-backend
service: backend
registry: 123456789.dkr.ecr.us-east-2.amazonaws.com
region: us-east-2
domain: api.myapp.com  # optional
```

**Monorepo:**
```yaml
project: myapp
registry: 123456789.dkr.ecr.us-east-2.amazonaws.com
region: us-east-2
domain: myapp.com  # optional
services:
  - api
  - web
  - worker
```

### Version Management

Versions are stored in `.rocket-version` files (one per service). The version is only bumped after a successful build.

- `--bump patch`: 0.1.0 -> 0.1.1 (bug fixes)
- `--bump minor`: 0.1.0 -> 0.2.0 (new features)
- `--bump major`: 0.1.0 -> 1.0.0 (breaking changes)

### Image Naming

`<project>_<service>:<version>` (e.g., `myapp_api:0.2.1`)

### Environment Variables

- `.env` -- Shared/fallback values
- `.env.development` -- Dev-specific values
- `.env.production` -- Runtime secrets (not in git, created manually on server, injected via `env_file`)

### Folder Structure

**Monorepo:**
```
project/
  rocket.yaml
  docker-compose.yml          # Dev (user-created)
  docker-compose.prod.yml     # Production (generated)
  caddy/Caddyfile             # Reverse proxy
  api/
    Dockerfile
    Dockerfile.production
    .rocket-version
    .env.production
  web/
    Dockerfile
    Dockerfile.production
    .rocket-version
    .env.production
```

**Single-Service:**
```
project/
  rocket.yaml
  docker-compose.yml
  docker-compose.prod.yml
  Dockerfile
  Dockerfile.production
  .rocket-version
  .env.production
```

## Commands

| Command | Description |
|---------|-------------|
| `rocketctl init` | Initialize project |
| `rocketctl build [service] --bump [patch\|minor\|major]` | Build production image and bump version |
| `rocketctl push [service]` | Push image to registry |
| `rocketctl test [service]` | Test production build locally |
| `rocketctl dev [service] [--build]` | Start dev environment |
| `rocketctl down` | Stop dev environment |
| `rocketctl deploy` | Deploy to production |
| `rocketctl ps` | List running containers |
| `rocketctl logs [service] [-f]` | Show service logs |
| `rocketctl exec [service] [cmd]` | Execute command in container |
| `rocketctl list` | List all services and versions |
| `rocketctl version [service]` | Show version(s) |
| `rocketctl prune` | Clean up old images |
| `rocketctl ecr create` | Create ECR repositories (idempotent) |

## Workflows

**Development:**
```bash
rocketctl dev            # Start dev environment
rocketctl logs api -f    # Tail logs
rocketctl exec api bash  # Shell into container
rocketctl down           # Stop all
```

**Release:**
```bash
rocketctl test api               # Test locally
rocketctl build api --bump minor # Build and version
rocketctl push api               # Push to registry
rocketctl deploy                 # Deploy (on server)
rocketctl ps                     # Verify
```

## Releasing RocketCTL

### Prerequisites

- Write access to the repository
- Clean git working directory, on `main` branch
- CHANGELOG.md updated with new version

### Automated via GitHub Actions (Recommended)

GitHub Actions automatically builds, creates releases, and publishes binaries when you push a version tag.

```bash
# 1. Update CHANGELOG.md with new version
# 2. Commit and push changes
git add CHANGELOG.md README.md
git commit -m "docs: prepare release v1.0.0"
git push origin main

# 3. Create and push tag (triggers GitHub Actions)
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0

# 4. GitHub Actions automatically:
#    - Builds binaries for both architectures
#    - Generates checksums
#    - Extracts changelog notes
#    - Creates GitHub Release
#    - Uploads all artifacts
```

Watch progress at: `https://github.com/cjairm/rocketctl/actions`

### Local Testing with scripts/release.sh

To test locally before pushing:

```bash
# 1. Update CHANGELOG.md
# 2. Run release script (builds locally, creates tag)
./scripts/release.sh 1.0.0

# 3. Test binaries
./dist/rocketctl-darwin-amd64 --help
./dist/rocketctl-darwin-arm64 --help

# 4. Push tag (triggers GitHub Actions for official release)
git push origin v1.0.0
```

### Fully Manual

```bash
# 1. Update CHANGELOG.md, commit and push
make check                   # fmt + vet + test
make release VERSION=1.0.0   # Build locally

# 2. Test, tag, push
./dist/rocketctl-darwin-arm64 --help
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0

# GitHub Actions will still run and create the official release
```

### Makefile Targets

| Target | Description |
|--------|-------------|
| `make build` | Build for current platform |
| `make install` | Build and install to ~/.local/bin |
| `make release VERSION=X.Y.Z` | Build release binaries (both architectures) |
| `make check` | Run fmt + vet + test |
| `make test` | Run tests |
| `make fmt` | Format code |
| `make vet` | Run go vet |
| `make clean` | Remove build artifacts |

### Pre-Release Checklist

- [ ] CHANGELOG.md updated
- [ ] `make check` passes
- [ ] Documentation up to date
- [ ] Git working directory clean, on `main`

## Contributing

Contributions welcome! See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup, coding standards, and PR guidelines.

## License

MIT
