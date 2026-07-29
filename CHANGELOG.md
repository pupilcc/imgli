# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

Product version comes from **git tags** (`vMAJOR.MINOR.PATCH`). Do not maintain a
separate version in `go.mod` or `web/package.json`.

## [Unreleased]

### Changed

- **License:** project default changed from MIT to **AGPL-3.0-only** to reduce
  closed SaaS / white-label freeloading of network-served modifications, while
  offering optional **commercial licenses** (see `COMMERCIAL.md`). Tags
  `v0.1.0` and `v0.1.1` remain MIT snapshots.

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
