# RocketCTL

A convention-based CLI tool that orchestrates Docker image building, versioning, pushing, and deployment for any project.

**📚 New to RocketCTL? Check out the [Quick Start Guide](QUICKSTART.md)**

## Features

- **Convention over Configuration**: Minimal config file, maximum automation
- **Semantic Versioning**: Automatic version management per service
- **Monorepo Support**: Handle multiple services in one repository
- **Single-Service Support**: Works with standalone service repositories
- **AWS ECR Integration**: Built-in authentication and image pushing
- **Docker Compose Integration**: Seamless dev and production workflows

## Installation

### Quick Install - One Command (Recommended)

No Go required! The install script automatically downloads the latest pre-built binary:

```bash
curl -sSL https://raw.githubusercontent.com/cjairm/rocketctl/main/install.sh | bash
```

Then activate in your current shell:

```bash
source ~/.zshrc  # or source ~/.bashrc for bash
```

**What the script does:**

1. Detects your macOS architecture (Intel or Apple Silicon)
2. Downloads the appropriate pre-built binary from GitHub Releases
3. Installs to `~/.local/bin/`
4. Adds to PATH automatically
5. Falls back to building from source if Go is installed and no release is available

### Manual Download (No Go Required)

**For macOS Intel (x86_64):**

```bash
curl -L https://github.com/cjairm/rocketctl/releases/latest/download/rocketctl-darwin-amd64 -o rocketctl
chmod +x rocketctl
sudo mv rocketctl /usr/local/bin/
```

**For macOS Apple Silicon (ARM64):**

```bash
curl -L https://github.com/cjairm/rocketctl/releases/latest/download/rocketctl-darwin-arm64 -o rocketctl
chmod +x rocketctl
sudo mv rocketctl /usr/local/bin/
```

### Install from Source (Requires Go 1.23+)

```bash
git clone https://github.com/cjairm/rocketctl.git
cd rocketctl
./install.sh  # Builds from source if no releases available
```

Or manually:

```bash
go install github.com/cjairm/rocketctl@latest
```

### Verify Installation

```bash
rocketctl --help
```

### Uninstall

```bash
# Download and run uninstall script
curl -sSL https://raw.githubusercontent.com/cjairm/rocketctl/main/uninstall.sh | bash

# Or if you cloned the repo
./uninstall.sh
```

## Prerequisites

RocketCTL assumes the following tools are installed and configured on your machine:

- **Docker** — for building and running images
- **Docker Compose** — for local development and production orchestration
- **AWS CLI** — installed and configured with credentials that have ECR permissions (`aws configure`)

## Quick Start

### 1. Initialize a Project

```bash
cd your-project
rocketctl init
```

This will:

- Create `rocket.yaml` configuration file
- Generate `.rocket-version` files (initialized to 0.1.0)
- Create `docker-compose.prod.yml` template
- Generate Caddy reverse proxy configuration (if domain provided)

### 2. Create ECR Repositories

```bash
rocketctl ecr create
```

This creates an ECR repository for each service defined in `rocket.yaml`, named `<project>_<service>` (e.g. `automatedhub_api`).

### 3. Create Your Dockerfiles

RocketCTL expects:

- `Dockerfile` - for development builds
- `Dockerfile.production` - for production builds

### 4. Build and Push

```bash
# Build production image (bumps patch version by default)
rocketctl build api --bump patch

# Push to registry
rocketctl push api
```

### 5. Deploy

```bash
# On your production server
rocketctl deploy
```

## Configuration

### rocket.yaml

#### Single-Service Repository

```yaml
project: my-backend
service: backend
registry: 423756184128.dkr.ecr.us-east-2.amazonaws.com
region: us-east-2
domain: api.myapp.com # optional
```

#### Monorepo

```yaml
project: myapp
registry: 423756184128.dkr.ecr.us-east-2.amazonaws.com
region: us-east-2
domain: myapp.com # optional

services:
  - api
  - web
  - worker
```

## Commands

### `rocketctl init`

Initialize a project for use with RocketCTL.

### `rocketctl build [service] --bump [patch|minor|major]`

