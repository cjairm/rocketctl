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

| Problem             | Fix                                                                          |
| ------------------- | ---------------------------------------------------------------------------- |
| `command not found` | `echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.zshrc && source ~/.zshrc` |
| `Permission denied` | `chmod +x ~/.local/bin/rocketctl`                                            |
| `Bad CPU type`      | Check `uname -m`: use `amd64` for Intel, `arm64` for Apple Silicon           |

### Uninstall

```bash
curl -sSL https://raw.githubusercontent.com/cjairm/rocketctl/main/uninstall.sh | bash
```

## Prerequisites

- **Docker** and **Docker Compose**
- **AWS CLI** configured with ECR permissions (`aws configure`)
- **SSH access** to production server (for deployment)

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
rocketctl up --prod              # Test production build locally (E2E)
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
domain: api.myapp.com      # optional
ip: 192.168.1.100          # optional - for SSH deployment
ssh_user: ubuntu           # optional - defaults to current user
ssh_key_path: ~/my-key.pem # optional - custom SSH key (e.g., AWS EC2 .pem file)
```

**Monorepo:**

```yaml
project: myapp
registry: 123456789.dkr.ecr.us-east-2.amazonaws.com
region: us-east-2
domain: myapp.com          # optional
ip: 192.168.1.100          # optional - for SSH deployment
ssh_user: ubuntu           # optional - defaults to current user
ssh_key_path: ~/my-key.pem # optional - custom SSH key (e.g., AWS EC2 .pem file)
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

| Command                                                  | Description                             |
| -------------------------------------------------------- | --------------------------------------- |
| `rocketctl init`                                         | Initialize project                      |
| `rocketctl build [service] --bump [patch\|minor\|major]` | Build production image and bump version |
| `rocketctl push [service]`                               | Push image to registry                  |
| `rocketctl up [service] [--build] [--no-cache]`          | Start dev environment                   |
| `rocketctl up --prod [service]`                          | Test production build locally (E2E)     |
| `rocketctl down`                                         | Stop dev environment                    |
| `rocketctl down --prod`                                  | Stop production/test environment        |
| `rocketctl deploy`                                       | Deploy to production                    |
| `rocketctl ps`                                           | List running containers                 |
| `rocketctl logs [service] [-f] [--prod]`                 | Show service logs                       |
| `rocketctl exec [service] [cmd]`                         | Execute command in container            |
| `rocketctl list`                                         | List all services and versions          |
| `rocketctl version [service]`                            | Show version(s)                         |
| `rocketctl prune`                                        | Clean up old images                     |
| `rocketctl ecr create`                                   | Create ECR repositories (idempotent)    |

## Workflows

**Development:**

```bash
rocketctl up             # Start dev environment
rocketctl up --build     # Rebuild images before starting
rocketctl logs api -f    # Tail logs
rocketctl exec api bash  # Shell into container
rocketctl down           # Stop all
```

**Testing Production Locally:**

```bash
rocketctl up --prod           # Build and start entire production stack (E2E)
rocketctl logs --prod api -f  # View production logs
rocketctl ps                  # List running containers
rocketctl down --prod         # Stop production stack
```

**Release:**

```bash
rocketctl up --prod              # Test locally (E2E)
rocketctl build api --bump minor # Build and version
rocketctl push api               # Push to registry
rocketctl deploy                 # Deploy to remote server
```

## Deployment

### Overview

The `rocketctl deploy` command automates deployment to a remote server via SSH. It:

