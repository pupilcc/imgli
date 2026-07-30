# WebDAV storage matrix

> SSOT: this file is the **engineering** matrix. Product docs should **link**
> here rather than duplicate the table. See [documentation-ssot.md](documentation-ssot.md).

imgli’s **WebDAV** driver (`tier=supported`) speaks standard HTTP WebDAV
operations used by the image host: `PUT`, `GET`/`HEAD`, `DELETE`, and `MKCOL`
for parent collections. It is **not** S3-equivalent (no presign, CDN not
recommended).

This page tracks **how to verify** a server and what we have seen. Prefer
**self-hosted** stacks for reports—you do **not** need a commercial SaaS
account for each brand.

## Do you need to register every vendor?

**No.**

| Approach | When | Account? |
|----------|------|----------|
| **Docker / compose self-host** (Nextcloud, Apache `mod_dav`, OpenList, …) | Default for matrix rows | None (local) |
| **Your own already-running Dav** (NAS, panel, OpenList in prod) | Real ops evidence | Yours only |
| **Commercial SaaS WebDAV** (e.g. some netdisks) | Optional community reports | Only if you already use them |

A **Verified** row means: Put → Open (incl. mid-file Seek) → Delete succeeded
against that stack, with notes on HEAD/Range. It does **not** require signing up
for every cloud product.

## How to test (live)

1. Point a WebDAV root at a **dedicated empty directory** (no production data).
2. Export (never commit secrets):

```bash
export IMGLI_TEST_WEBDAV_LIVE=1
export IMGLI_TEST_WEBDAV_ENDPOINT='http://127.0.0.1:8080/dav'  # no userinfo in URL
export IMGLI_TEST_WEBDAV_USERNAME='user'   # optional
export IMGLI_TEST_WEBDAV_PASSWORD='pass'   # optional
```

3. Run:

```bash
./scripts/webdav-vendor-matrix.sh
# or:
go test ./internal/storage/webdav/ -run TestDriverSurfaceLive -v -count=1
```

4. Open a GitHub issue or PR updating the table below (no credentials).

## Matrix

| Stack | Auth | PUT+MKCOL | HEAD+CL | Range 206 | Notes | Status | imgli |
|-------|------|-----------|---------|-----------|-------|--------|-------|
| In-tree HTTP mock | Basic | yes | yes | yes | Unit tests | **Verified** (CI) | ≥0.1 |
| Apache `mod_dav` | Basic | yes (403→MKCOL) | yes* | varies | *some omit CL → buffer fallback | Community welcome | — |
| Nextcloud | Basic/App password | yes | yes* | usually | App password recommended | Community welcome | — |
| OpenList (WebDAV out) | Basic | yes | yes* | usually | Preferred FTP→WebDAV bridge | Community welcome | — |
| 坚果云 / other SaaS | varies | TBD | TBD | TBD | Optional if you already have an account | Untested in-tree | — |

\* Driver falls back to full `GET` buffer when `HEAD` lacks `Content-Length`, and
falls back to full buffer when `Range` is ignored (HTTP 200 on ranged GET).

**Status legend:** Verified = automated or production evidence; Community
welcome = please run the live test and report; Untested = no report yet.

## Capability vs S3 (honest)

| Capability | S3 (imgli) | WebDAV (imgli) |
|------------|------------|----------------|
| Object CRUD | yes | yes |
| Streaming read / TTFB | yes | yes (HEAD+Range or buffer fallback) |
| Private presign | yes | **no** (always via app) |
| Public CDN offload | recommended | **not recommended** |
| Vendor matrix scripts | yes | this page + `webdav-vendor-matrix.sh` |

## Operator tips

- Put credentials in **username/password** fields, never in the endpoint URL.
- Require HTTPS in production when the path is not localhost.
- Prefer **OpenList/rclone** when the real backend is FTP; use WebDAV as the
  imgli-facing protocol ([storage-ftp.md](storage-ftp.md)).
- Caps in admin: `tier=supported`, no presign, CDN warning if `cdn_domain` set.

## Related

- [s3-compatibility.md](s3-compatibility.md) — object storage matrix  
- [storage-ftp.md](storage-ftp.md) — FTP dual track  
- [design/storage-caps-draft.md](design/storage-caps-draft.md) — Caps model  
