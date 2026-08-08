# Deployment

No Docker anywhere in this stack — Go compiles to one static binary, so
the server needs nothing beyond that binary, PostgreSQL, and Caddy.

## Layout on the server

```
/opt/app/
  api               # HTTP server binary
  cli               # CLI binary (migrations, provisioning, cleanup)
  api.backup        # previous binary, for instant rollback

/etc/app/api.env    # config, chmod 600
/etc/systemd/system/app-api.service
/etc/systemd/system/app-cleanup.service
/etc/systemd/system/app-cleanup.timer
```

## First-time setup

```bash
sudo useradd -r -s /usr/sbin/nologin app
sudo mkdir -p /opt/app /etc/app
sudo chown app:app /opt/app
git clone <your-repo-url> /opt/app
sudo cp deploy/app-api.service deploy/app-cleanup.service deploy/app-cleanup.timer /etc/systemd/system/
sudo cp .env.example /etc/app/api.env   # then edit it with real values
sudo chmod 600 /etc/app/api.env
sudo systemctl daemon-reload
sudo systemctl enable --now app-cleanup.timer
```

Install Caddy, then:
```bash
sudo cp deploy/Caddyfile /etc/caddy/Caddyfile   # edit the domain first
sudo systemctl reload caddy
```

### Ubuntu/Debian vs Rocky

Package manager differs (`apt install postgresql caddy` vs
`dnf install postgresql-server caddy`); everything else — systemd units,
the Go binary, the deploy script — is identical.

**Rocky-specific:** SELinux is enabled by default and blocks Caddy from
connecting to `localhost:8080` unless told otherwise:
```bash
sudo setsebool -P httpd_can_network_connect 1
```
Without this, requests through Caddy fail with a "Permission denied" that
has nothing obviously to do with SELinux — if Caddy can't reach the API on
Rocky, this is the first thing to check.

## Deploying an update

```bash
cd /opt/app
./deploy/deploy.sh
```
This pulls, backs up the current binaries, rebuilds, runs platform +
tenant migrations, syncs permissions, and restarts the service. Expect
1–2 seconds of downtime at the restart — schedule deploys outside business
hours for apps with fixed operating hours (like POS).

### Rollback

```bash
cp api.backup api && sudo systemctl restart app-api
```

## Backups

```bash
sudo cp deploy/pg-backup.service deploy/pg-backup.timer /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now pg-backup.timer
```
This runs `pg_dump` daily and keeps 35 days locally. **Copy backups off
the server** (rsync/rclone to remote storage) — a backup on the same disk
that fails doesn't protect you. Test a restore periodically; an untested
backup is a guess, not a backup.

## Environment variables

See `.env.example` for the full list. All of them are readable/writable
without a rebuild — edit `/etc/app/api.env` and `systemctl restart app-api`.
