# Deploying to an Oracle Cloud Always Free VM

This puts the whole stack on **one** Oracle Cloud "Always Free" virtual
machine, reachable over HTTPS at a domain you own, so a few non-technical test
users can sign in and use it without a terminal. It is the target for
[issue #20](https://github.com/shurikai/role-model/issues/20).

It runs the same `docker compose` stack as local development, plus a
`compose.prod.yml` override that adds [Caddy](https://caddyserver.com) in front
for automatic TLS and stops publishing everything except Caddy.

```
internet ──443──▶ caddy ──▶ web (nginx) ──▶ server (Go) ──▶ renderer (Python)
                                    │                └────────▶ db (Postgres)
                                    └─ serves the React bundle
```

**Scope.** Single VM, single region, ~2–3 users, closed signup. No CDN, no
redundancy — one box is a single point of failure, which is fine for test
users. Scaling past that is the Phase 4 work still tracked on #20.

---

## 0. What you need

- An Oracle Cloud account (the Always Free tier never expires and needs no
  paid upgrade for this).
- A domain, and the ability to add a DNS record to it.
- An Anthropic API key. You pay for your own tokens; the server refuses to
  start without one.
- This repository.

---

## 1. Provision the VM

Console → **Compute → Instances → Create instance**.

- **Image:** Ubuntu 22.04 (its firewall is simpler than Oracle Linux's — see
  step 2). Oracle Linux 9 works too.
- **Shape:** *Ampere* → `VM.Standard.A1.Flex`, **2 OCPU / 12 GB**. This is
  inside the Always Free allowance (up to 4 OCPU / 24 GB of A1 total). Two
  AMD micro VMs are also free but 1 GB RAM will not run Postgres + Go +
  Python + nginx together.
- **Boot volume:** 50 GB is plenty (the database is tiny); you have 200 GB of
  Always Free block storage to draw on.
- **SSH keys:** upload your public key.

> **"Out of host capacity."** A1 capacity is frequently exhausted in popular
> regions. If Create fails with that error: try a different Availability
> Domain, try again later, or script a retry with the OCI CLI
> (`oci compute instance launch` in a loop). Your *home region* is set at
> account creation and is hard to change, so pick a quieter one if you can.

Note the instance's **public IP** once it is running.

---

## 2. Open the firewall — both layers

Traffic has to pass an OCI-level rule **and** the VM's own firewall. Missing
either looks identical: the connection just hangs.

### 2a. OCI security list / NSG

Console → **Networking → Virtual Cloud Networks → your VCN → Security Lists →
Default Security List** → **Add Ingress Rules**:

| Source        | Protocol | Dest. port | Purpose        |
|---------------|----------|------------|----------------|
| `0.0.0.0/0`   | TCP      | `80`       | HTTP → HTTPS redirect + ACME |
| `0.0.0.0/0`   | TCP      | `443`      | HTTPS          |
| `0.0.0.0/0`   | UDP      | `443`      | HTTP/3 (optional) |
| *your IP*/32  | TCP      | `22`       | SSH (tighten from the default `0.0.0.0/0`) |

### 2b. The VM's own firewall

**Ubuntu** (iptables, pre-seeded by Oracle's image):

```bash
sudo iptables -I INPUT 6 -m state --state NEW -p tcp --dport 80 -j ACCEPT
sudo iptables -I INPUT 6 -m state --state NEW -p tcp --dport 443 -j ACCEPT
sudo iptables -I INPUT 6 -m state --state NEW -p udp --dport 443 -j ACCEPT
sudo netfilter-persistent save
```

(`INPUT 6` inserts the rules *before* the image's catch-all `REJECT`. Check
`sudo iptables -L INPUT --line-numbers` and adjust the index if yours differs.)

**Oracle Linux** (firewalld):

```bash
sudo firewall-cmd --permanent --add-service=http --add-service=https
sudo firewall-cmd --permanent --add-port=443/udp
sudo firewall-cmd --reload
```

---

## 3. Install Docker

```bash
# Ubuntu
sudo apt-get update
sudo apt-get install -y ca-certificates curl git
sudo install -m 0755 -d /etc/apt/keyrings
sudo curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu $(. /etc/os-release && echo $VERSION_CODENAME) stable" | sudo tee /etc/apt/sources.list.d/docker.list
sudo apt-get update
sudo apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin

sudo usermod -aG docker "$USER"
newgrp docker   # or log out and back in
```

Confirm `docker compose version` reports **v2.24 or newer** — `compose.prod.yml`
uses the `!override` YAML tag, and older Compose silently ignores it and leaves
the database port open.

---

## 4. DNS

Add an **A** record for your hostname pointing at the VM's public IP (and an
**AAAA** record if the VM has an IPv6 address):

```
resume.example.com.   A   <public-ip>
```

Wait for it to resolve (`dig +short resume.example.com`) **before** the first
`up` — Caddy needs the name to point at this box to solve the ACME challenge.

---

## 5. Get the code and configure

```bash
git clone https://github.com/shurikai/role-model.git
cd role-model
cp .env.example .env
```

Edit `.env`:

```dotenv
# Required
ANTHROPIC_API_KEY=sk-ant-...
JWT_SECRET=<paste `openssl rand -base64 32`>

# Public instance
DOMAIN=resume.example.com
ACME_EMAIL=you@example.com
ENVIRONMENT=production
SIGNUP_ENABLED=false
```

Then:

```bash
chmod 600 .env
```

Notes:

- Most dev-only lines in `.env.example` — `DATABASE_URL`, `RENDERER_URL`,
  `SEED_DIR`, `PORT` — do **not** affect the running stack. Compose injects the
  correct internal values into the containers; those variables are only read by
  host tools like `make migrate-up`. Leaving them is harmless.
- **Do** clear `CORS_ALLOWED_ORIGINS` (set it empty or delete the line). It is
  passed into the server, and the bundled deployment is same-origin — Caddy,
  nginx and the API are one origin — so there is no cross-origin request to
  allow. A stray dev value here just widens what the API accepts for no reason.
- Pre-flight the merge before the first `up`:

  ```bash
  docker compose -f docker-compose.yml -f compose.prod.yml config | grep -A3 'ports:'
  ```

  Only `caddy` should list published ports. If `db` or `web` show one, your
  Docker Compose is too old for the `!override` tag (step 3).
- `SEED_DIR` / `make seed` is for loading *your own* career data into a *local*
  database from a private repo. A deployed instance starts empty and users fill
  it through the UI (Stage 0 import / onboarding). It is not part of this
  deployment.

---

## 6. First run

```bash
docker compose -f docker-compose.yml -f compose.prod.yml up -d --build
```

- `migrate` runs once and exits; `server` waits for it to succeed, so there is
  never a manual migration step.
- Watch Caddy get its certificate:

  ```bash
  docker compose -f docker-compose.yml -f compose.prod.yml logs -f caddy
  ```

  Look for `certificate obtained successfully`. If it loops on an ACME error,
  DNS is not pointing here yet or port 80 is not open (step 2).

- Check it:

  ```bash
  curl -fsS https://resume.example.com/health && echo OK
  ```

`docker compose -f docker-compose.yml -f compose.prod.yml ps` should show
`caddy`, `web`, `server`, `renderer`, `db` up and `migrate` exited `0`.

---

## 7. Create the first account

Signup is closed, so there is no way in yet. Until a `cmd/adduser` exists
(tracked in [#120](https://github.com/shurikai/role-model/issues/120)), open
the window briefly:

```bash
# 1. flip it on
sed -i 's/^SIGNUP_ENABLED=.*/SIGNUP_ENABLED=true/' .env
docker compose -f docker-compose.yml -f compose.prod.yml up -d server

# 2. create the account (or just use the sign-up form in the browser)
curl -fsS -X POST https://resume.example.com/api/v1/auth/signup \
  -H 'Content-Type: application/json' \
  -d '{"email":"you@example.com","password":"<a real password>"}'

# 3. close it again
sed -i 's/^SIGNUP_ENABLED=.*/SIGNUP_ENABLED=false/' .env
docker compose -f docker-compose.yml -f compose.prod.yml up -d server
```

Repeat step 2 for each test user while the window is open, or add them one at a
time. Then confirm `SIGNUP_ENABLED=false` is back and the server was recreated.

---

## 8. Backups

The `role_model_pgdata` volume holds **everything entered through the app** —
applications, extracted `jd_signals`, resume versions, fit reports. None of that
is in any git repo. Back it up.

Create `~/backup-role-model.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail
cd "$HOME/role-model"
mkdir -p "$HOME/backups"
stamp=$(date +%Y%m%d-%H%M%S)
out="$HOME/backups/role_model-$stamp.sql.gz"
docker compose -f docker-compose.yml -f compose.prod.yml exec -T db \
  pg_dump -U rolemodel role_model | gzip > "$out"
# keep 14 days
find "$HOME/backups" -name 'role_model-*.sql.gz' -mtime +14 -delete
echo "wrote $out"
```

```bash
chmod +x ~/backup-role-model.sh
( crontab -l 2>/dev/null; echo "15 4 * * * $HOME/backup-role-model.sh >> $HOME/backups/cron.log 2>&1" ) | crontab -
```

Optionally push the dump to the 10 GB Always Free Object Storage bucket with
`oci os object put` at the end of the script.

**Restore** into a fresh database:

```bash
docker compose -f docker-compose.yml -f compose.prod.yml down
docker volume rm role-model_role_model_pgdata      # or start clean another way
docker compose -f docker-compose.yml -f compose.prod.yml up -d db
gunzip -c ~/backups/role_model-<stamp>.sql.gz | \
  docker compose -f docker-compose.yml -f compose.prod.yml exec -T db \
  psql -U rolemodel role_model
docker compose -f docker-compose.yml -f compose.prod.yml up -d
```

---

## 9. Updating

```bash
cd ~/role-model
git pull
docker compose -f docker-compose.yml -f compose.prod.yml up -d --build
```

`migrate` re-runs (golang-migrate is idempotent) and applies any new
migrations before `server` restarts. Caddy keeps its certificate across the
restart because it is on a named volume.

**Rolling back** a bad deploy is `git checkout <previous-sha> && … up -d
--build` — **but** `migrate` only moves forward. If the bad deploy added a
migration, roll the schema back by hand first:

```bash
docker compose -f docker-compose.yml -f compose.prod.yml run --rm migrate \
  -path=/migrations \
  -database=postgres://rolemodel:rolemodel@db:5432/role_model?sslmode=disable \
  down 1
```

Take a backup (step 8) before any update that ships a migration.

---

## 10. Known limitations and follow-ups

- **Rate limiting is per-socket.** `internal/api/middleware/ratelimit.go`
  buckets by the connection's remote address and deliberately ignores
  `X-Forwarded-For`. Behind Caddy → nginx that address is always the nginx
  container, so `/auth/login`'s 20/min limit is shared across all callers.
  Acceptable at 2–3 users with signup closed; if it is ever abused, add a
  rate-limit module to Caddy (needs a custom image) or run `fail2ban` on the
  host against the nginx access log.
- **`X-Forwarded-Proto` reaching the Go server is `http`.** The internal
  Caddy → nginx hop is plain HTTP, and `frontend/nginx.conf` overwrites the
  header with `$scheme`. Nothing reads it today, but **OIDC (#120) will** — it
  needs to know the request came in on HTTPS to build redirect URLs. The fix
  is a one-liner in `nginx.conf`: trust Caddy's header via a `map` that falls
  back to `$scheme` when it is absent. Do it as part of #120.
- **Default database credentials.** `db` keeps `rolemodel:rolemodel`. Safe
  only because the port is closed and nothing else is on the Docker network.
  To change it, also update the two hard-coded DSNs in `docker-compose.yml`
  (the `server` `environment` block and the `migrate` `command`).
- **The renderer has no authentication** and is never published. Keep it that
  way — it is an unauthenticated document generator that assumes a private
  network.
- **Single VM, single region.** No failover. The Phase 4 tier work on #20 is
  where that changes.

---

## 11. Teardown

```bash
cd ~/role-model
docker compose -f docker-compose.yml -f compose.prod.yml down          # keep data
docker compose -f docker-compose.yml -f compose.prod.yml down -v       # delete data too
```

Then terminate the instance in the OCI console.
