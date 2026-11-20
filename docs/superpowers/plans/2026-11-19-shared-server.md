# Shared-server support: implementation plan

Goal: several rocketctl projects on one server (the Lightsail move in
`docs/lightsail-shared-server.md`) with no hand-edited proxy, no wrong-CPU images and no cleanup
that touches other projects. Four small changes, each its own PR, in this order. None of them
blocks the migration, which works today with the runbook's workarounds. They remove the manual
steps and the ways the workarounds can go wrong.

| #   | Change                                     | Fixes                                                    | Size     |
| --- | ------------------------------------------ | -------------------------------------------------------- | -------- |
| 1   | `platform` in `rocket.yaml`                | arm64 images pushed from the Mac, found only at deploy   | ~30 LOC  |
| 2   | `--clean` acts on this project only        | deleting other projects' containers, images and networks | net −20  |
| 3   | `shared_proxy: true`: deploy owns the edge | hand-edited mount lines, no Caddy reload, `edge` setup   | ~120 LOC |
| 4   | `init` generates the shared layout         | every new project starts with Caddy on ports 80/443      | net −20  |

Not doing, on purpose: a `rocketctl edge` command (deploy sets up the edge), multi-arch
images, a check that the server's CPU matches `platform` (if it's set, it's almost always
right), choosing which proxy to use (Caddy only), or moving AutomatedHub to rocketctl.

---

## 1. `platform` in `rocket.yaml`

**Problem.** `docker.Build` runs `docker build` with no `--platform`, so the image matches the
machine that builds it. On the Apple Silicon Mac that's arm64. The server is x86_64, so the
containers fail with `exec format error`. The only safeguard today is remembering to
`export DOCKER_DEFAULT_PLATFORM`, and the bad image is already pushed under a real version tag
by the time you notice.

**Design.**

```yaml
# rocket.yaml
platform: linux/amd64 # optional; what the server runs. Omit to build for this machine.
```

- `config.Config` gets `Platform string \`yaml:"platform,omitempty"\``. `Validate`checks it
against`^[a-z0-9]+/[a-z0-9]+(/[a-z0-9]+)?$` (`os/arch[/variant]`) so a typo fails at load
  time and not halfway through a build.
- `docker.BuildOptions` gets `Platform string`. `Build` appends `--platform <value>` when it's
  set. The value is an exec argument, not shell text, so nothing needs quoting.
- `cmd/build.go` passes `cfg.Platform`. When it's empty, print one line:
  `⚠️  platform not set in rocket.yaml: building for this machine (<runtime.GOARCH>)`.
  That's the reminder that was missing.
- `cmd/up.go --prod` **does not** pass it. That command runs the image on this machine, so it
  should build for this machine.
- `init` writes `platform: linux/amd64`. Almost every server is x86_64. On an ARM server
  (Graviton, Raspberry Pi), edit the line. Known trade-off: the warning only fires when the
  field is empty, so a wrong value still fails late with `exec format error`. To keep that
  from going unnoticed, `init`'s next-steps output says: "platform is linux/amd64: change it if
  your server isn't x86_64 (check with `uname -m`)".

**Tests.** A table test in `config_test.go` with valid values (`linux/amd64`,
`linux/arm64/v8`, empty) and rejected ones (`amd64`, `linux/`, `Linux/AMD64`, `linux/amd64;x`).
Building the argument is a single `if`, so it doesn't need its own test.

**Check by hand.** `rocketctl build api` on the Mac with `platform: linux/amd64`, then
`docker image inspect --format '{{.Architecture}}' <image>` should print `amd64`. Do this in
both single-service and monorepo mode.

**Docs.** README (rocket.yaml reference and build section), `docs/architecture.md`, and the
Lightsail guide (drop the `export DOCKER_DEFAULT_PLATFORM` lines).

---

## 2. `--clean` acts on this project only

**Problem.** `remoteCleanupCommand` runs `docker container prune`, `docker image prune -a`,
`docker volume prune`, `docker network prune` and `docker system prune -a`. On a shared server
that deletes other projects' stopped containers, their rollback images, the `edge` network
whenever Caddy is stopped, and any anonymous volumes. The Docker version check
(`volumePruneIsSafe`) exists only to make one of those steps less dangerous.

