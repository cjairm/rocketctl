# Contributing to RocketCTL

Thank you for your interest in contributing to RocketCTL! This document provides guidelines and instructions for contributing.

## Getting Started

1. **Fork the repository**
   ```bash
   # Click "Fork" on GitHub, then clone your fork
   git clone https://github.com/YOUR_USERNAME/rocketctl.git
   cd rocketctl
   ```

2. **Install dependencies**
   ```bash
   go mod download
   ```

3. **Build and test**
   ```bash
   go build -o rocketctl
   ./rocketctl --help
   ```

## Development Setup

### Prerequisites

- Go 1.23 or higher
- Docker and Docker Compose
- AWS CLI (for testing ECR integration)
- Git

### Project Structure

```
rocketctl/
├── cmd/              # CLI commands (each command is a separate file)
├── internal/         # Internal packages
│   ├── config/      # Configuration management
│   ├── docker/      # Docker operations
│   ├── compose/     # Docker Compose wrapper
│   ├── version/     # Version management
│   ├── registry/    # Registry authentication (ECR)
│   └── templates/   # Embedded templates
├── main.go          # Entry point
└── go.mod           # Go module definition
```

### Coding Standards

1. **Follow Go conventions**
   - Use `gofmt` to format code
   - Run `go vet` to catch common issues
   - Use meaningful variable and function names

2. **Error handling**
   - Always handle errors explicitly
   - Provide context in error messages
   - Use `fmt.Errorf` with `%w` for error wrapping

3. **Documentation**
   - Add doc comments for exported functions
   - Update README.md for user-facing changes
   - Include examples in command help text

4. **Testing**
   - Test your changes manually
   - Ensure existing commands still work
   - Test both monorepo and single-service scenarios

## Making Changes

### 1. Create a Branch

```bash
git checkout -b feature/your-feature-name
# or
git checkout -b fix/issue-description
```

### 2. Make Your Changes

Follow these guidelines:

- **Keep changes focused**: One feature/fix per PR
- **Write clear commit messages**: Use conventional commits format
  ```
  feat: add support for custom Dockerfile names
  fix: correct version bump calculation
  docs: update installation instructions
  ```
- **Update documentation**: If you change behavior, update docs
- **Test thoroughly**: Test with real projects if possible

### 3. Test Your Changes

```bash
# Build
go build -o rocketctl

# Test a command
./rocketctl init

# Test in a real project
cd /tmp/test-project
/path/to/rocketctl init
```

### 4. Commit and Push

```bash
git add .
git commit -m "feat: your feature description"
git push origin feature/your-feature-name
```

### 5. Create a Pull Request

1. Go to your fork on GitHub
2. Click "New Pull Request"
3. Select your branch
4. Fill in the PR template:
   - **Description**: What does this PR do?
   - **Motivation**: Why is this change needed?
   - **Testing**: How did you test this?
   - **Breaking Changes**: Does this break existing functionality?

## Areas for Contribution

### High Priority

- [ ] Unit tests for core packages
- [ ] Integration tests
- [ ] Support for other container registries (Docker Hub, GCR, etc.)
- [ ] Shell completion scripts
- [ ] Windows support

### Medium Priority

- [ ] Better error messages
- [ ] Progress bars for long operations
- [ ] Dry-run mode for commands
- [ ] Configuration validation command
- [ ] Health check integration

### Documentation

- [ ] More examples in README
- [ ] Tutorial videos
- [ ] Blog posts about usage patterns
- [ ] Troubleshooting guide

### New Features

Ideas for new features (discuss in issues first):
- Multi-environment support (staging, prod, etc.)
- Rollback command
- Image scanning integration
- Slack/Discord notifications
- GitHub Actions integration

## Reporting Issues

When reporting issues, please include:

1. **RocketCTL version**: Run `rocketctl --help` to see version info
2. **Go version**: Run `go version`
3. **OS and version**: e.g., macOS 13.0, Ubuntu 22.04
4. **Steps to reproduce**: Clear, numbered steps
5. **Expected behavior**: What should happen?
6. **Actual behavior**: What actually happened?
7. **Error messages**: Full error output
8. **Configuration**: Your rocket.yaml (remove secrets)

### Example Issue

```markdown
**RocketCTL Version**: Latest from main branch
**Go Version**: 1.23.6
**OS**: macOS 14.0

**Steps to Reproduce**:
1. Run `rocketctl init` in a new directory
2. Answer prompts with default values
3. Run `rocketctl build api --bump patch`

**Expected**: Build should succeed
**Actual**: Error: "Dockerfile.production not found"

**Error Output**:
```
Error: failed to build image: stat Dockerfile.production: no such file or directory
```

**rocket.yaml**:
```yaml
project: test
service: api
registry: example.com
region: us-east-2
```
```

## Pull Request Review Process

1. **Automated checks**: PRs must pass:
   - Build successfully
   - No `go vet` warnings
   - Code is formatted with `gofmt`

2. **Code review**: A maintainer will review your PR:
   - Code quality and style
   - Functionality and correctness
   - Documentation completeness
   - Potential edge cases

3. **Feedback**: Address review comments:
   - Push new commits to the same branch
   - No need to create a new PR

4. **Merge**: Once approved, a maintainer will merge your PR

## Code of Conduct

- Be respectful and inclusive
- Welcome newcomers
- Focus on constructive feedback
- Assume good intentions
- Help others learn and grow

## Questions?

- **General questions**: Open a GitHub Discussion
- **Bug reports**: Open a GitHub Issue
- **Feature requests**: Open a GitHub Issue with [Feature Request] prefix
- **Security issues**: Email maintainers directly (see README)

## License

By contributing to RocketCTL, you agree that your contributions will be licensed under the MIT License.

## Recognition

Contributors will be:
- Listed in the README
- Mentioned in release notes
- Appreciated in the community! 🎉

Thank you for contributing to RocketCTL! 🚀
