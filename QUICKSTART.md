# RocketCTL Quick Start Guide

Get started with RocketCTL in 5 minutes!

## Installation

```bash
git clone https://github.com/cjairm/rocketctl.git
cd rocketctl
./install.sh
source ~/.zshrc  # or source ~/.bashrc for bash
```

## Your First Project

### 1. Initialize a New Project

```bash
cd your-project
rocketctl init
```

Answer the prompts:
- **Project name**: `myapp` (defaults to directory name)
- **Container registry URL**: `123456789.dkr.ecr.us-east-2.amazonaws.com`
- **AWS region**: `us-east-2`
- **Domain** (optional): `myapp.com`
- **Monorepo?**: `n` (for single service) or `y` (for multiple services)
- **Service name**: `api` (or comma-separated list for monorepo)

This creates:
```
✓ rocket.yaml
✓ .rocket-version (initialized to 0.1.0)
✓ docker-compose.prod.yml
✓ caddy/Caddyfile (if domain provided)
✓ .env.production.example
```

### 2. Create Your Dockerfiles

Create two Dockerfiles in your project root (or in service folders for monorepo):

**Dockerfile** (for development):
```dockerfile
FROM node:18
WORKDIR /app
COPY package*.json ./
RUN npm install
COPY . .
CMD ["npm", "run", "dev"]
```

**Dockerfile.production** (for production):
```dockerfile
FROM node:18 AS builder
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM node:18-alpine
WORKDIR /app
COPY --from=builder /app/dist ./dist
COPY --from=builder /app/package*.json ./
RUN npm ci --production
CMD ["npm", "start"]
```

### 3. Set Up Environment Variables

```bash
cp .env.production.example .env.production
# Edit .env.production with your secrets
```

Example `.env.production`:
```env
DATABASE_URL=postgresql://user:pass@db:5432/myapp
API_KEY=your-secret-api-key
NODE_ENV=production
```

### 4. Build Your First Image

```bash
# Build production image (bumps version 0.1.0 -> 0.1.1)
rocketctl build api --bump patch
```

This will:
- Read current version (0.1.0)
- Bump to 0.1.1
- Build using Dockerfile.production
- Tag as `myapp_api:0.1.1` and `registry/myapp_api:0.1.1`
- Update .rocket-version

### 5. Test Locally

```bash
rocketctl test api
```

This runs the production image locally so you can verify it works.

### 6. Push to Registry

First, create the ECR repository:
```bash
aws ecr create-repository --repository-name myapp_api --region us-east-2
```

Then push:
```bash
rocketctl push api
```

RocketCTL automatically authenticates with AWS ECR before pushing.

### 7. Deploy to Production

On your production server:

```bash
# Clone your project
git clone https://github.com/yourname/yourproject.git
cd yourproject

# Install rocketctl
git clone https://github.com/cjairm/rocketctl.git /tmp/rocketctl
cd /tmp/rocketctl
./install.sh
source ~/.zshrc

# Back to your project
cd ~/yourproject

# Copy production environment file
cp .env.production.example .env.production
# Edit with real values: nano .env.production

# Deploy!
rocketctl deploy
```

This will:
- Authenticate with AWS ECR
- Pull latest images
- Start services using docker-compose.prod.yml

### 8. Monitor Your Services

```bash
# Check running containers
rocketctl ps

# View logs (follow mode)
rocketctl logs api -f

# Execute commands in container
rocketctl exec api bash
```

## Common Workflows

### Development Workflow

```bash
# Create docker-compose.yml for dev environment
# Then start dev environment
rocketctl dev

# View logs
rocketctl logs api -f

# Restart with rebuild
rocketctl down
rocketctl dev --build

# Shell into container
rocketctl exec api sh
```

### Release Workflow

```bash
# 1. Test production build locally
rocketctl test api

# 2. Build new version
rocketctl build api --bump minor  # 0.1.1 -> 0.2.0

# 3. Push to registry
rocketctl push api

# 4. Deploy (on production server)
rocketctl deploy

# 5. Verify
rocketctl ps
rocketctl logs api -f
```

### Version Management

```bash
# Check current versions
rocketctl version        # All services
rocketctl version api    # Specific service

# List all services
rocketctl list

# Clean up old images
rocketctl prune
```

## Monorepo Example

For a monorepo with multiple services:

```
myapp/
  rocket.yaml              # Lists all services
  docker-compose.yml       # Dev environment
  docker-compose.prod.yml  # Production
  api/
    Dockerfile
    Dockerfile.production
    .env.production
    .rocket-version        # 0.2.1
  web/
    Dockerfile
    Dockerfile.production
    .env.production
    .rocket-version        # 0.3.0
  worker/
    Dockerfile
    Dockerfile.production
    .env.production
    .rocket-version        # 0.1.5
```

Commands:
```bash
# Build specific service
rocketctl build api --bump patch
rocketctl build web --bump minor

# Push specific service
rocketctl push api

# Dev commands
rocketctl dev api         # Start only API
rocketctl logs web -f     # View web logs
rocketctl exec worker sh  # Shell into worker
```

## Tips & Best Practices

1. **Version Bumps**:
   - `patch`: Bug fixes (0.1.0 → 0.1.1)
   - `minor`: New features (0.1.0 → 0.2.0)
   - `major`: Breaking changes (0.1.0 → 1.0.0)

2. **Environment Variables**:
   - Never commit `.env.production` to git
   - Always update `.env.production.example` when adding new vars
   - Use `.env.production` for build-time vars (like `NEXT_PUBLIC_*`)

3. **Docker Optimization**:
   - Use multi-stage builds in Dockerfile.production
   - Use `.dockerignore` to exclude unnecessary files
   - Keep images small (use alpine base images)

4. **AWS ECR**:
   - Create repositories before first push
   - ECR auth expires after 12 hours (rocketctl re-authenticates automatically)
   - Use lifecycle policies to clean up old images

5. **Production**:
   - Always test with `rocketctl test` before building
   - Review generated docker-compose.prod.yml
   - Customize Caddyfile for your routing needs

## Getting Help

```bash
# General help
rocketctl --help

# Command-specific help
rocketctl build --help
rocketctl init --help
```

For more details, see the [full README](README.md).
