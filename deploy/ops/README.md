# Ops helpers

| File | Purpose |
|------|---------|
| [`backup-sqlite.sh`](backup-sqlite.sh) | Online SQLite `.backup` + retention (env-overridable paths) |
| [`health-check.sh`](health-check.sh) | Disk / process / queue snapshot (legacy VIP defaults) |
| [`../compose.prod.example.yml`](../compose.prod.example.yml) | Production-oriented Compose |
| [`../Caddyfile.example`](../Caddyfile.example) | Caddy TLS reverse proxy |
| [`../traefik.labels.example.yml`](../traefik.labels.example.yml) | Traefik labels snippet |
| [`../../docs/backup.md`](../../docs/backup.md) | Backup / restore guide |

Backup and restore narrative lives in **docs/backup.md** (not only this folder).
