# Checking the shared-server features by hand

`platform`, the project-only `--clean`, `shared_proxy` and the new `init` layout shell out to
docker and ssh, so unit tests don't cover them. Run these checks on a test server before relying
on them, and again after changing `cmd/build.go`, `cmd/deploy.go`, `cmd/init.go` or `cmd/up.go`.
Run each check in **both** a single-service project and a monorepo.

Setup: one test server with Docker, and two throwaway projects, A and B, each with a `domain`
pointing at the server and `shared_proxy: true` in `rocket.yaml`.

## 1. `platform`

| Do                                                             | Expect                                                                     |
| -------------------------------------------------------------- | -------------------------------------------------------------------------- |
| `platform: linux/amd64`, then `rocketctl build api` on the Mac | `docker image inspect --format '{{.Architecture}}' <image>` prints `amd64` |
| Remove `platform`, build again                                 | The line `⚠️  platform not set in rocket.yaml: building for this machine`  |
| `platform: amd64`, run any command                             | `invalid platform "amd64" in rocket.yaml` before anything runs             |
| `rocketctl up --prod` with `platform: linux/amd64` on the Mac  | Builds for the Mac (arm64), on purpose: it runs the image locally          |

## 2. `--clean` touches only this project

1. Deploy A and B, then build and deploy A three more times so it has old tags.
2. `rocketctl deploy --clean` in A.

Expect, on the server:

- `docker images` lists only A's current and previous tags, and **every** tag of B.
- `docker network ls` still shows `edge`; `docker ps -a` and `docker volume ls` are unchanged.
- The deploy output shows `removed <old A tag>` lines and ends with `✅ Cleanup complete`.

## 3. `shared_proxy`

| Do                                                            | Expect                                                                                                                                    |
| ------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------- |
| First deploy of A on a clean server                           | `~/apps/edge` exists, `edge-caddy` is running, A's domain serves over HTTPS                                                               |
| First deploy of B                                             | `~/apps/edge/sites/B.caddy` exists, B serves, `docker ps` shows `edge-caddy` not restarted                                                |
| Change A's Caddyfile, deploy                                  | The change is live with no manual step                                                                                                    |
| Put a syntax error in A's Caddyfile, deploy                   | Deploy fails with `shared proxy update failed`; `sites/A.caddy` is the old file; after `docker restart edge-caddy` both sites still serve |
| Edit `~/apps/edge/Caddyfile` (e.g. add `email`), deploy again | Your edit is still there                                                                                                                  |
| Keep an old per-project Caddy on port 80, deploy B            | Deploy fails at `docker compose up -d` in `~/apps/edge`; `sites/` unchanged                                                               |
| Delete `caddy/Caddyfile`, deploy                              | Fails before connecting: `caddy/Caddyfile is missing`                                                                                     |
| Comment out `- edge` on every service, deploy                 | Fails before connecting: `no service in docker-compose.prod.yml joins the 'edge' network`                                                 |
| `project: edge` with `shared_proxy: true`                     | `the name 'edge' is reserved` on any command                                                                                              |

## 4. `init`

In an empty folder, `rocketctl init` with a domain. Expect: no email prompt; `rocket.yaml` has
`shared_proxy: true` and `platform: linux/amd64`; `docker-compose.prod.yml` has no `caddy` service
and no port 80, declares `edge: external: true`, and has a commented `#   - edge` on each service;
`caddy/Caddyfile` has no `{ email }` block. Uncomment `- edge` on one service and
`rocketctl up --prod` starts without "network edge declared as external, but could not be found".

## When a check fails

Open a bug with the check that failed, the exact command, and the output:

```bash
./scripts/ticket.sh find "shared proxy"        # check it isn't already reported
./scripts/ticket.sh new bug "deploy: <what went wrong>" -d "<check name>: expected X, got Y" -r cmd/deploy.go
```

Include in the issue: single-service or monorepo, `docker version` on the server, the full
deploy output, and for proxy failures `docker logs edge-caddy --tail 50` and
`ls -la ~/apps/edge/sites`.