Build a production Docker image and bump the version.

```bash
# Monorepo
rocketctl build api --bump minor

# Single-service (service name inferred)
rocketctl build --bump patch
```

### `rocketctl push [service]`

Push a built image to the container registry.

```bash
rocketctl push api
```

### `rocketctl test [service]`

Test a production build locally without bumping the version.

```bash
rocketctl test api
```

### `rocketctl dev [service] [--build] [--no-cache]`

Start the development environment using docker-compose.yml.

```bash
# Start all services
rocketctl dev

# Start specific service
rocketctl dev api

# Rebuild before starting
rocketctl dev --build
```

### `rocketctl down`

Stop the development environment.

### `rocketctl deploy`

Deploy services on production (pulls images and restarts).

### `rocketctl ps`

List running containers for the current project.

### `rocketctl logs [service] [-f]`

Show logs for a service.

```bash
rocketctl logs api -f
```

### `rocketctl exec [service] [command...]`

Execute a command in a running container.

```bash
rocketctl exec api bash
```

### `rocketctl list`

List all services with their versions.

### `rocketctl version [service]`

Show version for a service or all services.

### `rocketctl prune`

Clean up old Docker images.

### `rocketctl ecr create`

Create ECR repositories for all services defined in `rocket.yaml`.

## Folder Structure Conventions

### Monorepo

```
project/
  rocket.yaml
  docker-compose.yml           # Dev environment (user-created)
  docker-compose.prod.yml      # Production (generated template)
  caddy/
    Caddyfile                  # Reverse proxy config
  api/
    Dockerfile
    Dockerfile.production
    .dockerignore
    .env.production
    .rocket-version
    [application code]
  web/
    Dockerfile
    Dockerfile.production
    .dockerignore
    .env.production
    .rocket-version
    [application code]
```

### Single-Service

```
project/
  rocket.yaml
  docker-compose.yml
  docker-compose.prod.yml
  Dockerfile                   # In project root
  Dockerfile.production
  .dockerignore
  .env.production
  .rocket-version
  [application code]
```

## Environment Variables

RocketCTL recognizes these environment files:

- `.env` - Shared/fallback values
- `.env.development` - Development-specific values
- `.env.production` - Production secrets (not in git, created manually on the server)

During builds, if `.env.production` exists, RocketCTL loads it and passes variables as `--build-arg` to Docker. This is essential for frameworks like Next.js that bake environment variables into the build.

## Image Naming

Images follow the pattern: `<project>_<service>:<version>`

Example:

- Project: `myapp`, Service: `api`, Version: `0.2.1`
- Image: `myapp_api:0.2.1`
- Full: `registry.example.com/myapp_api:0.2.1`

## Version Management

Versions are stored in `.rocket-version` files (one per service) and follow semantic versioning:

- `--bump patch`: 0.1.0 → 0.1.1 (default)
- `--bump minor`: 0.1.0 → 0.2.0
- `--bump major`: 0.1.0 → 1.0.0

The version is only updated after a successful build.

## AWS ECR Setup

Before pushing images, create ECR repositories with:

```bash
rocketctl ecr create
```

This creates a repository for each service in `rocket.yaml` using the naming convention `<project>_<service>`, with AES-256 encryption and mutable tags. The command is idempotent — safe to run multiple times.

RocketCTL automatically authenticates with ECR before pushing or deploying.

## Typical Workflows

### Development

```bash
rocketctl dev                    # Start dev environment
rocketctl logs api -f            # Tail API logs
rocketctl exec api bash          # Shell into container
rocketctl down                   # Stop everything
```

### Build, Test, and Deploy

```bash
rocketctl test api               # Test production build locally
rocketctl build api --bump patch # Build and bump version
rocketctl push api               # Push to registry

# On production server:
rocketctl deploy                 # Pull and restart
rocketctl ps                     # Verify containers
rocketctl logs api -f            # Check for errors
```

### Maintenance

```bash
rocketctl list                   # See all services and versions
rocketctl version api            # Check specific version
rocketctl prune                  # Clean up old images
```

## License

MIT

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
