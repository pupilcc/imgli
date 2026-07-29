# Security Policy

## Reporting a Vulnerability

Please **do not** open a public issue for security vulnerabilities.

Report privately via GitHub's private vulnerability reporting:
**Security → Report a vulnerability** on this repository.

You can expect an initial response within a week. Please include reproduction
steps and the affected version/commit.

## Supported Versions

Only the latest release tag (and `main`) receives security fixes. There is no
LTS branch while the project is on `0.x`. Check the running build with
`imgli version`.

## Scope Notes for Self-Hosters

- Image keys are 12-char random base62; **private images rely on key
  unguessability at the object-storage layer** if your bucket allows anonymous
  `GetObject`. Keep buckets private where possible and never enable anonymous
  `ListBucket`.
- The OCR sidecar (`deploy/ocr-paddle/`) must be protected with both
  `ALLOW_IPS` and a Bearer `TOKEN`, plus a cloud firewall if available.
- Run the app behind a reverse proxy with TLS; set `trust_proxy: true` only
  when the proxy is trusted (it controls client-IP attribution for rate
  limiting).
