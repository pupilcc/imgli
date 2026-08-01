# imgli public roadmap (mirror)

> **Planning SSOT is internal** (`imgli/roadmap` in the project knowledge base).  
> This file is a **public, de-sensitized mirror** for contributors. Prefer GitHub Issues + Milestones for execution.

## Latest shipped — v0.7.2 · One-click upgrade + admin shell

Release: https://github.com/yixian-huang/imgli/releases/tag/v0.7.2

Binary upgrade under systemd `ProtectSystem` + re-exec; admin layout sticky nav/title. See [CHANGELOG](CHANGELOG.md#072---2026-08-01).

## Previous — v0.7.1 · Storage probe reliability

Release: https://github.com/yixian-huang/imgli/releases/tag/v0.7.1

## Previous — v0.7.0 · Ops Console · Health · Deploy

Release: https://github.com/yixian-huang/imgli/releases/tag/v0.7.0  
Milestone: https://github.com/yixian-huang/imgli/milestone/6

| Theme | Highlights |
|-------|------------|
| Health | Admin `GET /system/health` (doctor + runtime summary) |
| Deploy clarity | Browser vs `base_url` mismatch, reverse-proxy checklist, CSRF docs |
| Ops IA | System / Ops page: upgrade preflight, cleanup UI, migrate/backup links |
| Onboarding | Unified three-step setup UI (upload + API Token) |

## Shipped earlier

- **v0.6.0** — Ops · Migrate · Trust (storage migrate jobs, version upgrade, lifecycle cleanup)  
- **v0.5.x** — Trust · Onboard · Community (R2 Verified, first-run Token, site ops, public ROADMAP)  
- **v0.4.x** — Storage Caps, FTP compatibility driver  
- **v0.3.0** — Password shares, public albums, width thumbs, import-dir, OIDC, webhooks  
- **v0.2.x** — CLI upload, doctor, EXIF strip, max views, light admin analytics  

## Next (not a committed schedule)

- **Community:** more S3-compatible vendors in the matrix (#51)  
- **Later candidates (internal SSOT):** single-instance team/org, fuller transform suite, more IdP connectors, async replicas / dual-write  

No open product milestone is required for community PRs — file an Issue or open a PR against `main`.

## Non-goals (do not open “white-label full” issues expecting Community)

Full BrandLockup replace, multi-tenant control plane, Open Core paywalls, video mainline, independent short-link product. See `COMMERCIAL.md` for dual-license inquiries.

## Contribute

- Bug / feature: GitHub Issues (templates under `.github/`)  
- S3 vendor reports: [`docs/s3-compatibility.md`](docs/s3-compatibility.md) + issue template  
- Security: [`SECURITY.md`](SECURITY.md)
