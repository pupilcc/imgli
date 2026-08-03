# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

Product version comes from **git tags** (`vMAJOR.MINOR.PATCH`). Do not maintain a
separate version in `go.mod` or `web/package.json`.

## [Unreleased]

## [0.9.0] - 2026-08-03

Theme: **Group lifecycle ops · admin stock clamp · cleanup observability**.

### Added

- **Admin images batch:** `POST /admin/images/batch` `{keys, action: trash|purge}` (max 100); list multi-select + batch bar; per-card permanent delete on hover.
- **User-group lifecycle / upload options:** `default_expires_in`, `max_expires_in`, `default_max_views`, `max_max_views`, `retention_days`, `force_max_age_days` on groups; enforced at upload + image PATCH; exposed on `/user/quota` and guest `/config`; admin Groups UI; hourly soft-delete by retention and hard purge by force max age. Guest seed defaults: 1d default / 7d max / 7d force age.
- **Image detail access presets:** library detail modal filters expiry / max-views Segmented options by the same group caps as the upload page; hides “remove expiry” when permanent is forbidden; out-of-policy banner + apply group max; dynamic cap presets.
- **Group stock lifecycle:** `POST /admin/groups/{id}/lifecycle/preview|apply` clamps permanent/over-cap live images to now+cap; Groups UI preview/apply + list badges + stock-only warning.
- **Cleanup kinds:** `group_retention`, `group_force_age` in admin cleanup preview/run (System page includes them by default).
- **Error codes:** `expires_over_group`, `max_views_over_group` with i18n mapping.
- **Review queue:** purge-all-on-page (permanent delete batch).
- **Docs:** [docs/user-groups-lifecycle.md](docs/user-groups-lifecycle.md); PicGo/ShareX/CDN cleanup notes; CLI `imgli upload -verbose` prints group limits.

## [0.8.0] - 2026-08-02

Theme: **Admin image ops · delete clarity** — storage locate, trash vs permanent delete, guest purge.

### Added

- **Admin image storage locate:** list/detail expose `policy_id`, `policy_name`, `policy_driver`, `surface`, and object `path` (copy in detail) so operators can find WebDAV/S3/local objects.
- **Admin permanent delete:** `DELETE /api/v1/admin/images/{key}?permanent=1` hard-deletes DB rows and enqueues `delete_file` for storage cleanup; response includes `physical_queued` / `object_retained` (instant-upload shared refs).
- **Admin trash scope:** `GET /api/v1/admin/images?deleted=live|trash|all` (default live); UI filter + trash badge.
- **Guest upload delete:** guests have no owner trash — admin default delete is permanent purge.
- **Audit:** `image_admin_purge` with owner/policy/path and physical-delete flags.

### Fixed

- **Library delete two-click UX:** card/list quick-delete arms with a visible **确认 / OK** label (was easy to miss as “no reaction”).
- **Trash cache after soft-delete:** user delete / batch delete invalidates the trash query so the recycle bin updates immediately.
- **AdminPurge race:** soft-delete-then-restore race no longer returns a silent success without purging.

### Changed

- User-facing copy treats soft-delete as **move to trash** (card, batch bar, toasts); detail already used “add to trash”.
- Admin list hover: clearer trash vs permanent labels; success toasts for soft vs hard delete (including shared-object retained / queue failure).
- Whitelist armed button shows a short confirm label.

## [0.7.4] - 2026-08-01

Theme: **WebDAV mount discovery on failed probe**.

### Added

- **WebDAV test-connection mount discovery (P0/P1):** when the write probe fails, imgli PROPFIND Depth:1 lists child collections and write-probes each (max 8), then appends copy-paste endpoint suggestions (e.g. OpenList virtual `/dav` → `…/dav/<mount>`). If discovery finds nothing, a short OpenList virtual-root hint is still shown.

## [0.7.3] - 2026-08-01

Theme: **OpenList WebDAV read via 302**.

