# Contributing to imgli

Issues and pull requests are welcome — bug reports, S3-compatible vendor test
results, translations, and features alike.

## Development setup

```bash
make build       # web (npm) + Go binary with embedded frontend
make run         # build + ./imgli serve  (http://localhost:8686)
make test        # go vet + go test
make test-web    # vitest
```

Go ≥ 1.26 and Node ≥ 24 are expected (see `.github/workflows/ci.yml`, which is
the source of truth). The default build is pure Go (CGO-free, SQLite included);
`make build-vips` enables libvips-backed WebP thumbnails if you have vips dev
headers installed.

## Guidelines

- Keep PRs focused; include tests for behavior changes. CI must pass
  (sqlite + postgres matrix, web tests, e2e smoke).
- The codebase comments are written in Chinese; either language is fine in
  contributions, but please match the style of the file you are editing.
- S3 vendor reports: run `scripts/s3-vendor-matrix.sh` and
  `scripts/s3-vendor-e2e.sh` against your vendor (see
  `scripts/imgli-s3-vendors.env.example`) and open an issue with the results —
  these directly improve the compatibility matrix.
- Security issues: see [SECURITY.md](SECURITY.md) — never via public issues.
