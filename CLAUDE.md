# RocketCTL

Convention-based CLI that builds, versions, pushes and deploys Docker images.
Cobra commands in `cmd/`, logic in `internal/`. Deeper detail: `docs/architecture.md`.

## Must

- **`rocket.yaml` is the only input.** Every command calls `config.Load()` itself — there is no
  global config, no persistent root flags, no context passing. Keep it that way.
- **Never build paths or image names by hand.** Use the `Config` methods:
  `GetServiceDirectory`, `GetVersionFilePath`, `GetDockerfilePath`, `GetEnvProductionPath`,
  `GetImageName`, `GetFullImageName`. The `<project>_<service>:<version>` scheme is not configurable.
- **Handle both repo modes.** `cfg.IsMonorepo()` decides: monorepo requires an explicit service arg
  and resolves to `./<service>/`; single-service infers from `cfg.Service` and resolves to `.`.
  Always `cfg.ValidateService(service)` before using a user-supplied name.
- **Bump the version only after the operation succeeds.** `version.Set` runs after `docker.Build`
  and `docker.Tag` (`cmd/build.go:104`). Writing it earlier orphans `.rocket-version` on failure.
- **Use `docker compose` (v2), never `docker-compose`.**
- **Run `make check` before finishing** (`fmt` + `vet` + `test`).

## Watch out

- **`version.Get` writes.** A missing `.rocket-version` is created at `0.1.0` as a side effect of
  reading. Intentional — don't "fix" it.
- **`rocketctl deploy` runs on a remote server and has no dry-run and no rollback.** It uploads
  files and restarts services over SSH. Never add auto-execution or widen its blast radius casually.
- **Remote paths are interpolated unquoted into shell strings** (`ssh.go` `mkdir -p %s`, `cat > %s`,
  `test -f %s`; `deploy.go` `cd %s && ...`). `cfg.Project` flows straight in. Quote or validate any
  new value you put on that path.
- **Host keys are not verified** — `ssh.InsecureIgnoreHostKey()` at `internal/ssh/ssh.go:29`.
- **AWS ECR is the only registry, macOS the only release target.** Both are deliberate scope limits.
- **Templates are `go:embed`ed and generated once by `init`.** Users edit the output, not the
  templates. Don't add a regenerate command that would overwrite their edits.

## Testing

`internal/config` and `internal/version` have table-driven tests; everything else is verified by
hand. Stdlib `testing` only — no assertion libraries.

CI gates every PR with `gofmt -l`, `go vet`, and `go test`. `make check` is the local equivalent,
but its `fmt` step **rewrites** files instead of failing — so a clean `make check` does not mean CI
will pass. Run `gofmt -l .` before pushing and make sure it prints nothing.

`make mutate` is advisory and local-only: act on `LIVED` mutants, ignore `TIMED OUT` — see
`docs/mutation-testing.md`.

Commands that shell out to docker, compose, ssh, or aws are still untested. Verify those by hand in
a real project, exercising **both** single-service and monorepo modes.

## Conventions

- Return errors, never panic; wrap with `fmt.Errorf("...: %w", err)`.
- Commands use `RunE`; flags registered in `init()`; logic in a separate `run<Name>` function.
- Error messages name the fix ("Available services: %v", "run 'rocketctl init' first").
- Shell scripts: `#!/bin/bash`, `set -e`, colour vars at top.

## Tracking

Never hand-write `gh issue` calls — `scripts/ticket.sh` holds the boilerplate, you pass only the
dynamic parts. Search by title, never by body.

```
./scripts/ticket.sh new bug  "title" -d "one line" -r internal/ssh/ssh.go:29
./scripts/ticket.sh new feat "title" -p 12      # -p links a follow-up to its parent
./scripts/ticket.sh find "terms"                # title-only search, one line per hit
./scripts/ticket.sh ls [bug|feat]
```

Kinds map to existing labels: `bug`→`bug`, `feat`→`enhancement`. Follow-ups carry `Parent: #N`
in the body, so GitHub cross-links them both ways. Check `find` before opening a duplicate.
