# Ops helpers

| File | Purpose |
|------|---------|
| [`backup-sqlite.sh`](backup-sqlite.sh) | Online SQLite `.backup` + retention (env-overridable paths) |
| [`health-check.sh`](health-check.sh) | Disk / process / queue snapshot (legacy VIP defaults) |
| [`../../scripts/ops-deploy-baili.sh`](../../scripts/ops-deploy-baili.sh) | Deploy GitHub Release binary → `/opt/baili` + restart `baili` |
| [`../../scripts/ops-smoke-public.sh`](../../scripts/ops-smoke-public.sh) | Public SPA smoke (root / healthz / JS / config) |
| [`../../scripts/ops-smoke-install.sh`](../../scripts/ops-smoke-install.sh) | Install-path smoke: docker/binary start + register + upload |
| [`../../scripts/pre-tag-check.sh`](../../scripts/pre-tag-check.sh) | Pre-tag gate: CHANGELOG section + clean tree + main CI green |
| [`../../docs/ops-release.md`](../../docs/ops-release.md) | End-to-end release / CI / deploy playbook |
| [`admin-stats-metering.md`](admin-stats-metering.md) | Admin traffic/referer = **origin-only**; CDN edge not included |
| [`cloudflare-i-img-li-cache-checklist.md`](cloudflare-i-img-li-cache-checklist.md) | Production `i.img.li` CF cache acceptance |
| [`../compose.prod.example.yml`](../compose.prod.example.yml) | Production-oriented Compose |
| [`../Caddyfile.example`](../Caddyfile.example) | Caddy TLS reverse proxy |
| [`../traefik.labels.example.yml`](../traefik.labels.example.yml) | Traefik labels snippet |
| [`../../docs/backup.md`](../../docs/backup.md) | Backup / restore guide |

Backup and restore narrative lives in **docs/backup.md** (not only this folder).

**Metering:** Admin dashboard views/referers count origin `/i` gates only. With
`cdn_domain` 302 offload, edge hits are invisible to the app—see
[`admin-stats-metering.md`](admin-stats-metering.md). `imgli doctor` warns when
CDN domains are configured.
