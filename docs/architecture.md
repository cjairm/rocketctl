# Architecture

Reference detail for RocketCTL. The short version an agent needs up front lives in `/CLAUDE.md`.

## Layout

```
main.go              cmd.Execute()
cmd/                 17 Cobra commands, one file each (~1,300 LOC)
internal/config/     rocket.yaml parsing, validation, all path/name derivation
internal/version/    .rocket-version read/write, semver bump
internal/docker/     subprocess wrapper around the docker CLI
internal/compose/    subprocess wrapper around docker compose v2, plus PinImages (compose YAML tag rewrite)
internal/registry/   AWS ECR login + repository creation
internal/ssh/        golang.org/x/crypto/ssh client for remote deploy
internal/container/  finds a service's container and runs an app-declared command in it, on the
                     server over SSH or on this machine (Local)
internal/migrate/    runs the image's rocketctl.migrate label command in its running container
internal/backup/     runs the image's rocketctl.backup label command in a throwaway local container
                     and saves its stdout; nothing runs on the server
internal/restore/    feeds a backup to the dev image's rocketctl.restore command in the local dev stack
internal/templates/  go:embed templates rendered by `init`
install.sh           end-user installer (curl | bash)
uninstall.sh         end-user uninstaller
scripts/release.sh   local release builder
.github/workflows/   release.yml — tag-triggered, the only workflow
specs/v1_0_0.md      original specification
```

Dependency direction is one-way: `cmd/` → `internal/*`. No `internal` package imports another
except through `config`, `container`/`backup`/`restore` → `ssh` for `ShellQuote` alone (one
quoting function, not two), `restore` → `backup` for `Newest` and `HumanSize`, and
`migrate`/`backup`/`restore` → `container`, which holds what they share: the container lookup,
the `docker exec` command lines, the local runner, the locked log writer and the exit-status hint.
There is no shared state between commands.

## Command shape

Every command follows the same sequence:

1. `cfg, err := config.Load()` — reads `rocket.yaml` from the working directory.
2. Resolve the service name (arg for monorepo, `cfg.Service` otherwise).
3. `cfg.ValidateService(service)`.
4. Derive paths through `Config` methods.
5. Shell out via `internal/docker`, `internal/compose`, or `internal/ssh`.
6. Return the error; `Execute` maps it to exit code 1 — except an app command's failure
   (`*migrate.ExitError`, `*backup.ExitError`, `*restore.ExitError`: anything with `ExitCode() int`), whose code is the
   app's own and is passed through.

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
| `GetImageRepository`      | `<registry>/<project>_<service>`           | same                                |
| `GetFullImageName`        | `<registry>/<project>_<service>:<version>` | same                                |

## Versioning

`.rocket-version` holds a strict `X.Y.Z` string — pre-release and build metadata are rejected by
`validateSemver`. `Get` auto-creates the file at `0.1.0` when missing (a write during a read).
`build` reads, computes the bump, builds, tags, and only then writes.

## Deploy flow

`rocketctl deploy` is remote-first:

1. Read `docker-compose.prod.yml` and run `compose.PinImages`: every service whose image
   repository is a built one (`GetImageRepository`) gets that service's version. Matching is by
   image, not service name, so services reusing a built image follow it. A built image no service
   uses is flagged "not pinned" (anchors and merge keys are not followed). Runs before connecting,
   so a bad file fails with nothing done on the server. The local file is not modified.
2. Connect over SSH (custom key → `~/.ssh/id_ed25519` → `~/.ssh/id_rsa`; passphrases unsupported).
3. `mkdir -p ~/apps/<project>` on the server.
4. Upload the pinned copy as `docker-compose.yml`.
5. Upload `caddy/Caddyfile` if `domain` is set and the file exists.
6. Upload `.env.example` → `.env` per service, only when `.env` is absent on the server.
7. Run ECR login, then `docker compose pull` and `docker compose up -d` remotely with each
   service's `<SERVICE>_VERSION` set from its local `.rocket-version`, so the server runs the
   version that was built rather than falling back to `:latest`.
8. Only with `--clean`, and only after step 7 succeeds (`cleanAfterDeploy`): run the server-wide
   Docker prune chain (`remoteCleanupCommand`; `volume prune` only when the server's Docker is
   23+, per `volumePruneIsSafe`), then delete ECR tags and local images that `version.Stale`
   returns: X.Y.Z tags older than the one just before the deployed version. Current and
   previous always survive, so a one-version rollback can still pull from ECR.

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
