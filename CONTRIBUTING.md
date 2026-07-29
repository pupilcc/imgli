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
- User-facing changes: add a bullet under **`[Unreleased]`** in
  [CHANGELOG.md](CHANGELOG.md). Mark breaking changes clearly (and prefer
  `BREAKING` in the PR title).

## Versioning and releasing

Product version is **only** the git tag (`vMAJOR.MINOR.PATCH`). Build injects it
via `-ldflags` (`make build` / Docker `VERSION` / GoReleaser). Do not duplicate
it in `go.mod` or `web/package.json`.

| Change | Bump |
|--------|------|
| Breaking API/config/storage semantics | MAJOR (or MINOR while still `0.x`) |
| New features | MINOR |
| Bug fixes, security patches | PATCH |

Internal DB `SchemaVersion` is independent of the product tag; call out
migrations in the changelog when operators must act.

### Maintainer release checklist

1. `main` is clean and CI is green.
2. Move `[Unreleased]` notes in `CHANGELOG.md` into a dated `## [X.Y.Z]` section;
   refresh the compare links at the bottom; leave an empty `[Unreleased]`.
3. Commit on `main`, then tag and push:

   ```bash
   git tag -a vX.Y.Z -m "vX.Y.Z"
   git push origin main --tags
   ```

4. GitHub Actions [release](.github/workflows/release.yml) will:
   - build multi-platform binaries with GoReleaser and publish a GitHub Release;
   - build/push multi-arch images to `ghcr.io/yixian-huang/imgli` with tags
     `vX.Y.Z`, `X.Y.Z`, `X.Y`, and `latest` (not for pre-releases containing `-`).

5. Smoke-check: download a release asset or pull the image and run
   `imgli version` — it should print `vX.Y.Z`.

Local dry-run (requires [GoReleaser](https://goreleaser.com/) installed):

```bash
make release-snapshot
```
