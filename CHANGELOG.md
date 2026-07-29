# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

Product version comes from **git tags** (`vMAJOR.MINOR.PATCH`). Do not maintain a
separate version in `go.mod` or `web/package.json`.

## [Unreleased]

### Fixed

- Plaza feed keyset cursor for sort `new` used nanosecond timestamps that did
  not match SQLite/GORM second-level comparisons, so the next page could repeat
  the previous boundary row.

### Added

- **CLI `imgli upload`:** multipart upload to `/api/v1/upload` via
  `IMGLI_BASE_URL` / `IMGLI_TOKEN` (or flags); file or stdin; output
  `url|markdown|json` (`internal/cliupload`).
- **Integrations docs:** ShareX custom uploader + sample `.sxcu`, uPic/PicList
  custom host mapping (`docs/integrations/`).
- **Settings API Token snippets:** curl / PicGo / ShareX / `imgli upload` CLI
  cards using public `base_url`; plain token only while create-once banner is
  open (`GET /api/v1/config` exposes `base_url`).
- **Public share page:** `GET /api/v1/s/{key}` + SPA `/s/{key}` for
  public+normal images (preview, dimensions, copy URL/Markdown); private /
  pending / rejected / expired → 404.
- **Monthly bandwidth hard cap (v1):** user-group `bandwidth_quota_month` (Free/default
  seed **5 GiB/month**, Asia/Shanghai calendar month); meter on successful `/i`/`/t`
  gate release by object size; block upload + 429 when exceeded; usage on
  `GET /user/quota`; admin group field; Nav/upload meters. See product decisions
  for scope (no CDN true-hit metering; hotlink still off by default).
- **Guest landing UX:** unauthenticated `/` stays on upload page with sign-in CTA
  when guest upload is off (no hard redirect to login only).
- **Auth `next` return:** login/register honors safe `?next=` (open-redirect safe)
  so users return to upload or the page they attempted.
- GitHub issue templates (bug / feature / S3 vendor report) and PR template.
- Docs: `docs/s3-compatibility.md` matrix stub and
  `docs/security-hardening.md` (private object storage / proxy checklist).
- **S4 slice:** refuse unauthenticated CDN URLs for `private/` object keys
  (`CDNEligibleObjectKey` + serve visibility/surface checks); operator probe
  `scripts/probe-private-object-anon.sh`.
- **Site slots (settings):** announcement bar, footer link groups, and
  admin-only HTML inject (`announcement` / `footer` / `html_inject` settings),
  exposed on public `/api/v1/config` and rendered in the SPA shell.

### Changed

- **License:** project default changed from MIT to **AGPL-3.0-only** to reduce
  closed SaaS / white-label freeloading of network-served modifications, while
  offering optional **commercial licenses** (see `COMMERCIAL.md`). Tags
  `v0.1.0` and `v0.1.1` remain MIT snapshots.
- CI/release workflows: `actions/checkout@v5`, `setup-go@v6`, `setup-node@v5`.

## [0.1.1] - 2026-07-29

### Added

- One-liner binary install script (`scripts/install.sh`) and Quick Start docs
  for Linux/macOS release assets.
- Automated GitHub Releases via GoReleaser (multi-platform binaries + checksums)
  and multi-arch Docker images on `ghcr.io/yixian-huang/imgli`.

## [0.1.0] - 2026-07-28

### Added

- Initial public release: single-binary image hosting with embedded React UI.
- Storage backends: local disk, S3-compatible, WebDAV; CDN `302` offload and
  presigned private serving.
- Pluggable moderation (NSFW + OCR sidecar), review queue, group policies.
- Accounts: groups/quotas, guest upload, invites, SMTP, albums, public gallery,
  recycle bin, image expiry.
- Upload API + API tokens; PicGo/Typora/VS Code guide.
- Bilingual UI (中文/English), PWA, dark mode, text watermark, admin audit logs.
- Docker Compose quick start and GitHub Actions CI (Go matrix, web, e2e smoke).

[Unreleased]: https://github.com/yixian-huang/imgli/compare/v0.1.1...HEAD
[0.1.1]: https://github.com/yixian-huang/imgli/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/yixian-huang/imgli/releases/tag/v0.1.0
