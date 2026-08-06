# Documentation single sources of truth (SSOT)

imgli documentation is **layered by audience**, not mirrored in full between
GitHub and the product knowledge base.

## Layers

| Layer | Location | Audience | What belongs here |
|-------|----------|----------|-------------------|
| **Engineering (Git)** | This repo `docs/`, `README*`, `CHANGELOG` | Contributors, self-hosters reading the source | Matrices, scripts, design drafts, ops checklists tied to code/tags |
| **Product docs (KB)** | omni paths `docs-imgli/*` → docs.imgli.com | End users / operators of the product UI | How-to, FAQ, install narrative, feature overview |
| **Internal (KB)** | omni `imgli/*`, `hub/imgli` | Maintainers only | Roadmap, production ops, commercial strategy, session decisions |

## Rules

1. **One duty → one edit place.** Summaries + links are OK; dual full copies are not.
2. **Behavior changed in a PR** → update **repo** `docs/` (and README/CHANGELOG as needed) in the same PR.
3. **After release / user-facing change** → refresh **`docs-imgli/*`** (and hub status if production version moved). Prefer linking to GitHub for long matrices.
4. **Conflict:** GitHub tag + `CHANGELOG` + latest Release win for product version and APIs.
5. **Never** put full internal roadmap or VIP secrets into `docs-imgli/`.
6. **WebDAV / S3 / FTP matrices** live in **repo** (`docs/webdav-compatibility.md`, `docs/s3-compatibility.md`, `docs/storage-ftp.md`); product site pages stay short and link out.

## Map

| Topic | Repo SSOT | Product docs (`docs-imgli`) |
|-------|-----------|-------------------------------|
| **Latest product version** | git tag + `CHANGELOG.md` + `ROADMAP.md` | version pins / release notes |
| S3 vendors | `docs/s3-compatibility.md` | `docs-imgli/s3` (summary) |
| WebDAV vendors | `docs/webdav-compatibility.md` | `docs-imgli/storage-cdn` + link (or `docs-imgli/webdav`) |
| FTP dual track | `docs/storage-ftp.md` | `docs-imgli/ftp` |
| Caps design | `docs/design/storage-caps-draft.md` | user-facing Caps described on storage pages only |
| **User-group lifecycle (v0.9)** | `docs/user-groups-lifecycle.md` | short how-to + link (no dual full copy) |
| **Site customization (L0 / L2)** | `docs/design/site-customization-ia.md` | Appearance + site name how-to + link |
| **v0.9.2 acceptance** | `docs/superpowers/plans/2026-08-04-v0.9.2-acceptance.md` | optional release notes pointer |
| **Release / CI / prod deploy** | `docs/ops-release.md` | not public play-by-play |
| **Deploy / SPA outage guard** | `docs/ops-deploy-checklist.md` | ops FAQ blurb |
| **Cleanup vs CDN** | `docs/ops-cleanup-cdn-boundary.md` | ops FAQ blurb + link |
| Security / reverse proxy | `docs/security-hardening.md` | hardening summary |
| Integrations (ShareX / uPic / PicGo) | `docs/integrations/`, `docs/picgo.md` | client setup pages |
| Production VIP deploy details | *(not in public repo detail)* | internal `imgli/ops-*` only |

## Agent / human checklist

- [ ] New feature → code + repo docs in one PR  
- [ ] Ship tag → hub `last_release` / production note + `docs-imgli` version pins  
- [ ] No second full matrix in KB  
- [ ] After appearance / theme keys change → update `site-customization-ia.md` + README feature bullets  
- [ ] After release process change → update `ops-release.md` + `CONTRIBUTING.md` pointer  