### Fixed

- **WebDAV Open/Exists against OpenList (and similar netdisk proxies):** write could succeed while "Test connection" failed on read-back because the peer returns **302** to a presigned object URL, and **HEAD on that URL often 403**. imgli now treats HEAD 302 as "exists / use buffered GET", follows **GET** redirects only (strips Basic auth), and leaves PUT/DELETE unfollowed. Verified against a live OpenList mount that fronts China Mobile EOS.

### Changed

- Clearer PUT 404 wording when the path is missing or the WebDAV root is not writable (e.g. OpenList virtual `/dav` root).

## [0.7.2] - 2026-08-01

Theme: **One-click upgrade + admin shell UX**.

### Fixed

- **One-click binary upgrade under systemd:** preflight checks that the binary directory is writable; clear error when `ProtectSystem=strict` leaves the bin path read-only (was a silent production failure). Successful upgrades **re-exec** the new binary so the version changes without a manual restart.
- **Update check:** fall back from HTTP `HEAD` to `GET` when resolving GitHub `releases/latest` (some networks omit `Location` on HEAD).
- **Doctor:** new `binary_upgrade` check reports whether in-place upgrade can write next to the running executable.

### Changed

- **Admin layout:** fixed viewport shell — top header stays put, left nav stays put, only the main content scrolls; page title + filter row (`PageHeader`) sticky within the content pane.
- **Ops docs:** `deploy/imgli.service.example` documents `ReadWritePaths` for both data dir and binary dir (required for admin upgrade).

## [0.7.1] - 2026-08-01

Theme: **Storage probe reliability** — fix local test-connection path and clearer remote probe errors.

### Fixed

- **Local storage “Test connection”:** probe now resolves `config.root` with the same rules as real uploads and doctor (`storage.LocalRoot` under `data_dir`; absolute roots unchanged). Fixes Docker/non-root false failures like `root 不可写: mkdir uploads: permission denied` when `/data/uploads` is actually writable.
- **WebDAV/S3/FTP probe key:** write probe objects under `imgli-probe/…` so WebDAV exercises parent `MKCOL` (closer to real upload paths; more friendly to OpenList-style servers).
- **Probe error messages:** one readable sentence with path or endpoint plus a short hint for common cases (permission, auth, unreachable, 404); audit stores `error` text; avoid noisy structured payloads.

### Changed

- Admin `UseDataDir` wires `cfg.DataDir` into policy test probes so local relative roots match production layout.

## [0.7.0] - 2026-08-01

Theme: **Ops Console · Health · Deploy** — admin-visible self-host diagnostics, reverse-proxy clarity, unified three-step setup UI.

### Added

