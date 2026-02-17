# Releasing RocketCTL

This guide explains how to create a new release of RocketCTL.

## Prerequisites

- Write access to the repository
- Git configured with your credentials
- Go 1.23+ installed
- Clean working directory (no uncommitted changes)

## Release Process

### Option 1: Automated with Scripts (Recommended)

```bash
# 1. Update CHANGELOG.md
# Add your release notes under a new version section

# 2. Run the release script
./scripts/release.sh 1.0.0

# 3. Push the tag (GitHub Actions will handle the rest)
git push origin v1.0.0
```

That's it! GitHub Actions will automatically:
- Build binaries for both macOS architectures
- Generate checksums
- Create the GitHub Release
- Upload all artifacts

### Option 2: Manual Release

If you prefer to do it manually or GitHub Actions isn't available:

#### Step 1: Update CHANGELOG.md

Add a new section at the top of CHANGELOG.md:

```markdown
## [1.0.0] - 2026-02-17

### Added
- New feature X
- New feature Y

### Changed
- Updated Z

### Fixed
- Bug fix A

[1.0.0]: https://github.com/cjairm/rocketctl/releases/tag/v1.0.0
```

#### Step 2: Commit the changes

```bash
git add CHANGELOG.md
git commit -m "chore: prepare release v1.0.0"
git push origin main
```

#### Step 3: Build the binaries

```bash
make release VERSION=1.0.0
```

This creates:
- `dist/rocketctl-darwin-amd64` - For Intel Macs
- `dist/rocketctl-darwin-arm64` - For Apple Silicon Macs
- `dist/checksums.txt` - SHA256 checksums

#### Step 4: Test the binaries

```bash
# Test Intel binary
./dist/rocketctl-darwin-amd64 --help

# Test ARM binary (if on Apple Silicon)
./dist/rocketctl-darwin-arm64 --help
```

#### Step 5: Create and push the git tag

```bash
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0
```

#### Step 6: Create GitHub Release

1. Go to: https://github.com/cjairm/rocketctl/releases/new
2. Select tag: `v1.0.0`
3. Release title: `v1.0.0`
4. Copy release notes from `dist/release-notes.md` or write your own
5. Upload files:
   - `dist/rocketctl-darwin-amd64`
   - `dist/rocketctl-darwin-arm64`
   - `dist/checksums.txt`
6. Click "Publish release"

## Release Checklist

Before releasing, make sure:

- [ ] CHANGELOG.md is updated with the new version
- [ ] All tests pass (`go test ./...`)
- [ ] Code is formatted (`go fmt ./...`)
- [ ] No `go vet` warnings (`go vet ./...`)
- [ ] README.md is up to date
- [ ] All new features are documented
- [ ] Git working directory is clean
- [ ] You're on the `main` branch
- [ ] Local branch is up to date with origin

## Version Numbering

RocketCTL follows [Semantic Versioning](https://semver.org/):

- **MAJOR** (1.0.0 → 2.0.0): Breaking changes
- **MINOR** (1.0.0 → 1.1.0): New features, backwards compatible
- **PATCH** (1.0.0 → 1.0.1): Bug fixes, backwards compatible

Examples:
- `1.0.0` - First stable release
- `1.0.1` - Bug fix release
- `1.1.0` - New features added
- `2.0.0` - Breaking changes introduced

## Post-Release

After releasing:

1. **Announce the release**:
   - Tweet about it
   - Post in relevant communities
   - Update documentation sites

2. **Monitor for issues**:
   - Watch GitHub issues
   - Check the Discussions tab
   - Monitor installation reports

3. **Update main branch**:
   - Merge any hotfixes
   - Start working on next version

## Hotfix Releases

For urgent bug fixes:

```bash
# 1. Fix the bug on main
git checkout main
# ... make fixes ...
git commit -m "fix: critical bug description"

# 2. Create hotfix release
./scripts/release.sh 1.0.1
git push origin v1.0.1
```

## Pre-releases

For beta or RC versions:

```bash
# Create a pre-release tag
git tag -a v1.1.0-beta.1 -m "Beta release"
git push origin v1.1.0-beta.1

# In GitHub Release, check "This is a pre-release"
```

## Rolling Back a Release

If you need to remove a bad release:

```bash
# Delete the tag locally
git tag -d v1.0.0

# Delete the tag on GitHub
git push origin :refs/tags/v1.0.0

# Delete the GitHub Release via web interface
# Then create a new fixed release
```

## Troubleshooting

### GitHub Actions fails

1. Check the Actions tab for error logs
2. Common issues:
   - Go version mismatch
   - Permission issues (check `GITHUB_TOKEN`)
   - Build errors (test locally first with `make release`)

### Binaries don't work

1. Test locally before pushing tag
2. Check build flags in Makefile
3. Verify target architecture matches

### Install script fails

1. Test with both architectures
2. Check GitHub Release is public
3. Verify binary names match expected format

## Questions?

If you encounter any issues with the release process:
- Check existing GitHub Issues
- Create a new Discussion
- Contact maintainers directly

## Tools Reference

- **Makefile targets**:
  - `make release` - Build all binaries
  - `make clean` - Clean build artifacts
  - `make build` - Build for current platform
  - `make test` - Run tests

- **Scripts**:
  - `scripts/release.sh <version>` - Automated release preparation
  - `install.sh` - User installation script
  - `uninstall.sh` - Uninstallation script

- **GitHub Actions**:
  - `.github/workflows/release.yml` - Automated release workflow
