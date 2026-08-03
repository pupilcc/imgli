# User groups: upload options & lifecycle

**Since v0.9.0.** Operator guide for group-level **expiry / max-views** and **retention / force age**.

中文摘要见文末 [简体中文](#简体中文)。

## Field semantics

| Field | Meaning | `0` means |
|-------|---------|-----------|
| `default_expires_in` | UI default expiry (seconds) | Default permanent (if max allows) |
| `max_expires_in` | Cap on `expires_in` | Permanent allowed (global **1 year** still applies when a value is set) |
| `default_max_views` | UI default view cap | Default unlimited (if max allows) |
| `max_max_views` | Cap on `max_views` | Unlimited allowed |
| `retention_days` | Soft-delete live images older than N days (`created_at`) | Off |
| `force_max_age_days` | Hard-purge live images older than N days; also clamps upload expiry | Off |

**Enforcement**

| Surface | Behavior |
|---------|----------|
| Upload / `PATCH /images/{key}` | Cap > 0 forbids permanent / unlimited; over-cap → `expires_over_group` / `max_views_over_group` |
| Defaults only (no max) | UI pre-fills defaults; **API that omits the field does not force default** |
| New vs stock | Saving group fields affects **new uploads only** until stock clamp or hourly jobs |

Global ceilings still apply: `expires_in` ≤ 366 days, `max_views` ≤ 10000.

## Recommended recipes

### Guest / public-good free tier

| Field | Example |
|-------|---------|
| `default_expires_in` | `86400` (1 day) |
| `max_expires_in` | `604800` (7 days) |
| `force_max_age_days` | `7` |
| `retention_days` | `0` (optional; force already hard-purges) |

On upgrade, if the guest group still has all lifecycle fields at `0`, seed migration writes the 1d / 7d / force-7d defaults (same as a fresh install).

### Paid / VIP

All lifecycle fields `0` → permanent allowed; no auto soft/hard age cleanup.

### Mid-tier free account

- `max_expires_in` or `force_max_age_days` = 90  
- optional `retention_days` = 60 (soft trash before hard force)

## Stock images after policy change

1. **Save group** in Admin → **User groups** (only new uploads constrained).  
2. **Preview stock** — UI button or `POST /api/v1/admin/groups/{id}/lifecycle/preview`  
   → `permanent_count`, `over_cap_count`, `total`, sample keys, `cap_sec`.  
3. **Clamp stock** — UI button or `POST /api/v1/admin/groups/{id}/lifecycle/apply`  
   body `{ "confirm": true, "limit": 500 }`  
   → sets `expires_at = now+cap` on permanent / over-cap **live** images (max 500 per run; re-run if needed).  
4. Over-age cleanup: hourly jobs **and** System cleanup kinds `group_retention` / `group_force_age`.

Audit action: `group_lifecycle_apply`.

## Admin image delete (v0.8+)

| Action | API | Notes |
|--------|-----|--------|
| Soft delete (owner trash) | `DELETE /api/v1/admin/images/{key}` | Guest uploads have no owner → auto permanent |
| Permanent purge | `DELETE /api/v1/admin/images/{key}?permanent=1` | DB hard delete + `delete_file` task |
| Batch | `POST /api/v1/admin/images/batch` `{ "keys": [...], "action": "trash"\|"purge" }` | Max 100 keys |

Review queue can purge the current page of pending images (same permanent batch path).

## Cleanup kinds (System / Ops)

| Kind | Effect |
|------|--------|
| `expired` | `expires_at` past → hard purge (not via trash) |
| `trash` | Soft-deleted longer than 30 days → hard purge |
| `group_retention` | Group `retention_days` → soft-delete live images by `created_at` |
| `group_force_age` | Group `force_max_age_days` → hard purge live images by `created_at` |

```http
POST /api/v1/admin/cleanup/preview
{ "kinds": ["expired", "trash", "group_retention", "group_force_age"] }

POST /api/v1/admin/cleanup/run
{ "kinds": [...], "confirm": true, "limit": 200 }
```

Hourly server jobs also run expired trash, expired images, group retention, and group force age.

## max_views vs storage

Hitting `max_views` **only blocks access**. Objects still count toward storage until expiry purge, trash age, force age, or admin permanent delete.

## Client / CLI

| Client | How to learn limits |
|--------|---------------------|
| Web upload / image detail | `GET /api/v1/user/quota` (logged-in) or `GET /api/v1/config` → `guest` |
| PicGo / ShareX / custom | Same APIs; over-cap upload returns 400 with code above — [picgo.md](picgo.md), [integrations/sharex.md](integrations/sharex.md) |
| CLI | `imgli upload -verbose …` prints group limits to stderr |

## CDN

Origin cleanup (expired, force age, admin purge) does **not** purge CDN. See [ops-cleanup-cdn-boundary.md](ops-cleanup-cdn-boundary.md).

## Related APIs (quick index)

| Method | Path | Role |
|--------|------|------|
| GET | `/api/v1/user/quota` | Logged-in group limits for UI |
| GET | `/api/v1/config` | Public config including `guest` limits |
| PATCH | `/api/v1/admin/groups/{id}` | Update group including lifecycle fields |
| POST | `/api/v1/admin/groups/{id}/lifecycle/preview` | Stock clamp dry-run |
| POST | `/api/v1/admin/groups/{id}/lifecycle/apply` | Stock clamp execute |
| POST | `/api/v1/admin/cleanup/preview` · `/run` | Lifecycle cleanup |
| DELETE | `/api/v1/admin/images/{key}` | Soft or permanent delete |
| POST | `/api/v1/admin/images/batch` | Batch trash / purge |

---

## 简体中文

**v0.9.0+** 用户组可配置上传选项与生命周期，用于减少管理员清图负担、避免游客/公益长期占盘。

| 字段 | 含义 | `0` |
|------|------|-----|
| `default_expires_in` | 默认有效期（秒） | 默认永久（若上限允许） |
| `max_expires_in` | 有效期上限 | 允许永久（仍受全局约 1 年） |
| `default_max_views` / `max_max_views` | 访问次数默认 / 上限 | 0=默认不限 / 允许不限 |
| `retention_days` | 按上传时间 N 天后软删进回收站 | 关 |
| `force_max_age_days` | 按上传时间 N 天后硬清；上传也会钳制有效期 | 关 |

**要点**

1. **改组配置只约束新上传**；存量请用后台「存量预览 / 钳制」或等小时任务 / 系统清理。  
2. **游客组**升级时若生命周期字段全为 0，会补 1 天默认 / 最长 7 天 / 强制 7 天。  
3. **访问次数用尽不释放存储**；腾空间靠过期清理、回收站超龄、强制存活或管理员彻底删除。  
4. **清理源站 ≠ 清 CDN**，见 [ops-cleanup-cdn-boundary.md](ops-cleanup-cdn-boundary.md)。  
5. CLI：`imgli upload -verbose` 可打印当前账号/游客组限制。

管理路径：后台 **用户组**、**系统 / 运维 → 生命周期清理**、**图片管理**（软删 / 彻底删除 / 批量）。
