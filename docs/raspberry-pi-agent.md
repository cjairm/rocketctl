# Personal agent on a Raspberry Pi 5

A runbook for running a personal AI agent (Hermes Agent) around the clock on a Raspberry Pi 5 at
home. You talk to it from your phone through Telegram, and reach its dashboard and SSH from
anywhere, without opening any port on your router.

## What you're building

```
   Your phone / laptop (anywhere)
        │                       │
        │ Telegram              │ Tailscale (private, encrypted)
        ▼                       ▼
   Telegram servers ◀──── Raspberry Pi 5 at home ────▶ AI provider API
                     outbound      │                  (Anthropic, OpenAI, …)
                     connections   ├─ Docker: hermes (agent + dashboard)
                     only          └─ No ports open to the internet
```

- **The Pi only makes outgoing connections.** Hermes asks Telegram for new messages and calls the
  AI provider's API. Nothing on the internet can start a connection to the Pi, so there's nothing
  to attack from outside.
- **Tailscale** puts the Pi, your phone and your laptop on a private network that only your devices
  can join. You reach the dashboard at an address like `https://agent-pi.<your-tailnet>.ts.net`,
  which is invisible to everyone else. You get HTTPS without a certificate setup, and no router
  changes.
- **Docker** keeps the agent in a container with its own files, so updates and removal are clean.

**Cost:** the hardware (roughly $100–150 one time) and electricity (a few watts). Tailscale's
Personal plan is free for non-commercial use: up to 6 users and unlimited devices.

## Which agent: Hermes Agent

The goal is good support and easy setup. A Pi 5 with 4–8 GB of RAM isn't short on memory, so the
ultra-light agents' main advantage (running on a $10 board with megabytes of RAM) doesn't matter here.

| Agent                           | GitHub stars (May 2026)      | Support and maturity                                                                                                      | Setup                                                                                                                                        | On a Pi 5                                                                                          |
| ------------------------------- | ---------------------------- | ------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------- |
| **Hermes Agent** (NousResearch) | ~160k, +3,800/week           | Official docs. No CVEs (publicly reported security flaws) as of May 2026. Weak spot: only 37% of GitHub issues get closed | Official Docker image for amd64 **and arm64**, a setup wizard, a Telegram allowlist that blocks everyone by default, runs as a non-root user | ✅ 1 GB minimum, 2–4 GB recommended                                                                |
| **ZeroClaw** (zeroclaw-labs)    | ~31–33k                      | Active, with a docs site and Discord. Risky actions need your approval by default ("supervised")                          | One-line installer, then `zeroclaw quickstart`. A single Rust program, so Docker is optional                                                 | ✅ Very light. The strongest alternative                                                           |
| **PicoClaw** (Sipeed)           | ~29–30k                      | Its README says "early rapid development… Do not deploy to production before v1.0"                                        | Web setup page or `picoclaw onboard`                                                                                                         | ⚠️ Works, but its maker says it isn't ready                                                        |
| **NemoClaw** (NVIDIA)           | ~20k                         | Not a separate agent: it runs OpenClaw inside NVIDIA's OpenShell sandbox                                                  | Complex                                                                                                                                      | ❌ Needs 4 vCPU and 8 GB minimum (16 GB recommended). Its arm64 support is tied to NVIDIA hardware |
| **OpenClaw**                    | ~374k, the biggest community | 9 CVEs in 4 days in March 2026 (worst rated 9.9 out of 10). Users report about a quarter of updates break a chat channel  | Users report setup friction. Docker is optional and builds the image on the Pi                                                               | ⚠️ 2 GB minimum, 4 GB recommended, 8 GB with browser automation                                    |

**Why Hermes over ZeroClaw:** about 5 times the users, so more answers when you're stuck, plus the
official Pi-ready Docker image and safe defaults this guide relies on. Pick ZeroClaw if you'd rather
run one light program without Docker.

The star counts come from a May 21, 2026 snapshot and the projects' README pages. Nobody has
published independent RAM measurements for these agents.

If you choose OpenClaw, the Pi preparation (steps 1–4) is the same. Only step 6 changes; see "Using
OpenClaw instead" at the end. For ZeroClaw, follow its own `zeroclaw quickstart` in place of step 6,
and keep everything else.

## Shopping list

