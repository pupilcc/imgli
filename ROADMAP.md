# imgli public roadmap (mirror)

> **Planning SSOT is internal** (`imgli/roadmap` in the project knowledge base).  
> This file is a **public, de-sensitized mirror** for contributors. Prefer GitHub Issues + Milestones for execution.

## Latest shipped — v0.9.2 · Instant reuse · site name · stats honesty

Release: https://github.com/yixian-huang/imgli/releases/tag/v0.9.2

Same-user instant reuse (no library dup / double quota), site_name on nav
wordmark, storage policy live/trash object stats. See
[CHANGELOG](CHANGELOG.md#092---2026-08-04) and acceptance
[docs/superpowers/plans/2026-08-04-v0.9.2-acceptance.md](docs/superpowers/plans/2026-08-04-v0.9.2-acceptance.md).

## Previous — v0.9.1 · Admin groups UX polish

Release: https://github.com/yixian-huang/imgli/releases/tag/v0.9.1

Groups save toasts, lifecycle badges (max vs force separate), collapsible form,
expiry UI in days. See [CHANGELOG](CHANGELOG.md#091---2026-08-03).

## Previous — v0.9.0 · Group lifecycle ops · stock clamp

Release: https://github.com/yixian-huang/imgli/releases/tag/v0.9.0

User-group expiry/max-views caps, retention & force-max-age, stock lifecycle
preview/apply, admin batch trash/purge, cleanup kinds, docs/CLI. See
[CHANGELOG](CHANGELOG.md#090---2026-08-03) and
[docs/user-groups-lifecycle.md](docs/user-groups-lifecycle.md).

## Previous — v0.8.0 · Admin image ops · delete clarity

Release: https://github.com/yixian-huang/imgli/releases/tag/v0.8.0

Admin storage locate (policy/driver/path), permanent purge vs trash, guest
auto-purge, clearer delete UX. See [CHANGELOG](CHANGELOG.md#080---2026-08-02).

## Previous — v0.7.x · Ops console · WebDAV · upgrade

- **v0.7.4** — WebDAV mount discovery on failed probe  
- **v0.7.3** — OpenList WebDAV read via 302  
- **v0.7.2** — One-click upgrade + admin shell UX  
- **v0.7.1** — Storage probe reliability  
- **v0.7.0** — Ops Console · Health · Deploy  

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
