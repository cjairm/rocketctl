# Several projects on one Lightsail server with rocketctl

A runbook for moving CochinitoApp and AutomatedHub off two EC2 instances and onto one Lightsail
server, and for adding more projects to that server later without them getting in each other's way.

> This repo is public. This guide uses placeholders (`<STATIC_IP>`, `<REGISTRY>`, `<you@example.com>`)
> on purpose. Keep real IPs, account IDs and emails in each project's own `rocket.yaml`.

## Why move

October 2026 bill for the two EC2 instances, matched line by line against AWS's prices (744 hours
in the month):

| Line        | Cost       | What it is                                                             |
| ----------- | ---------- | ---------------------------------------------------------------------- |
| EC2 compute | $10.49     | t3a.micro ($0.0094/h) + t3a.nano ($0.0047/h)                           |
| Amazon VPC  | $11.16     | 3 public IPv4 addresses × $0.005/h. One is probably an idle Elastic IP |
| EC2 - Other | $3.04      | EBS disks (about 38 GB of gp3)                                         |
| ECR         | $0.07      |                                                                        |
| **Total**   | **$24.76** |                                                                        |

On Lightsail one plan is a flat price that already includes the disk, the public IPv4 address and
data transfer:

| Plan  | RAM  | vCPU | Disk  | Transfer | Price  |
| ----- | ---- | ---- | ----- | -------- | ------ |
| Small | 2 GB | 2    | 60 GB | 3 TB     | $12/mo |
| Next  | 4 GB | 2    | 80 GB | 4 TB     | $24/mo |

Both apps on the $12 plan cost half of today's bill and get more RAM than the two instances have
together (1.5 GB).

**What Lightsail is:** the simple version of EC2. The server underneath is EC2 hardware, but
instead of paying separately for the server, disk, IP and traffic, and setting up VPCs and security
groups yourself, you pick a plan and get all of it for one price, with a firewall of plain checkboxes.
What you give up: no instance IAM roles, burstable CPU only (same as the t3a instances today), and a
small set of plan sizes.

## How it will look

```
                    Internet
                       │
          ┌────────────┼──────────────────────────────┐
          │  Lightsail server (Debian, Docker)        │
          │            │                              │
          │   :80/:443 ▼                   :1883      │
          │   ┌──────────────┐               │        │
          │   │  edge-caddy  │  ~/apps/edge  │        │
          │   └──────┬───────┘               │        │
          │          │  docker network "edge"│        │
          │    ┌─────┴──────────┬────────────┼─────┐  │
          │    ▼                ▼            ▼     ▼  │
          │ cochinitoapp-web  automatedhub-web -api -mqtt
          │    │ (~/apps/cochinitoapp)  (~/apps/automatedhub)
          │    ▼ network "cochinitoapp-network"       │
          │ cochinitoapp-api   (never reachable from edge)
          └───────────────────────────────────────────┘
```

Each project lives in its own folder and its own Compose project. One shared Caddy (the reverse
proxy that holds the HTTPS certificates) owns ports 80 and 443 and routes each domain to the right
container over a shared Docker network called `edge`. Only the containers Caddy needs to reach join
`edge`. Everything else stays on the project's private network.

## What rocketctl already keeps apart

I read `cmd/deploy.go`, `internal/container` and the `init` templates. Per project,
rocketctl gives you:

| What            | How                                                                                                                                | Where                          |
| --------------- | ---------------------------------------------------------------------------------------------------------------------------------- | ------------------------------ |
| Folder          | Every deploy goes to `~/apps/<project>/`                                                                                           | `cmd/deploy.go`                |
| Compose project | `docker compose` runs inside that folder, so the project name is `<project>`. Networks and volumes get a `<project>_` prefix       | `cmd/deploy.go`                |
| Container names | The template names them `<project>-<service>`                                                                                      | `docker-compose.prod.yml.tmpl` |
| Secrets         | One `.env` per service, mode 600, uploaded once and never overwritten                                                              | `cmd/deploy.go`                |
| Migrations      | Finds the container by the label `com.docker.compose.service=<project>-<service>`. Unique as long as names keep the project prefix | `internal/container`           |
| Versions        | Each service's tag comes from its own `.rocket-version`                                                                            | `cmd/deploy.go`                |

Deploying CochinitoApp never restarts AutomatedHub's containers.

## What collides, and the fix for each

