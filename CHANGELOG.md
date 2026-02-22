# Changelog

All notable changes to RocketCTL will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.4.0] - 2026-02-21

### Added
- **SSH remote deployment** - `rocketctl deploy` now deploys to a remote server via SSH instead of running locally
  - Connects to the server, creates `~/apps/<PROJECT>/` directory structure
  - Uploads `docker-compose.prod.yml`, `Caddyfile` (if domain configured), and `.env` (from `.env.example`, only if `.env` doesn't already exist on server)
  - Runs ECR authentication, image pull, and service restart remotely
  - Supports monorepo per-service `.env` file handling
  - Prints helpful SSH commands for viewing logs and status after deployment
- **New `internal/ssh` package** - SSH client abstraction built on `golang.org/x/crypto/ssh`
  - `Connect()` - Establishes SSH connections with custom or default key support
  - `Exec()` / `ExecInteractive()` - Remote command execution (buffered or streaming to stdout/stderr)
  - `UploadFile()` - Uploads local files to the remote server
  - `MkdirAll()` - Creates remote directories recursively
  - `FileExists()` - Checks if a file exists on the remote server
- **SSH configuration fields** in `rocket.yaml`
  - `ssh_user` - SSH user for deployment (defaults to current OS user)
  - `ssh_key_path` - Custom SSH key path (e.g., AWS EC2 `.pem` files)
- **SSH key setup prompts** in `rocketctl init` - When a server IP is provided, init now asks for SSH user and key path
- **Template render-to-string functions** - `RenderDockerComposeProd()`, `RenderCaddyfile()`, `RenderEnvExample()` for rendering templates to strings without writing to disk

### Changed
- **`deploy` command** - Complete rewrite from local execution to remote SSH-based deployment
  - Requires `ip` to be configured in `rocket.yaml` (errors with helpful message if missing)
  - No longer depends on `internal/compose` or `internal/registry` packages directly; all operations run remotely
- **Go version** bumped from `1.23.6` to `1.24.0`
- **`init` command** - IP prompt updated from "for SSH access during development" to "for SSH deployment"
- **`up` command** - Minor wording updates: `docker-compose` to `docker compose` in command descriptions
- **Template variable rename** - `envProductionExampleTemplate` renamed to `envExampleTemplate` for clarity

### Dependencies
- Added `golang.org/x/crypto v0.48.0` (SSH client support)
- Added `golang.org/x/sys v0.41.0` (transitive dependency)

### Documentation
- Updated README.md with `ssh_user`, `ssh_key_path`, and `ip` fields in configuration examples
- Added comprehensive **Deployment** section to README covering:
  - Deployment overview and workflow
  - Prerequisites (SSH access, server setup)
  - SSH key setup (default keys and custom `.pem` files)
  - First-time and subsequent deployment instructions
  - Remote directory structure
  - Managing services on production (logs, restart, status)
  - Cleanup commands for Docker resources
  - Troubleshooting guide (SSH, ECR, services, port conflicts)

## [1.3.0] - 2026-02-21

### Added
- **`up` command** - Unified command for starting services in both dev and production modes
  - `rocketctl up [service]` - Start dev environment (replaces `dev`)
  - `rocketctl up --prod [service]` - Build and run production stack locally for E2E testing (replaces `test`)
  - `--build` flag - Rebuild images before starting (dev mode)
  - `--no-cache` flag - Rebuild without cache (requires --build, dev mode)
- **`--prod` flag support** - Added to multiple commands for production/test environment operations
  - `rocketctl down --prod` - Stop production/test containers
  - `rocketctl logs --prod [service] -f` - View production container logs
- **Port exposure documentation** - Added commented examples in docker-compose.prod.yml template showing how to expose ports for local testing

### Changed
- **Command structure** - Renamed and consolidated commands for better consistency
  - `test` → `up --prod` - More intuitive, matches Docker Compose conventions
  - `dev` → `up` - Unified interface, less duplication
- **Service resolution** - Updated to support optional service argument for both monorepo and single-service repos
  - `rocketctl up` (no args) now builds all services in monorepo, then starts entire stack
  - `rocketctl up [service]` builds only specified service, but starts entire stack for E2E testing
- **Helpful command output** - Updated `up --prod` to suggest using `rocketctl` commands instead of raw docker commands

### Removed
- **`dev` command** - Functionality merged into `up` command with same flags (--build, --no-cache)
- **`test` command** - Functionality moved to `up --prod` for better consistency

### Fixed
- **E2E testing** - `up --prod` now starts entire docker-compose stack instead of single service, enabling proper end-to-end testing
- **Production environment matching** - Test environment now uses docker-compose.prod.yml, ensuring it matches actual production deployment

### Documentation
- Updated README.md with new command structure and workflows
- Added "Testing Production Locally" workflow section
- Updated all command examples to use `up` and `down --prod`
- Updated command reference table with new flags and usage

## [1.2.0] - 2026-02-21

### Documentation

- **Consolidated README.md** - Merged INSTALLATION_GUIDE.md, QUICKSTART.md, and RELEASING.md into a single concise README (418 -> 276 lines, 34% reduction)
- **Clarified GitHub Actions automated release workflow** - Documented that pushing a tag triggers automatic builds, release creation, and binary publishing via `.github/workflows/release.yml`
- Fixed incorrect documentation stating `.env.production` is passed as `--build-arg` (behavior changed in 1.1.0 to runtime injection via `env_file`)
- Removed incorrect reference to `.env.production.example` file (no longer generated since 1.1.0)
- Removed duplicate sections (Version Management, AWS ECR Setup, Best Practices, Image Naming)
- Condensed installation troubleshooting to a single table
- Added complete Makefile targets reference with descriptions
- Updated release process documentation with three workflows: GitHub Actions (recommended), local testing with scripts, and fully manual
- Removed `.dockerignore` from folder structure examples (not generated by rocketctl)
- Simplified Quick Start guide with direct, to-the-point instructions
- Contributing section now references CONTRIBUTING.md instead of duplicating content

### Notes

- INSTALLATION_GUIDE.md, QUICKSTART.md, and RELEASING.md are now redundant and can be removed
- CONTRIBUTING.md and CHANGELOG.md remain as separate files
- All documentation is now accurate to current codebase behavior (v1.1.0+)

## [1.1.0] - 2026-02-18

### Added
- `ecr create` command to create ECR repositories for all services defined in `rocket.yaml`, named `<project>_<service>` with AES-256 encryption and mutable tags (idempotent)

### Changed
- `init` no longer generates `.env.production.example` files — `.env.production` is created manually on the server
- `build` no longer loads `.env.production` as `--build-arg`s; env vars are injected at runtime via `env_file` in `docker-compose.prod.yml`
- `test` no longer passes `.env.production` as build args; still injects them at runtime into the test container

### Fixed
- `push` command produced an invalid image reference (`image::tag`) due to double colon when constructing the full image name

### Documentation
- Added Prerequisites section to README (Docker, Docker Compose, AWS CLI)
- Updated Quick Start to include `rocketctl ecr create` as step 2
- Updated AWS ECR Setup section to use `rocketctl ecr create` instead of raw `aws` CLI commands
- Removed `.env.production.example` references from folder structure and environment variable docs
- Updated next steps in `init` output to reflect that `.env.production` is created manually on the server

## [1.0.0] - 2026-02-16

### Added
- Initial release of RocketCTL
- Convention-based Docker orchestration
- Support for monorepo and single-service repositories
- Automatic semantic versioning per service
- AWS ECR integration with automatic authentication
- Docker Compose integration for dev and production
- Template generation for docker-compose.prod.yml, Caddyfile, and .env files
- Complete command set:
  - `init` - Initialize projects
  - `build` - Build production images with version bumping
  - `push` - Push images to container registry
  - `test` - Test production builds locally
  - `dev` - Start development environment
  - `down` - Stop development environment
  - `deploy` - Deploy to production
  - `ps` - List running containers
  - `logs` - Show service logs
  - `exec` - Execute commands in containers
  - `prune` - Clean up old images
  - `list` - List all services and versions
  - `version` - Show service versions
- Installation script (install.sh)
- Uninstallation script (uninstall.sh)
- Comprehensive documentation:
  - README.md with full feature documentation
  - QUICKSTART.md for new users
  - CONTRIBUTING.md for contributors
  - FULL_PROJECT_OVERVIEW.md with complete specifications
- MIT License

### Features
- `.rocket-version` files for version tracking
- Environment variable support (.env, .env.development, .env.production)
- Build args from .env.production for frameworks like Next.js
- Interactive project initialization
- Service name validation
- Image naming convention: `<project>_<service>:<version>`
- Docker Compose v2 support
- Color-coded terminal output
- Helpful error messages

[1.4.0]: https://github.com/cjairm/rocketctl/releases/tag/v1.4.0
[1.3.0]: https://github.com/cjairm/rocketctl/releases/tag/v1.3.0
[1.2.0]: https://github.com/cjairm/rocketctl/releases/tag/v1.2.0
[1.1.0]: https://github.com/cjairm/rocketctl/releases/tag/v1.1.0
[1.0.0]: https://github.com/cjairm/rocketctl/releases/tag/v1.0.0