| Item                                                                               | Why                                                                                                                               |
| ---------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------- |
| **Raspberry Pi 5, 8 GB**                                                           | 4 GB works for chat-only use. 8 GB leaves room for browser automation and later experiments                                       |
| **Official 27 W USB-C power supply**                                               | The Pi 5 needs 5 V / 5 A. A weaker phone charger causes random throttling and crashes                                             |
| **Active Cooler** (official)                                                       | Runs all day. Without cooling it slows down when it gets hot                                                                      |
| **Storage: NVMe SSD + M.2 HAT+** (recommended), or a good A2 microSD card to start | The agent writes logs and memory all day, and microSD cards wear out from constant writes. An SSD is faster and lasts much longer |
| **Ethernet cable**                                                                 | More reliable than Wi-Fi for a server                                                                                             |

## Step 1 — Install the operating system

On your Mac, install **Raspberry Pi Imager** (raspberrypi.com/software) and:

1. Device: **Raspberry Pi 5**. OS: **Raspberry Pi OS Lite (64-bit)**. "Lite" means no desktop,
   which saves RAM. It **must be the 64-bit version**, because the Hermes image is arm64 only.
2. Storage: the SSD (in a USB enclosure while flashing) or the microSD card.
3. **Edit settings** (OS customisation):
   - Hostname: `agent-pi`
   - Username: your choice (this guide uses `pi`), with a strong password
   - Wi-Fi only if you can't use Ethernet
   - Services → **Enable SSH → Allow public-key authentication only**, and paste the contents of
     `~/.ssh/id_ed25519.pub`. Create it with `ssh-keygen -t ed25519` if it doesn't exist.
4. Write, put it in the Pi, connect Ethernet and power.

From the Mac, on the same home network:

```bash
ssh pi@agent-pi.local
uname -m          # must print aarch64. armv7l means the 32-bit OS: flash again
```

## Step 2 — Base setup

```bash
sudo apt update && sudo apt -y full-upgrade
sudo apt install -y unattended-upgrades ufw   # automatic security updates, simple firewall
sudo reboot
```

If you boot from an SSD on the M.2 HAT+ and the Pi doesn't start from it, run
`sudo raspi-config` → Advanced Options → Boot Order → NVMe/USB.

## Step 3 — Tailscale

```bash
curl -fsSL https://tailscale.com/install.sh | sh
sudo tailscale up
```

`tailscale up` prints a link. Open it and log in. Then:

1. Install the Tailscale app on your phone and laptop, and log in with the **same account**.
2. In the Tailscale admin console → Machines → `agent-pi` → **Disable key expiry**. Otherwise the Pi
   drops off your network after the default expiry period (180 days) until you log in on it again.
3. Test from the laptop with Wi-Fi off and the phone's hotspot on, so you're really outside your
   home: `ssh pi@agent-pi`. Tailscale's MagicDNS (its built-in name lookup) resolves the name.

**Firewall.** Allow everything that comes through Tailscale, allow SSH from your home network as a
backup, and block everything else:

```bash
sudo ufw default deny incoming
sudo ufw default allow outgoing
sudo ufw allow in on tailscale0
sudo ufw allow from 192.168.0.0/16 to any port 22 proto tcp   # adjust to your home network's range
sudo ufw enable
```

> **Docker gotcha:** ports that Docker publishes skip ufw's rules completely. `ports: "9119:9119"`
> would be open to your whole home network whatever ufw says. That's why step 6 publishes only on
> `127.0.0.1`, and Tailscale brings the dashboard to your devices.

## Step 4 — Docker

```bash
curl -fsSL https://get.docker.com | sudo sh
sudo usermod -aG docker $USER
exit
ssh pi@agent-pi
docker run --rm hello-world
```

Docker starts at boot on its own, and with `restart: unless-stopped` the agent comes back after a
power cut.

## Step 5 — Accounts and keys (separate from everything else)

The agent reads web pages, messages and files. Any of them can contain text written to manipulate
it, known as prompt injection. Plan as if one day it will be tricked, and limit what it can reach:

1. **A dedicated AI API key with a monthly spending limit.** Create a new key only for the agent in
   the provider's console (Anthropic, OpenAI, OpenRouter, …), and set a low budget or limit there.
   Never reuse your products' keys (CochinitoApp, AutomatedHub).