**Design.** Clean the server the same way the code already cleans ECR and this machine:
remove this project's version tags older than the previous one, then the dangling layers.

```
for each service:
    docker images --format '{{.Repository}}:{{.Tag}}' --filter reference=<GetImageRepository>
    -> staleLocalImages(cfg, service, current, images)   # reused as-is
    -> docker rmi <each>                                  # warn on failure, don't stop
docker image prune -f                                     # dangling only: untagged and unused
```

- `docker image prune -f` without `-a` removes only untagged images that no container uses.
  Those are safe to delete whichever project they came from, and that includes the old
  `caddy:2-alpine` left behind after Caddy updates.
- Delete `remoteCleanupCommand`, `volumePruneIsSafe`, the `docker version` call, and their two
  tests. Nothing else uses them.
- Containers, networks and volumes are never touched. They take almost no disk space, and they
  are what keeps other projects alive and their data safe.
- `staleLocalImages` already checks the plain and the registry-prefixed name. Only the
  registry-prefixed one exists on the server, so it works unchanged. Rename it to
  `staleImages`, since it's no longer only about local images.
- List the server's images with `container.Query(client, …)`, with the reference wrapped in
  `ssh.ShellQuote`. `ssh.Client` already satisfies `container.Runner` (`cmd/migrate.go` passes
  it straight in), and `Query` keeps stderr out of the output that gets parsed.

Users with a single project lose the server-wide prune. On a deploy-only server the only things
it cleaned were stale tags (now handled) and dangling layers (still handled).

**Tests.** `TestStaleLocalImages` keeps covering the selection. A small test for the function
that builds the remote `docker images` command, checking that the reference is quoted.

**Check by hand.** Run two projects on one test server. Deploy project A with `--clean` and
confirm that `docker images` still lists every tag of project B, and that `docker network ls`
still shows `edge`.

**Docs.** The `--clean` help text in `cmd/deploy.go`, README, `docs/architecture.md` (deploy
flow step 8), `CLAUDE.md` ("it prunes the server" becomes "it removes this project's old
images"), and collision 2 of the Lightsail guide, which goes away.

---

## 3. `shared_proxy: true`: deploy manages the edge

**Problem.** In the runbook, the shared Caddy needs a mount line per project in
`~/apps/edge/docker-compose.yml`, a manual `caddy reload` after every Caddyfile change, and
careful ordering: project folders must exist before Caddy starts, and the `edge` network must
exist before any project starts. Every one of those manual steps can be forgotten.

**Design.** Opt in per project:

```yaml
# rocket.yaml
domain: cochinito.app
shared_proxy: true
```

Without the field, deploy behaves exactly as it does today. Existing projects don't change.

With it, deploy replaces step 5 ("upload Caddyfile") with:

1. **Make sure `~/apps/edge/` exists.** `MkdirAll ~/apps/edge/sites`, created as the SSH user
   so it isn't owned by root. Upload the embedded `edge/docker-compose.yml` and `edge/Caddyfile`
   **only if each is missing**, the same rule as `.env`. They're yours to edit after that
   (memory limit, global `email`), and deploy never overwrites them.
2. **Upload this project's Caddyfile** to `~/apps/edge/sites/<project>.caddy.new`.
3. **Install it and reload Caddy in one remote command**, restoring the old file if Caddy
   rejects the new one:

   ```sh
   cd ~/apps/edge &&
   docker compose up -d &&
   { [ -f sites/P.caddy ] && cp sites/P.caddy sites/P.caddy.bak || rm -f sites/P.caddy.bak; } &&
   mv sites/P.caddy.new sites/P.caddy &&
   { docker exec edge-caddy caddy reload --config /etc/caddy/Caddyfile ||
     { if [ -f sites/P.caddy.bak ]; then mv sites/P.caddy.bak sites/P.caddy; else rm -f sites/P.caddy; fi; exit 1; }; }
   ```

   The rule: a new site file is installed only when a running Caddy can check it right away,
   and it's taken back out if Caddy rejects it. Without that, a bad site block (for example a
   stray `{ email }` global block) stays on disk, and the next time Caddy restarts **every**
   site on the server goes down, not only this project's. So `docker compose up -d` runs
   **before** the `mv`. If it fails (ports 80/443 still held by an old per-project Caddy, or a
   broken hand-edited edge compose file), the command stops with `sites/` unchanged. The
   leftover `P.caddy.new` doesn't match the `*.caddy` import, and the next deploy overwrites it.
   `up -d` is also what creates the `edge` network and starts Caddy the first time. On a fresh
   server `sites/` is still empty at that point, which Caddy accepts: an `import` glob that
   matches nothing is a warning, not an error. When Caddy is already running, `up -d` does
   nothing.