1. Connects to your server via SSH
2. Creates directory structure: `~/apps/<PROJECT-NAME>/`
3. Uploads necessary files:
   - `docker-compose.yml` (generated from template)
   - `Caddyfile` (if domain is configured)
   - `.env` (if it doesn't exist on the server)
4. Authenticates with ECR
5. Pulls latest Docker images
6. Restarts services with zero-downtime

### Prerequisites

1. **SSH Access**: Ensure you can SSH into your server with key-based authentication
2. **Server Setup**: Your production server must have:
   - Docker and Docker Compose installed
   - AWS CLI configured with ECR access (`aws configure`)
   - SSH key added to `~/.ssh/authorized_keys`

3. **RocketCTL Configuration**: Run `rocketctl init` and provide:
   - Server IP address
   - SSH user (optional, defaults to your current username)

### SSH Key Setup

RocketCTL supports multiple authentication methods:

**Option 1: Default SSH Keys** (automatic)

RocketCTL automatically looks for keys in:
1. `~/.ssh/id_ed25519` (recommended, modern)
2. `~/.ssh/id_rsa` (traditional)

```bash
# Generate SSH key
ssh-keygen -t ed25519 -C "your_email@example.com"

# Copy public key to server
ssh-copy-id user@server-ip

# Test connection
ssh user@server-ip
```

**Option 2: Custom Key Path** (e.g., AWS EC2 .pem files)

For custom keys like AWS EC2 `.pem` files, specify the path in `rocket.yaml`:

```yaml
ssh_key_path: ~/Downloads/my-ec2-key.pem
```

Or set it during `rocketctl init` when prompted.

Example for AWS EC2:

```bash
# Download your .pem file from AWS Console
# Set permissions (required)
chmod 400 ~/Downloads/my-ec2-key.pem

# Configure in rocket.yaml
ssh_key_path: ~/Downloads/my-ec2-key.pem

# Deploy
rocketctl deploy
```

**Equivalent SSH command:**
```bash
# RocketCTL does this automatically:
ssh -i ~/Downloads/my-ec2-key.pem ubuntu@192.168.1.100
```

### First-Time Deployment

```bash
# 1. Build and push images
rocketctl build api --bump patch
rocketctl push api

# 2. Deploy to server (uploads files, pulls images, starts services)
rocketctl deploy

# 3. SSH into server and configure .env with actual values
ssh user@server-ip
cd ~/apps/myproject
nano .env  # Add your secrets: DATABASE_URL, API_KEYS, etc.

# 4. Restart services to pick up new env vars
docker compose up -d
```

### Subsequent Deployments

```bash
# Build, push, and deploy
rocketctl build api --bump patch
rocketctl push api
rocketctl deploy
```

### Deployment Directory Structure

On your production server, files are organized as:

```
~/apps/
  <PROJECT-NAME>/
    docker-compose.yml  # Uploaded by rocketctl
    Caddyfile           # Uploaded if domain is configured
    .env                # Created once, you edit manually with secrets
```

### Managing Services on Production

**View logs:**

```bash
ssh user@server-ip 'cd ~/apps/myproject && docker compose logs -f'
```

**View running services:**

```bash
ssh user@server-ip 'cd ~/apps/myproject && docker compose ps'
```

**Restart specific service:**

```bash
ssh user@server-ip 'cd ~/apps/myproject && docker compose pull api && docker compose up -d --force-recreate api'
```

**Restart all services:**

```bash
ssh user@server-ip 'cd ~/apps/myproject && docker compose pull && docker compose up -d'
```

**Restart Caddy reverse proxy:**

```bash
ssh user@server-ip 'cd ~/apps/myproject && docker stop apps_caddy_1 && docker rm apps_caddy_1 && docker compose up -d'
```

### Cleanup Commands

```bash
# SSH into server
ssh user@server-ip
cd ~/apps/myproject

# Remove stopped containers
docker container prune

# Remove unused images
docker image prune

# Remove unused volumes (CAUTION: may delete data)
docker volume prune

# Remove unused networks
docker network prune

# Full cleanup (CAUTION: removes all unused resources)
docker system prune
```

### Troubleshooting Deployment

**SSH connection fails:**

```bash
# Test SSH connection manually
ssh -v user@server-ip

# Or with custom key
ssh -v -i ~/my-key.pem user@server-ip

# Check if SSH keys exist
ls -la ~/.ssh/

# Verify SSH agent
ssh-add -l

# Check custom key permissions (must be 400 or 600)
ls -l ~/my-key.pem
chmod 400 ~/my-key.pem  # Fix if needed
```

**ECR authentication fails on server:**

```bash
# SSH into server and test AWS CLI
ssh user@server-ip
aws ecr get-login-password --region us-east-2

# If fails, configure AWS CLI on server
aws configure
```

**Services won't start:**

```bash
# SSH into server and check logs
ssh user@server-ip
cd ~/apps/myproject
docker compose logs
docker compose ps

# Check if .env is properly configured
cat .env
```

**Port conflicts:**

```bash
# Check which ports are in use
ssh user@server-ip 'netstat -tuln | grep LISTEN'

# Update docker-compose.yml port mappings if needed
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

| Target                       | Description                                 |
| ---------------------------- | ------------------------------------------- |
| `make build`                 | Build for current platform                  |
| `make install`               | Build and install to ~/.local/bin           |
| `make release VERSION=X.Y.Z` | Build release binaries (both architectures) |
| `make check`                 | Run fmt + vet + test                        |
| `make test`                  | Run tests                                   |
| `make fmt`                   | Format code                                 |
| `make vet`                   | Run go vet                                  |
| `make clean`                 | Remove build artifacts                      |

### Pre-Release Checklist

- [ ] CHANGELOG.md updated
- [ ] `make check` passes
- [ ] Documentation up to date
- [ ] Git working directory clean, on `main`

## Contributing

Contributions welcome! See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup, coding standards, and PR guidelines.

## License

MIT