2. **A Telegram bot:** in Telegram, message **@BotFather** → `/newbot` → copy the token.
3. **Your numeric Telegram user ID:** message **@userinfobot**. It replies with a number like
   `123456789`. That number goes in the allowlist.
4. Don't give the agent your main email, bank or password-manager logins. Add access later, one
   piece at a time, with accounts made for it.

## Step 6 — Hermes Agent

**First run: the setup wizard.** It asks for your AI provider and key, and for the chat platform
(pick Telegram and paste the bot token). Run it over SSH, not a browser console, because the docs
warn that browser consoles can mangle characters like `:` `@` `=` `/`.

```bash
mkdir -p ~/hermes && cd ~/hermes
mkdir -p data
docker run -it --rm -v ~/hermes/data:/opt/data nousresearch/hermes-agent setup
```

The wizard writes `~/hermes/data/.env` and `config.yaml`. Lock the folder down, since it holds your
keys:

```bash
chmod 700 ~/hermes/data && chmod 600 ~/hermes/data/.env
```

**Add the allowlist and dashboard password** to `~/hermes/data/.env`:

```bash
TELEGRAM_ALLOWED_USERS=123456789          # your number from @userinfobot
HERMES_DASHBOARD_BASIC_AUTH_USERNAME=me
HERMES_DASHBOARD_BASIC_AUTH_PASSWORD=<output of: openssl rand -hex 16>
```

The gateway already refuses users who aren't allowlisted or paired, and the allowlist makes it
explicit. Never set `GATEWAY_ALLOW_ALL_USERS=true`: the docs warn against it for bots with terminal
access. The dashboard refuses to start without login settings when it listens on anything other
than localhost, which it does inside the container. Tailscale already limits who can reach it; the
password is a second lock.

**`~/hermes/docker-compose.yml`:**

```yaml
services:
  hermes:
    image: nousresearch/hermes-agent:latest # once it works, pin a version (X.Y.Z) and upgrade on purpose
    container_name: hermes
    restart: unless-stopped
    command: gateway run
    env_file: ./data/.env
    environment:
      - HERMES_DASHBOARD=1
      - HERMES_UID=1000 # your user's id (check with: id -u). Matches file ownership of ./data
      - HERMES_GID=1000 # id -g
    volumes:
      - ./data:/opt/data
    ports:
      - "127.0.0.1:9119:9119" # dashboard: only this Pi. Tailscale brings it to your devices
    shm_size: 1g # browser automation needs it
    mem_limit: 4g # on an 8 GB Pi; use 2g on a 4 GB Pi
    cpus: 2
    logging:
      driver: json-file
      options: { max-size: "10m", max-file: "3" }
```

- Hermes's own docs map `~/.hermes`. This guide keeps everything in `~/hermes/` so the Compose file
  and its data sit together. Either works.
- Port 8642 (an OpenAI-compatible API) is left out on purpose. Add it only if another program needs
  to call the agent, and then set `API_SERVER_KEY`.
- Never mount `/var/run/docker.sock` or your home folder into this container. Either would give the
  agent control of the whole Pi.

**Start it:**

```bash
cd ~/hermes
docker compose up -d
docker compose logs -f        # wait for the gateway to report Telegram connected
```

Send your bot a message in Telegram. If someone else messages it, they get no answer.

**The dashboard on your devices only:**

```bash
sudo tailscale serve --bg 9119
tailscale serve status        # shows the https://agent-pi.<tailnet>.ts.net address
```

Open that address on your phone or laptop, with Tailscale connected, and log in with the dashboard
password. It isn't reachable from the internet. Tailscale **Funnel** would make it public, so don't
enable it.

## Step 7 — Day to day

| Task           | Command                                                                                                                   |
| -------------- | ------------------------------------------------------------------------------------------------------------------------- |
| Logs           | `cd ~/hermes && docker compose logs -f`                                                                                   |
| Update Hermes  | `docker compose pull && docker compose up -d`. Data stays. Hermes backs up `config.yaml` and `.env` before migrating them |
| Restart        | `docker compose restart`                                                                                                  |
| Memory and CPU | `docker stats --no-stream`                                                                                                |
| Temperature    | `vcgencmd measure_temp`. Under about 70 °C is fine; above 80 °C it slows down                                             |
| OS updates     | Automatic for security updates. `sudo apt update && sudo apt -y full-upgrade` once a month                                |

