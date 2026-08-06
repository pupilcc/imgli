#!/usr/bin/env sh
# Gate before `git tag vX.Y.Z`: CHANGELOG section + clean tree + main CI green.
#
# Usage:
#   ./scripts/pre-tag-check.sh v0.9.6
#   ./scripts/pre-tag-check.sh v0.9.6 --skip-ci   # only CHANGELOG + git state
set -eu

TAG="${1:-}"
SKIP_CI=0
[ "${2:-}" = "--skip-ci" ] && SKIP_CI=1

if [ -z "$TAG" ]; then
  echo "usage: $0 vMAJOR.MINOR.PATCH [--skip-ci]" >&2
  exit 2
fi

case "$TAG" in
  v[0-9]*.[0-9]*.[0-9]*) ;;
  *)
    echo "tag must look like v0.9.5, got: $TAG" >&2
    exit 2
    ;;
esac

VER="${TAG#v}"
ROOT="$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

# --- git state ---
branch="$(git rev-parse --abbrev-ref HEAD)"
if [ "$branch" != "main" ]; then
  echo "warn: not on main (on $branch); release tags should usually be cut from main" >&2
fi

if [ -n "$(git status --porcelain)" ]; then
  echo "error: working tree not clean; commit or stash first" >&2
  git status --short >&2
  exit 1
fi

if git rev-parse -q --verify "refs/tags/$TAG" >/dev/null 2>&1; then
  echo "error: tag $TAG already exists locally" >&2
  exit 1
fi

# --- CHANGELOG ---
if ! grep -qE "^## \[${VER}\]" CHANGELOG.md; then
  echo "error: CHANGELOG.md missing section: ## [${VER}]" >&2
  echo "move Unreleased notes under that heading before tagging" >&2
  exit 1
fi

# --- remote CI on main (optional) ---
if [ "$SKIP_CI" -eq 0 ]; then
  if ! command -v gh >/dev/null 2>&1; then
    echo "error: gh CLI required for CI gate (or pass --skip-ci)" >&2
    exit 1
  fi
  # Latest completed CI run on main must be success
  line="$(gh run list --workflow=ci.yml --branch main --limit 5 --json status,conclusion,headSha,databaseId \
    -q '[.[] | select(.status=="completed")][0] | "\(.conclusion) \(.headSha) \(.databaseId)"' 2>/dev/null || true)"
  if [ -z "$line" ]; then
    echo "error: could not query CI runs for main" >&2
    exit 1
  fi
  conc="$(echo "$line" | awk '{print $1}')"
  sha="$(echo "$line" | awk '{print $2}')"
  rid="$(echo "$line" | awk '{print $3}')"
  head="$(git rev-parse HEAD)"
  if [ "$conc" != "success" ]; then
    echo "error: latest completed main CI is not success: $conc (run $rid)" >&2
    exit 1
  fi
  if [ "$sha" != "$head" ]; then
    echo "warn: latest green CI is for $sha, HEAD is $head — push main and wait for green before tagging" >&2
    # soft fail only if HEAD is ancestor? require exact match for hard gate
    echo "error: refuse to tag; HEAD must match latest successful CI commit" >&2
    exit 1
  fi
  echo "ok: main CI success for $sha (run $rid)"
fi

echo "ok: pre-tag checks passed for $TAG"
echo "next:"
echo "  git tag -a $TAG -m \"$TAG\""
echo "  git push origin $TAG"
echo "  # wait for goreleaser, then:"
echo "  ./scripts/ops-deploy-baili.sh $TAG"
echo "  ./scripts/ops-smoke-public.sh https://img.li"
