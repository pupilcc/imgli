# S3-compatible storage matrix

imgli talks to object storage through the S3 API (path-style or virtual-host).
This page tracks **what we have verified** and how to contribute a report.

## How to test

1. Create a **dedicated test bucket** (anonymous `GetObject` / `ListBucket` off).
2. Copy `scripts/imgli-s3-vendors.env.example` → `~/.secrets/imgli-s3-vendors.env`
   and fill one vendor prefix (never commit secrets).
3. Run:

```bash
# example: minio | qiniu | cos | oss | upyun (see script)
./scripts/s3-vendor-matrix.sh minio
# optional deeper e2e
./scripts/s3-vendor-e2e.sh   # see script header for env
```

4. Open a GitHub issue with the **S3 vendor test report** template, or a PR that
   updates the table below (no credentials).

## Matrix (community + maintainer)

| Vendor | Endpoint style | Path-style | Put/Get/Delete | Presign GET | Notes | Status | imgli |
|--------|----------------|------------|----------------|-------------|-------|--------|-------|
| MinIO | custom host | yes | yes | yes | Local / CI-friendly | **Verified** (maintainer toolkit) | ≥0.1 |
| RustFS | S3-compatible | varies | yes | yes | Production path on img.li ops docs | **Verified** (self-host ops) | ≥0.1 |
| Cloudflare R2 | `*.r2.cloudflarestorage.com` | often no | TBD | TBD | High demand — **contributions welcome** | Untested in-tree | — |
| AWS S3 | regional | no (typical) | TBD | TBD | Baseline expectation | Untested in-tree | — |
| Aliyun OSS (S3 API) | `oss-*.aliyuncs.com` | no | TBD | TBD | Template in env example | Untested in-tree | — |
| Tencent COS (S3 API) | `cos.*.myqcloud.com` | no | TBD | TBD | Bucket often `name-APPID` | Untested in-tree | — |
| Qiniu Kodo (S3 API) | `s3.*.qiniucs.com` | no | TBD | TBD | Template in env example | Untested in-tree | — |
| Upyun (S3 API) | vendor docs | TBD | TBD | TBD | Prefer S3 endpoint over FTP | Untested in-tree | — |
| WebDAV | (separate driver) | n/a | yes | n/a | Not S3; listed for completeness | Unit-tested | ≥0.1 |

**Status legend:** Verified = live or production evidence; Untested in-tree = no
automated report in this repo yet.

## Operator tips

- Prefer **private buckets**; public CDN should front only **public/** keys if you
  use path-prefix surfaces (see [security-hardening.md](security-hardening.md)).
- Set `presign_domain` / CDN domain per storage policy in admin when offloading.
- Path-style is required for many MinIO deployments (`force path style`).

## Out of scope

- FTP as a first-class driver (roadmap: not planned; use S3-compatible if available).
- Guaranteeing every regional quirk without a report — file an issue with the template.
