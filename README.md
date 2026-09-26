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

Create `.env.example` (per service in a monorepo) with the variables your services need. On the first deploy, `rocketctl deploy` uploads it to the server as `.env` (mode 0600) and never overwrites it afterwards, so edit the real values on the server. The `.env` is injected at runtime via `env_file` in `docker-compose.prod.yml` -- never commit secrets to git.

### 4. Build, Push, Deploy

```bash
rocketctl ecr create             # Create ECR repos (once)
rocketctl up --prod              # Test production build locally (E2E)
rocketctl build api --bump patch --push # Build, bump version, push to registry
rocketctl deploy                        # Deploy (on production server)
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
- `.env.example` -- Template uploaded by `deploy` as `.env` on the server on first deploy (real secrets live only in the server's `.env`, injected via `env_file`)

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
    .env.example
  web/
    Dockerfile
    Dockerfile.production
    .rocket-version
    .env.example
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
  .env.example
```

## Commands

Every command has a description and examples in `rocketctl <command> --help`.

| Command                                                  | Description                             |
| -------------------------------------------------------- | --------------------------------------- |
| `rocketctl init`                                         | Initialize project                      |
| `rocketctl build [service] --bump [patch\|minor\|major]` | Build production image and bump version |
| `rocketctl build [service] --push`                       | Build, then push the image to registry  |
| `rocketctl push [service]`                               | Push image to registry                  |
| `rocketctl up [service] [--build] [--no-cache]`          | Start dev environment                   |
| `rocketctl up --prod [service]`                          | Test production build locally (E2E)     |
| `rocketctl down`                                         | Stop dev environment                    |
| `rocketctl down --prod`                                  | Stop production/test environment        |
| `rocketctl deploy`                                       | Deploy to production                    |
| `rocketctl deploy --clean`                               | Deploy, then free space (see below)     |
| `rocketctl migrate [service]`                            | Dry-run the app's migrations on server  |
| `rocketctl migrate [service] --apply [--yes]`            | Apply the app's migrations on server    |
| `rocketctl backup [service] [--keep N] [--out DIR]`      | Back up the app's data to this machine  |
| `rocketctl restore [service] [file] [--yes]`             | Load a backup into the local dev stack  |
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
rocketctl build api --bump minor --push # Build, version and push
rocketctl deploy                        # Deploy to remote server
rocketctl migrate api            # Preview the release's migrations (dry run)
rocketctl migrate api --apply    # Apply them, after confirmation
```

**Backup:**

```bash
rocketctl backup api             # Save a backup to ~/.rocketctl/backups/<project>/
rocketctl restore api            # Load the newest one into the local dev stack
```

## Deployment

### Overview

The `rocketctl deploy` command automates deployment to a remote server via SSH. It:

1. Pins every built image in `docker-compose.prod.yml` to its `.rocket-version` (a bad file fails here, before connecting)
2. Connects to your server via SSH
3. Creates directory structure: `~/apps/<PROJECT-NAME>/`
4. Uploads necessary files:
   - `docker-compose.yml` (the pinned copy of your `docker-compose.prod.yml`)
   - `Caddyfile` (if domain is configured)
   - `.env` (if it doesn't exist on the server)
5. Authenticates with ECR
6. Pulls latest Docker images
7. Restarts services with zero-downtime

Deploy sets the tag of every compose service by image, not by name: any service whose `image:` is a repository rocketctl builds gets that service's current version, whatever tag the file has. Services that reuse a built image (e.g. a worker or scheduler running the API image with another command) are deployed with the same tag automatically. Other images (e.g. `caddy:2-alpine`) are left as written. Once the deploy succeeds, your local `docker-compose.prod.yml` gets the same tags (only the `image:` values change; comments and formatting stay), so commit it with `.rocket-version`. If the file changed while deploying, it is left alone and deploy says so. Deploy prints what it pinned:

```
🏷  Pinning image tags:
   myapp_api:1.4.0 → myapp-api, myapp-scheduler