4. Then the existing steps run as before: `.env`, ECR login, `pull`, `up -d`. The project's
   `up -d` now always finds the `edge` network.

Build that command in a function, `edgeInstallCommand(project string) string`, quoting the
project with `ssh.ShellQuote` and keeping `~` outside the quotes. Like `versionEnvAssignments`,
it's testable without a server.

**Embedded edge files** (new, in `internal/templates/`):

```yaml
# edge/docker-compose.yml
name: edge
networks:
  edge:
    name: edge
services:
  caddy:
    image: caddy:2-alpine
    container_name: edge-caddy
    restart: unless-stopped
    ports: ["80:80", "443:443"]
    volumes:
      - ./Caddyfile:/etc/caddy/Caddyfile:ro
      - ./sites:/etc/caddy/sites:ro # only Caddyfiles, never a project's .env
      - caddy_data:/data # certificates; a named volume, safe from any prune
      - caddy_config:/config
    networks: [edge]
    mem_limit: 128m
    logging:
      driver: json-file
      options: { max-size: "10m", max-file: "3" }
volumes:
  caddy_data:
  caddy_config:
```

```
# edge/Caddyfile
# Global options go here, once, e.g.:
# {
# 	email you@example.com
# }
# Each project's site blocks: sites/<project>.caddy, written by rocketctl deploy.
import /etc/caddy/sites/*.caddy
```

A single `./sites` mount replaces the per-project mount lines. Adding a project means editing
nothing on the server. Caddy doesn't require an email for Let's Encrypt, so the edge works
without any setup. The commented block shows where to add one.

**Validation** in `config.Validate`:

- `shared_proxy: true` without a `domain` → error. The shared proxy exists only to serve the
  domain.
- `shared_proxy: true` with `project: edge` → error ("the name `edge` is reserved for the shared
  proxy"), because the two would share `~/apps/edge`.

When `shared_proxy` is set, deploy fails before connecting in two cases where the site would
silently not be served:

- `caddy/Caddyfile` is missing. Today this is only a warning. The error names the file.
- No service in `docker-compose.prod.yml` joins the `edge` network, so Caddy can't reach
  anything and every request returns 502. The error says to add `- edge` under `networks:` of
  the service Caddy routes to. Check this with a new `compose.UsesNetwork(content, "edge")` in
  `internal/compose`. It reuses the `yaml.Node` walk and `mappingValue` from `pin.go`, and is
  true when any service's `networks` contains `edge`, written as a list item or as a mapping
  key. A `strings.Contains` check isn't enough, because change 4's template always declares
  `edge: { external: true }` and has `#   - edge` in a comment. Contains would pass for exactly
  the file where you forgot to uncomment the line. Known trade-off: a site with no upstream
  container on this server (a `redir`-only site, or `reverse_proxy` to an outside host) also
  trips this error. The fix is to join any one service to `edge`. Nearly every time, the error
  means the line was never uncommented, so it stays a hard error rather than a warning.

**Projects not deployed with rocketctl (AutomatedHub).** They follow the same rule by hand:
`scp caddy/Caddyfile admin@<ip>:~/apps/edge/sites/automatedhub.caddy`, then run the
`caddy reload` command. They no longer edit the edge compose file.

**Tests.** Table tests for the two new `Validate` rules, and a test that
`edgeInstallCommand("my-app")` quotes the name and leaves `~` unquoted. A table test for
`compose.UsesNetwork`: list form, mapping form, network declared but no service joins it,
`- edge` only in a comment. The embedded edge compose file gets a test that checks it parses as
YAML.

**Check by hand** on a test server:

- First deploy of project A creates `~/apps/edge`, starts `edge-caddy` and serves A.
- First deploy of project B adds `sites/B.caddy`, serves B, and doesn't restart A or Caddy (no
  dropped connections).
