# Mutation Testing + Foundational Tests Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stand up a mutation-testing harness and back it with table-driven unit tests for the two pure packages (`internal/version`, `internal/config`), so regressions in RocketCTL's core invariants are caught automatically.

**Architecture:** Task 1 installs the harness and records a baseline. Tasks 2–3 add table-driven tests for the pure, I/O-free packages. Task 4 characterises the mutation results and decides whether they can gate anything (measured answer: no). Task 5 enforces the deterministic checks on pull requests. Tests live beside the code they cover (`internal/<pkg>/<pkg>_test.go`), in the same package, so unexported helpers stay testable.

**Tech Stack:** Go 1.24 (`go.mod`), stdlib `testing` only (no assertion library — the repo has zero third-party test deps and should keep it that way), `gremlins` v0.6.0 for mutation testing, GitHub Actions for CI.

## Global Constraints

- Test framework: **stdlib `testing` only**. Do not add testify, gomega, or any assertion library.
- Test style: **table-driven** with `t.Run` subtests, matching idiomatic Go. Name subtests as sentences describing behaviour.
- Temp files: use `t.TempDir()`. Working-directory changes: use `t.Chdir()` (Go 1.24+). Never leave a test dependent on repo state.
- Do not modify production behaviour in Tasks 1–3. If a test reveals a bug, file it with `./scripts/ticket.sh new bug` and keep the test asserting **current** behaviour.
- Commit after every task using Conventional Commit prefixes (`test:`, `build:`, `ci:`, `fix:`) — matches existing history.
- Every commit message ends with:
  `Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>`
- gremlins binary path: it installs to `$(go env GOBIN)`. On this machine that is
  `/Users/jair.mendez/.local/share/mise/installs/go/1.26.3/bin`. Ensure it is on `PATH` before running `make mutate`.
- Rollback: every task is one commit over purely additive changes, so undoing a task is
  `git revert <sha>` for that task alone. No task requires unwinding a later one first.

## Verified findings (measured 2026-09-20, gremlins v0.6.0, Go 1.26.3)

Confirmed by running the tools against this repo. Trust these over intuition.

1. **Zero-test baseline:** `gremlins unleash --dry-run ./internal/version/` reports
   `Runnable: 0, Not covered: 17, Mutator coverage: 0.00%`. gremlins only tests _covered_ mutants,
   so the score stays at 0 until Task 2 lands. Expected, not a failure.
2. **The Task 2 and Task 3 test files were executed and pass** against the current implementation
   (26 tests in `config`, all green in `version`). They are characterisation tests — expect them to
   go green immediately.
3. **There are no surviving mutants to kill.** With both test files in place, `version` reported
   `Killed: 13, Lived: 0` and `config` reported `Killed: 3, Lived: 0` — 100% efficacy on both.
   Task 4 is therefore about _stability_, not about chasing survivors.
4. **gremlins is non-deterministic on this repo.** Two identical back-to-back runs of
   `gremlins unleash ./internal/config/` produced `Killed: 2, efficacy 100.00%` and then
   `Killed: 0, efficacy 0.00%`. Mutants land in `TIMED OUT` at random. The likely cause is that the
   suite is very fast (0.16–0.67s), so the per-mutant timeout gremlins derives from that baseline is
   smaller than Go's compile-and-run overhead. Raising `--timeout-coefficient` made it _worse_.

   **Consequence: do not gate CI on mutation thresholds.** Tasks 4 and 5 reflect this.

---

## Precondition: commit the untracked scaffolding first

At the time of writing, `git status --short` reports three untracked entries:

```
?? CLAUDE.md
?? docs/
?? scripts/ticket.sh
```

Several later steps assume these are already tracked — Task 3 Step 3 checks that `git status` is
otherwise clean, and Task 5 Step 5 _updates_ `CLAUDE.md` rather than creating it. Commit them before
starting Task 1:

```bash
git add CLAUDE.md docs/ scripts/ticket.sh
```

```bash
git commit -m "$(cat <<'EOF'
docs: add agent instructions, architecture notes and ticket helper

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>
EOF
)"
```

Verify: `git status --short` prints nothing before continuing.

---

### Task 1: Mutation harness + unblock `make check`

`make check` currently mutates the working tree because `main.go` is not gofmt-clean, and `go.mod`
mislabels a direct dependency. Both must be fixed first or every later task's verification step is
untrustworthy.