```

A built image that no compose service uses is flagged `⚠️  not pinned`. Services whose `image:` comes from a YAML anchor or merge key (`<<: *base`) are not followed; write `image:` on the service itself.

Because the tag comes from `.rocket-version`, editing the tag in `docker-compose.prod.yml` no longer deploys an older version. To roll back, set `.rocket-version` to the older version and run `rocketctl deploy`, then set it back to the newest built version before the next `build`. Otherwise the next build reuses a version that already exists in ECR and overwrites that image.

### Freeing space with `--clean`

`rocketctl deploy --clean` runs only after the deploy succeeds, then:

1. On the server: `docker container prune -f && docker image prune -a -f && docker volume prune -f && docker network prune -f && docker system prune -a -f`. This is server-wide: it affects every app on the server, not just this project. `docker volume prune` is skipped (with a warning) when the server's Docker is older than 23, because older versions also delete unused named volumes.
2. In ECR: deletes this project's version tags older than the previous one. Deploying `1.3.0` keeps `1.3.0` and `1.2.0` and deletes `1.1.0` and older. Tags that aren't `X.Y.Z` (e.g. `latest`) are never touched. Your AWS user needs `ecr:BatchDeleteImage`.
3. Locally: removes the same old versions of this project's images.

The previous version stays in ECR, so rolling back one version (above) still works after a clean; anything older has to be rebuilt.

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

## Migrations

`rocketctl migrate` runs an app's release migrations (schema and data scripts) after a deploy.
rocketctl knows nothing about the app: **the app declares, rocketctl runs.**

### Declaring the command

The app's production image declares one migration command with a label:

```dockerfile
LABEL rocketctl.migrate="python bin/migrate.py"
```

The label ships inside the image, so the migration code always matches the code that is deployed.
An image without the label has nothing to migrate: `rocketctl migrate` says so and exits 0.

### Running it

```bash
rocketctl migrate api             # dry run - always the default
rocketctl migrate api --apply     # real run, asks for confirmation first
rocketctl migrate api --apply -y  # real run, no prompt (CI)
```

Single-service projects can leave out the service name. Over SSH to the `ip` in `rocket.yaml`,
rocketctl:

1. Finds the service's **running** container (compose service `<project>-<service>`). It refuses if
   none is running, or if more than one is (scaled services), and names them.
2. Reads `rocketctl.migrate` from that container's image (`docker image inspect`).
3. With `--apply`, shows host, container, image tag and command, and asks `(y/n)` unless `--yes`.
4. Runs `<command> --dry-run` or `<command> --apply` in that container with `docker exec … sh -c`.
   It always passes exactly one of the two. Without `--apply` it is always `--dry-run`.
5. Streams stdout and stderr live, and saves them to `.rocket-logs/migrate-<service>-<timestamp>.log`
   next to `rocket.yaml`. Add `.rocket-logs/` to your `.gitignore`.
6. Exits with the command's own exit code. A non-zero exit is reported as a failed migration, along
   with the path to the log.

The image needs a `/bin/sh`. If the command is Python, set `PYTHONUNBUFFERED=1` in the image so
output streams live instead of arriving in one block at the end.

### Contract for apps

An app that wants migrations provides **one** command that:

1. Takes `--dry-run` or `--apply`, exactly one. Given neither, both, or anything else, it refuses
   to run and exits non-zero.
2. Runs every step itself, in order, with the rules in code. It reads no input files and takes no
   other arguments.
3. Is idempotent: running `--apply` twice is safe, and the second run changes nothing.
4. With `--dry-run`, changes nothing and prints what `--apply` would do.
5. Exits non-zero on the first failure and stops.

## Backups

`rocketctl backup` saves a backup of the app's data to your machine, and `rocketctl restore` loads
one into your local dev stack. Like migrations: **the app declares, rocketctl runs.** Nothing runs
on the server: the backup runs on this machine, so the server does none of the work.

### Declaring the commands

The app's production image declares its backup command, and optionally the saved file's
extension. Its **dev** image (`Dockerfile`) declares the restore command, because that is the
image the dev container runs:

```dockerfile
# Dockerfile.production
LABEL rocketctl.backup="sh bin/backup.sh"
LABEL rocketctl.backup.suffix=".tar.gz"

