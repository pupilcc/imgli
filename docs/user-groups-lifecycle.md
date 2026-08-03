# User groups: upload options & lifecycle

Operator guide for group-level **expiry / max-views** and **retention / force age**.

## Field semantics

| Field | Meaning | `0` means |
|-------|---------|-----------|
| `default_expires_in` | UI default expiry (seconds) | Default permanent (if max allows) |
| `max_expires_in` | Cap on `expires_in` | Permanent allowed (global 1y still applies when set) |
| `default_max_views` | UI default view cap | Default unlimited (if max allows) |
| `max_max_views` | Cap on `max_views` | Unlimited allowed |
| `retention_days` | Soft-delete live images older than N days (`created_at`) | Off |
| `force_max_age_days` | Hard-purge live images older than N days; also clamps upload expiry | Off |

**Enforcement**

- **Upload / PATCH image:** if `max_expires_in > 0` or `force_max_age_days > 0`, permanent is forbidden; over-cap rejected (`expires_over_group`). Same for max views (`max_views_over_group`).
- **Defaults only (no max):** UI pre-fills defaults; **API omitting the field does not force default** (clients may still choose permanent).
- **New uploads only:** changing group fields does not rewrite existing rows until you **Apply stock lifecycle** or hourly jobs run.

## Recommended recipes

### Guest / public-good free tier

| Field | Example |
|-------|---------|
| `default_expires_in` | `86400` (1 day) |
| `max_expires_in` | `604800` (7 days) |
| `force_max_age_days` | `7` |
| `retention_days` | `0` (optional; force already hard-purges) |

Seeded guest group uses this pattern when lifecycle fields are still all zero.

### Paid / VIP

All lifecycle fields `0` → permanent allowed; no auto soft/hard age cleanup.

### Mid-tier free account

- `max_expires_in` or `force_max_age_days` = 90  
- optional `retention_days` = 60 (soft trash before hard force)

## Stock images after policy change

1. **Save group** → only new uploads constrained.  
2. **Preview stock** (`POST /admin/groups/{id}/lifecycle/preview`) → permanent + over-cap counts.  
3. **Clamp stock** (`POST /admin/groups/{id}/lifecycle/apply` `{confirm:true}`) → set `expires_at = now+cap` (max 500/run).  
4. Over-age cleanup: hourly jobs + System cleanup kinds `group_retention` / `group_force_age`.

## max_views vs storage

Hitting `max_views` **only blocks access**. Objects still count toward storage until expiry, trash purge, force age, or admin permanent delete.

## CDN

Origin cleanup (expired, force age, admin purge) does **not** purge CDN. See [ops-cleanup-cdn-boundary.md](ops-cleanup-cdn-boundary.md).

## Related APIs

- `GET /api/v1/user/quota` — logged-in group limits for upload/detail UI  
- `GET /api/v1/config` → `guest` — guest limits  
- `POST /api/v1/admin/cleanup/preview|run` kinds: `expired`, `trash`, `group_retention`, `group_force_age`
