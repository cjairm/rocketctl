# Changelog

All notable changes to RocketCTL will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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

[1.1.0]: https://github.com/cjairm/rocketctl/releases/tag/v1.1.0
[1.0.0]: https://github.com/cjairm/rocketctl/releases/tag/v1.0.0
