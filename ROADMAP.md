# imgli public roadmap (mirror)

> **Planning SSOT is internal** (`imgli/roadmap` in the project knowledge base).  
> This file is a **public, de-sensitized mirror** for contributors. Prefer GitHub Issues + Milestones for execution.

## Latest shipped — v0.5.0 · Trust · Onboard · Community

Release: https://github.com/yixian-huang/imgli/releases/tag/v0.5.1 (patch) · [v0.5.0](https://github.com/yixian-huang/imgli/releases/tag/v0.5.0)  
Milestone: https://github.com/yixian-huang/imgli/milestone/4 (**closed**, 14/14)

| Theme | Highlights |
|-------|------------|
| Trust | Cloudflare R2 **Verified** in S3 matrix; moderation operator spot-check docs |
| Onboard | First-run Token path; welcome email (SMTP); scenario copy (`?from=` / UTM) |
| Site ops | Favicon URL, title strategy, optional “based on imgli”, source URL, About page |
| Community | Public `ROADMAP.md`, README docs map; other S3 vendors via community reports |

## Shipped earlier

- **v0.4.x** — Storage Caps, FTP compatibility driver  
- **v0.3.0** — Password shares, public albums, width thumbs, import-dir, OIDC, webhooks  
- **v0.2.x** — CLI upload, doctor, EXIF strip, max views, light admin analytics  

## Next (not a committed schedule)

- **Community:** more S3-compatible vendors in the matrix (OSS/COS/Qiniu/Upyun/AWS…) — use the vendor report template + `scripts/s3-vendor-matrix.sh`  
- **Polish (optional):** README demo GIF, more `good first issue` labels  
- **Later candidates (internal SSOT):** single-instance team/org, fuller transform suite, more IdP connectors  

No open milestone is required for community PRs — file an Issue or open a PR against `main`.

## Non-goals (do not open “white-label full” issues expecting Community)

Full BrandLockup replace, multi-tenant control plane, Open Core paywalls, video mainline, independent short-link product. See `COMMERCIAL.md` for dual-license inquiries.

## Contribute

- Bug / feature: GitHub Issues (templates under `.github/`)  
- S3 vendor reports: [`docs/s3-compatibility.md`](docs/s3-compatibility.md) + issue template  
- Security: [`SECURITY.md`](SECURITY.md)
