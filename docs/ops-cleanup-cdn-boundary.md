# Cleanup vs CDN and quota boundaries

Operator cleanup (Admin `POST /admin/cleanup/*` or hourly server jobs) acts on
the **origin** database and storage drivers.

**Related:** group policy and stock clamp — [user-groups-lifecycle.md](user-groups-lifecycle.md) (v0.9.0+).

## Cleanup kinds

| Kind | What it selects | Result |
|------|-----------------|--------|
| `expired` | Live images with `expires_at` in the past | Hard purge (not via trash) |
| `trash` | Soft-deleted longer than **30 days** | Hard purge + free origin quota |
| `group_retention` | Live images older than group `retention_days` (`created_at`) | Soft-delete into owner trash |
| `group_force_age` | Live images older than group `force_max_age_days` | Hard purge |

Default Admin System/Ops UI and empty `kinds` arrays use all four kinds.
Group jobs process up to **500 images per group per pass**; cleanup `limit`
applies to expired/trash batch size (UI default 200).

```http
POST /api/v1/admin/cleanup/preview
{ "kinds": ["expired", "trash", "group_retention", "group_force_age"] }

POST /api/v1/admin/cleanup/run
{ "kinds": [...], "confirm": true, "limit": 200 }
```

Hourly server loop also runs: purge expired trash, purge expired images,
soft-delete by group retention, hard purge by group force age.

## What cleanup does

- Deletes or soft-deletes `images` rows as above.
- Hard purge enqueues physical object deletes on the configured storage policy.
- Frees **origin** storage quota accounting for affected users after hard purge.

## What cleanup does not do

| System | Effect of cleanup |
|--------|-------------------|
| **CDN** (Cloudflare, vendor CDN, `cdn_domain`) | Cached public objects may still be served until TTL/purge. **imgli does not auto-purge CDN.** Use your CDN console or API after bulk cleanup of public keys. |
| **CDN billing** | Unrelated to origin delete counts; dashboard traffic may under-count CDN hits (see `deploy/ops/admin-stats-metering.md`). |
| **Presigned private URLs** | Already short-lived; no CDN offload for private by design. |

## Manual admin deletes (not cleanup kinds)

| Action | API |
|--------|-----|
| Soft delete → owner trash | `DELETE /api/v1/admin/images/{key}` |
| Permanent purge | `DELETE /api/v1/admin/images/{key}?permanent=1` |
| Batch trash / purge | `POST /api/v1/admin/images/batch` `{keys, action: "trash"|"purge"}` (max 100) |

Guest images have no owner trash — soft delete is upgraded to permanent.
Same CDN caveat: mass permanent delete of **public** keys may need a CDN purge.

## Doctor / ops tips

- Prefer dry-run (`/admin/cleanup/preview`) before `/admin/cleanup/run`.
- After large public cutovers or mass deletes, plan a CDN purge for high-traffic
  prefixes if visitors still see old content.
- Turning on group `force_max_age_days` does not rewrite history until the hourly
  job or cleanup run; use group **stock lifecycle clamp** for permanent/over-cap
  live images that should get a short `expires_at` first.
