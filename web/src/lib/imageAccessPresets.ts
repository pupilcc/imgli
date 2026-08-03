/** Shared expiry / max-views presets for upload options and image detail edit. */

/** value = expires_in seconds; 0 = never */
export const EXPIRY_PRESETS = [
  { key: 'never', sec: 0 },
  { key: '1h', sec: 3600 },
  { key: '1d', sec: 86400 },
  { key: '7d', sec: 604800 },
  { key: '30d', sec: 2592000 },
] as const

export type ExpiryKey = (typeof EXPIRY_PRESETS)[number]['key']

export const EXPIRY_LABEL_KEY: Record<ExpiryKey, string> = {
  never: 'upload.expiryNever',
  '1h': 'upload.expiry1h',
  '1d': 'upload.expiry1d',
  '7d': 'upload.expiry7d',
  '30d': 'upload.expiry30d',
}

/** max_views presets; 0 = unlimited */
export const MAX_VIEWS_PRESETS = [
  { key: 'unlimited', n: 0 },
  { key: '1', n: 1 },
  { key: '3', n: 3 },
  { key: '10', n: 10 },
] as const

export type MaxViewsKey = (typeof MAX_VIEWS_PRESETS)[number]['key']

export const MAX_VIEWS_LABEL_KEY: Record<MaxViewsKey, string> = {
  unlimited: 'upload.maxViewsUnlimited',
  '1': 'upload.maxViews1',
  '3': 'upload.maxViews3',
  '10': 'upload.maxViews10',
}

/** 组策略：有效期 cap 秒；0=允许永久。force_max_age_days 与 max_expires_in 取更严。 */
export function groupExpiresCapSec(opts: {
  max_expires_in?: number
  force_max_age_days?: number
}): number {
  let cap = opts.max_expires_in ?? 0
  const forceDays = opts.force_max_age_days ?? 0
  if (forceDays > 0) {
    const force = forceDays * 86400
    if (cap <= 0 || force < cap) cap = force
  }
  return cap > 0 ? cap : 0
}

export type ExpiryPreset = { key: string; sec: number }
export type MaxViewsPreset = { key: string; n: number }

/** 过滤有效期预设；cap 不在标准档时追加「最长」动态档。 */
export function filterExpiryPresets(capSec: number): ExpiryPreset[] {
  if (capSec <= 0) {
    return EXPIRY_PRESETS.map((p) => ({ key: p.key as string, sec: p.sec as number }))
  }
  const base: ExpiryPreset[] = EXPIRY_PRESETS
    .filter((p) => p.sec > 0 && p.sec <= capSec)
    .map((p) => ({ key: p.key as string, sec: p.sec as number }))
  if (!base.some((p) => p.sec === capSec)) {
    base.push({ key: `cap:${capSec}`, sec: capSec })
  }
  return base
}

/** 过滤访问次数预设；cap 不在标准档时追加动态档。 */
export function filterMaxViewsPresets(maxMaxViews: number): MaxViewsPreset[] {
  if (maxMaxViews <= 0) {
    return MAX_VIEWS_PRESETS.map((p) => ({ key: p.key as string, n: p.n as number }))
  }
  const base: MaxViewsPreset[] = MAX_VIEWS_PRESETS
    .filter((p) => p.n > 0 && p.n <= maxMaxViews)
    .map((p) => ({ key: p.key as string, n: p.n as number }))
  if (!base.some((p) => p.n === maxMaxViews)) {
    base.push({ key: `cap:${maxMaxViews}`, n: maxMaxViews })
  }
  return base
}

/** 动态档 i18n 标签：标准 key 走 EXPIRY_LABEL_KEY / MAX_VIEWS；cap:N 用 formatter。 */
export function expiryPresetLabel(
  p: ExpiryPreset,
  t: (key: string, vars?: Record<string, string | number>) => string,
): string {
  if (p.key in EXPIRY_LABEL_KEY) return t(EXPIRY_LABEL_KEY[p.key as ExpiryKey])
  if (p.sec % 86400 === 0) return t('upload.expiryCapDays', { days: p.sec / 86400 })
  if (p.sec % 3600 === 0) return t('upload.expiryCapHours', { hours: p.sec / 3600 })
  return t('upload.expiryCapSec', { sec: p.sec })
}

export function maxViewsPresetLabel(
  p: MaxViewsPreset,
  t: (key: string, vars?: Record<string, string | number>) => string,
): string {
  if (p.key in MAX_VIEWS_LABEL_KEY) return t(MAX_VIEWS_LABEL_KEY[p.key as MaxViewsKey])
  return t('upload.maxViewsCap', { n: p.n })
}

/** 将组默认值落到预设档位；无匹配时取 cap 内最近更短档，或 cap 本身。 */
export function resolveDefaultExpiresIn(
  defaultSec: number,
  capSec: number,
  presets: { sec: number }[],
): number {
  if (capSec > 0) {
    const d = defaultSec > 0 && defaultSec <= capSec ? defaultSec : capSec
    const hit = presets.find((p) => p.sec === d)
    if (hit) return hit.sec
    const le = [...presets].filter((p) => p.sec > 0 && p.sec <= d).sort((a, b) => b.sec - a.sec)[0]
    return le?.sec ?? d
  }
  if (defaultSec > 0) {
    const hit = presets.find((p) => p.sec === defaultSec)
    return hit?.sec ?? defaultSec
  }
  return 0
}

export function resolveDefaultMaxViews(
  defaultN: number,
  maxN: number,
  presets: { n: number }[],
): number {
  if (maxN > 0) {
    const d = defaultN > 0 && defaultN <= maxN ? defaultN : maxN
    const hit = presets.find((p) => p.n === d)
    if (hit) return hit.n
    const le = [...presets].filter((p) => p.n > 0 && p.n <= d).sort((a, b) => b.n - a.n)[0]
    return le?.n ?? d
  }
  if (defaultN > 0) {
    const hit = presets.find((p) => p.n === defaultN)
    return hit?.n ?? defaultN
  }
  return 0
}