**1. Every project brings its own Caddy on ports 80 and 443.** The `init` template, CochinitoApp's
`docker-compose.prod.yml` and AutomatedHub's `docker-compose.yml` each define a `caddy` service that
publishes `80:80` and `443:443`. Whichever project starts second fails with "port is already
allocated".
_Fix:_ remove the `caddy` service from every project and run one shared Caddy in `~/apps/edge`
(step 3). Each project keeps its own `caddy/Caddyfile` with only its own site blocks, and rocketctl
keeps uploading it as it does today. The shared Caddy reads all of them with `import`.

**3. AutomatedHub isn't deployed with rocketctl.** Its own CLI (`automatedhub_cli`) builds images
and pushes them to ECR, and the deploy is done by hand on the server from `~/apps` (not
`~/apps/automatedhub`), with the old `docker-compose` v1.
_Fix:_ on the new server, give it `~/apps/automatedhub/` and follow the same rules as every other
project (step 5). Moving it to rocketctl is a separate decision (see "Not covered").

**4. Image CPU type.** rocketctl builds without `--platform`, so Docker builds for the machine
running the build. This Mac is Apple Silicon (`arm64`). Lightsail servers run x86_64 (check with
`uname -m` on the server). An `arm64` image on an x86_64 server fails with `exec format error`.
_Fix:_ set `platform: linux/amd64` in `rocket.yaml`. Check what runs today with
`docker image inspect --format '{{.Architecture}}' <image>` on the current servers.

**5. AutomatedHub devices connect to the broker by IP address.** The firmware examples hardcode the
current server's IP in `automatedhub_examples/*/config.cpp` (`MQTT_SERVER = "…"`). An Elastic IP
can't be moved to Lightsail, so every device flashed with that IP loses its connection at cutover.
_Fix:_ step 7.

**6. ECR login on the server needs AWS keys.** `rocketctl deploy` runs
`aws ecr get-login-password` on the server. On EC2 that can use an instance role, but Lightsail
instances can't have one.
_Fix:_ an IAM user that can only pull images (step 2).

## Step 0 — Check these before starting

1. **Where the MySQL databases run.** Neither production Compose file includes a database.
   CochinitoApp's `docs/DEPLOYMENT.md` says "Database: External (not on this instance)", and the
   bill has no RDS. On each old server:
   ```bash
   grep -h MYSQL_HOST ~/apps/*/.env ~/apps/*/*/.env ~/apps/*/*.env 2>/dev/null
   docker ps --format '{{.Names}}\t{{.Image}}'
   sudo ss -tlnp | grep 3306        # MySQL installed directly on the server?
   ```
   The answer decides which path you take in step 6.
2. **Memory used today:** `docker stats --no-stream` and `free -h` on both servers. Add the numbers
   up to choose between the 2 GB and 4 GB plan.
3. **Image CPU type** (collision 4).
4. **Which devices use the hardcoded broker IP** (collision 5).
5. **The SSH key path in each `rocket.yaml`** must point to a key that exists on the machine you
   deploy from. You'll change it in step 4 anyway.

## Step 1 — Create the server

In the Lightsail console:

1. **Create instance** → region **Ohio (us-east-2)**, the same region as ECR, so image pulls stay
   close.
2. Platform **Linux/Unix** → Blueprint **OS Only** → the newest **Debian**. Debian's default user
   is `admin`, the same user CochinitoApp's `rocket.yaml` uses today.
3. SSH key: upload `~/.ssh/id_ed25519.pub` (create it with `ssh-keygen -t ed25519` if it doesn't
   exist). Or use the region's default key, which you can download later from Account → SSH keys.
4. Plan: the **$12, 2 GB** plan **with public IPv4**. Skip the IPv6-only plans: some users and
   networks can't reach IPv6-only servers.
5. Name it, for example `apps-1`, and create it.
6. **Networking → Create static IP** → attach it to the instance. This IP stays the same when you
   upgrade or replace the server. It's free while attached, so don't leave one unattached.
