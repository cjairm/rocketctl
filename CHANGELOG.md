# Changelog

All notable changes to RocketCTL will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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

[1.0.0]: https://github.com/cjairm/rocketctl/releases/tag/v1.0.0