**Files:**

- Modify: `main.go:1-5`
- Modify: `go.mod:5-15` (both `require` blocks)
- Modify: `Makefile:1` (`.PHONY`), and append a `mutate` target after the `check` target

**Interfaces:**

- Consumes: nothing.
- Produces: `make mutate` target; a gremlins binary on `PATH`; a gofmt-clean tree so
  `make check` is a no-op on success.

- [ ] **Step 1: Confirm the tree is currently dirty under gofmt**

Run: `gofmt -l .`
Expected: prints `main.go`

- [ ] **Step 2: Fix the gofmt violation**

`main.go` has a stray blank line inside the header comment. Replace the whole header so the file reads:

```go
/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package main

import "github.com/cjairm/rocketctl/cmd"

func main() {
	cmd.Execute()
}
```

- [ ] **Step 3: Verify gofmt is clean**

Run: `gofmt -l .`
Expected: no output

- [ ] **Step 4: Fix the mislabelled dependency**

`golang.org/x/crypto` is marked `// indirect` but `internal/ssh/ssh.go` imports it directly.

Run: `go mod tidy`
Expected: `go.mod` now lists `golang.org/x/crypto v0.48.0` in the **first** `require` block with no
`// indirect` comment.

- [ ] **Step 5: Verify the build still works**

Run: `go build ./... && go vet ./...`
Expected: no output, exit 0

- [ ] **Step 6: Install gremlins**

Run: `go install github.com/go-gremlins/gremlins/cmd/gremlins@v0.6.0`
Then: `gremlins --version`
Expected: `gremlins version dev <os>/<arch>` (the binary self-reports `dev`; the pinned module
version is what matters). The version is pinned deliberately — every measured finding above was
taken against v0.6.0, and a newer release may not reproduce them.

If `gremlins: command not found`, add `$(go env GOBIN)` to `PATH`.

- [ ] **Step 7: Add the `mutate` target to the Makefile**

Change line 1 from:

```make
.PHONY: build clean release install test help
```

to:

```make
.PHONY: build clean release install test help fmt vet check mutate
```

Then append after the `check` target:

```make
## mutate: Run mutation testing on the core packages
mutate:
	@echo "Running mutation tests..."
	@gremlins unleash ./internal/version/
	@gremlins unleash ./internal/config/
```

Note: `gremlins unleash` takes a single path, so the two packages are invoked separately.
Thresholds are deliberately absent, and Task 4 explains why they stay absent.

- [ ] **Step 8: Record the zero baseline**

Run: `make mutate`
Expected: both packages report `Killed: 0, Lived: 0, Not covered: <N>` and
`Mutator coverage: 0.00%`. (`Runnable: 0` is the _dry-run_ wording — a real run does not print it.)
This is the documented starting point — the harness works, there is simply nothing covered yet.

No `.gitignore` change is needed: line 13 already has `*.out`, which covers coverage artifacts.

- [ ] **Step 9: Commit**

```bash
git add main.go go.mod go.sum Makefile
git commit -m "$(cat <<'EOF'
build: add mutation testing target and fix check preconditions

main.go was not gofmt-clean and go.mod mislabelled golang.org/x/crypto as
indirect, which made `make check` mutate the working tree.

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 2: Tests for `internal/version`

The semver rules and the "bump only after success" contract live here. `Get` writes on read
(auto-initialising to `0.1.0`) — that is intentional behaviour and must be pinned by a test.

**Files:**

- Create: `internal/version/version_test.go`

**Interfaces:**

- Consumes: `make mutate` (Task 1).
- Produces: coverage over `Get(path string) (string, error)`, `Set(path, version string) error`,
  `CalculateBump(currentVersion, bumpType string) (string, error)`.

- [ ] **Step 1: Write the characterisation tests**

Create `internal/version/version_test.go`:

```go
package version

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCalculateBump(t *testing.T) {
	tests := []struct {
		name    string
		current string
		bump    string
		want    string
		wantErr bool
	}{
		{name: "patch increments the patch component", current: "0.1.0", bump: "patch", want: "0.1.1"},
		{name: "minor increments minor and resets patch", current: "0.1.3", bump: "minor", want: "0.2.0"},
		{name: "major increments major and resets minor and patch", current: "1.2.3", bump: "major", want: "2.0.0"},
		{name: "multi digit components are handled", current: "10.20.30", bump: "patch", want: "10.20.31"},
		{name: "unknown bump type is rejected", current: "1.0.0", bump: "huge", wantErr: true},
		{name: "empty bump type is rejected", current: "1.0.0", bump: "", wantErr: true},
		{name: "two component version is rejected", current: "1.0", bump: "patch", wantErr: true},
		{name: "prerelease suffix is rejected", current: "1.0.0-beta", bump: "patch", wantErr: true},
		{name: "non numeric component is rejected", current: "1.x.0", bump: "patch", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CalculateBump(tt.current, tt.bump)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("CalculateBump(%q, %q) = %q, want error", tt.current, tt.bump, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("CalculateBump(%q, %q) unexpected error: %v", tt.current, tt.bump, err)
			}
			if got != tt.want {
				t.Errorf("CalculateBump(%q, %q) = %q, want %q", tt.current, tt.bump, got, tt.want)
			}
		})
	}
}

func TestSetThenGetRoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".rocket-version")

	if err := Set(path, "2.3.4"); err != nil {
		t.Fatalf("Set unexpected error: %v", err)
	}

	got, err := Get(path)
	if err != nil {
		t.Fatalf("Get unexpected error: %v", err)
	}
	if got != "2.3.4" {
		t.Errorf("Get = %q, want %q", got, "2.3.4")
	}
}

func TestGetInitialisesMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".rocket-version")

	got, err := Get(path)
	if err != nil {
		t.Fatalf("Get unexpected error: %v", err)
	}
	if got != "0.1.0" {
		t.Errorf("Get on missing file = %q, want %q", got, "0.1.0")
	}

	// Get persists the initial version as a deliberate side effect.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Get did not create %s: %v", path, err)
	}
	if string(data) != "0.1.0\n" {
		t.Errorf("file contents = %q, want %q", string(data), "0.1.0\n")
	}
}

func TestGetTrimsSurroundingWhitespace(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".rocket-version")
	if err := os.WriteFile(path, []byte("  1.2.3\n\n"), 0644); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	got, err := Get(path)
	if err != nil {
		t.Fatalf("Get unexpected error: %v", err)
	}
	if got != "1.2.3" {
		t.Errorf("Get = %q, want %q", got, "1.2.3")
	}
}

func TestGetRejectsEmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".rocket-version")
	if err := os.WriteFile(path, []byte("   \n"), 0644); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	if _, err := Get(path); err == nil {
		t.Fatal("Get on a whitespace-only file returned nil error, want error")
	}
}

func TestGetRejectsMalformedVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".rocket-version")
	if err := os.WriteFile(path, []byte("not-a-version\n"), 0644); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	if _, err := Get(path); err == nil {
		t.Fatal("Get on a malformed version returned nil error, want error")
	}
}

func TestSetRejectsInvalidVersionWithoutWriting(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".rocket-version")

	if err := Set(path, "1.0"); err == nil {
		t.Fatal("Set with invalid semver returned nil error, want error")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("Set created %s despite rejecting the input", path)
	}
}
```

- [ ] **Step 2: Run the tests to verify they pass**

Run: `go test ./internal/version/ -v`
Expected: PASS for all subtests. These describe existing behaviour, so they pass immediately —
this is characterisation testing, not red-green TDD. If any test **fails**, stop: you have found a
real bug. File it with `./scripts/ticket.sh new bug "<title>" -r internal/version/version.go:<line>`
and adjust the test to assert current behaviour before continuing.

- [ ] **Step 3: Confirm mutation coverage moved off zero**

Run: `gremlins unleash ./internal/version/`
Expected: mutants are now reported as `KILLED` rather than `NOT COVERED`. The measured run gave
`Killed: 13, Lived: 0`. **`Lived: 0` is the only part that must hold.** The `Killed` and
`Timed out` counts vary between identical runs — see finding 4 above. Do not treat a low `Killed`
count or a `0.00%` efficacy reading as a failure at this step; Task 4 deals with that instability.

- [ ] **Step 4: Commit**

```bash
git add internal/version/version_test.go
git commit -m "$(cat <<'EOF'
test: cover semver bump and .rocket-version file handling

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 3: Tests for `internal/config`

This package derives every path and image name in the tool. The exact string values below were
verified against `filepath.Join` — note it **cleans** the result, so a monorepo service directory is
`"api"`, not `"./api"`.

**Files:**

- Create: `internal/config/config_test.go`

**Interfaces:**

