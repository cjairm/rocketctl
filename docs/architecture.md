# Architecture

Reference detail for RocketCTL. The short version an agent needs up front lives in `/CLAUDE.md`.

## Layout

```
main.go              cmd.Execute()
cmd/                 15 Cobra commands, one file each (~1,300 LOC)
internal/config/     rocket.yaml parsing, validation, all path/name derivation
internal/version/    .rocket-version read/write, semver bump
internal/docker/     subprocess wrapper around the docker CLI
internal/compose/    subprocess wrapper around docker compose v2
internal/registry/   AWS ECR login + repository creation
internal/ssh/        golang.org/x/crypto/ssh client for remote deploy
internal/migrate/    runs the image's rocketctl.migrate label command in its running container
internal/templates/  go:embed templates rendered by `init`
install.sh           end-user installer (curl | bash)
uninstall.sh         end-user uninstaller
scripts/release.sh   local release builder
.github/workflows/   release.yml — tag-triggered, the only workflow
specs/v1_0_0.md      original specification
```

Dependency direction is one-way: `cmd/` → `internal/*`. No `internal` package imports another
except through `config`, and `migrate` → `ssh` for `ShellQuote` alone (one quoting function, not
two). There is no shared state between commands.

## Command shape

Every command follows the same sequence:

1. `cfg, err := config.Load()` — reads `rocket.yaml` from the working directory.
2. Resolve the service name (arg for monorepo, `cfg.Service` otherwise).
3. `cfg.ValidateService(service)`.
4. Derive paths through `Config` methods.
5. Shell out via `internal/docker`, `internal/compose`, or `internal/ssh`.
6. Return the error; `Execute` maps it to exit code 1 — except a `*migrate.ExitError`, whose code
   is the app's own and is passed through.

`ecr` is the only command with a subcommand (`ecr create`).

## Config

`Config` requires `project`, `registry`, `region`, and exactly one of `service` or `services`.
Optional: `domain`, `ip`, `ssh_user`, `ssh_key_path`. The README's example blocks match the struct
field-for-field.

Derived values. Note `filepath.Join` **cleans** its result, so a monorepo service directory is
`api`, not `./api`:

| Method                    | Single-service                             | Monorepo                            |
| ------------------------- | ------------------------------------------ | ----------------------------------- |
| `GetServiceDirectory`     | `.`                                        | `<service>`                         |
| `GetVersionFilePath`      | `.rocket-version`                          | `<service>/.rocket-version`         |
| `GetDockerfilePath(prod)` | `Dockerfile[.production]`                  | `<service>/Dockerfile[.production]` |
| `GetImageName`            | `<project>_<service>`                      | same                                |
| `GetFullImageName`        | `<registry>/<project>_<service>:<version>` | same                                |

## Versioning

`.rocket-version` holds a strict `X.Y.Z` string — pre-release and build metadata are rejected by
`validateSemver`. `Get` auto-creates the file at `0.1.0` when missing (a write during a read).
`build` reads, computes the bump, builds, tags, and only then writes.

## Deploy flow

`rocketctl deploy` is remote-first:

1. Connect over SSH (custom key → `~/.ssh/id_ed25519` → `~/.ssh/id_rsa`; passphrases unsupported).
2. `mkdir -p ~/apps/<project>` on the server.
3. Upload `docker-compose.prod.yml` as `docker-compose.yml`.
4. Upload `caddy/Caddyfile` if `domain` is set and the file exists.
5. Upload `.env.example` → `.env` per service, only when `.env` is absent on the server.
6. Run ECR login, then `docker compose pull` and `docker compose up -d` remotely with each
   service's `<SERVICE>_VERSION` set from its local `.rocket-version`, so the server runs the
   version that was built rather than falling back to `:latest`.

Uploads use a stdin pipe into `cat > path`, not SCP. There is no rollback, no dry-run, and no
backup of the files it overwrites.

## Build and release

`make` targets: `build`, `clean`, `release`, `install`, `test`, `fmt`, `vet`, `check`, `mutate`, `help`.
Default goal is `help`. Releases are macOS-only (`darwin/amd64`, `darwin/arm64`).

Tagging `v*` triggers `.github/workflows/release.yml`, which builds both binaries, generates
checksums, slices the matching section out of `CHANGELOG.md`, and publishes a GitHub Release. The
release workflow builds both binaries and asserts the built binary reports the tag it was built
from. `.github/workflows/ci.yml` gates pull requests and pushes to `main` with `gofmt -l`,
`go vet`, and `go test -race` on both `ubuntu-latest` and `macos-latest`, plus a `golangci-lint`
job — see the "Testing" section in `CLAUDE.md`.

## Known defects

Verified against the tree, worth fixing:

- Go version disagrees: `go.mod` says `1.24.0`, README says `1.23+` (CI and the release workflow
  both now derive their toolchain from `go.mod`, so only the README claim remains stale).
- `main.go` and `cmd/root.go` still carry the `Copyright © 2026 NAME HERE <EMAIL ADDRESS>` placeholder.

## Out of scope by decision

Multi-registry support, Linux/Windows binaries, dry-run, rollback, shell completion. Treat these as
product decisions rather than gaps unless a tracker issue says otherwise.
