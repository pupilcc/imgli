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
| WebDAV | (separate driver) | n/a | yes | n/a | See [webdav-compatibility.md](webdav-compatibility.md) | Unit-tested (+ live script) | ≥0.1 |

**Status legend:** Verified = live or production evidence; Untested in-tree = no
automated report in this repo yet.

## Operator tips

- Prefer **private buckets**; public CDN should front only **public/** keys if you
  use path-prefix surfaces (see [security-hardening.md](security-hardening.md)).
- Set `presign_domain` / CDN domain per storage policy in admin when offloading.
- Path-style is required for many MinIO deployments (`force path style`).

## FTP and legacy vendors (dual track)

Some hosts only expose **FTP** (e.g. certain virtual-host / panel storage). imgli
treats this as a **compatibility** concern, not a first-class object-store path.

1. **Preferred (ops):** front FTP with an external tool, then use a normal imgli
   driver:
   - [OpenList](https://github.com/OpenListTeam/OpenList) (or similar) mount FTP →
     expose **WebDAV** (or sync to disk) → imgli **webdav** / **local** policy
   - **rclone** sync/mount → local disk or S3-compatible bucket → imgli **local** / **s3**
2. **Optional (product):** in-tree **FTP compat driver** (when shipped): same
   `Driver` contract only, `tier=compat`, explicit feature loss (no presign, CDN
   not recommended, not for hot traffic). **No special cases in the serve path.**

Do **not** expect FTP (proxy or in-tree) to match S3 CDN offload or private
presign. Prefer S3-compatible APIs when the vendor offers them (including Upyun
S3).

Design / checklist: [design/storage-caps-draft.md](design/storage-caps-draft.md),
[design/storage-caps-impl-checklist.md](design/storage-caps-impl-checklist.md).

## Out of scope

- FTP as a **first-class** driver (equal to S3 in docs, defaults, or CDN/presign).
- Guaranteeing every regional quirk without a report — file an issue with the template.