- Consumes: `make mutate` (Task 1).
- Produces: coverage over `Load() (*Config, error)`, `(*Config).Validate() error`,
  `(*Config).IsMonorepo() bool`, `(*Config).GetServices() []string`,
  `(*Config).ValidateService(string) error`, `(*Config).GetServiceDirectory(string) (string, error)`,
  `(*Config).GetVersionFilePath(string) (string, error)`,
  `(*Config).GetDockerfilePath(string, bool) (string, error)`,
  `(*Config).GetEnvProductionPath(string) (string, error)`,
  `(*Config).GetImageName(string) string`,
  `(*Config).GetFullImageName(string, string) string`,
  `(*Config).Save(string) error`.

- [ ] **Step 1: Write the characterisation tests**

Create `internal/config/config_test.go`:

```go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func singleService() *Config {
	return &Config{
		Project:  "myapp",
		Service:  "backend",
		Registry: "reg.example.com",
		Region:   "us-east-2",
	}
}

func monorepo() *Config {
	return &Config{
		Project:  "myapp",
		Services: []string{"api", "web"},
		Registry: "reg.example.com",
		Region:   "us-east-2",
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
	}{
		{name: "single service config is valid", cfg: singleService()},
		{name: "monorepo config is valid", cfg: monorepo()},
		{
			name:    "missing project is rejected",
			cfg:     &Config{Service: "backend", Registry: "reg.example.com", Region: "us-east-2"},
			wantErr: true,
		},
		{
			name:    "missing registry is rejected",
			cfg:     &Config{Project: "myapp", Service: "backend", Region: "us-east-2"},
			wantErr: true,
		},
		{
			name:    "missing region is rejected",
			cfg:     &Config{Project: "myapp", Service: "backend", Registry: "reg.example.com"},
			wantErr: true,
		},
		{
			name: "service and services are mutually exclusive",
			cfg: &Config{
				Project:  "myapp",
				Service:  "backend",
				Services: []string{"api"},
				Registry: "reg.example.com",
				Region:   "us-east-2",
			},
			wantErr: true,
		},
		{
			name:    "neither service nor services is rejected",
			cfg:     &Config{Project: "myapp", Registry: "reg.example.com", Region: "us-east-2"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr && err == nil {
				t.Fatal("Validate returned nil error, want error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("Validate unexpected error: %v", err)
			}
		})
	}
}

func TestIsMonorepoAndGetServices(t *testing.T) {
	if singleService().IsMonorepo() {
		t.Error("IsMonorepo on a single-service config = true, want false")
	}
	if !monorepo().IsMonorepo() {
		t.Error("IsMonorepo on a monorepo config = false, want true")
	}

	got := singleService().GetServices()
	if len(got) != 1 || got[0] != "backend" {
		t.Errorf("GetServices (single) = %v, want [backend]", got)
	}

	got = monorepo().GetServices()
	if len(got) != 2 || got[0] != "api" || got[1] != "web" {
		t.Errorf("GetServices (monorepo) = %v, want [api web]", got)
	}
}

func TestValidateService(t *testing.T) {
	if err := monorepo().ValidateService("api"); err != nil {
		t.Errorf("ValidateService(api) unexpected error: %v", err)
	}
	if err := monorepo().ValidateService("nope"); err == nil {
		t.Error("ValidateService(nope) returned nil error, want error")
	}
	if err := singleService().ValidateService("backend"); err != nil {
		t.Errorf("ValidateService(backend) unexpected error: %v", err)
	}
	if err := singleService().ValidateService("api"); err == nil {
		t.Error("ValidateService(api) on single-service returned nil error, want error")
	}
}

// Path values assert filepath.Join's cleaned output: a monorepo service
// directory is "api", not "./api".
func TestPathDerivation(t *testing.T) {
	tests := []struct {
		name    string
		got     func() (string, error)
		want    string
		wantErr bool
	}{
		{
			name: "single service directory is the project root",
			got:  func() (string, error) { return singleService().GetServiceDirectory("backend") },
			want: ".",
		},
		{
			name: "monorepo service directory is a cleaned subfolder",
			got:  func() (string, error) { return monorepo().GetServiceDirectory("api") },
			want: "api",
		},
		{
			name:    "unknown service is rejected",
			got:     func() (string, error) { return monorepo().GetServiceDirectory("nope") },
			wantErr: true,
		},
		{
			name: "single service version file sits at the root",
			got:  func() (string, error) { return singleService().GetVersionFilePath("backend") },
			want: ".rocket-version",
		},
		{
			name: "monorepo version file sits under the service",
			got:  func() (string, error) { return monorepo().GetVersionFilePath("api") },
			want: "api/.rocket-version",
		},
		{
			name: "single service production dockerfile",
			got:  func() (string, error) { return singleService().GetDockerfilePath("backend", true) },
			want: "Dockerfile.production",
		},
		{
			name: "single service development dockerfile",
			got:  func() (string, error) { return singleService().GetDockerfilePath("backend", false) },
			want: "Dockerfile",
		},
		{
			name: "monorepo production dockerfile",
			got:  func() (string, error) { return monorepo().GetDockerfilePath("api", true) },
			want: "api/Dockerfile.production",
		},
		{
			name: "single service env production path",
			got:  func() (string, error) { return singleService().GetEnvProductionPath("backend") },
			want: ".env.production",
		},
		{
			name: "monorepo env production path",
			got:  func() (string, error) { return monorepo().GetEnvProductionPath("api") },
			want: "api/.env.production",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.got()
			if tt.wantErr {
				if err == nil {
					t.Fatalf("got %q, want error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestImageNaming(t *testing.T) {
	cfg := monorepo()

	if got := cfg.GetImageName("api"); got != "myapp_api" {
		t.Errorf("GetImageName = %q, want %q", got, "myapp_api")
	}

	want := "reg.example.com/myapp_api:1.2.3"
	if got := cfg.GetFullImageName("api", "1.2.3"); got != want {
		t.Errorf("GetFullImageName = %q, want %q", got, want)
	}
}

func TestLoadReadsRocketYAMLFromWorkingDirectory(t *testing.T) {
	dir := t.TempDir()
	contents := "project: myapp\nservice: backend\nregistry: reg.example.com\nregion: us-east-2\n"
	if err := os.WriteFile(filepath.Join(dir, "rocket.yaml"), []byte(contents), 0644); err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	t.Chdir(dir)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load unexpected error: %v", err)
	}
	if cfg.Project != "myapp" || cfg.Service != "backend" {
		t.Errorf("Load = %+v, want project=myapp service=backend", cfg)
	}
}

func TestLoadFailsWhenRocketYAMLIsMissing(t *testing.T) {
	t.Chdir(t.TempDir())

	if _, err := Load(); err == nil {
		t.Fatal("Load without rocket.yaml returned nil error, want error")
	}
}

func TestLoadRejectsInvalidConfig(t *testing.T) {
	dir := t.TempDir()
	// Missing the required region field.
	contents := "project: myapp\nservice: backend\nregistry: reg.example.com\n"
	if err := os.WriteFile(filepath.Join(dir, "rocket.yaml"), []byte(contents), 0644); err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	t.Chdir(dir)

	if _, err := Load(); err == nil {
		t.Fatal("Load with a missing region returned nil error, want error")
	}
}

func TestSaveWritesALoadableConfig(t *testing.T) {
	dir := t.TempDir()
	if err := monorepo().Save(filepath.Join(dir, "rocket.yaml")); err != nil {
		t.Fatalf("Save unexpected error: %v", err)
	}
	t.Chdir(dir)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load after Save unexpected error: %v", err)
	}
	if !cfg.IsMonorepo() || len(cfg.Services) != 2 {
		t.Errorf("round-tripped config = %+v, want monorepo with 2 services", cfg)
	}
}
```

