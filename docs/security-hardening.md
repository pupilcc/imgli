# Security hardening for self-hosters

Companion to [SECURITY.md](../SECURITY.md). Focus: **private images**, reverse
proxy, and moderation sidecars.

## Threat model (short)

| Asset | Typical risk |
|-------|----------------|
| Public image keys (`/i/{key}`) | Enumeration is hard (random key) but **not** a secret capability |
| Private images | App checks session ownership; **object store must not allow anonymous Get** |
| Pending / rejected | App returns placeholder to non-owners; do not expose raw objects |
| Admin / SMTP / moderation keys | Compromise of admin session or DB |

## Object storage (private surface)

imgli prefixes objects by surface (`public/` vs `private/` when using path
layout). Application routes:

- Public: may `302` to CDN / public URL.
- Private: may `302` to a **short-lived presigned URL**, or stream via the app.

**You still must:**

1. Disable anonymous `GetObject` and `ListBucket` on the bucket.
2. Never point a **public CDN** at the same origin path that can serve
   `private/*` without signing.
3. Prefer a dedicated bucket or IAM policy that only the imgli process can read
   private keys.
4. Treat previews/logs: do not paste full private URLs into public issues.

Application-level gate (owner session / API token scope) is **necessary but not
sufficient** if the bucket is world-readable. Hardening the bucket is “S4” in
the product roadmap sense: object-layer enforcement.

### Application S4 slice (in-tree)

Defense in depth already in code (not a substitute for bucket ACLs):

- Object keys use `public/` vs `private/` prefixes after surface migration (S1).
- **CDN 302** (`ObjectURL`) **never** builds URLs for keys under `private/`
  (`CDNEligibleObjectKey` / serve triple-check: visibility + surface + key).
- Private images may **presign** (short TTL) or **stream via the app**, never
  unauthenticated CDN for private keys.

### Operator probe

After pointing a storage policy at S3/CDN, pick any private object URL (or a
deliberate test object under `private/`) and:

```bash
./scripts/probe-private-object-anon.sh 'https://your-bucket-or-cdn/private/.../object.ext'
```

Expect **non-2xx** for anonymous GET. HTTP 200 here means S4 failure at the
bucket/CDN layer.

### Checklist

- [ ] Bucket private; no public ACL on `private/`
- [ ] No open `ListBucket`
- [ ] CDN domain only for public offload (or signed CDN, if you use one)
- [ ] CDN origin must not serve `private/*` anonymously
- [ ] Run `scripts/probe-private-object-anon.sh` on a private key after deploy
- [ ] Presign TTL left short (app default is brief; do not cache signed URLs)
- [ ] Backups of DB + objects both access-controlled

## Reverse proxy

- Terminate TLS at the proxy; set `IMGLI_TRUST_PROXY=true` **only** when the
  proxy is trusted (rate limits and audit IPs depend on it).
- Set `IMGLI_BASE_URL` (or YAML `base_url`) to the **exact public origin** users
  type in the browser (`https://img.example.com`, no trailing path). This is
  used for generated links/emails, `Secure` session cookies, **and** the CSRF
  Origin allowlist for browser cookie auth.
- Forward the original `Host` (or equivalent) and prefer
  `X-Forwarded-Proto` / `X-Forwarded-For`. Examples:
  [`deploy/Caddyfile.example`](../deploy/Caddyfile.example),
  [`deploy/traefik.labels.example.yml`](../deploy/traefik.labels.example.yml),
  [`deploy/compose.prod.example.yml`](../deploy/compose.prod.example.yml).
- Do not expose the OCR / NSFW sidecars to the internet without IP allowlist
  and bearer token (`deploy/ocr-paddle/`).

### FAQ: reverse proxy login/register (cross-site rejected)

**Symptom:** After putting Nginx / Caddy / Traefik / **1Panel** (or similar) in
front, registration and login fail with HTTP 403 and message
`跨站请求被拒绝` (cross-site request rejected). Accessing the app via raw
`http://IP:port` works.

**Cause:** Browser POSTs send an `Origin` header (e.g. `https://your.domain`).
imgli’s `OriginCheck` allows the write only if Origin matches either:

1. `scheme://` + request `Host` (scheme is `https` only when the app itself
   terminates TLS — **not** when TLS ends at the reverse proxy), or
2. the origin derived from **`IMGLI_BASE_URL`**.

Behind HTTPS reverse proxy the first check is often
`http://your.domain` vs browser `https://your.domain`, so it fails. If
`IMGLI_BASE_URL` is still the default (`http://localhost:8686`) or an IP:port
URL, the second check fails too → 403. Direct IP:port access works because
Origin and Host both stay `http://IP:port`.

**Fix:**

1. Set `IMGLI_BASE_URL=https://your.domain` (same scheme + host users open).
2. Set `IMGLI_TRUST_PROXY=true` when the proxy is trusted.
3. Restart the process/container.
4. Confirm the proxy preserves the public host (and does not strip needed
   forwarded headers).

Optional check: `imgli doctor` warns when `base_url` is still localhost-shaped;
it does not print this exact error string. **v0.7+** admins can also open
**System / Ops** in the console (`GET /api/v1/admin/system/health`) for the same
checks plus a browser vs `base_url` mismatch banner.

## Content safety

- Fail-open vs fail-closed is a **policy choice** in admin moderation settings;
  know which you run on a public demo.
- Pending images must not be hotlinked by guests (app gate); still keep object
  storage locked down.
- See operator runbooks in the private knowledge base / ops docs for production
  wording; do not commit production secrets here.

## Reporting

Vulnerabilities: GitHub private security advisory — not public issues.
