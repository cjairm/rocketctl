# RocketCTL Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `rocketctl deploy` actually deploy the version it built, close the remote shell-injection and host-key holes, give the binary a real version, and remove the dead code that makes the package APIs lie.

**Architecture:** Four independently landable phases. Phase 1 fixes the broken build→push→deploy loop by introducing one shared naming rule (`config.EnvVersionKey`) used by both the Go code and the compose template, so the version variable can never drift again. Phase 2 hardens the SSH path. Phase 3 is release tooling only. Phase 4 is cleanup. Each phase ends green (`gofmt -l .` silent, `go vet` clean, `go test ./...` passing) and is safe to ship on its own.

**Tech Stack:** Go 1.24, Cobra v1.10.2, `golang.org/x/crypto/ssh` v0.48.0 (includes `ssh/knownhosts` — no new dependency), `gopkg.in/yaml.v3`, stdlib `testing` only.

## Global Constraints

These apply to every task. Copied from the repo's CLAUDE.md and the user's stated requirement.

- **Easy to use above all.** No new _required_ configuration. Every change must keep working for a user who never edits `rocket.yaml` again. New config fields are optional and default to the safe behaviour.
- **`rocket.yaml` is the only input.** Every command calls `config.Load()` itself. No global config, no persistent root flags, no context passing.
- **Never build paths or image names by hand.** Use the `Config` methods. The `<project>_<service>:<version>` scheme is not configurable.
- **Handle both repo modes** in every change: `cfg.IsMonorepo()` (explicit service arg, `./<service>/`) and single-service (infer from `cfg.Service`, resolves to `.`).
- **Bump the version only after the operation succeeds.**
- **Use `docker compose` (v2), never `docker-compose`.**
- Return errors, never panic; wrap with `fmt.Errorf("...: %w", err)`.
- Commands use `RunE`; flags registered in `init()`; logic in a separate `run<Name>` function.
- Error messages name the fix ("Available services: %v", "run 'rocketctl init' first").
- Stdlib `testing` only — no assertion libraries. Table-driven where there is more than one case.
- **Run `gofmt -l .` before every commit and make sure it prints nothing.** `make check`'s `fmt` step rewrites files instead of failing, so a clean `make check` does not mean CI passes.
- **Do not add a command that regenerates templates over a user's edited output.**

---

## Validation Evidence

Every claim below was reproduced against a real scratch project and a real Docker daemon (Docker 27.4.0, Compose v2.31.0) before this plan was written. Implementers can re-run these to see the bug before fixing it.

| #   | Claim                                                                         | How it was proven                                                                                                                                                                                    | Result                                                                                                                                                                                                                                            |
| --- | ----------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1   | `deploy` pulls a tag that was never pushed                                    | Generated a monorepo project, copied `docker-compose.prod.yml` to a dir laid out exactly as `deploy` leaves the remote (`docker-compose.yml` + `api/.env` + `web/.env`), ran `docker compose config` | `image: …/myapp_api:latest` — but `build`/`push` only ever create `:<version>`                                                                                                                                                                    |
| 2   | A root `.env` fixes interpolation without leaking into containers             | Added `API_VERSION=0.1.4` to root `.env`, re-ran `docker compose config`                                                                                                                             | `:0.1.4`, and `DATABASE_URL` from the per-service `env_file` still present                                                                                                                                                                        |
| 3   | Inline env vars work and avoid clobbering the secrets `.env`                  | `API_VERSION=9.9.9 docker compose config` with a root `.env` holding `SECRET=shh`                                                                                                                    | `:9.9.9` — shell env wins, no file touched. **This is the chosen fix.**                                                                                                                                                                           |
| 4   | Hyphenated service names silently corrupt the compose file                    | `init` with service `my-api`, then `docker compose config`                                                                                                                                           | Emits `image: …/myapp_my-api:API_VERSION:-latest` with **exit code 0**; `docker compose pull` then fails `invalid reference format`. Bash cannot even set `MY-API_VERSION`. No hyphenated deploy has ever worked, so normalizing is non-breaking. |
| 5   | `init` accepts an empty service name                                          | `init` answering `api,,web`                                                                                                                                                                          | `rocket.yaml` gets `services: ["api", "", "web"]` and a stray `.rocket-version` is written to the **project root**                                                                                                                                |
| 6   | Remote shell injection is live                                                | Ran the unquoted `fmt.Sprintf` form from `deploy.go` through `sh -c` with project `$(id)`                                                                                                            | `id` executed                                                                                                                                                                                                                                     |
| 7   | Quoting fixes it, but naive quoting breaks `~`                                | Compared `'~/apps/x'` vs `~/apps/'x'`                                                                                                                                                                | Fully quoted → literal `~` directory. **`~/` must stay outside the quotes.**                                                                                                                                                                      |
| 8   | `prune` misses registry-tagged images                                         | Tagged 4 images (2 bare, 2 registry-prefixed), ran prune's `reference=myapp_*` filter                                                                                                                | Matched only the 2 bare ones                                                                                                                                                                                                                      |
| 9   | A second filter catches them                                                  | `reference=<registry>/myapp_*`                                                                                                                                                                       | Matched the 2 registry-prefixed ones                                                                                                                                                                                                              |
| 10  | `--version` does not exist                                                    | `rocketctl --version`                                                                                                                                                                                | `Error: unknown flag: --version`                                                                                                                                                                                                                  |
| 11  | `rootCmd.Version` does **not** collide with the existing `version` subcommand | Built a Cobra v1.10.2 prototype with both                                                                                                                                                            | `--version` → `rocketctl 1.2.3-test`; `version` → `api: 0.1.0`. Cobra also auto-assigns `-v`.                                                                                                                                                     |
| 12  | A package-level `rootCmd` captures the ldflags value                          | Prototype with `Version: binaryVersion` in the struct literal, no reassignment in `main()`                                                                                                           | `rocketctl 4.5.6-pkgvar` — works, no reassignment needed                                                                                                                                                                                          |
| 13  | Makefile version has drifted                                                  | `VERSION?=1.0.0` vs `git describe --tags`                                                                                                                                                            | Real latest tag is `v1.4.0`                                                                                                                                                                                                                       |
| 14  | `knownhosts` needs no new dependency                                          | `go doc golang.org/x/crypto/ssh/knownhosts` against the pinned v0.48.0                                                                                                                               | Present. `KeyError.Want` empty = unknown host, non-empty = mismatch                                                                                                                                                                               |
| 15  | Existing test fixtures survive new name validation                            | Read `config_test.go` fixtures                                                                                                                                                                       | All use `myapp`/`backend`/`api`/`web` — all legal                                                                                                                                                                                                 |

**Bonus found during validation:** every compose invocation prints `the attribute 'version' is obsolete`. Fixed in Task 2 under the "easy to use" constraint.

---

## File Structure

**Phase 1 — deploy correctness**

- `internal/config/config.go` — add `EnvVersionKey` (the single source of truth for the compose version variable) and name validation in `Validate()`.
- `internal/config/config_test.go` — table-driven tests for both.
- `internal/templates/templates.go` — share one `template.FuncMap` exposing `envkey`; delete the duplicated one.
- `internal/templates/docker-compose.prod.yml.tmpl` — use `envkey`, drop obsolete `version:`.
- `internal/templates/templates_test.go` — **new.** Renders the template in-process; no Docker needed.
- `internal/ssh/ssh.go` — add `ShellQuote`.
- `internal/ssh/ssh_test.go` — **new.** Tests `ShellQuote` only (no network).
- `cmd/deploy.go` — collect versions, pass them as an env prefix, quote the remote dir.
- `cmd/deploy_test.go` — **new.** Tests the pure `versionEnvAssignments` helper.
- `cmd/up.go` — use `config.EnvVersionKey` instead of `strings.ToUpper`.

**Phase 2 — SSH hardening**

- `internal/config/config.go` — optional `insecure_skip_host_key_check`.
- `internal/ssh/ssh.go` — `hostKeyCallback`, `trustOnFirstUse`, `UploadFileMode`.
- `cmd/deploy.go` — pass the flag through; upload `.env` files as 0600.

**Phase 3 — release tooling** (no product code)

- `cmd/root.go`, `Makefile`, `.github/workflows/release.yml`, `.github/workflows/ci.yml`, `.github/dependabot.yml` (new).

**Phase 4 — cleanup**

- `cmd/prune.go`, `internal/docker/docker.go`, `internal/compose/compose.go`, `internal/templates/templates.go`, `internal/config/config.go`, `cmd/init.go`, `CLAUDE.md`, `README.md`.

---

# Phase 1 — Deploy deploys what you built

Landing this phase alone makes the core loop correct.

### Task 1: One naming rule for the version variable

**Files:**

- Modify: `internal/config/config.go`
- Test: `internal/config/config_test.go`

**Interfaces:**

- Consumes: nothing.
- Produces:
  - `func EnvVersionKey(service string) string` — package-level function in `config`. `"my-api"` → `"MY_API_VERSION"`. Used by Task 2 (template), Task 3 (deploy) and Task 4 (up).
  - `Config.Validate()` gains name checks; signature unchanged: `func (c *Config) Validate() error`.