- [ ] **Step 2: Run the tests**

Run: `go test ./internal/config/ -v`
Expected: PASS for all subtests. As in Task 2, a failure means a real bug — file a ticket and pin
current behaviour rather than silently changing production code.

- [ ] **Step 3: Run the whole suite and the quality gate**

Run: `make check`
Expected: `✓ All checks passed`, and `git status --short` shows only
`?? internal/config/config_test.go`. If it shows anything else, the precondition commit was skipped.

- [ ] **Step 4: Commit**

```bash
git add internal/config/config_test.go
git commit -m "$(cat <<'EOF'
test: cover config validation and path derivation

Pins the single-service vs monorepo path rules and the
<project>_<service>:<version> image naming scheme.

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 4: Characterise mutation stability and decide the gate

There are **no surviving mutants** — both packages already report `Lived: 0`. The real problem
measured on this repo is that gremlins randomly classifies mutants as `TIMED OUT`, which makes the
efficacy number swing between 100% and 0% across identical runs. This task establishes whether that
is reproducible on your machine and records the decision.

**Files:**

- Create: `docs/mutation-testing.md`
- Modify: `Makefile` (the `mutate` target from Task 1)

**Interfaces:**

- Consumes: the test files from Tasks 2 and 3; the `mutate` target from Task 1.
- Produces: `docs/mutation-testing.md` documenting how to run and read the tool; a `mutate` target
  that stays advisory rather than gating.

- [ ] **Step 1: Measure run-to-run variance**

```bash
for i in 1 2 3 4 5; do
  echo "--- run $i ---"
  gremlins unleash ./internal/config/ 2>&1 | grep -E "Killed:|efficacy|coverage"
