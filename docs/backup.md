# Backup and restore

Self-host operators should back up **database metadata** and **object storage**
(or the local data directory). Product settings live in the DB; files live under
`data_dir` (local) and/or remote S3/WebDAV per storage policy.

## What to back up

| Component | Default / notes |
|-----------|-----------------|
| SQLite DB | `{data_dir}/imgli.db` (Docker volume often `/data/imgli.db`) |
| Local objects | `{data_dir}/uploads` (and thumbs if separate under same tree) |
| Avatars / watermarks | under `{data_dir}` if used |
| Postgres | full logical dump (`pg_dump`) when `IMGLI_DATABASE_DRIVER=postgres` |
| Remote S3/WebDAV | provider-side lifecycle / cross-region copy (app does not snapshot remote buckets) |

Stop the process only if you cannot use online SQLite `.backup` or Postgres dump;
online backups are preferred.

## SQLite (recommended online)

Use the repo script (paths overridable via env):

```bash
# Defaults target VIP paths; override for generic installs:
export DATA_DIR=/data                 # Docker: map volume here
export DB="$DATA_DIR/imgli.db"      # or baili.db on legacy hosts
export BACKUP_DIR="$DATA_DIR/backups"
export RETENTION_DAYS=14

# From a host with sqlite3:
./deploy/ops/backup-sqlite.sh
# prints path to new backup file on success
```

Cron example (daily 03:15):

```cron
15 3 * * * DATA_DIR=/opt/imgli/data DB=/opt/imgli/data/imgli.db /opt/imgli/bin/backup-sqlite.sh >>/var/log/imgli-backup.log 2>&1
```

### Restore SQLite

1. Stop imgli (`systemctl stop …` / `docker compose stop imgli`).
2. Keep a copy of the broken DB.
3. Replace with the backup:

```bash
# example
cp -a "$DATA_DIR/imgli.db" "$DATA_DIR/imgli.db.broken-$(date +%Y%m%d%H%M%S)"
cp -a "$BACKUP_DIR/imgli.backup-YYYYMMDD.db" "$DATA_DIR/imgli.db"
# match ownership to the runtime user
chown imgli:imgli "$DATA_DIR/imgli.db"   # if applicable
```

4. Ensure local files under `uploads/` match the restore point (or accept broken
   object keys for missing files).
5. Start imgli; run `imgli doctor` and open `/healthz`.

### Docker volume note

If data is in a named volume (`imgli-data`):

```bash
# backup DB from a helper container
docker compose exec imgli sh -c 'ls -la /data'
# copy out:
docker compose cp imgli:/data/imgli.db ./imgli.db.backup
```

## PostgreSQL

Dump (online):

```bash
pg_dump -h 127.0.0.1 -U imgli -d imgli -Fc -f imgli-$(date +%Y%m%d).dump
# or plain SQL:
# pg_dump -h 127.0.0.1 -U imgli -d imgli > imgli.sql
```

Restore (service stopped or connections drained):

```bash
# custom format
pg_restore -h 127.0.0.1 -U imgli -d imgli --clean --if-exists imgli-YYYYMMDD.dump
# plain SQL
# psql -h 127.0.0.1 -U imgli -d imgli < imgli.sql
```

Then restore/sync object storage independently.

## After restore

```bash
imgli doctor -config /path/to/imgli.yaml
curl -fsS http://127.0.0.1:8686/healthz
```

Confirm uploads, login, and a known image `/i/…` still resolve.

## Related

- Production compose: [`deploy/compose.prod.example.yml`](../deploy/compose.prod.example.yml)
- Caddy / Traefik: [`deploy/Caddyfile.example`](../deploy/Caddyfile.example) · [`deploy/traefik.labels.example.yml`](../deploy/traefik.labels.example.yml)
- Config example: [`deploy/imgli.example.yaml`](../deploy/imgli.example.yaml)
- Security checklist: [`docs/security-hardening.md`](security-hardening.md)
- `imgli doctor` (CLI)