**Backups.** `~/hermes/data` holds the agent's memory, skills and **your keys**. Back it up
encrypted, never as a plain zip in cloud storage:

```bash
# On the Mac: copy over Tailscale, then encrypt with a passphrase
ssh pi@agent-pi 'tar czf - -C ~/hermes data' | openssl enc -aes-256-cbc -pbkdf2 -salt -out hermes-$(date +%F).tar.gz.enc
```

To restore, copy it back, decrypt with `openssl enc -d -aes-256-cbc -pbkdf2 -in … | tar xzf -`
into `~/hermes`, and run `docker compose up -d`.

## Optional — A public URL (only if you really need one)

The setup above needs no public URL. If you later want `agent.yourdomain.com` to work for people
without Tailscale, use **Cloudflare Tunnel**:

1. Move your domain's DNS to Cloudflare (free plan).
2. Zero Trust dashboard → Networks → Tunnels → create a tunnel → it gives you a `cloudflared`
   command to run on the Pi.
3. Add a public hostname `agent.yourdomain.com` → `http://localhost:9119`.
4. **Before sharing the link, add Cloudflare Access** (Zero Trust → Access → Applications) with a
   policy that only allows your email. Without it, anyone who finds the URL reaches your agent's
   login page.

Avoid port forwarding on your router. It exposes your home IP and network, and many home internet
providers share one IP among several customers (CGNAT), so it often doesn't work anyway.

## Security checklist

- [ ] 64-bit OS, SSH with keys only (step 1)
- [ ] ufw blocking incoming traffic except Tailscale and SSH from home (step 3)
- [ ] Dashboard published on `127.0.0.1` only, reached through `tailscale serve`, never Funnel
- [ ] `TELEGRAM_ALLOWED_USERS` set to your ID only
- [ ] A dedicated AI API key with a spending limit
- [ ] `~/hermes/data` is mode 700, `.env` is mode 600, and backups are encrypted
- [ ] No Docker socket or home folder mounted into the container
- [ ] Recommended: if your router has a **guest network** or VLAN (a separate network segment), put
      the Pi on it. If the agent is ever tricked into running commands, it then can't reach your
      laptop, NAS or other devices at home
- [ ] Keep in mind that Telegram bot chats are not end-to-end encrypted, so don't send the agent
      passwords through it

## Using OpenClaw instead

Steps 1–5 stay the same. For step 6, follow OpenClaw's own docs (docs.openclaw.ai): install with its
script, or with Docker via `./scripts/docker/setup.sh`, which builds the image on the Pi. Then:

- Use its Tailscale **serve** mode (tailnet only) for the gateway, never **funnel**.
- Turn on its sandbox (`agents.defaults.sandbox.mode: "all"`, `workspaceAccess: "ro"`) so the
  agent's commands run in a throwaway container with read-only access to its workspace.
- Run `openclaw security audit --deep` after setup.
- Plan for 8 GB if you use browser automation.

I took these settings from OpenClaw's docs index and third-party guides, not from reading the full
official pages, so confirm the exact keys there.

## Sources

- Agent popularity and security comparison (May 2026): https://www.glukhov.org/ai-systems/comparisons/openclaw-hermes-alternatives-popularity/
- ZeroClaw: https://github.com/zeroclaw-labs/zeroclaw
- PicoClaw: https://github.com/sipeed/picoclaw
- NemoClaw prerequisites (NVIDIA): https://docs.nvidia.com/nemoclaw/latest/get-started/prerequisites
- Hermes Docker setup (official): https://hermes-agent.nousresearch.com/docs/user-guide/docker
- Hermes messaging and allowlists (official): https://hermes-agent.nousresearch.com/docs/user-guide/messaging/
- Hermes ARM64 support (maintainer answer): https://github.com/NousResearch/hermes-agent/issues/10218
- Tailscale Linux install: https://tailscale.com/kb/1031/install-linux
- Tailscale pricing: https://tailscale.com/pricing
- OpenClaw docs: https://docs.openclaw.ai/
- OpenClaw Docker install: https://github.com/openclaw/openclaw/blob/main/docs/install/docker.md
- OpenClaw + Tailscale: https://allclaw.org/blog/openclaw-tailscale