- Change A's Caddyfile and deploy: the change is live without any manual step.
- Deploy A with a broken Caddyfile: deploy fails, A's old site file is back, and
  `docker restart edge-caddy` still serves both sites.
- Keep an old per-project Caddy on port 80 and deploy B: deploy fails at the edge `up -d`, and
  `sites/` doesn't change.
- Run all of this in single-service mode and in monorepo mode.

**Docs.** README (a "Several projects on one server" section with the two `rocket.yaml` lines,
plus the `rocket.yaml` reference), `docs/architecture.md` deploy flow, and the Lightsail guide:
step 3 shrinks to "nothing to do, the first deploy creates it", and rule 3 becomes "drop
`<project>.caddy` into `~/apps/edge/sites/`".

---

## 4. `init` generates the shared layout

**Problem.** `init` with a domain generates a `caddy` service on ports 80/443 and a Caddyfile
with a global `{ email }` block. Both break on a shared server and have to be deleted by hand.

**Design.** Templates are generated once, so changing them affects only new projects.

- `docker-compose.prod.yml.tmpl`: remove the `caddy` service and the `caddy_data` and
  `caddy_config` volumes. When a domain is set, declare `edge: { external: true }` and add a
  comment on each service: `# Join "edge" only if Caddy routes to this service:` followed by
  `#   - edge`. Each project's API stays off the shared network unless you choose otherwise.
- `Caddyfile.tmpl`: remove the `{ email }` block. Site blocks only.
- `cmd/init.go`: stop asking for the email (one prompt and the `Email` template field go
  away). When a domain is given, write `shared_proxy: true`, and always write
  `platform: linux/amd64` (change 1).
- `cmd/up.go --prod`: before `compose.Up`, run `docker network create edge` and ignore the
  "already exists" error. Without it, running the production file on your own machine fails
  with "network edge declared as external, but could not be found". One `exec.Command`, run
  only when `compose.UsesNetwork(content, "edge")` (change 3) is true.

A project generated before this change keeps its own Caddy and keeps deploying exactly as
before.

**Tests.** Extend `templates_test.go`: with a domain, the rendered compose file has no `caddy`
service and no port 80. The rendered Caddyfile has no `email`.

**Docs.** README (`init` section, and the "Restart Caddy" snippet becomes the edge `caddy
reload` command).

---

## Fixed in `docs/lightsail-shared-server.md` (independent of the above)

Two statements in the guide were wrong according to the code. Both are now corrected in the
guide:

- **Step 4 says `rocketctl build api --push` builds the "current version, no bump".** It
  doesn't. `--bump` defaults to `patch` (`cmd/build.go:42`), so every build is a new version.
  Also, the images in ECR today already run on the t3a (x86_64) instances, so they're amd64.
  The first Lightsail deploy can run `rocketctl deploy` with no build at all.
- **The guide cites `internal/jobs` and scheduled jobs in three places**, and neither that
  package nor the `rocketctl.scheduler` label exists in this repo: the intro sentence of "What
  rocketctl already keeps apart" (`:68`), the "Scheduled jobs" table row (`:78`), and rule 4
  ("`migrate` and jobs depend on it", `:534`). Remove all three.

## Order and release

Ship 1 and 2 first. They're independent, small, and make the migration safer right away. Then
ship 3, and 4 on top of it. Each PR updates its own docs and `CHANGELOG.md`, and passes
`make check` plus `gofmt -l .` with no output. After 3 lands, migrate CochinitoApp with
`shared_proxy: true`, and simplify the Lightsail guide to match.