- [ ] **Step 1: Write the failing tests**

Append to `internal/config/config_test.go`:

```go
func TestEnvVersionKey(t *testing.T) {
	tests := []struct {
		service string
		want    string
	}{
		{"api", "API_VERSION"},
		{"my-api", "MY_API_VERSION"},
		{"web.ui", "WEB_UI_VERSION"},
		{"Api2", "API2_VERSION"},
	}
	for _, tt := range tests {
		t.Run(tt.service, func(t *testing.T) {
			if got := EnvVersionKey(tt.service); got != tt.want {
				t.Errorf("EnvVersionKey(%q) = %q, want %q", tt.service, got, tt.want)
			}
		})
	}
}

func TestValidateRejectsUnsafeNames(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
	}{
		{
			name:    "shell metacharacters in project",
			cfg:     &Config{Project: "a;touch /tmp/pwned", Service: "backend", Registry: "reg.example.com", Region: "us-east-2"},
			wantErr: true,
		},
		{
			name:    "command substitution in project",
			cfg:     &Config{Project: "$(id)", Service: "backend", Registry: "reg.example.com", Region: "us-east-2"},
			wantErr: true,
		},
		{
			name:    "empty service in monorepo list",
			cfg:     &Config{Project: "myapp", Services: []string{"api", "", "web"}, Registry: "reg.example.com", Region: "us-east-2"},
			wantErr: true,
		},
		{
			name:    "path traversal in service name",
			cfg:     &Config{Project: "myapp", Services: []string{"api/../etc"}, Registry: "reg.example.com", Region: "us-east-2"},
			wantErr: true,
		},
		{
			name:    "hyphens and dots stay legal",
			cfg:     &Config{Project: "my.app", Services: []string{"my-api", "web"}, Registry: "reg.example.com", Region: "us-east-2"},
			wantErr: false,
		},
		{
			name:    "plain single service stays legal",
			cfg:     &Config{Project: "myapp", Service: "backend", Registry: "reg.example.com", Region: "us-east-2"},
			wantErr: false,
		},
		{
			name:    "injection via registry",
			cfg:     &Config{Project: "myapp", Service: "backend", Registry: "reg.example.com;id", Region: "us-east-2"},
			wantErr: true,
		},
		{
			name:    "injection via region",
			cfg:     &Config{Project: "myapp", Service: "backend", Registry: "reg.example.com", Region: "us-east-2 $(id)"},
			wantErr: true,
		},
		{
			name:    "real ECR registry stays legal",
			cfg:     &Config{Project: "myapp", Service: "backend", Registry: "123456789.dkr.ecr.us-east-2.amazonaws.com", Region: "us-east-2"},
			wantErr: false,
		},
		{
			name:    "registry with an explicit port stays legal",
			cfg:     &Config{Project: "myapp", Service: "backend", Registry: "reg.example.com:5000", Region: "us-east-2"},
			wantErr: false,
		},
		{
			name:    "services colliding on one compose variable",
			cfg:     &Config{Project: "myapp", Services: []string{"my-api", "my.api"}, Registry: "reg.example.com", Region: "us-east-2"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr && err == nil {
				t.Error("expected an error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("expected no error, got %v", err)
			}
		})
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/config/ -run 'TestEnvVersionKey|TestValidateRejectsUnsafeNames' -v`

Expected: FAIL — `undefined: EnvVersionKey`, and the unsafe-name cases report "expected an error, got nil".

- [ ] **Step 3: Implement**

In `internal/config/config.go`, add `regexp` and `strings` to the import block, then add below `Validate`:

```go
// namePattern matches names that are safe everywhere RocketCTL puts them: a
// Docker image component, a remote shell path segment and a compose variable.
var namePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// hostPattern matches a registry host, optionally with a port. Registry and
// region are interpolated unquoted into the remote ECR login command, so they
// need the same treatment as project and service names.
var hostPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9.-]*(:[0-9]+)?$`)

// validateName rejects names that would be unsafe once interpolated into a
// remote shell command or an image reference.
func validateName(kind, name string) error {
	if name == "" {
		return fmt.Errorf("%s name cannot be empty in rocket.yaml", kind)
	}
	if !namePattern.MatchString(name) {
		return fmt.Errorf(
			"invalid %s name %q in rocket.yaml: use only letters, digits, '.', '_' and '-', starting with a letter or digit",
			kind, name,
		)
	}
	return nil
}

// validateHost rejects a registry or region that would be unsafe once
// interpolated into the remote "aws ecr get-login-password ... | docker login"
// command.
func validateHost(kind, value string) error {
	if !hostPattern.MatchString(value) {
		return fmt.Errorf(
			"invalid %s %q in rocket.yaml: use only letters, digits, '.', '-' and an optional ':port'",
			kind, value,
		)
	}
	return nil
}

// EnvVersionKey returns the compose interpolation variable carrying a service's
// version, e.g. "my-api" -> "MY_API_VERSION". Anything that is not a letter or
// digit becomes an underscore because compose variables must be shell
// identifiers - "MY-API_VERSION" silently renders an invalid image reference.
func EnvVersionKey(service string) string {
	safe := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return r
		default:
			return '_'
		}
	}, service)
	return strings.ToUpper(safe) + "_VERSION"
}
```

Then wire the checks into `Validate`, immediately before its final `return nil`:

```go
	if err := validateName("project", c.Project); err != nil {
		return err
	}
	if err := validateHost("registry", c.Registry); err != nil {
		return err
	}
	if err := validateName("region", c.Region); err != nil {
		return err
	}

	// Two services whose names differ only by '-', '.' or '_' would share one
	// compose variable, silently pinning both to whichever version is written
	// last. Reject that here rather than deploying the wrong image.
	seen := make(map[string]string, len(c.Services))
	for _, service := range c.GetServices() {
		if err := validateName("service", service); err != nil {
			return err
		}
		key := EnvVersionKey(service)
		if other, ok := seen[key]; ok {
			return fmt.Errorf(
				"services %q and %q both map to the compose variable %s in rocket.yaml: rename one so their versions can be pinned independently",
				other, service, key,
			)
		}
		seen[key] = service
	}

	return nil
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/config/ -v`

Expected: PASS, including the pre-existing `Validate` tests (all their fixtures use legal names).

- [ ] **Step 5: Verify formatting and commit**

```bash
gofmt -l .   # must print nothing
go vet ./...
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(config): validate project and service names, add EnvVersionKey

Names now have to be safe as image components, remote shell path segments
and compose variable names. EnvVersionKey normalises a service name into a
shell identifier so hyphenated services stop rendering invalid image refs.

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>"
```

---

### Task 2: Template emits a shell-safe version variable

**Files:**

- Modify: `internal/templates/templates.go`
- Modify: `internal/templates/docker-compose.prod.yml.tmpl`
- Test: `internal/templates/templates_test.go` (create)

**Interfaces:**

- Consumes: `config.EnvVersionKey` from Task 1.
- Produces: `RenderDockerComposeProd(TemplateData) (string, error)` — already exists but is currently unused; this task makes it the test seam, so **do not delete it** in Phase 4.

- [ ] **Step 1: Write the failing test**

Create `internal/templates/templates_test.go`:

```go
package templates

import (
	"strings"
	"testing"
)

func TestRenderDockerComposeProdUsesShellSafeVersionVars(t *testing.T) {
	out, err := RenderDockerComposeProd(TemplateData{
		Project:    "myapp",
		Services:   []string{"my-api"},
		Registry:   "reg.example.com",
		IsMonorepo: true,
	})
	if err != nil {
		t.Fatalf("render failed: %v", err)
	}

	want := "image: reg.example.com/myapp_my-api:${MY_API_VERSION:-latest}"
	if !strings.Contains(out, want) {
		t.Errorf("rendered compose is missing %q\ngot:\n%s", want, out)
	}
	if strings.Contains(out, "MY-API_VERSION") {
		t.Error("rendered compose still contains the shell-invalid MY-API_VERSION")
	}
}

