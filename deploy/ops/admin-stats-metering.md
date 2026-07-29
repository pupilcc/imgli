# Admin stats metering boundary (origin-only)

imgli’s admin dashboard **traffic** and **referer** numbers come from the
application origin when a request hits the public gate routes (primarily
`GET /i/{key}` and related serve paths that call `stats.Record`).

## What is counted

| Signal | Source | Grain |
|--------|--------|--------|
| Views / day (`access_stats`) | Successful public serve on origin | per image, per day |
| Referer hosts (`referer_stats`) | `Referer` header host (empty → `(direct)`) | per host, per day |
| Per-image referers (`referer_image_stats`) | same | per image × host × day |

Counts are **first-party**, buffered in memory and flushed periodically.
Full referer URLs are **never** stored—only the host.

## What is **not** counted (CDN caveat)

When a storage policy sets **`cdn_domain`** (object CDN prefix), public images
often:

1. Hit origin `BaseURL` + `/i/{key}` (gate: hotlink, max_views, bandwidth, …)
2. Respond with **302** to `{cdn_domain}/…`
3. Subsequent browsers load bytes from the **edge / object CDN**

**Edge cache hits never re-enter origin**, so they do **not** increment
dashboard views or referer totals. Therefore:

- Admin “traffic” ≠ full CDN billable requests or egress
- Top referers are a **lower bound** of real external embedding
- Use **Cloudflare / bucket analytics / billing** for cost truth

This is intentional: origin remains the **control plane** (authz, hotlink,
quotas); CDN is the **data plane**.

## Operator checklist

1. Treat dashboard metrics as **abuse + trend** signals, not media-grade analytics.
2. For cost spikes, check CDN/provider dashboards first, then origin Top referers
   and bandwidth period counters on users.
3. Enable **hotlink** allowlists when a non-owned host dominates origin hits.
4. Run `imgli doctor` — if any enabled policy has `cdn_domain`, doctor emits a
   **WARN** reminding you of this boundary.

## Related

- [`cloudflare-i-img-li-cache-checklist.md`](cloudflare-i-img-li-cache-checklist.md) — production CF setup for `i.img.li`
- Product roadmap SSOT: omni KB `imgli/roadmap` (ops dashboard epic)