7. **Networking → IPv4 firewall.** Debian starts with SSH 22 and HTTP 80 open. Add:
   - **HTTPS** TCP 443
   - **Custom** TCP 1883 (AutomatedHub's MQTT, for the devices)
   - Optional: limit SSH 22 to your home IP. If your IP changes, deploys fail until you update the rule.
8. **Snapshots → Enable automatic snapshots.** It takes one daily and keeps the last 7. Storage
   costs $0.05 per GB-month, so check the bill after the first week.

Test the connection:

```bash
ssh admin@<STATIC_IP>
uname -m        # expect x86_64
```

## Step 2 — Base setup (once, on the server)

```bash
# Updates, and automatic security updates from now on
sudo apt update && sudo apt -y upgrade
sudo apt install -y unattended-upgrades unzip

# Docker Engine plus the Compose v2 plugin (rocketctl uses `docker compose`, not `docker-compose`)
curl -fsSL https://get.docker.com | sudo sh
sudo usermod -aG docker admin
exit                       # log out and back in so the docker group applies
ssh admin@<STATIC_IP>
docker compose version     # must print v2.x

# 2 GB of swap: keeps a memory spike from killing containers. It isn't a substitute for RAM.
sudo fallocate -l 2G /swapfile && sudo chmod 600 /swapfile
sudo mkswap /swapfile && sudo swapon /swapfile
echo '/swapfile none swap sw 0 0' | sudo tee -a /etc/fstab

# AWS CLI v2, for `aws ecr get-login-password` during rocketctl deploy
curl -fsSL "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o awscliv2.zip
unzip -q awscliv2.zip && sudo ./aws/install && rm -rf aws awscliv2.zip
```

**ECR credentials.** In the IAM console, create a user called `lightsail-ecr-pull` with only the AWS
managed policy **AmazonEC2ContainerRegistryReadOnly**, and create an access key for it. Then on the
server:

```bash
aws configure          # that key, region us-east-2, output json
aws ecr get-login-password --region us-east-2 >/dev/null && echo "ECR OK"
```

Run this as `admin`, because rocketctl runs its commands as the SSH user. The key can only read
images. If the server is compromised, the attacker can download your images but can't push or
delete anything. Rotate the key once a year.

**Folders.** Create every project folder **before** the shared Caddy starts. Otherwise Docker
creates the folders as `root`, and rocketctl, connecting as `admin`, can't write into them:

```bash
mkdir -p ~/apps/edge ~/apps/cochinitoapp/caddy ~/apps/automatedhub/caddy
```

## Step 3 — The shared Caddy (`~/apps/edge`)

**For rocketctl projects there is nothing to do:** set `shared_proxy: true` in `rocket.yaml` and the
first deploy creates `~/apps/edge`, starts Caddy and installs the project's site file in
`~/apps/edge/sites/`. The manual setup below is only for projects not deployed with rocketctl
(AutomatedHub) or if you prefer to do it by hand; its mount-per-project layout is replaced by the
single `./sites` mount that rocketctl's embedded edge compose file uses.

`~/apps/edge/docker-compose.yml`:

```yaml
name: edge

networks:
  edge:
    name: edge # a fixed name, so other projects can join it as an external network

services:
  caddy:
    image: caddy:2-alpine
    container_name: edge-caddy
    restart: unless-stopped
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./Caddyfile:/etc/caddy/Caddyfile:ro
      # One line per project: Caddy sees only each project's caddy/ folder, never its .env
      - ../cochinitoapp/caddy:/sites/cochinitoapp:ro
      - ../automatedhub/caddy:/sites/automatedhub:ro
      - caddy_data:/data # certificates. Named, so --clean never deletes it
      - caddy_config:/config
    networks:
      - edge
    mem_limit: 128m
    logging:
      driver: json-file
      options: { max-size: "10m", max-file: "3" }

volumes:
  caddy_data:
  caddy_config:
```

`~/apps/edge/Caddyfile`:

```
{
	email <you@example.com>
}

import /sites/*/Caddyfile
```

The global options block (`{ email … }`) can only appear once, at the top of the main Caddyfile.
That's why project Caddyfiles must contain **site blocks only**.

Start it **after** the projects have uploaded their Caddyfiles (steps 4 and 5):

```bash
cd ~/apps/edge && docker compose up -d && docker compose logs -f
```

When a project's `caddy/Caddyfile` changes, rocketctl uploads it, but nothing tells Caddy. Reload it
by hand. A reload doesn't drop connections:

```bash
docker exec edge-caddy caddy reload --config /etc/caddy/Caddyfile
```

## Step 4 — CochinitoApp (deployed with rocketctl)

**`docker-compose.prod.yml`:** delete the `caddy` service and the `caddy_data`/`caddy_config`
volumes, and put the web container on `edge`. The API stays on the private network only, as it is
today ("API is not exposed").

```yaml
networks:
  cochinitoapp-network:
    driver: bridge
  edge:
    external: true # created by ~/apps/edge

services:
  cochinitoapp-api:
    # unchanged: networks: [cochinitoapp-network] only
  cochinitoapp-web:
    # ...unchanged...
    networks:
      - cochinitoapp-network
      - edge
```

**`caddy/Caddyfile`:** remove the `{ email … }` block, which now lives in the edge Caddyfile, and
keep only the site:

```
cochinito.app {
	reverse_proxy cochinitoapp-web:3000
}
```

**`rocket.yaml`:** set `ip: <STATIC_IP>` and `ssh_user: admin`. Set `ssh_key_path` to the key from
step 1, or remove the line if you uploaded `~/.ssh/id_ed25519`, which rocketctl finds on its own.
Keep `domain` set, because rocketctl only uploads the Caddyfile when it is.

**Deploy.** The images already in ECR run on the t3a (x86_64) instances today, so they are amd64
and the first deploy needs no build. Check with
`docker image inspect --format '{{.Architecture}}' <image>` on the old server. If you do build,
every `rocketctl build` bumps the version (`--bump` defaults to `patch`), and on this Mac it
needs `platform: linux/amd64` in `rocket.yaml`:

```bash
# Only if you need a new build:
rocketctl build api --push                    # bumps the patch version
rocketctl build web --push

rocketctl deploy
```

The first connection asks you to trust the new server's host key. Answer yes. The first deploy
uploads `api/.env.example` and `web/.env.example` as `.env`. Replace them with the real values
before the app is used:

```bash
ssh admin@<STATIC_IP>
nano ~/apps/cochinitoapp/api/.env     # copy the values from the old server's file
nano ~/apps/cochinitoapp/web/.env
cd ~/apps/cochinitoapp && docker compose up -d
```

Running `docker compose up -d` by hand also works for later restarts: the tags are already pinned
in the uploaded `docker-compose.yml`.

## Step 5 — AutomatedHub (deployed by hand, same server rules)

On the server it gets its own folder and follows the same rules as a rocketctl project:

```
~/apps/automatedhub/
  docker-compose.yml          # from automatedhub_cli/docker-compose.yml, edited as below
  caddy/Caddyfile             # site blocks only
  automatedhub_web/.env       # the automatedhub_cli/automatedhub_web/.production.env values
  automatedhub_api/.env       #   same for api
  automatedhub_mqtt/.env      #   same for mqtt   (chmod 600 all three)
```

Changes to `automatedhub_cli/docker-compose.yml`:

- Delete the `caddy` service and its volumes.
- Remove the obsolete `version: "3.9"` line. Compose v2 ignores it and prints a warning.
- Put web, api and mqtt on both `default` (their private network) and `edge`. If a service lists
  any network, it leaves `default` unless `default` is listed too.
- Keep `1883:1883` published. No other project may use that port.
- Add memory limits. The numbers below are estimates; adjust them after looking at `docker stats`.

```yaml
networks:
  edge:
    external: true

services:
  automatedhub-web: # nginx serving the React build
    # ...image, env_file, restart, logging unchanged...
    networks: [default, edge]
    mem_limit: 64m
  automatedhub-api: # Django + gunicorn
    networks: [default, edge]
    mem_limit: 320m
  automatedhub-mqtt: # Go broker
    ports: ["1883:1883"]
    networks: [default, edge]
    mem_limit: 128m
```

`caddy/Caddyfile` (the current one, without the `{ email }` block):

```
automatedhub.io {
	reverse_proxy automatedhub-web:80
}

api.automatedhub.io {
	reverse_proxy automatedhub-api:8000
}

ws.automatedhub.io {
	reverse_proxy automatedhub-mqtt:9001
}
```

Deploy from `automatedhub_cli/` (build and push with its CLI first, as today):

```bash
scp docker-compose.yml admin@<STATIC_IP>:~/apps/automatedhub/
scp caddy/Caddyfile    admin@<STATIC_IP>:~/apps/automatedhub/caddy/
ssh admin@<STATIC_IP> 'cd ~/apps/automatedhub \
  && aws ecr get-login-password --region us-east-2 | docker login --username AWS --password-stdin <REGISTRY> \
  && docker compose pull && docker compose up -d'
```

Copy the three `.env` files once, by hand, with `chmod 600`.

Now start the edge Caddy (step 3).

## Step 6 — The database

Use the answer from step 0. **Confirmed: both databases are hosted outside AWS, so path A applies.**

**A. A hosted database outside these servers.** Nothing moves. If it only accepts connections from
certain IPs, add `<STATIC_IP>` to its allowlist before cutover, and remove the old IPs afterwards.

**B. MySQL running on one of the old EC2 servers.** Move it into its own project on the new server,
shared by both apps, with a separate database and user per app:

```yaml
# ~/apps/mysql/docker-compose.yml
name: mysql
networks:
  db:
    name: db
services:
  mysql:
    image: mysql:8.4
    container_name: mysql
    restart: unless-stopped
    env_file: .env # MYSQL_ROOT_PASSWORD, chmod 600
    command: --innodb-buffer-pool-size=128M --performance-schema=OFF
    ports:
      - "127.0.0.1:3306:3306" # this server only: for SSH tunnels, never the internet
    volumes:
      - mysql_data:/var/lib/mysql # named: --clean never deletes it
    networks: [db]
    mem_limit: 512m
volumes:
  mysql_data:
```

- Each app's API service joins `db` as an external network (the same pattern as `edge`), and its
  `MYSQL_HOST` becomes `mysql`. The web containers never join `db`.
- The two `command` flags cut MySQL's memory from about 400 MB to about 200 MB, which fits small apps.
- Moving the data: stop the apps on the old server so nothing writes, run
  `mysqldump --single-transaction --routines --databases <db>`, copy the file over, and load it
  with `docker exec -i mysql mysql -uroot -p < dump.sql`.
- **`rocketctl backup` needs a change in this setup.** It runs on your Mac and connects straight to
  the production database. With MySQL closed to the internet, open a tunnel first,
  `ssh -L 13306:127.0.0.1:3306 admin@<STATIC_IP>`, and point `api/.env.backup` at
  `host.docker.internal:13306`. The backup runs in a container on the Mac, so `localhost` would mean
  the container itself.
- With MySQL on the server, the 2 GB plan gets tight: about 0.5 GB per app, 0.2 GB for MySQL, plus
  the OS. Check `free -h` after a day, and move up to 4 GB if swap stays in use.

## Step 7 — Switching over

1. **A day before:** lower the TTL (how long others cache the record) of every DNS record you will
   change to 300 seconds, so the switch spreads in minutes.
2. **AutomatedHub devices (collision 5).** Before switching, give the broker a hostname:
   - Add a DNS A record `mqtt.automatedhub.io → <STATIC_IP>`.
   - Change `MQTT_SERVER` in `automatedhub_examples/*/config.cpp` to `"mqtt.automatedhub.io"`, and
     reflash your devices. After this, a future move only needs a DNS change.
   - If devices you can't reflash are using the old IP, keep the old t3a.nano running for a few
     weeks as a relay that forwards 1883 to the new server:
     `docker run -d --restart unless-stopped -p 1883:1883 alpine/socat TCP-LISTEN:1883,fork,reuseaddr TCP:<STATIC_IP>:1883`.
     It costs about $7/month while it runs. Also check the dashboard's
     `REACT_APP_MQTT_SERVER_HOST`: if it holds an IP, the web build shows that IP to users.
3. **Database:** follow path A or B from step 6.
4. **Switch DNS:** point these A records at `<STATIC_IP>`: `cochinito.app`, `automatedhub.io`,
   `api.automatedhub.io`, `ws.automatedhub.io`, `mqtt.automatedhub.io`.
5. **Watch Caddy get the certificates.** It requests them as soon as DNS points at the new server,
   usually within a minute: `docker logs -f edge-caddy`.
6. **Check:** open each site, log in, use a device, and run `rocketctl migrate api` (a dry run) to
   confirm rocketctl finds the container.
7. **Undo plan:** keep the old servers running and unchanged for 3–7 days. If something is wrong,
   point DNS back. With path B, anything written on the new server after the switch would have to be
   copied back, so decide quickly.

## Step 8 — Shut down the old setup

After a few quiet days:

1. Take a final snapshot of each EC2 disk if you want a safety copy (EC2 → Volumes → Create
   snapshot). Delete it after a month.
2. **Terminate** both instances.
3. **EC2 → Elastic IPs → select → Actions → Release Elastic IP addresses.** "Release" is AWS's word
   for delete. Check every region you have used.
4. **EC2 → Volumes:** delete any `available` volumes, meaning disks not attached to anything.
5. **VPC → IP Address Manager → Public IP insights:** confirm no public IPv4 addresses are left.
   Also check VPC → Endpoints.
6. Leave the default VPC alone. It's free.
7. Next month, Cost Explorer grouped by service should show only Lightsail (about $12), ECR (cents),
   and snapshots.

## Day-to-day

**Deploying CochinitoApp:** the same as `docs/DEPLOYMENT.md` in that repo, with
`platform: linux/amd64` in `rocket.yaml`:

```bash
rocketctl build api --bump minor --push
rocketctl deploy            # --clean only touches this project's images
rocketctl migrate api && rocketctl migrate api --apply
```

**Deploying AutomatedHub:** build and push with its CLI, then the `scp` + `ssh` commands from step 5.

**Memory:** `ssh admin@<STATIC_IP> 'docker stats --no-stream; free -h'`.

**Moving to a bigger plan** (about 15–30 minutes, a few minutes of downtime): Snapshots → Create
snapshot → **Create new instance** from it on the bigger plan → move the static IP to the new
instance → check the sites → delete the old instance. If SSH warns that the host key changed, run
`ssh-keygen -R <STATIC_IP>`. **Going down a size isn't possible from a snapshot**, because a
snapshot can't go onto a smaller disk. To shrink, build a fresh server with steps 1–5 and restore
the data.

## Rules for every project on this server

Adding a project means following these rules. They're what keeps projects out of each other's way:

1. It lives in `~/apps/<project>/`. rocketctl does this automatically.
2. It never publishes ports 80 or 443. Only the services Caddy routes to join the external `edge`
   network.
3. Its `caddy/Caddyfile` contains site blocks only. Drop it into `~/apps/edge/sites/<project>.caddy`
   and reload Caddy (rocketctl projects with `shared_proxy: true` do this on every deploy).
4. Container and Compose service names start with `<project>-`. The rocketctl template does this,
   and `migrate` depends on it.
5. Every service has a `mem_limit`, so one project's leak can't take down the others.
6. Data lives in **named** volumes, so `--clean` can't delete it.
7. Any other host port (like 1883) is unique on the server and opened in the Lightsail firewall.
8. Its secrets live in its own `.env` files, mode 600. Each project uses its own credentials, never
   another project's.

`rocketctl init` generates the shared layout: no `caddy` service, `shared_proxy: true`, and a commented `- edge` line to uncomment on the service Caddy routes to.

## Not covered, and rocketctl changes worth considering

These are gaps found while writing this guide. Nothing here has been built:

- **Moving AutomatedHub to rocketctl.** Its image and service names already match rocketctl's
  scheme (`automatedhub_<service>` images, `automatedhub-<service>` services). What doesn't fit:
  rocketctl expects `<service>/Dockerfile.production` and `<service>/.rocket-version` next to
  `rocket.yaml`, but AutomatedHub keeps each service in its own repository (`automatedhub_api/`, …)
  with versions in `automatedhub_cli/automatedhub_<service>/.version`. Using rocketctl only for
  deploys would mean keeping two version files in sync by hand.

## Sources

- AWS Lightsail pricing: https://aws.amazon.com/lightsail/pricing/
- Lightsail SSH users and keys: https://docs.aws.amazon.com/lightsail/latest/userguide/amazon-lightsail-ssh-using-terminal.html
- Lightsail firewall: https://docs.aws.amazon.com/lightsail/latest/userguide/understanding-firewall-and-port-mappings-in-amazon-lightsail.html
- Lightsail snapshots: https://docs.aws.amazon.com/lightsail/latest/userguide/amazon-lightsail-faq-snapshots.html
- Lightsail upsizing from a snapshot: https://docs.aws.amazon.com/en_us/lightsail/latest/userguide/how-to-create-larger-instance-from-snapshot-using-console.html
- Lightsail creating from a snapshot (same size or larger only): https://docs.aws.amazon.com/lightsail/latest/userguide/lightsail-how-to-create-instance-from-snapshot.html
- ECR from a Lightsail instance with an IAM user: https://dev.to/shayan-araghi/deploying-to-aws-lightsail-with-a-docker-image-from-ecr-55bn
- Public IPv4 charge: https://aws.amazon.com/blogs/networking-and-content-delivery/identify-and-optimize-public-ipv4-address-usage-on-aws/