func TestRenderDockerComposeProdOmitsObsoleteVersionAttribute(t *testing.T) {
	out, err := RenderDockerComposeProd(TemplateData{
		Project:  "myapp",
		Services: []string{"api"},
		Registry: "reg.example.com",
	})
	if err != nil {
		t.Fatalf("render failed: %v", err)
	}
	if strings.HasPrefix(strings.TrimSpace(out), "version:") {
		t.Error("rendered compose still declares the obsolete top-level version attribute")
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/templates/ -v`

Expected: FAIL — the first test reports the `MY-API_VERSION` string, the second reports the obsolete attribute.

- [ ] **Step 3: Update the template**

In `internal/templates/docker-compose.prod.yml.tmpl`, delete the first line (`version: '3.8'`) **and the blank line after it**, so the file now starts with `networks:`.

Then change the image line from:

```
    image: {{$.Registry}}/{{$.Project}}_{{.}}:${{"{"}}{{. | upper}}_VERSION:-latest{{"}"}}
```

to:

```
    image: {{$.Registry}}/{{$.Project}}_{{.}}:${{"{"}}{{. | envkey}}:-latest{{"}"}}
```

- [ ] **Step 4: Share one FuncMap in the Go code**

In `internal/templates/templates.go`, add `"github.com/cjairm/rocketctl/internal/config"` to the imports and remove `"strings"` if nothing else uses it. Add near the top, after the `//go:embed` block:

```go
// composeFuncs is shared by both compose renderers so the version variable
// spelled in the template can never drift from the one the Go code sets.
var composeFuncs = template.FuncMap{
	"envkey": config.EnvVersionKey,
}
```

In `GenerateDockerComposeProd`, replace the local `funcMap` declaration and its use:

```go
	tmpl, err := template.New("docker-compose.prod.yml").
		Funcs(composeFuncs).
		Parse(dockerComposeProdTemplate)
```

In `RenderDockerComposeProd`, make the identical replacement:

```go
	tmpl, err := template.New("docker-compose.prod.yml").
		Funcs(composeFuncs).
		Parse(dockerComposeProdTemplate)
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/templates/ -v`

Expected: PASS for both tests.

- [ ] **Step 6: Confirm against a real Docker daemon**

Build the binary, generate a project with a hyphenated service, and render it:

```bash
go build -o /tmp/rocketctl-t . && D=$(mktemp -d) && cd "$D" && \
  printf 'myapp\nreg.example.com\nus-east-2\n\n\ny\nmy-api\n' | /tmp/rocketctl-t init >/dev/null && \
  mkdir -p my-api && printf 'X=1\n' > my-api/.env && \
  cp docker-compose.prod.yml docker-compose.yml && \
  MY_API_VERSION=1.2.3 docker compose config 2>&1 | grep -E "image:|obsolete"
```

Expected: `image: reg.example.com/myapp_my-api:1.2.3` and **no** `obsolete` warning line.

- [ ] **Step 7: Verify formatting and commit**

```bash
gofmt -l .   # must print nothing
go vet ./...
git add internal/templates/
git commit -m "fix(templates): emit shell-safe version vars, drop obsolete version key

Hyphenated services rendered \${MY-API_VERSION}, which compose resolved to
the invalid tag 'API_VERSION:-latest' with exit code 0. Both renderers now
share one FuncMap backed by config.EnvVersionKey.

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>"
```

---

### Task 3: `deploy` pins the versions it built

This is the fix for the headline bug. Evidence rows 1–3 and 7.

**Files:**

- Modify: `internal/ssh/ssh.go`
- Test: `internal/ssh/ssh_test.go` (create)
- Modify: `cmd/deploy.go`
- Test: `cmd/deploy_test.go` (create)

**Interfaces:**

- Consumes: `config.EnvVersionKey` (Task 1).
- Produces:
  - `func ShellQuote(s string) string` in package `ssh` — used again by Task 5.
  - `func versionEnvAssignments(serviceVersions map[string]string) string` in package `cmd` (unexported; pure, sorted, testable).

- [ ] **Step 1: Write the failing `ShellQuote` test**

Create `internal/ssh/ssh_test.go`:

```go
package ssh

import (
	"os/exec"
	"strings"
	"testing"
)

func TestShellQuoteNeutralisesInjection(t *testing.T) {
	payloads := []string{
		"myapp",
		"my app",
		"a;touch /tmp/rocketctl-pwned",
		"$(id)",
		"a`id`",
		"it's",
	}
	for _, payload := range payloads {
		t.Run(payload, func(t *testing.T) {
			// Echo the quoted value through a real shell; it must come back
			// byte-for-byte with nothing executed or word-split.
			out, err := exec.Command("sh", "-c", "printf %s "+ShellQuote(payload)).Output()
			if err != nil {
				t.Fatalf("shell rejected the quoted value: %v", err)
			}
			if string(out) != payload {
				t.Errorf("round trip changed the value: got %q, want %q", string(out), payload)
			}
		})
	}
}

func TestShellQuoteKeepsTildeExpandable(t *testing.T) {
	// The '~' must stay OUTSIDE the quotes or the remote shell creates a
	// literal '~' directory instead of expanding to $HOME.
	remoteDir := "~/apps/" + ShellQuote("myapp")
	out, err := exec.Command("sh", "-c", "printf %s "+remoteDir).Output()
	if err != nil {
		t.Fatalf("shell rejected the path: %v", err)
	}
	if strings.HasPrefix(string(out), "~") {
		t.Errorf("tilde was not expanded: %q", string(out))
	}
	if !strings.HasSuffix(string(out), "/apps/myapp") {
		t.Errorf("unexpected expansion: %q", string(out))
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/ssh/ -v`

Expected: FAIL — `undefined: ShellQuote`.

- [ ] **Step 3: Implement `ShellQuote`**

In `internal/ssh/ssh.go`, add at the end of the file:

```go
// ShellQuote wraps s in single quotes so a remote shell takes it as one
// literal argument. Embedded single quotes are closed, escaped and reopened.
//
// Callers must keep a leading "~" outside the quotes - a quoted tilde is not
// expanded, and the remote shell would create a literal "~" directory.
func ShellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
```

`strings` is already imported.

- [ ] **Step 4: Run it to verify it passes**

Run: `go test ./internal/ssh/ -v`

Expected: PASS for both tests.

- [ ] **Step 5: Write the failing `versionEnvAssignments` test**

Create `cmd/deploy_test.go`:

```go
package cmd

import "testing"

func TestVersionEnvAssignments(t *testing.T) {
	tests := []struct {
		name     string
		versions map[string]string
		want     string
	}{
		{
			name:     "single service",
			versions: map[string]string{"backend": "0.1.4"},
			want:     "BACKEND_VERSION='0.1.4' ",
		},
		{
			name:     "monorepo is sorted for a stable command",
			versions: map[string]string{"web": "2.0.1", "api": "0.1.4"},
			want:     "API_VERSION='0.1.4' WEB_VERSION='2.0.1' ",
		},
		{
			name:     "hyphenated service becomes a shell identifier",
			versions: map[string]string{"my-api": "1.0.0"},
			want:     "MY_API_VERSION='1.0.0' ",
		},
		{
			name:     "no services yields an empty prefix",
			versions: map[string]string{},
			want:     "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := versionEnvAssignments(tt.versions); got != tt.want {
				t.Errorf("versionEnvAssignments() = %q, want %q", got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 6: Run it to verify it fails**

Run: `go test ./cmd/ -run TestVersionEnvAssignments -v`

Expected: FAIL — `undefined: versionEnvAssignments`.

- [ ] **Step 7: Implement the helpers in `cmd/deploy.go`**

Add `"sort"` and `"strings"` to the import block, plus `"github.com/cjairm/rocketctl/internal/version"`. Add these two functions above `runDeploy`:

```go
// versionEnvAssignments renders the environment prefix that pins each service's
// image tag for the remote compose commands, e.g.
// "API_VERSION='0.1.4' WEB_VERSION='2.0.1' ".
//
// Without it the generated compose file falls back to ":latest", a tag that
// build and push never create. Services are sorted so the command is stable
// and readable in the deploy output.
func versionEnvAssignments(serviceVersions map[string]string) string {
	services := make([]string, 0, len(serviceVersions))
	for service := range serviceVersions {
		services = append(services, service)
	}
	sort.Strings(services)

	var b strings.Builder
	for _, service := range services {
		fmt.Fprintf(&b, "%s=%s ", config.EnvVersionKey(service), ssh.ShellQuote(serviceVersions[service]))
	}
	return b.String()
}

// collectServiceVersions reads the local .rocket-version for every service so
// the remote pulls exactly what the last build produced.
func collectServiceVersions(cfg *config.Config) (map[string]string, error) {
	versions := make(map[string]string, len(cfg.GetServices()))
	for _, service := range cfg.GetServices() {
		versionPath, err := cfg.GetVersionFilePath(service)
		if err != nil {
			return nil, err
		}
		ver, err := version.Get(versionPath)
		if err != nil {
			return nil, err
		}
		versions[service] = ver
	}
	return versions, nil
}
```

- [ ] **Step 8: Run the test to verify it passes**

Run: `go test ./cmd/ -run TestVersionEnvAssignments -v`

Expected: PASS for all four cases.

- [ ] **Step 9: Use the prefix in `runDeploy` and quote the remote dir**

In `cmd/deploy.go`, change the `remoteDir` assignment (currently line 66) from:

```go
	remoteDir := fmt.Sprintf("~/apps/%s", cfg.Project)
```

to:

```go
	// The "~" stays outside the quotes so the remote shell still expands it.
	remoteDir := "~/apps/" + ssh.ShellQuote(cfg.Project)
	// Unquoted twin, only for the copy-pasteable hints printed at the end.
	// Safe to show because Validate restricts Project to [A-Za-z0-9._-].
	remoteDirDisplay := fmt.Sprintf("~/apps/%s", cfg.Project)
```

Collect the versions **before** connecting, so a missing or malformed
`.rocket-version` fails locally and leaves the server untouched. Add this
directly after the `cfg.IP == ""` check, above the `sshUser` block:

```go
	serviceVersions, err := collectServiceVersions(cfg)
	if err != nil {
		return err
	}
	versionEnv := versionEnvAssignments(serviceVersions)
	fmt.Println("📌 Deploying versions:")
	for _, service := range cfg.GetServices() {
		fmt.Printf("   %s %s\n", service, serviceVersions[service])
	}
```

Then change the pull and up invocations (currently lines 199 and 206) to carry the prefix:

```go
	if err := client.ExecInteractive(fmt.Sprintf("cd %s && %s%s", remoteDir, versionEnv, pullCmd)); err != nil {
		return fmt.Errorf("failed to pull images: %w", err)
	}
```

```go
	if err := client.ExecInteractive(fmt.Sprintf("cd %s && %s%s", remoteDir, versionEnv, upCmd)); err != nil {
		return fmt.Errorf("failed to start services: %w", err)
	}
```

The ECR login invocation keeps its own form — it interpolates no image tags —
but it does interpolate `cfg.Region` and `cfg.Registry` into a remote shell
string. Task 1's `validateHost`/`validateName` checks are what make that safe;
do not skip them.

Finally, swap the two closing hints (currently lines 211–222) to the display
twin so the printed command does not look mangled:

```go
	fmt.Printf(
		"\n📊 To view logs: ssh %s@%s 'cd %s && docker compose logs -f'\n",
		sshUser,
		cfg.IP,
		remoteDirDisplay,
	)
	fmt.Printf(
		"📊 To view status: ssh %s@%s 'cd %s && docker compose ps'\n",
		sshUser,
		cfg.IP,
		remoteDirDisplay,
	)
```

- [ ] **Step 10: Make `up --prod` use the same rule**

In `cmd/up.go`, replace lines 121–122:

```go
		envKey := fmt.Sprintf("%s_VERSION", strings.ToUpper(service))
		os.Setenv(envKey, currentVersion)
```

with:

```go
		// Explicitly discarded: Setenv only fails on an invalid key, and
		// EnvVersionKey always produces a valid one. Also keeps errcheck quiet
		// for the lint gate Task 7 adds.
		_ = os.Setenv(config.EnvVersionKey(service), currentVersion)
```

Remove `"strings"` from `cmd/up.go`'s imports if nothing else there uses it.

- [ ] **Step 11: Verify the whole loop against a real daemon**

```bash
go build -o /tmp/rocketctl-t . && D=$(mktemp -d) && cd "$D" && \
  printf 'myapp\nreg.example.com\nus-east-2\n\n\ny\napi, web\n' | /tmp/rocketctl-t init >/dev/null && \
  mkdir -p api web && printf 'X=1\n' > api/.env && printf 'X=1\n' > web/.env && \
  cp docker-compose.prod.yml docker-compose.yml && \
  API_VERSION='0.1.4' WEB_VERSION='2.0.1' docker compose config 2>&1 | grep image:
```

Expected — the exact tags, not `latest`:

```
    image: reg.example.com/myapp_api:0.1.4
    image: reg.example.com/myapp_web:2.0.1
```

- [ ] **Step 12: Run everything, verify formatting, commit**

```bash
gofmt -l .   # must print nothing
go vet ./...
go test ./...
git add internal/ssh/ cmd/deploy.go cmd/deploy_test.go cmd/up.go
git commit -m "fix(deploy): pin the built versions instead of falling back to latest

deploy uploaded a compose file pinned to \${SERVICE_VERSION:-latest} but
never set those variables, so the remote resolved ':latest' - a tag build
and push never create. It now reads each .rocket-version and passes the
versions as an env prefix on the remote pull/up. The remote project dir is
shell-quoted, with '~' left outside the quotes so it still expands.

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>"
```

**Phase 1 is now shippable.** `deploy` deploys what you built, and hyphenated services work.

---

# Phase 2 — SSH hardening

### Task 4: Verify host keys, with trust-on-first-use

**Files:**

- Modify: `internal/config/config.go`
- Modify: `internal/ssh/ssh.go`
- Modify: `cmd/deploy.go`

**Interfaces:**

- Consumes: nothing from earlier tasks.
- Produces: `Connect` changes signature to `func Connect(host, user, keyPath string, insecureHostKey bool) (*Client, error)`. `cmd/deploy.go` is the only caller.

There is no automated test here — it needs a real SSH server, which CLAUDE.md already treats as manual verification. Step 5 is the manual check.

- [ ] **Step 1: Add the optional escape hatch to the config**

In `internal/config/config.go`, add a field to `Config`:

```go
	InsecureSkipHostKeyCheck bool `yaml:"insecure_skip_host_key_check,omitempty"` // Opt out of known_hosts verification
```

It defaults to `false`, so existing `rocket.yaml` files get the secure behaviour with no edit — this satisfies the "easy to use" constraint.

- [ ] **Step 2: Implement the callback**

In `internal/ssh/ssh.go`, extend the imports with `"bufio"`, `"errors"`, `"net"` and `"golang.org/x/crypto/ssh/knownhosts"`, then add:

```go
// hostKeyCallback verifies the server against ~/.ssh/known_hosts. An unknown
// host is recorded after the user confirms (trust on first use); a key that
// changed is always a hard failure, because that is what a MITM looks like.
func hostKeyCallback(insecure bool) (ssh.HostKeyCallback, error) {
	if insecure {
		fmt.Println("⚠️  Host key verification disabled (insecure_skip_host_key_check: true)")
		return ssh.InsecureIgnoreHostKey(), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to locate home directory for known_hosts: %w", err)
	}
	khPath := filepath.Join(home, ".ssh", "known_hosts")
	if err := os.MkdirAll(filepath.Dir(khPath), 0700); err != nil {
		return nil, fmt.Errorf("failed to create %s: %w", filepath.Dir(khPath), err)
	}
	// knownhosts.New fails on a missing file, so make sure one exists.
	f, err := os.OpenFile(khPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open %s: %w", khPath, err)
	}
	f.Close()

	verify, err := knownhosts.New(khPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", khPath, err)
	}

	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		if err := verify(hostname, remote, key); err != nil {
			var keyErr *knownhosts.KeyError
			// Want is empty when the host is simply unknown; non-empty means
			// the recorded key did not match.
			if errors.As(err, &keyErr) && len(keyErr.Want) == 0 {
				return trustOnFirstUse(khPath, hostname, key)
			}
			return fmt.Errorf(
				"host key verification failed for %s: %w\n"+
					"If the server was rebuilt on purpose, remove its line from %s and deploy again",
				hostname, err, khPath,
			)
		}
		return nil
	}, nil
}

// trustOnFirstUse shows the fingerprint and records the key once the user
// types "yes", mirroring what OpenSSH does on a first connection.
func trustOnFirstUse(khPath, hostname string, key ssh.PublicKey) error {
	fmt.Printf("\nThe authenticity of host %s can't be established.\n", hostname)
	fmt.Printf("%s key fingerprint is %s\n", key.Type(), ssh.FingerprintSHA256(key))
	fmt.Print("Are you sure you want to continue connecting? (yes/no): ")

	answer, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read confirmation: %w", err)
	}
	if strings.TrimSpace(strings.ToLower(answer)) != "yes" {
		return fmt.Errorf("host key not accepted, aborting deploy")
	}

	f, err := os.OpenFile(khPath, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("failed to open %s for append: %w", khPath, err)
	}
	defer f.Close()

	line := knownhosts.Line([]string{knownhosts.Normalize(hostname)}, key)
	if _, err := f.WriteString(line + "\n"); err != nil {
		return fmt.Errorf("failed to record host key in %s: %w", khPath, err)
	}
	fmt.Printf("✓ Added %s to %s\n", hostname, khPath)
	return nil
}
```

- [ ] **Step 3: Use it in `Connect`**

Replace the `Connect` signature and its `config` literal:

```go
// Connect establishes an SSH connection to the given host
// If keyPath is empty, it will look for default keys in ~/.ssh/
func Connect(host, user, keyPath string, insecureHostKey bool) (*Client, error) {
	authMethods, err := getAuthMethods(keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get SSH auth methods: %w", err)
	}
	hostKeys, err := hostKeyCallback(insecureHostKey)
	if err != nil {
		return nil, err
	}
	config := &ssh.ClientConfig{
		User:            user,
		Auth:            authMethods,
		HostKeyCallback: hostKeys,
	}
```

The rest of `Connect` is unchanged. The `// TODO: Consider using known_hosts` comment goes away with the old line.

- [ ] **Step 4: Update the caller**

In `cmd/deploy.go`, change the `ssh.Connect` call (currently line 53):

```go
	client, err := ssh.Connect(cfg.IP, sshUser, cfg.SSHKeyPath, cfg.InsecureSkipHostKeyCheck)
```

- [ ] **Step 5: Build, test, and verify by hand**

```bash
gofmt -l .   # must print nothing
go vet ./...
go test ./...
go build -o /tmp/rocketctl-t .
```

Manual check against any host you can SSH to (CLAUDE.md requires hand-verification for SSH paths). In a scratch project whose `rocket.yaml` points `ip:` at that host:

1. Remove its line from `~/.ssh/known_hosts`, run `rocketctl deploy`, and confirm the fingerprint prompt appears and that answering `no` aborts.
2. Run again, answer `yes`, and confirm the host is appended to `~/.ssh/known_hosts`.
3. Run a third time and confirm there is no prompt.
4. Corrupt that known_hosts line, run again, and confirm it fails with the "remove its line" message rather than connecting.

- [ ] **Step 6: Commit**

```bash
git add internal/ssh/ssh.go internal/config/config.go cmd/deploy.go
git commit -m "feat(ssh): verify host keys against known_hosts

Replaces InsecureIgnoreHostKey with known_hosts verification plus an
OpenSSH-style trust-on-first-use prompt. A changed key is a hard failure.
insecure_skip_host_key_check: true in rocket.yaml restores the old
behaviour for hosts with rotating addresses.

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>"
```

---

### Task 5: Upload `.env` files as 0600

`UploadFile` copies the _local_ file's mode to the remote, so a 0644 `.env.example` becomes a world-readable `.env` full of production secrets on a shared host.

**Files:**

- Modify: `internal/ssh/ssh.go`
- Modify: `cmd/deploy.go`

**Interfaces:**

- Consumes: the already-quoted `remotePath` that Task 3's `remoteDir` change
  produces. This task calls `ShellQuote` nowhere itself — do not look for it.
- Produces: `func (c *Client) UploadFileMode(localPath, remotePath string, mode os.FileMode) error`. `UploadFile` keeps its signature and delegates.

- [ ] **Step 1: Split `UploadFile`**

In `internal/ssh/ssh.go`, replace the whole `UploadFile` function with:

```go
// UploadFile uploads a local file to a remote path, preserving its mode.
func (c *Client) UploadFile(localPath, remotePath string) error {
	info, err := os.Stat(localPath)
	if err != nil {
		return fmt.Errorf("failed to stat local file %s: %w", localPath, err)
	}
	return c.UploadFileMode(localPath, remotePath, info.Mode().Perm())
}

// UploadFileMode uploads a local file and forces the remote permissions, which
// matters for secrets: a 0644 .env.example must not become a 0644 .env.
func (c *Client) UploadFileMode(localPath, remotePath string, mode os.FileMode) error {
	local, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to read local file %s: %w", localPath, err)
	}
	// Wrapped rather than a bare `defer local.Close()` so errcheck stays quiet
	// for the lint gate Task 7 adds.
	defer func() { _ = local.Close() }()

	// Create remote directory if needed
	remoteDir := filepath.Dir(remotePath)
	if err := c.MkdirAll(remoteDir); err != nil {
		return err
	}

	session, err := c.client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	defer func() { _ = session.Close() }()

	stdin, err := session.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	// Create the file with the right mode before any bytes land in it, so the
	// contents are never briefly readable by other users.
	cmd := fmt.Sprintf(
		"umask 077 && cat > %s && chmod %o %s",
		remotePath, mode, remotePath,
	)
	if err := session.Start(cmd); err != nil {
		return fmt.Errorf("failed to start upload command: %w", err)
	}
	if _, err := io.Copy(stdin, local); err != nil {
		return fmt.Errorf("failed to write file content: %w", err)
	}
	// Explicit discard, not a bare stdin.Close(): the remote command's real
	// outcome comes from session.Wait() below, and errcheck flags the bare form.
	_ = stdin.Close()

	if err := session.Wait(); err != nil {
		return fmt.Errorf("upload command failed: %w", err)
	}
	return nil
}
```

This also drops the old `strings.NewReader(string(data))` round trip, which read the whole file into memory and copied it twice.

- [ ] **Step 2: Upload `.env` with 0600 in deploy**

In `cmd/deploy.go`, inside the `uploadEnvFile` closure, change the upload call from:

```go
				if err := client.UploadFile(localEnvExamplePath, remoteDotEnvPath); err != nil {
```

to:

```go
				// 0600: this file holds production secrets.
				if err := client.UploadFileMode(localEnvExamplePath, remoteDotEnvPath, 0600); err != nil {
```

- [ ] **Step 3: Build and verify**

```bash
gofmt -l .   # must print nothing
go vet ./...
go test ./...
```

Manual check (needs the SSH host from Task 4): run `rocketctl deploy` against a server with no `.env` yet, then `ssh <host> 'ls -l ~/apps/<project>/.env'` and confirm the mode is `-rw-------`.

- [ ] **Step 4: Commit**

```bash
git add internal/ssh/ssh.go cmd/deploy.go
git commit -m "fix(ssh): upload .env with 0600 and stream instead of buffering

UploadFile copied the local mode to the remote, so a 0644 .env.example
became a world-readable .env of production secrets. Adds UploadFileMode,
sets umask before the redirect, and streams the file rather than loading
it into memory twice.

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>"
```

**Phase 2 is now shippable.**

---

# Phase 3 — Release tooling

No product behaviour changes here.

### Task 6: `rocketctl --version`

Evidence rows 10–12: `--version` does not exist today, `rootCmd.Version` does not collide with the existing `version` subcommand, and a package-level var picks up the linker value.

**Files:**

- Modify: `cmd/root.go`
- Modify: `Makefile`
- Modify: `.github/workflows/release.yml`

**Interfaces:**

- Produces: `cmd.binaryVersion`, the ldflags target `-X github.com/cjairm/rocketctl/cmd.binaryVersion=<v>`. Task 7 does not depend on it.

- [ ] **Step 1: Add the version to the root command**

In `cmd/root.go`, replace the `rootCmd` declaration block with:

```go
// binaryVersion is injected at build time with
// -ldflags "-X github.com/cjairm/rocketctl/cmd.binaryVersion=<version>".
// It stays "dev" for local `go build` and `go run`.
var binaryVersion = "dev"

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "rocketctl",
	Short: "Convention-based Docker orchestration CLI",
	Long: `RocketCTL is a convention-based CLI tool that orchestrates Docker image
building, versioning, pushing, and deployment for any project.

It works by reading a minimal rocket.yaml config file and following folder
structure conventions. Supports both monorepo and single-service repositories.`,
	// Version adds --version. It does not shadow the `version` subcommand,
	// which reports per-service versions from .rocket-version.
	Version: binaryVersion,
}
```

Then in `init()`, replace the `// No persistent flags needed for MVP` comment with:

```go
func init() {
	rootCmd.SetVersionTemplate("rocketctl {{.Version}}\n")
}
```

- [ ] **Step 2: Verify it works and does not shadow the subcommand**

```bash
go build -ldflags="-X github.com/cjairm/rocketctl/cmd.binaryVersion=9.9.9-test" -o /tmp/rocketctl-t .
/tmp/rocketctl-t --version
```

Expected: `rocketctl 9.9.9-test`

```bash
D=$(mktemp -d) && cd "$D" && \
  printf 'myapp\nreg.example.com\nus-east-2\n\n\nn\nbackend\n' | /tmp/rocketctl-t init >/dev/null && \
  /tmp/rocketctl-t version
```

Expected: `backend: 0.1.0` — the subcommand still reports service versions.

```bash
go build -o /tmp/rocketctl-dev . && /tmp/rocketctl-dev --version
```

Expected: `rocketctl dev`

- [ ] **Step 3: Derive the Makefile version from git**

In the `Makefile`, replace:

```make
# Version - can be overridden: make release VERSION=1.0.1
VERSION?=1.0.0
```

with:

```make
# Version - derived from the git tag so it cannot drift from what ships.
# Override for a one-off build: make release VERSION=1.0.1
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

# Injected into the binary so `rocketctl --version` is meaningful.
LDFLAGS=-s -w -X github.com/cjairm/rocketctl/cmd.binaryVersion=$(VERSION)
```

Then update the three build lines to use it. In `build`:

```make
	@go build -ldflags="$(LDFLAGS)" -o $(BINARY_NAME) .
```

In `release`, both cross-builds:

```make
	@GOOS=darwin GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 .
```

```make
	@GOOS=darwin GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 .
```

- [ ] **Step 4: Verify the Makefile**

```bash
make build && ./rocketctl --version
```

Expected: the current `git describe` value, e.g. `rocketctl v1.4.0-8-g988cdaa`. Then `rm -f rocketctl`.

- [ ] **Step 5: Inject the tag in the release workflow**

In `.github/workflows/release.yml`, replace the `Build binaries` step's `run` block with:

```yaml
- name: Build binaries
  run: |
    LDFLAGS="-s -w -X github.com/cjairm/rocketctl/cmd.binaryVersion=v${{ steps.get_version.outputs.VERSION }}"

    # Build for Intel Macs (amd64)
    GOOS=darwin GOARCH=amd64 go build -ldflags="$LDFLAGS" -o rocketctl-darwin-amd64 .

    # Build for Apple Silicon (arm64)
    GOOS=darwin GOARCH=arm64 go build -ldflags="$LDFLAGS" -o rocketctl-darwin-arm64 .

- name: Verify the built binary reports the tag
  run: |
    ACTUAL=$(./rocketctl-darwin-arm64 --version || ./rocketctl-darwin-amd64 --version)
    echo "Built: $ACTUAL"
    case "$ACTUAL" in
      *"v${{ steps.get_version.outputs.VERSION }}"*) echo "✓ version matches tag" ;;
      *) echo "✗ version does not match tag"; exit 1 ;;
    esac
```

The runner is `macos-latest`, so the arm64 binary runs natively.

- [ ] **Step 6: Verify formatting and commit**

```bash
gofmt -l .   # must print nothing
go vet ./...
go test ./...
git add cmd/root.go Makefile .github/workflows/release.yml
git commit -m "feat: report the binary version via --version

The version subcommand reports per-service versions, so there was no way to
tell which rocketctl you were running. Adds an ldflags-injected version to
the root command, derives the Makefile version from git describe (it had
drifted to 1.0.0 against a v1.4.0 tag), and has the release workflow assert
the built binary matches its tag.

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>"
```

---

### Task 7: Widen CI to match the release target

CI runs Linux only, yet macOS is the only release target.

**The tree does not pass `golangci-lint` today**, so the lint findings are fixed
_first_, in the same task, before the gate that would go red on them is added.
Verified on 2026-09-23 with golangci-lint v2.12.2 and a cleared cache:
**11 errcheck findings.** Note that a bare `golangci-lint run ./...` _displays_
only 9 — it caps issues sharing the same message text at 3 — so pass
`--max-same-issues=0 --max-issues-per-linter=0` to see them all. The gate itself
is unaffected: CI passes only at zero issues either way.

The full set, by file:

| File                              | Lines           | Call                                     |
| --------------------------------- | --------------- | ---------------------------------------- |
| `internal/ssh/ssh.go`             | 52, 66, 97, 134 | `defer session.Close()`                  |
| `internal/ssh/ssh.go`             | 111             | `stdin.Close()` — **bare, not deferred** |
| `internal/templates/templates.go` | 49, 74, 95      | `defer file.Close()`                     |
| `internal/docker/docker.go`       | 155             | `defer file.Close()`                     |
| `cmd/deploy.go`                   | 57              | `defer client.Close()`                   |
| `cmd/up.go`                       | 122             | `os.Setenv(...)`                         |

Earlier phases already resolve three of these by rewriting the code: `cmd/up.go:122`
in Task 3 Step 10, and `ssh.go`'s upload session plus its `stdin.Close()` in Task 5.
Task 9 deletes two more later, with `LoadEnvFile` and `GenerateEnvProductionExample` —
but Task 9 runs _after_ this one, so expect those two to still be present at Step 1.

**Do not match the linter's output against this table.** It is a snapshot taken
before any phase landed; what remains depends on which phases are already in.
Run the linter and fix exactly what it reports.

**Files:**

- Modify: `internal/ssh/ssh.go`, `internal/templates/templates.go`, `internal/docker/docker.go`, `cmd/deploy.go`
- Modify: `.github/workflows/ci.yml`
- Create: `.github/dependabot.yml`

- [ ] **Step 1: See the failures before fixing them**

```bash
golangci-lint --version   # expect 2.x; install from https://golangci-lint.run if missing
golangci-lint cache clean
golangci-lint run ./... --max-same-issues=0 --max-issues-per-linter=0
```

The two flags matter: without them the linter hides same-text findings past the
third, and you would fix what you can see, re-run, and be surprised by more.

Expected: a non-zero exit listing errcheck findings. If it exits 0, earlier
phases already fixed them all — skip to Step 3.

- [ ] **Step 2: Silence each finding explicitly**

For every close the linter names — **deferred or not** — wrap it so the discard
is deliberate rather than accidental. In `internal/ssh/ssh.go`,
`internal/templates/templates.go`, `internal/docker/docker.go` and
`cmd/deploy.go`, replace each reported line of the form:

```go
	defer session.Close()
```

with the wrapped form, keeping the original receiver name (`session`, `file`,
`client`, `local`):

```go
	defer func() { _ = session.Close() }()
```

A bare, non-deferred close such as `internal/ssh/ssh.go:111` takes the simpler
form — no closure needed, because nothing is being deferred:

```go
	_ = stdin.Close()
```

For any reported `os.Setenv`, prefix the call with `_ =`:

```go
	_ = os.Setenv(key, value)
```

Do not add a `//nolint` comment and do not add a `.golangci.yml` to exclude
errcheck — these are real discards, and writing them out keeps them visible.

- [ ] **Step 3: Confirm the tree is clean and unbroken**

```bash
golangci-lint run ./...   # must exit 0 with no findings
gofmt -l .                # must print nothing
go vet ./...
go test -race ./...
```

Expected: lint silent, and all tests still passing. The race detector already
passes today, so any failure here came from this task's edits.

- [ ] **Step 4: Add a platform matrix and the race detector**

Replace the `jobs:` block of `.github/workflows/ci.yml` with:

```yaml
jobs:
  test:
    name: Test (${{ matrix.os }})
    runs-on: ${{ matrix.os }}
    strategy:
      fail-fast: false
      matrix:
        # macOS is the only release target, so it has to gate PRs too.
        os: [ubuntu-latest, macos-latest]

    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version-file: go.mod

      - name: Verify formatting
        run: |
          if [ -n "$(gofmt -l .)" ]; then
            echo "These files are not gofmt-clean:"
            gofmt -l .
            exit 1
          fi

      - name: Vet
        run: go vet ./...

      - name: Test
        run: go test -race ./...

  lint:
    name: Lint
    runs-on: ubuntu-latest
    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version-file: go.mod

      - name: golangci-lint
        # v8 is the first action major that works with golangci-lint >= v2.1.0;
        # v6 drives golangci-lint v1 only and would fail to install v2 at all.
        # The version is pinned so CI and local runs agree - bump both together.
        uses: golangci/golangci-lint-action@v8
        with:
          version: v2.12.2
```

`actions/setup-go` must stay ahead of this step: the action has required an
explicit Go install since its v4.

- [ ] **Step 5: Add Dependabot**

Create `.github/dependabot.yml`:

```yaml
version: 2
updates:
  # golang.org/x/crypto carries the SSH client, so security updates matter here.
  - package-ecosystem: gomod
    directory: "/"
    schedule:
      interval: weekly
    open-pull-requests-limit: 5

  - package-ecosystem: github-actions
    directory: "/"
    schedule:
      interval: weekly
    open-pull-requests-limit: 5
```

- [ ] **Step 6: Re-run the full gate exactly as CI will**

```bash
golangci-lint run ./...   # must exit 0 - this is the new gate
gofmt -l .                # must print nothing
go vet ./...
go test -race ./...
```

Expected: all four silent/passing. If lint reports anything now, the new job
would go red on the very first PR — fix it here, not after the gate lands.

- [ ] **Step 7: Commit**

```bash
git add internal/ssh/ssh.go internal/templates/templates.go internal/docker/docker.go cmd/deploy.go
git add .github/workflows/ci.yml .github/dependabot.yml
git commit -m "ci: test on macOS, enable -race, add lint and Dependabot

macOS was the only release target but never gated a PR. Also runs the race
detector and golangci-lint, and has Dependabot watch gomod (x/crypto ships
the SSH client) and the actions themselves.

The tree had 9 unchecked-error findings, so the deferred closes and the
Setenv call are now explicit discards - otherwise the new lint gate would
have gone red on the first PR after it landed. Pins the action to v8 and
golangci-lint to v2.12.2; action v6 drives golangci-lint v1 only.

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>"
```

- [ ] **Step 8: Confirm the gate is green on the first PR**

After pushing, open the PR and confirm **all three** checks pass: `Test (ubuntu-latest)`,
`Test (macos-latest)` and `Lint`. If the lint job fails to _install_ rather than
reporting findings, the action major and the pinned golangci-lint version have
drifted apart — check the action's README compatibility table and bump the
`uses:` major, keeping the `version:` pin.

**Phase 3 is now shippable.**

---

# Phase 4 — Prune and cleanup

### Task 8: `prune` sees registry-tagged images

Evidence rows 8–9: `build` creates two tags per build but `prune` only ever matched one of them, so the registry-prefixed copies — the bulk of the disk usage — accumulated forever.

**Files:**

- Modify: `internal/docker/docker.go`
- Modify: `cmd/prune.go`
- Test: `internal/docker/docker_test.go` (create)

**Interfaces:**

- Consumes: nothing.
- Produces: `func MatchesAnyCurrent(image string, current []string) bool` in package `docker` — pure, so it is testable without a daemon.

- [ ] **Step 1: Write the failing test**

Create `internal/docker/docker_test.go`:

```go
package docker

import "testing"

func TestMatchesAnyCurrent(t *testing.T) {
	current := []string{
		"myapp_api:0.1.0",
		"reg.example.com/myapp_api:0.1.0",
	}
	tests := []struct {
		name  string
		image string
		want  bool
	}{
		{"bare current image is kept", "myapp_api:0.1.0", true},
		{"registry-tagged current image is kept", "reg.example.com/myapp_api:0.1.0", true},
		{"bare old image is pruned", "myapp_api:0.0.9", false},
		{"registry-tagged old image is pruned", "reg.example.com/myapp_api:0.0.9", false},
		{"unrelated image is pruned", "postgres:16", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MatchesAnyCurrent(tt.image, current); got != tt.want {
				t.Errorf("MatchesAnyCurrent(%q) = %v, want %v", tt.image, got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/docker/ -v`

Expected: FAIL — `undefined: MatchesAnyCurrent`.

- [ ] **Step 3: Implement**

In `internal/docker/docker.go`, add `"slices"` to the imports and add:

```go
// MatchesAnyCurrent reports whether image is one of the current images.
// build produces two tags per service - a bare one and a registry-prefixed
// one - and both have to survive a prune.
func MatchesAnyCurrent(image string, current []string) bool {
	return slices.Contains(current, image)
}
```

- [ ] **Step 4: Run it to verify it passes**

Run: `go test ./internal/docker/ -v`

Expected: PASS for all five cases.

- [ ] **Step 5: Make prune list and keep both tag forms**

In `cmd/prune.go`, replace the listing block (currently lines 49–59, from the
`// List all images for this project` comment through the closing brace of the
`if len(images) == 0` block) with:

```go
	// build tags every image twice, bare and registry-prefixed, so both
	// patterns have to be listed or half the images are never pruned.
	patterns := []string{
		fmt.Sprintf("%s_*", cfg.Project),
		fmt.Sprintf("%s/%s_*", cfg.Registry, cfg.Project),
	}

	var images []string
	for _, pattern := range patterns {
		found, err := docker.ListImages(pattern)
		if err != nil {
			return err
		}
		images = append(images, found...)
	}

	if len(images) == 0 {
		fmt.Println("No images found to prune")
		return nil
	}
```

Then replace the filtering block (currently lines 61–76, from the
`// Filter out images that match current versions` comment through the closing
brace of the `for` loop) with:

```go
	// Every tag that must survive: both forms, for every service.
	var keep []string
	for service, ver := range currentVersions {
		imageName := cfg.GetImageName(service)
		keep = append(keep,
			fmt.Sprintf("%s:%s", imageName, ver),
			fmt.Sprintf("%s/%s:%s", cfg.Registry, imageName, ver),
		)
	}

	var imagesToRemove []string
	for _, image := range images {
		if !docker.MatchesAnyCurrent(image, keep) {
			imagesToRemove = append(imagesToRemove, image)
		}
	}
```

- [ ] **Step 6: Verify against a real daemon**

```bash
docker pull -q alpine:3.19 >/dev/null
docker tag alpine:3.19 myapp_api:0.1.0
docker tag alpine:3.19 myapp_api:0.0.9
docker tag alpine:3.19 reg.example.com/myapp_api:0.1.0
docker tag alpine:3.19 reg.example.com/myapp_api:0.0.9

go build -o /tmp/rocketctl-t . && D=$(mktemp -d) && cd "$D" && \
  printf 'myapp\nreg.example.com\nus-east-2\n\n\nn\napi\n' | /tmp/rocketctl-t init >/dev/null && \
  printf '0.1.0\n' > .rocket-version && \
  printf 'n\n' | /tmp/rocketctl-t prune
```

Expected: the removal list contains **both** `myapp_api:0.0.9` and `reg.example.com/myapp_api:0.0.9`, and neither `:0.1.0` tag. Answering `n` cancels.

Clean up:

```bash
docker rmi myapp_api:0.1.0 myapp_api:0.0.9 reg.example.com/myapp_api:0.1.0 reg.example.com/myapp_api:0.0.9
```

- [ ] **Step 7: Verify formatting and commit**

```bash
gofmt -l .   # must print nothing
go vet ./...
go test ./...
git add internal/docker/ cmd/prune.go
git commit -m "fix(prune): remove registry-tagged images too

build tags each image twice, bare and registry-prefixed, but prune only
filtered on the bare name, so the registry copies - the ones actually
filling the disk - were never removed.

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>"
```

---

### Task 9: Delete dead code and fix the docs it contradicts

Four template functions, two docker/compose functions and one config method have no callers. Worse, `GetEnvProductionPath` and the `.env.production` convention are documented as real but nothing implements them — `deploy` actually uses `.env.example` → `.env`.

**Files:**

- Modify: `internal/templates/templates.go`
- Modify: `internal/docker/docker.go`
- Modify: `internal/compose/compose.go`
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `cmd/init.go`
- Modify: `CLAUDE.md`
- Modify: `README.md`
- Modify: `docs/architecture.md`

**Interfaces:**

- Consumes: nothing.
- Produces: nothing. This task only removes.

- [ ] **Step 1: Confirm each target really has no callers**

```bash
for sym in GenerateEnvProductionExample RenderCaddyfile RenderEnvExample GetEnvProductionPath LoadEnvFile 'docker\.Run' 'compose\.Pull'; do
  echo "--- $sym ---"
  grep -rn "$sym" cmd internal main.go | grep -v "_test.go" | grep -v "^internal/templates/templates.go:.*func " | grep -v "^internal/docker/docker.go:.*func " | grep -v "^internal/compose/compose.go:.*func " | grep -v "^internal/config/config.go:.*func "
done
```

Expected: no output under any heading other than the declarations themselves. **`RenderDockerComposeProd` is deliberately not in this list — Task 2 made it the template test seam.** If any symbol does have a caller, leave it and note it in the commit message.

- [ ] **Step 2: Remove the unused template functions**

In `internal/templates/templates.go`, delete `GenerateEnvProductionExample`, `RenderCaddyfile` and `RenderEnvExample` in full. Then delete the now-unused embed:

```go
//go:embed env.production.example.tmpl
var envExampleTemplate string
```

and delete the file `internal/templates/env.production.example.tmpl`. Keep `GenerateDockerComposeProd`, `GenerateCaddyfile` and `RenderDockerComposeProd`.

- [ ] **Step 3: Remove the unused docker and compose functions**

In `internal/docker/docker.go`, delete `Run` and `LoadEnvFile` in full, then drop `"strings"` from the imports if `ListImages` no longer needs it (it does — keep it) and drop `"bufio"` if `LoadEnvFile` was its only user (`ListImages` uses it — keep it). Re-check with `go build ./...`.

In `internal/compose/compose.go`, delete `Pull` in full.

- [ ] **Step 4: Remove `GetEnvProductionPath` and its tests**

In `internal/config/config.go`, delete the `GetEnvProductionPath` method.

In `internal/config/config_test.go`, delete the two table entries that call it (around lines 166 and 171) — the ones whose `got` closures call `GetEnvProductionPath`.

- [ ] **Step 5: Fix the init guidance that describes a convention that does not exist**

In `cmd/init.go`, replace the "Next steps" line 3:

```go
	fmt.Println("3. Create .env.production on the server with the required secrets")
```

with:

```go
	fmt.Println("3. Create .env.example with the variables your services need")
	fmt.Println("   (deploy uploads it as .env on the server the first time)")
```

Renumber the following lines so the list still reads 1–5:

```go
	fmt.Println("4. Customize docker-compose.prod.yml as needed")
	if cfg.Domain != "" {
		fmt.Println("5. Customize caddy/Caddyfile as needed")
	}
```

- [ ] **Step 6: Update CLAUDE.md**

In the **Must** section, remove `GetEnvProductionPath` from the list of required `Config` methods, so it reads:

```markdown
- **Never build paths or image names by hand.** Use the `Config` methods:
  `GetServiceDirectory`, `GetVersionFilePath`, `GetDockerfilePath`,
  `GetImageName`, `GetFullImageName`, `EnvVersionKey`. The
  `<project>_<service>:<version>` scheme is not configurable.
```

Under **Watch out**, replace the stale remote-path warning with what is now true:

```markdown
- **Remote paths are shell-quoted, not sanitised by luck.** `ssh.ShellQuote` wraps values that
  reach a remote shell, and `config.Validate` restricts `project`/`service` to
  `[A-Za-z0-9._-]`. Keep a leading `~` OUTSIDE the quotes - a quoted tilde is not expanded.
- **Host keys are verified** against `~/.ssh/known_hosts` with a trust-on-first-use prompt;
  `insecure_skip_host_key_check: true` opts out.
```

- [ ] **Step 7: Update `docs/architecture.md`**

This is the doc CLAUDE.md points to for "deeper detail", and its **Known defects**
section currently lists three things this plan fixes. Leaving it would have the
repo's primary architecture doc advertise a host-key hole and an injection
surface that no longer exist — worse than no doc at all in an agent-driven repo.

First confirm the section still looks as expected:

```bash
grep -n "Known defects" -A 12 docs/architecture.md
```

In the **Known defects** list (currently lines 92–97), delete these three entries:

```markdown
- `Makefile` `VERSION?=1.0.0` is stale (current release is 1.4.0).
- `ssh.InsecureIgnoreHostKey()` (`internal/ssh/ssh.go:29`) — no host key verification.
- Remote shell strings interpolate paths unquoted — an injection surface fed by `rocket.yaml`.
```

Leave the other two entries alone — the Go version mismatch and the
`Copyright © 2026 NAME HERE <EMAIL ADDRESS>` placeholder are both still true and
out of this plan's scope.

Then replace the CI sentence (currently lines 84–86):

```markdown
release workflow runs no tests, vet, or fmt check. `.github/workflows/ci.yml` gates pull requests
and pushes to `main` with `gofmt -l`, `go vet`, and `go test` — see the "Testing" section in
`CLAUDE.md`.
```

with:

```markdown
release workflow builds both binaries and asserts the built binary reports the tag it was built
from. `.github/workflows/ci.yml` gates pull requests and pushes to `main` with `gofmt -l`,
`go vet`, and `go test -race` on both `ubuntu-latest` and `macos-latest`, plus a `golangci-lint`
job — see the "Testing" section in `CLAUDE.md`.
```

Finally, update step 6 of the **Deploy flow** list (currently line 72):

```markdown
6. Run ECR login, `docker compose pull`, `docker compose up -d` remotely.
```

to record that the pull and up are version-pinned:

```markdown
6. Run ECR login, then `docker compose pull` and `docker compose up -d` remotely with each
   service's `<SERVICE>_VERSION` set from its local `.rocket-version`, so the server runs the
   version that was built rather than falling back to `:latest`.
```

- [ ] **Step 8: Update README.md**

Search for `.env.production`:

```bash
grep -n "env.production" README.md
```

For each hit, correct it to describe the real flow: `deploy` uploads `.env.example` to the server as `.env` on first deploy and never overwrites it afterwards. If there are no hits, skip this step.

- [ ] **Step 9: Verify nothing broke**

```bash
go build ./...
gofmt -l .   # must print nothing
go vet ./...
go test ./...
```

Expected: builds clean, all tests pass. A compile error naming a deleted function means it had a caller — restore that one function.

- [ ] **Step 10: Commit**

```bash
git add internal/ cmd/init.go CLAUDE.md README.md docs/architecture.md
git commit -m "refactor: drop dead code and the .env.production fiction

GenerateEnvProductionExample, RenderCaddyfile, RenderEnvExample,
docker.Run, docker.LoadEnvFile, compose.Pull and GetEnvProductionPath had
no callers. GetEnvProductionPath was listed in CLAUDE.md as a required
accessor and init told users to create .env.production, but deploy has
always used .env.example -> .env.

architecture.md's 'Known defects' also still listed the Makefile version
drift, InsecureIgnoreHostKey and the unquoted remote interpolation, all of
which earlier phases fixed, and its CI paragraph predated the matrix, race
detector and lint job. Docs now match the code.

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>"
```

---

## Final verification

After the last task:

```bash
gofmt -l .        # must print nothing - make check's fmt step REWRITES instead of failing
go vet ./...
go test -race ./...
golangci-lint run ./...   # exit 0 - this is a CI gate from Task 7 onward
make build && ./rocketctl --version && rm -f rocketctl
```

Then the manual checks CLAUDE.md requires for the shell-out paths, in a real project, in **both** modes:

| Command               | Single-service                              | Monorepo           |
| --------------------- | ------------------------------------------- | ------------------ |
| `rocketctl init`      | rejects an empty service name               | rejects `api,,web` |
| `rocketctl build`     | image tagged `<project>_<service>:<v>`      | same, per service  |
| `rocketctl up --prod` | stack starts on the built version           | same               |
| `rocketctl push`      | pushes `<registry>/<project>_<service>:<v>` | same               |
| `rocketctl deploy`    | remote runs the built version, not `latest` | same, all services |
| `rocketctl prune`     | lists both bare and registry tags           | same               |

Optionally run `make mutate` — advisory only, act on `LIVED` mutants and ignore `TIMED OUT` (see `docs/mutation-testing.md`).

## Self-Review

**Spec coverage** — every item from the review maps to a task:

| Finding                                              | Task                        |
| ---------------------------------------------------- | --------------------------- |
| deploy resolves `:latest`                            | 3                           |
| hyphenated services break interpolation              | 1, 2                        |
| empty service name from `init`                       | 1                           |
| unquoted `cfg.Project` in remote shell               | 1 (validation), 3 (quoting) |
| `InsecureIgnoreHostKey`                              | 4                           |
| `.env` uploaded 0644                                 | 5                           |
| `UploadFile` buffers the whole file                  | 5                           |
| no binary `--version`                                | 6                           |
| Makefile `VERSION` drift                             | 6                           |
| Linux-only CI, no race/lint/Dependabot               | 7                           |
| prune misses registry tags                           | 8                           |
| dead code                                            | 9                           |
| `.env.production` documented but unimplemented       | 9                           |
| obsolete compose `version:` (found while validating) | 2                           |

**Placeholder scan** — no TBDs. Every code step carries the real code; every command carries its expected output.

**Type consistency** — `config.EnvVersionKey(string) string` is defined in Task 1 and used with that exact name and signature in Tasks 2, 3 and Task 9's doc update. `ssh.ShellQuote(string) string` is defined in Task 3; Task 5 consumes its _output_ via the pre-quoted `remotePath` but never calls it. `Connect` gains its fourth parameter in Task 4, where its only caller is updated in the same task. `RenderDockerComposeProd` is claimed as a test seam in Task 2 and explicitly excluded from the deletions in Task 9.

**Known risk** — Tasks 4 and 5 cannot be automated-tested without an SSH server, which this environment does not have. Both carry explicit manual verification steps, matching how CLAUDE.md already treats the docker/compose/ssh/aws paths.

## Revision 2 — changes from code review (2026-09-23)

Each item below was re-verified against the tree before the plan was edited.

| #   | Change                                                                                          | Verified how                                                                                                                                                                                                                 |
| --- | ----------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| n5  | Task 7 now fixes the tree's lint findings **before** adding the lint gate (new Steps 1–3, 6, 8) | `golangci-lint run ./...` with a cleared cache reports errcheck findings today (9 displayed, 11 actual — see Revision 3); the `_ =` / `defer func() { _ = x.Close() }()` form was proven to silence them in a scratch module |
| n5b | Task 7 pins `golangci-lint-action@v8` + `version: v2.12.2` instead of `@v6` + `latest`          | The action's own compatibility table: `v7.0.0` is the first major supporting golangci-lint v2, `v8.0.0` needs >= v2.1.0. **`@v6` would have failed to install v2 at all** — the review flagged the red gate but not this     |
| n6  | Task 9 Step 7 now updates `docs/architecture.md`                                                | Its "Known defects" (lines 92–97) lists the Makefile drift, `InsecureIgnoreHostKey` and unquoted interpolation; its CI paragraph (lines 84–86) and deploy-flow step 6 (line 72) also go stale                                |
| —   | Task 1 validates `registry` and `region`                                                        | `cmd/deploy.go:187-191` interpolates both unquoted into the remote ECR login; `hostPattern`/`namePattern` were run against real ECR hosts, `reg.example.com:5000` and injection payloads                                     |
| —   | Task 1 rejects services colliding on one compose variable                                       | Confirmed `my-api`, `my.api` and `my_api` all map to `MY_API_VERSION`                                                                                                                                                        |
| —   | Task 3 collects versions **before** `ssh.Connect`                                               | Fails locally and leaves the server untouched; also avoids `version.Get`'s documented create-at-0.1.0 side effect surfacing as a confusing remote registry error                                                             |
| —   | Task 3 prints hints via an unquoted `remoteDirDisplay`                                          | The quoted form still works when pasted, but reads as broken                                                                                                                                                                 |
| —   | Task 8 line refs corrected to 49–59 and 61–76                                                   | Read `cmd/prune.go` directly                                                                                                                                                                                                 |
| —   | Tasks 3 and 5 emit lint-clean code                                                              | So they do not hand Task 7 new findings                                                                                                                                                                                      |

Two known-defect entries in `docs/architecture.md` are deliberately left in place: the Go version mismatch (README says `1.23+`, `go.mod` says `1.24.0`) and the `Copyright © 2026 NAME HERE <EMAIL ADDRESS>` placeholder in `main.go` and `cmd/root.go`. Both are still true after this plan lands. The placeholder sits in `cmd/root.go`, which Task 6 edits — a one-line fix if scope is ever widened, but it is not in this plan.

## Revision 3 — second review round (2026-09-23)

| Change                                                                               | Verified how                                                                                                                                                                                 |
| ------------------------------------------------------------------------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Task 7's preamble corrected from **9 findings to 11**                                | A bare `golangci-lint run ./...` reports 9; `--max-same-issues=0 --max-issues-per-linter=0` reports 11, because the default display caps same-text issues at 3. Step 1 now passes both flags |
| `internal/ssh/ssh.go:111` added to the enumeration                                   | Confirmed a bare, non-deferred `stdin.Close()` at that line; the previous list held only the four deferred `session.Close()` calls, and the header said 9 while listing 10                   |
| Task 7 Step 2 broadened to non-deferred closes, showing the `_ = stdin.Close()` form | The closure wrapper is unnecessary where nothing is deferred                                                                                                                                 |
| Task 5's `UploadFileMode` now writes `_ = stdin.Close()`                             | Its replacement had carried the bare form forward, so the revision-2 claim that Tasks 3 and 5 emit lint-clean code was false for Task 5 until this edit                                      |
| Preamble no longer invites matching the linter's output against the table            | The table is a pre-phase snapshot; what remains at Step 1 depends on which phases have landed, and Task 9's two deletions come _after_ Task 7                                                |

The gate itself was never at risk from this: Steps 3 and 6 require the linter's
actual output to be clean, so a wrong count in the prose could not have landed a
red CI check. The correction is about the plan telling the truth, not about the
outcome changing.
