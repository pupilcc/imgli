# Cross-policy storage migration

Move object rows between storage policies **after** both policies work for
upload/serve. Prefer a dedicated test pass with `-dry-run` and a small `-limit`.

## Prerequisites

- Target policy **enabled** and credentials valid (`imgli doctor` / admin probe).
- New uploads should already target the destination (group `allowed_policy_ids` /
  user default) so the source is not still growing.
- Same object key (`files.path`) is reused; S3 path templates must remain
  compatible with existing keys.

## CLI

```bash
# Preview
imgli storage-migrate -config /path/to/imgli.yaml -from 1 -to 2 -dry-run

# Small batch
imgli storage-migrate -config /path/to/imgli.yaml -from 1 -to 2 -limit 50

# Full cutover (optional delete source objects after DB update)
imgli storage-migrate -config /path/to/imgli.yaml -from 1 -to 2 -delete-source
```

| Flag | Meaning |
|------|---------|
| `-from` / `-to` | `storage_policies.id` |
| `-dry-run` | Count only; no Put / no DB update |
| `-limit N` | Process at most N `files` rows |
| `-delete-source` | After successful retarget, delete object (+ thumbs) on source |

Thumbs under `.thumbs/…` are copied best-effort; missing thumbs do not fail the row.

## What this is not

- Continuous **dual-write** or replica sync (see design draft).
- Import from a foreign tree → use `imgli import-dir` instead.
- CDN cache purge (do that in your CDN console after public keys move).

## Design (multi-policy / sync roadmap)

See [design/storage-migrate-sync-draft.md](design/storage-migrate-sync-draft.md).