- **Admin system health (H1/H2):** `GET /api/v1/admin/system/health` returns doctor checks (same as CLI `imgli doctor`) plus read-only runtime summary (`base_url`, `trust_proxy`, listen, install shape, request Host / forwarded headers). (#74, #75, #80)
- **Admin System / Ops page:** health table, browser vs `base_url` mismatch banner (reverse-proxy CSRF), first-run checklist, version upgrade with preflight notes, lifecycle cleanup UI, links to migrate/backup. Nav **系统 / 运维**. (#74–#78, #80)

### Changed

- **Onboarding UI:** shared `StepGuide` for upload first-run and Settings → API Token “three-step setup” (console design language: mono kicker, numbered steps, shared buttons). (#81)
- **Docs:** reverse-proxy CSRF FAQ in README / security-hardening and product FAQ (docs.imgli.com); ROADMAP points at v0.7.0 milestone.

## [0.6.0] - 2026-07-31

Theme: **Ops · Migrate · Trust** — admin storage migrate jobs, lifecycle cleanup, version probe/upgrade.

### Added

- **Storage migrate safety (M2):** process-local mutex per source policy (`ErrMigrateBusy`); disabled target returns `ErrMigrateTargetDisabled`; `MigrateResult.Progress()` / `RedactStoragePath` for admin-safe status (counts + redacted paths, no policy secrets). (#53)
- **Admin storage migrate jobs (M1):** `POST/GET /api/v1/admin/storage/migrate` with batch cursor resume, progress polling, and policies UI wizard (dry-run / delete-source / limit). CLI `storage-migrate` unchanged. (#54)
- **Docs:** `docs/storage-migrate.md` documents Admin migrate path, API, and operator acceptance sketch. (#55)
- **Admin version + update probe (U1):** `GET /admin/system/version` and operator-triggered `POST /admin/system/check-update` (GitHub `releases/latest`); dashboard shows running version. Build injects `internal/version.Version` via ldflags. (#56)
- **Admin one-click binary upgrade (U2):** `POST /admin/system/upgrade` with `confirm=true`, checksum-verified GitHub Release asset, in-place binary replace + restart guidance; Docker/container installs refuse binary replace. (#57)
- **Storage migrate filters + size verify (M3):** optional `user_id` / created time window; post-Put size check blocks silent policy flip on mismatch. (#58)
- **Admin lifecycle cleanup (L1):** `POST /admin/cleanup/preview` and `POST /admin/cleanup/run` (confirm required) for expired images and old trash; dry-run samples image keys. (#59)
- **Docs (P2):** cleanup vs CDN boundary, OIDC operator troubleshooting, migrate estimate note. (#61, #62, #63)

## [0.5.1] - 2026-07-31

Theme: **Patch — dark onboarding + storage-migrate docs**

### Fixed

- **Dark theme:** first-run Token onboarding card uses design tokens (readable on dark backgrounds). (#50)

### Added

- **Docs:** `docs/storage-migrate.md` (CLI cutover) + `docs/design/storage-migrate-sync-draft.md` (multi-policy migrate vs sync roadmap).

## [0.5.0] - 2026-07-31

Theme: **Trust · Onboard · Community**

### Added

- **Site ops:** optional `favicon_url`, `source_url` (AGPL corresponding source), `oss_credit` footer toggle, About page (`about_enabled` / `about_body`), `welcome_email` on register when SMTP configured.
- **Onboarding:** dismissible first-run Token path on upload page; auth scenario copy via `?from=` / `utm_campaign`.
- **Preferences:** `auto_copy_format=share` copies share page URL after upload.
- **Docs:** `docs/moderation-spot-check.md`, public `ROADMAP.md` mirror, README docs map.

### Changed

- **Document title:** custom `site_name` is primary (product brand only when still default img.li).
- **WebDAV:** Open falls back to full GET when HEAD lacks `Content-Length` or is unsupported; Range-ignored servers fall back to full-object buffer on mid-file read; clearer auth/status errors.
- **Docs:** `docs/webdav-compatibility.md` + `scripts/webdav-vendor-matrix.sh` (self-hosted live probe; no SaaS signup required for a matrix row).
- **Docs SSOT:** `docs/documentation-ssot.md` — layered truth (repo engineering vs product `docs-imgli` vs internal KB); CONTRIBUTING pointer.
- **Docs:** Cloudflare R2 matrix row marked **Verified** (live surface + Presign GET, 2026-07-31; post-tag docs commit on main).

## [0.4.1] - 2026-07-30

Theme: **FTP hot-path polish (compat)**

### Changed

- **FTP driver:** in-package control connection pool + remembered TLS mode (no dual-dial per request); still compat-tier, no serve changes.
- **FTP Open streaming:** use `SIZE` + lazy `RETR`/`REST` for better TTFB; full-buffer fallback when `SIZE` unsupported.

### Fixed

- **e2e:** policies admin assert `本地磁盘` with `exact: true` (Caps summary collision).

## [0.4.0] - 2026-07-30

Theme: **Storage Caps · FTP · Detail UX**

### Added

- **Storage Caps:** each storage policy exposes `tier` / `caps` / `effective` / `warnings` on admin API; policy UI shows capability panel and CDN/compat warnings (advisory; serve path unchanged).
- **FTP compatibility driver:** optional `ftp` backend (FTPS preferred, `allow_insecure` for plain FTP); no serve special-cases. Prefer OpenList/proxy — see `docs/storage-ftp.md` and dual-track notes in `docs/s3-compatibility.md`.
- **CDN domain validation:** `cdn_domain` must be http(s) origin/path prefix without userinfo/query/fragment.
- **Doctor:** storage caps / CDN-vs-cap / insecure transport / compat-only checks.
- **Design:** `docs/design/storage-caps-draft.md` + implementation checklist.
- **Detail modal density:** sticky header/actions; primary copy+QR first; access control & stats in collapsible `<details>`; body scroll lock + fixed modal height so pane scrolls (not the page behind). Redesigned access help and layout polish (#34).
- **e2e:** guest-off landing stays on `/` with login-gate; specs register via `/login` instead of expecting hard redirect.
- **Admin slots density:** sub-tabs (CTA / announcement / footer / HTML); zh|en side-by-side; schedule fields folded.
- **Site slots UI polish:** announcement bar soft strip (NOTICE kicker, chip CTA, dismiss control) aligned with Nav/Quota; footer links use equal-width `auto-fit` columns (not left-packed `auto-fill`) + centered meta; no brand billboard.
- **Locale-safe slots:** public config announcement/footer/`register_notice` accept `{zh,en}` maps (legacy string still OK); `pickLocale` in SPA. Regression tests in `SiteSlots.test.tsx` / `locale.test.ts`. Ops checklist: `docs/ops-deploy-checklist.md`.
- **Operator-owned public copy:** settings `help_url`, `upgrade_url`, `register_notice`, `share_branding` (`off|site|links`) on admin + public `/config` for help/upgrade CTAs (defaults empty). **Product brand kept:** SPA meta/title `img.li · 图鲤`, auth copyright 开源 imgli, share footer always shows OSS credit (imgli · 图鲤); `share_branding` only toggles instance name/links (default `site`).
- **Share UX:** detail modal lists share page URL first (public); upload success row copies share page; access password Generate + copy before save; clearer hint that existing  links become gated.
- **Ops helpers:** `scripts/ops-set-public-slots.py` (slots + public copy for img.li), `scripts/ops-patch-imglicom-home.py` (+ JSON); `docs/community/post-drafts-zh.md` for launch posts.

### Fixed

- **Detail modal scroll:** opening 访问控制 / 访问统计 no longer scrolls the page behind; body is fixed-locked and the right pane is a real overflow container.
- **Plaza switch lag:** admin `PutSettings` now uses the process-shared `settings.Service`, so `plaza_enabled` cache invalidates immediately (fixes e2e 404 race and up-to-30s stale enable).
- **CI unit/e2e:** default theme is `system` (store test); Nav quota `getAllByText(/GB/)`; e2e registers via `/login` and guest-off asserts login-gate on `/`.

## [0.3.0] - 2026-07-30

Theme: **Share · Migrate · Integrate**

### Added

- **OIDC login (generic):** admin GET/PUT `/admin/oidc`; `/auth/oidc/start` + callback; auto-provision user by email; `oidc_enabled` on public config.
- **Outbound webhooks:** admin GET/PUT `/admin/webhooks`; async `image.uploaded` / `image.moderated` with optional HMAC `X-Imgli-Signature`.
- **Admin users ops:** filter by signup `channel`, sort by `bandwidth`, CSV export `/admin/export/users.csv`.
- **Open Graph meta** on SPA `/s/{key}` and `/a/{id}` HTML shell for crawlers (passworded shares omit image).
- **Theme `system`**: cycle light → dark → system in the UI toggle.
- **CLI `imgli import-dir`:** bulk-import a local directory via upload API (recursive, dry-run, continue-on-error).
- **Public album visitor page:** `GET /api/v1/a/{id}` + `/a/{id}/images` and SPA `/a/:id` (public+normal images only).
- **Controlled thumbnail width query:** `GET /t/{name}?w=200|400|800` with disk cache keys under `.thumbs/w{N}/` (isolated from content-hash); invalid `w` → 400.
- **Access password for images:** optional password gate on `/i`/`/t` and share page; argon2 hash at rest; unlock via `POST /api/v1/s/{key}/unlock` cookie or `X-Image-Password`; no public CDN when set; detail UI to set/clear.

- **Admin ops dashboard (light analytics):** signup trend + coarse channels
  (`direct|invite|utm|referer`); traffic 7d/30d; referer Top with window toggle,
  suspect flag, host → top images; bandwidth period sum + top users; origin-only
  metering footnote.
- **Signup attribution (register-time only):** optional `utm_*` / `referer_host`
  on `POST /auth/register`; SPA passes URL UTM + `document.referrer` host.
- **Referer image rollup:** `referer_image_stats` + `GET /api/v1/admin/referers/images`.
- **Stats retention:** rolling 90-day purge of access/referer tables.
- **Ops docs:** `deploy/ops/admin-stats-metering.md` (CDN under-count boundary).
- **`imgli doctor`:** WARN when enabled policies set `cdn_domain` (dashboard ≠ CDN bill).

## [0.2.0] - 2026-07-29

Theme: **Workflow & Trust** — CLI/integrations, share landing, privacy
(EXIF strip, max views), ops (`doctor`, compose/backup docs), README demos.

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
- **Upload success polish:** large primary URL + one-click copy, multi-format
  chips, and “open share page” for public uploads.
- **Strip EXIF/GPS on upload (default on):** site `processing.strip_exif`
  (nil/missing = on); re-encode JPEG/PNG before scale/watermark; admin toggle;
  content-hash after strip.
- **Max views / burn-after-read:** `max_views` + `views_served` on images;
  non-owner `/i` atomic claim; exhausted → 410; limited images skip public CDN
  302; upload/detail UI presets (1/3/10). Multi-instance needs shared DB.
- **`imgli doctor`:** self-host diagnostics for data dir, DB, base_url,
  trust_proxy, listen, and local storage policy probes (`internal/doctor`).
- **Ops docs:** production Compose example, Caddy/Traefik snippets, backup &
  restore guide (`deploy/compose.prod.example.yml`, `docs/backup.md`).
- **README demos:** product screenshots (upload / share / admin) and honest
  comparison table vs Lsky Pro / Chevereto (`docs/screenshots/`).
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

[Unreleased]: https://github.com/yixian-huang/imgli/compare/v0.8.0...HEAD
[0.8.0]: https://github.com/yixian-huang/imgli/compare/v0.7.4...v0.8.0
[0.7.4]: https://github.com/yixian-huang/imgli/compare/v0.7.3...v0.7.4
[0.7.3]: https://github.com/yixian-huang/imgli/compare/v0.7.2...v0.7.3
[0.7.2]: https://github.com/yixian-huang/imgli/compare/v0.7.1...v0.7.2
[0.7.1]: https://github.com/yixian-huang/imgli/compare/v0.7.0...v0.7.1
[0.7.0]: https://github.com/yixian-huang/imgli/compare/v0.6.0...v0.7.0
[0.6.0]: https://github.com/yixian-huang/imgli/compare/v0.5.1...v0.6.0
[0.5.1]: https://github.com/yixian-huang/imgli/compare/v0.5.0...v0.5.1
[0.5.0]: https://github.com/yixian-huang/imgli/compare/v0.4.1...v0.5.0
[0.3.0]: https://github.com/yixian-huang/imgli/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/yixian-huang/imgli/compare/v0.1.1...v0.2.0
[0.1.1]: https://github.com/yixian-huang/imgli/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/yixian-huang/imgli/releases/tag/v0.1.0