done
```

Expected on the machine this plan was written on: the numbers differ between runs, with `Killed`
varying and `Timed out` absorbing the remainder. `Lived` should be `0` in every run.

- [ ] **Step 2: Branch on the result**

- **If `Lived: 0` in every run** (the measured outcome): the tests kill everything gremlins can
  actually execute. Proceed to Step 3 and keep the target advisory.
- **If any run reports `Lived: N` with `N > 0`**: you have found a genuine test gap. Run
  `gremlins unleash -S l ./internal/config/` to list only the survivors, then add one table case per
  survivor to the relevant test file. Do **not** change production code to make a mutant die — the
  mutant is reporting a missing test, not a broken implementation. Re-run until `Lived` is 0.

- [ ] **Step 3: Keep the target advisory, and say why in the Makefile**

Replace the `mutate` target from Task 1 with:

```make
## mutate: Run mutation testing on the core packages (advisory, not a gate)
mutate:
	@echo "Running mutation tests (advisory - results are not stable enough to gate CI)..."
	@echo "Look at the 'Lived' count. Ignore 'Timed out' - see docs/mutation-testing.md."
	@gremlins unleash ./internal/version/ || true
	@gremlins unleash ./internal/config/ || true
```

The `|| true` is deliberate: a non-deterministic tool must not be able to fail a build. Do not add
`--threshold-efficacy` or `--threshold-mcover` until the timeout behaviour is understood.

- [ ] **Step 4: Document how to read the output**

Create `docs/mutation-testing.md`:

```markdown
# Mutation testing