# Dockerfile
LABEL rocketctl.restore="sh bin/restore.sh"
```

An image without the label has nothing to back up or restore: the command says so and exits 0.
Without `rocketctl.backup.suffix` the file ends in `.backup`. The suffix must be a plain extension
(a dot, then letters and digits, e.g. `.tar.gz`, at most 16 characters); anything else is refused
before the command runs.

### Backing up

```bash
rocketctl backup api                  # save to ~/.rocketctl/backups/<project>/
rocketctl backup api --keep 10        # keep the newest 10 instead of 5
rocketctl backup api --out /mnt/safe  # save in another folder
rocketctl backup api --env-file ~/secrets/api.env
```

The backup command connects to the **real** data store, so it needs its settings. Put them in
`<service dir>/.env.backup` (or pass `--env-file`), separate from `.env` so your dev stack never
points at production:

```bash
chmod 600 api/.env.backup
echo ".env.backup" >> .gitignore
```

rocketctl refuses a settings file that others can read or that git tracks, and never reads it
itself: docker gets its path, so no value appears in output, logs or process arguments. The data
store must accept connections from this machine.

Single-service projects can leave out the service name. rocketctl:

1. Reads `rocketctl.backup` and `rocketctl.backup.suffix` from the service's image at its current
   `.rocket-version`, as `rocketctl build` tagged it on this machine. It never pulls.
2. Runs the command in a throwaway container of that image:
   `docker run --rm --pull never --env-file <settings> <image> sh -c <command>`.
3. Streams its **stdout** into `<project>-<service>-<timestamp><suffix>.partial`, reporting
   progress every 10 MB. Its **stderr** is shown live and saved to
   `.rocket-logs/backup-<service>-<timestamp>.log`.
4. On exit 0 with a non-empty file, renames it to `<project>-<service>-<timestamp><suffix>` and
   prints the image, path, size and sha256 (also recorded in the log).
5. Then deletes that service's backups beyond the newest `--keep` (default 5). Only files named
   exactly like its own backups, with the current suffix, are ever touched.

Backups hold real data, so they live outside any repository, in folders only you can open (700)
and files only you can read (600). A non-zero exit or an empty backup is reported as a failure,
the partial file is deleted, and nothing is pruned; a non-zero exit is passed through as
rocketctl's own. A `.partial` file left in the folder is from a run that was interrupted and is
never a usable backup.

### Restoring

```bash
rocketctl restore api                 # the newest backup of api
rocketctl restore api <file>          # a specific backup
rocketctl restore api --yes           # no prompt
```

`restore` only ever touches the **local dev stack** started by `rocketctl up`, never the server.
It finds the service's running container with
`docker compose -f docker-compose.yml ps <project>-<service>` (the same name every command uses), shows the container, file, size and date,
asks `(y/n)` unless `--yes`, then runs `docker exec -i <container> sh -c <command>` with the
backup on stdin. Output is streamed and saved to `.rocket-logs/restore-<service>-<timestamp>.log`,
and a non-zero exit is passed through.

### Contract for apps

The backup command:

1. Writes one complete backup to **stdout**, and nothing else: every message goes to stderr.
2. Exits non-zero on any failure. For a pipeline (`dump | compress`) that means the exit status
   must reflect every step, not just the last one - otherwise a failed first step still exits 0
   and the "backup" is a valid but empty archive.
3. Only reads. It never changes the app's data.
4. Works in a fresh container using only the environment it is given. It takes no arguments.

The restore command:

1. Reads one backup from **stdin** and replaces the app's **local** data with it.
2. Refuses to run if its settings point anywhere but a local data store. rocketctl cannot check
   this: it is the app's own guard against restoring into production.
3. Exits non-zero on failure.

### Follow-up: scheduled jobs

Not built yet. Cron-style jobs can use the same pattern later: a `rocketctl.jobs` label listing
`schedule command` pairs that rocketctl installs on the server, so schedules also ship with the
image they run against.

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