`make mutate` runs [gremlins](https://github.com/go-gremlins/gremlins) over `internal/version` and
`internal/config` — the two packages with no I/O, and therefore the only ones currently testable in
isolation.

## Reading the output

| Status        | Meaning                                         | Action                                                    |
| ------------- | ----------------------------------------------- | --------------------------------------------------------- |
| `KILLED`      | A test caught the mutation.                     | None. This is the goal.                                   |
| `LIVED`       | The code was changed and **no test noticed**.   | Add a test case. Never change production code to kill it. |
| `NOT COVERED` | No test exercises that line at all.             | Add coverage, or accept it deliberately.                  |
| `TIMED OUT`   | The mutated test run exceeded gremlins' budget. | Ignore — see below.                                       |

**Only `LIVED` matters.** It is the one status that reports a real gap in the test suite.

## Why this is advisory and not a CI gate

Measured 2026-09-20 with gremlins v0.6.0: two identical back-to-back runs of
`gremlins unleash ./internal/config/` returned `Killed: 2, efficacy 100.00%` and then
`Killed: 0, efficacy 0.00%`. Mutants land in `TIMED OUT` at random.

The suspected cause is that gremlins derives its per-mutant timeout from the baseline test run, and
this suite finishes in well under a second — faster than Go's compile-and-run overhead for the
mutated binary. Raising `--timeout-coefficient` made the problem worse rather than better, so the
mechanism is not fully understood.

Until that is resolved, `make mutate` never fails the build and CI does not run it. Run it locally
when changing `internal/version` or `internal/config`, and act on `LIVED` results only.

## Re-evaluating

Worth revisiting if the suite grows substantially slower (which may stabilise the timeout
calculation), or if gremlins ships a fix. Alternatives if it stays unusable:
[ooze](https://github.com/gtramontina/ooze), which runs as an ordinary Go test, or
[avito-tech/go-mutesting](https://github.com/avito-tech/go-mutesting).
```

- [ ] **Step 5: Verify the target cannot fail the build**

Run: `make mutate; echo "exit=$?"`
Expected: `exit=0`, regardless of what the mutation output reported.

- [ ] **Step 6: Commit**

```bash
git add Makefile docs/mutation-testing.md
```

```bash
git commit -m "$(cat <<'EOF'
docs: document mutation testing and keep it advisory

gremlins returns non-deterministic results on this repo - identical runs
swing between 100% and 0% efficacy as mutants randomly time out. The target
reports findings but cannot fail a build until that is understood.

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 5: Enforce the suite on pull requests

Nothing currently runs on a pull request — `release.yml` only fires on `v*` tags and runs no checks.
Tests that cannot be enforced are decoration.

**Files:**

- Create: `.github/workflows/ci.yml`
- Modify: `.github/workflows/release.yml:25`

**Interfaces:**

- Consumes: `gofmt`, `go vet`, `go test`. Deliberately **not** `make mutate` — see finding 4.
- Produces: a required-status-check candidate named `test`.

- [ ] **Step 1: Create the CI workflow**

Create `.github/workflows/ci.yml`:

```yaml
name: CI

on:
  pull_request:
  push:
    branches: [main]

permissions:
  contents: read

concurrency:
  group: ci-${{ github.ref }}
  cancel-in-progress: true

jobs:
  test:
    name: Test
    runs-on: ubuntu-latest

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
        run: go test ./...
```

Mutation testing is intentionally **not** in this workflow. It returned non-deterministic results
when measured (finding 4), so running it here would fail pull requests at random. `make mutate`
stays a local, advisory tool until that is fixed.

`go-version-file: go.mod` makes the toolchain track `go.mod` automatically, which removes the
version drift between `go.mod` and CI. The README's "Go 1.23+" claim stays stale — that is listed
under Out of scope.

The runner is `ubuntu-latest`, not `macos-latest`: the two tested packages are pure Go with no
platform-specific behaviour (`filepath.Join` produces the same `/`-separated results on Linux and
macOS), and Linux runners cost roughly a tenth of macOS minutes. Darwin build coverage already comes
from `release.yml`.

Note the formatting step does not call `make fmt` — `go fmt` **rewrites** files and would always
exit 0, so it cannot gate anything. `gofmt -l` reports without rewriting.

- [ ] **Step 2: Fix the pinned Go version in the release workflow**

In `.github/workflows/release.yml`, replace:

```yaml
with:
  go-version: "1.23"
```

with:

```yaml
with:
  go-version-file: go.mod
```

- [ ] **Step 3: Validate the workflow files parse**

Run: `gh workflow list` after pushing, or locally:
`python3 -c "import yaml,sys; yaml.safe_load(open('.github/workflows/ci.yml')); yaml.safe_load(open('.github/workflows/release.yml')); print('both parse OK')"`
Expected: `both parse OK`

- [ ] **Step 4: Commit**

```bash
git add .github/workflows/ci.yml .github/workflows/release.yml
git commit -m "$(cat <<'EOF'
ci: run format, vet and test checks on pull requests

Nothing ran on PRs before; release.yml only fires on tags. Both workflows now
derive the Go toolchain from go.mod instead of pinning a stale version.

Mutation testing is deliberately excluded: gremlins returns non-deterministic
results on this repo, so gating on it would fail PRs at random.

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>
EOF
)"
```

- [ ] **Step 5: Update the testing note in CLAUDE.md**

The "Testing" section currently claims there are zero tests. Replace it with:

```markdown
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
```

- [ ] **Step 6: Commit**

```bash
git add CLAUDE.md
git commit -m "$(cat <<'EOF'
docs: update testing guidance now that core packages are covered

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>
EOF
)"
```

---

## Out of scope

Deliberately excluded — file with `./scripts/ticket.sh` if wanted:

- Tests for `cmd/`, `internal/docker`, `internal/compose`, `internal/ssh`, `internal/registry`.
  These shell out to external binaries and need an injectable runner before they are testable. That
  refactor is a separate plan.
- The remaining known defects in `docs/architecture.md` (copyright placeholders, stale
  `Makefile VERSION`, README Go version, SSH host-key verification, unquoted remote paths).
- Diagnosing gremlins' `TIMED OUT` behaviour well enough to turn mutation testing into a CI gate.
  Worth a ticket: `./scripts/ticket.sh new bug "gremlins mutation results are non-deterministic" -r docs/mutation-testing.md:1`
